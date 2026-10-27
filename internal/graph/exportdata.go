package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// exportListTimeout bounds the toolchain invocation. `go list -export` compiles
// every dependency on a cold build cache, so the budget is generous; it exists
// only so a wedged toolchain cannot hang a scan indefinitely.
const exportListTimeout = 15 * time.Minute

// exportIndex maps a dependency import path to the export data file produced by
// the Go toolchain.
//
// It exists because the default gc importer resolves packages through GOROOT
// and GOPATH only. A module-mode repository populates neither, so dependency
// types outside the scanned repository are unresolvable without asking the
// toolchain where its compiled export data lives.
type exportIndex map[string]string

// newExportIndex asks the Go toolchain for the export data of every dependency
// reachable from the modules in the repository. A toolchain failure is reported
// and leaves the index empty rather than failing the scan: type enrichment is
// an enrichment pass, and an unavailable toolchain must degrade to AST facts
// instead of discarding them.
func newExportIndex(root string, files []domain.File, buildContext domain.BuildContext) (exportIndex, []string) {
	index := make(exportIndex)
	var diagnostics []string
	for _, module := range moduleDirs(files) {
		paths, err := listExportData(filepath.Join(root, filepath.FromSlash(module)), buildContext)
		if err != nil {
			diagnostics = append(diagnostics, fmt.Sprintf("go list in %s: %v", module, err))
			continue
		}
		for importPath, exportFile := range paths {
			// Modules are visited in sorted order and export data for one import
			// path is identical across them, so keeping the first keeps the index
			// independent of module iteration order.
			if _, exists := index[importPath]; !exists {
				index[importPath] = exportFile
			}
		}
	}
	return index, diagnostics
}

// moduleDirs returns the repository-relative directory of every discovered
// go.mod, sorted. A repository with no go.mod still reports its root so that a
// GOPATH-style checkout is listed once.
func moduleDirs(files []domain.File) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, file := range files {
		if filepath.Base(file.RelativePath) != "go.mod" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(file.RelativePath))
		if !seen[dir] {
			seen[dir] = true
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) == 0 {
		dirs = append(dirs, ".")
	}
	sort.Strings(dirs)
	return dirs
}

type listedPackage struct {
	ImportPath string
	Export     string
}

func listExportData(moduleDir string, buildContext domain.BuildContext) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), exportListTimeout)
	defer cancel()

	// -test includes test packages and their dependencies. Without it the
	// closure omits every import that only a _test.go file makes, and the
	// internal-test variant of a package then fails to type-check.
	args := []string{"list", "-deps", "-test", "-export", "-json=ImportPath,Export"}
	if len(buildContext.BuildTags) > 0 {
		args = append(args, "-tags", strings.Join(buildContext.BuildTags, ","))
	}
	args = append(args, "./...")

	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = moduleDir
	command.Env = append(os.Environ(),
		"GOOS="+buildContext.GOOS,
		"GOARCH="+buildContext.GOARCH,
		"CGO_ENABLED="+boolEnv(buildContext.CgoEnabled),
	)
	// `go list` reports per-package problems on stderr and still emits usable
	// JSON for the packages that did resolve. Discard stderr and judge the run
	// by whether any export data came back.
	command.Stderr = io.Discard
	output, err := command.Output()
	if len(output) == 0 {
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("no package list produced")
	}

	paths := make(map[string]string)
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var listed listedPackage
		if decodeErr := decoder.Decode(&listed); decodeErr == io.EOF {
			break
		} else if decodeErr != nil {
			return paths, decodeErr
		}
		if listed.ImportPath != "" && listed.Export != "" {
			paths[listed.ImportPath] = listed.Export
		}
	}
	return paths, nil
}

func boolEnv(value bool) string {
	if value {
		return "1"
	}
	return "0"
}

// importerFor returns the dependency importer for this index. An empty index
// falls back to the compiler default so behaviour is unchanged when the
// toolchain could not be consulted.
func (index exportIndex) importerFor(fset *token.FileSet) types.Importer {
	if len(index) == 0 {
		return importer.Default()
	}
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		exportFile, ok := index[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(exportFile)
	})
}
