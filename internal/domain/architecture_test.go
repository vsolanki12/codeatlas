package domain_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The shared evidence planner must remain reusable without importing scanners,
// type loaders, or transport adapters into its graph consumer boundary.
func TestPackageDependencyBoundaries(t *testing.T) {
	for _, pkg := range []string{"domain", "query"} {
		path := filepath.Join("..", pkg)
		entries, err := os.ReadDir(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(path, entry.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imported := range file.Imports {
				name, _ := strconv.Unquote(imported.Path.Value)
				const prefix = "github.com/vsolanki12/codeatlas/internal/"
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				allowed := pkg == "query" && (name == prefix+"domain" || name == prefix+"storage")
				if !allowed {
					t.Errorf("%s/%s imports %s across its architecture boundary", pkg, entry.Name(), name)
				}
			}
		}
	}
}
