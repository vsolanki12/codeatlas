package parser

import (
	"bufio"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// packageResolver gives entities a repository-unique Go package identity.
// The package clause alone is not unique in a large repository: many
// directories commonly use the same short package name.
type packageResolver struct {
	root            string
	modules         []moduleRoot
	productionNames map[string]map[string]bool
}

type moduleRoot struct {
	dir  string
	path string
}

func newPackageResolver(root string) *packageResolver {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	r := &packageResolver{root: filepath.Clean(absRoot)}
	_ = filepath.WalkDir(r.root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".worktrees", "vendor", "node_modules", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != "go.mod" {
			return nil
		}
		modulePath := readModulePath(path)
		if modulePath != "" {
			dir := filepath.Clean(filepath.Dir(path))
			r.modules = append(r.modules, moduleRoot{dir: dir, path: modulePath})
		}
		return nil
	})
	sort.Slice(r.modules, func(i, j int) bool {
		if len(r.modules[i].dir) != len(r.modules[j].dir) {
			return len(r.modules[i].dir) > len(r.modules[j].dir)
		}
		return r.modules[i].dir < r.modules[j].dir
	})
	return r
}

func (r *packageResolver) resolve(filePath, packageName string) string {
	if r == nil {
		return packageName
	}
	dir := filepath.Dir(filepath.Join(r.root, filepath.FromSlash(filePath)))
	for _, module := range r.modules {
		rel, err := filepath.Rel(module.dir, dir)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		packagePath := module.path
		if rel != "." {
			packagePath = strings.TrimSuffix(module.path, "/") + "/" + filepath.ToSlash(rel)
		}
		if r.isExternalTestPackage(filePath, packageName) {
			packagePath += "_test"
		}
		return packagePath
	}

	rel, err := filepath.Rel(r.root, dir)
	if err != nil || rel == "." {
		return packageName
	}
	packagePath := filepath.ToSlash(rel)
	if r.isExternalTestPackage(filePath, packageName) {
		packagePath += "_test"
	}
	return packagePath
}

// The suffix alone is insufficient: a production package may itself be named
// foo_test. Only a different package clause denotes the external test variant.
func (r *packageResolver) isExternalTestPackage(filePath, packageName string) bool {
	if !strings.HasSuffix(filePath, "_test.go") || !strings.HasSuffix(packageName, "_test") {
		return false
	}
	dir := filepath.Dir(filepath.Join(r.root, filepath.FromSlash(filePath)))
	if r.productionNames == nil {
		r.productionNames = make(map[string]map[string]bool)
	}
	names, exists := r.productionNames[dir]
	if !exists {
		names = make(map[string]bool)
		entries, err := os.ReadDir(dir)
		if err == nil {
			fset := token.NewFileSet()
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
					continue
				}
				file, err := goparser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, goparser.PackageClauseOnly)
				if err == nil && file.Name != nil {
					names[file.Name.Name] = true
				}
			}
		}
		r.productionNames[dir] = names
	}
	return !names[packageName]
}

func readModulePath(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}
