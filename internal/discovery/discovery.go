// Package discovery walks a repository's filesystem and collects file metadata.
package discovery

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// Discovery discovers files in a repository and returns their filesystem metadata.
type Discovery struct {
	repo domain.Repository
}

// Scan walks the repository tree and returns metadata for every discovered file.
func (d *Discovery) Scan() ([]domain.File, error) {
	root := d.repo.RootPath
	var files []domain.File
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		// WalkDir invokes this callback for every file and directory.
		// path = the file path (like{} in the find -exec)
		// entry = info about the file (is it directory? what's its name?)
		// err = did something go wrong while accessing the file?
		if err != nil {
			return err // stop walking and return the error
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".worktrees" || name == "vendor" || name == "node_modules" || name == "testdata" {
				return filepath.SkipDir // skip this directory and its contents
			}
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relPath = filepath.ToSlash(relPath)
			if isExcluded(relPath, d.repo.ExcludePaths) {
				return filepath.SkipDir
			}
			return nil // keep going, it's a directory we want to scan
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err // stop walking and return the error
		}
		relPath = strings.ReplaceAll(relPath, string(filepath.Separator), "/")
		if isExcluded(relPath, d.repo.ExcludePaths) {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err // stop walking and return the error
		}
		contentHash, err := contentHash(path, filepath.Ext(relPath))
		if err != nil {
			return err
		}
		files = append(files, domain.File{
			RelativePath: relPath,
			Size:         info.Size(),
			ModifiedTime: info.ModTime(),
			ContentHash:  contentHash,
		})
		return nil // keep going
	})
	if walkErr != nil {
		return nil, walkErr // return the error from WalkDir
	}
	return files, nil
}

func isExcluded(relativePath string, excluded []string) bool {
	for _, value := range excluded {
		if relativePath == value || strings.HasPrefix(relativePath, value+"/") {
			return true
		}
	}
	return false
}

func contentHash(path, extension string) (string, error) {
	switch extension {
	case ".go", ".yaml", ".yml", ".md":
	default:
		if !sourceExtensions[strings.ToLower(extension)] {
			return "", nil
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil)), nil
}

var sourceExtensions = map[string]bool{
	".go": true, ".py": true, ".pyi": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".ts": true, ".tsx": true, ".java": true, ".kt": true, ".kts": true, ".rs": true,
	".rb": true, ".php": true, ".c": true, ".h": true, ".cc": true, ".cpp": true,
	".hpp": true, ".cs": true, ".swift": true, ".scala": true, ".sc": true, ".sh": true,
	".bash": true, ".zsh": true, ".sql": true, ".proto": true, ".tf": true, ".vue": true,
	".svelte": true, ".ex": true, ".exs": true, ".clj": true, ".cljs": true, ".dart": true,
	".lua": true, ".pl": true, ".pm": true, ".r": true, ".m": true, ".mm": true,
	".groovy": true, ".gradle": true, ".ipynb": true,
	".html": true, ".htm": true, ".css": true, ".scss": true, ".sass": true, ".less": true,
	".tmpl": true, ".gotmpl": true, ".tpl": true, ".ejs": true, ".hbs": true,
	".handlebars": true, ".mustache": true, ".twig": true, ".jinja": true,
	".jinja2": true, ".j2": true, ".jsp": true, ".aspx": true, ".cshtml": true,
}

// IsSourceFile reports whether the extension contains source code that needs
// an explicit parser or an explicit repository-profile exclusion.
func IsSourceFile(relativePath string) bool {
	return sourceExtensions[strings.ToLower(filepath.Ext(relativePath))]
}

// Fingerprint returns the stable file identity used by incremental scans and
// freshness checks. Supported source files use their content hash; other
// discovered files fall back to metadata because they are not parsed.
func Fingerprint(file domain.File) string {
	if file.ContentHash != "" {
		return file.ContentHash
	}
	return fmt.Sprintf("%d:%d", file.ModifiedTime.UnixNano(), file.Size)
}

// FingerprintTimestamp is kept small and explicit for callers that need to
// compare a discovery result with a legacy timestamp-only graph.
func FingerprintTimestamp(file domain.File) string {
	return file.ModifiedTime.UTC().Format(time.RFC3339)
}

// New validates the repository path and returns a ready-to-use Discovery.
func New(repo domain.Repository) (*Discovery, error) {
	if repo.RootPath == "" {
		return nil, fmt.Errorf("repository root path is empty %q: %w", repo.RootPath, os.ErrInvalid)
	}
	info, err := os.Stat(repo.RootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid repository path %q: %w", repo.RootPath, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repository path %q is not a directory", repo.RootPath)
	}
	return &Discovery{repo: repo}, nil
}
