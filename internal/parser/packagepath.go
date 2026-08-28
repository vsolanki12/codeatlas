package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// packageResolver gives entities a repository-unique Go package identity.
// The package clause alone is not unique in a large repository: many
// directories commonly use the same short package name.
type packageResolver struct {
	root    string
	modules []moduleRoot
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
		if rel == "." {
			return module.path
		}
		return strings.TrimSuffix(module.path, "/") + "/" + filepath.ToSlash(rel)
	}

	rel, err := filepath.Rel(r.root, dir)
	if err != nil || rel == "." {
		return packageName
	}
	return filepath.ToSlash(rel)
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
