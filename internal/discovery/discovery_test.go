package discovery

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestScan(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "atlas_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	os.WriteFile(filepath.Join(tempDir, "file1.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tempDir, "file2_test.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tempDir, "README.md"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte(""), 0644)
	defer os.RemoveAll(tempDir)

	repo := domain.Repository{RootPath: tempDir}
	discovery, err := New(repo)
	if err != nil {
		t.Fatalf("Failed to create discovery instance: %v", err)
	}
	expected := []struct {
		path string
	}{
		{"file1.go"},
		{"file2_test.go"},
		{"config.yaml"},
		{"README.md"},
		{"test.txt"},
	}
	files, err := discovery.Scan()
	if err != nil {
		t.Fatalf("Failed to scan repository: %v", err)
	}
	for _, tc := range expected { // loop test cases
		t.Run(tc.path, func(t *testing.T) { // tc.path works - it's from expected
			found := false
			for _, f := range files { // search for scan results
				if f.RelativePath == tc.path { // compare Relative.Path to expected path
					found = true
				}
			}
			if !found {
				t.Errorf("File %s not found in scan results", tc.path)
			}
		})
	}
}

func TestNewEmptyPath(t *testing.T) {
	_, err := New(domain.Repository{RootPath: ""})
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

func TestNewNonExistentPath(t *testing.T) {
	_, err := New(domain.Repository{RootPath: "/non/existent/path"})
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

func TestNewFilePath(t *testing.T) {
	f, err := os.CreateTemp("", "atlas-test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	f.Close()

	_, err = New(domain.Repository{RootPath: f.Name()})
	if err == nil {
		t.Fatal("expected error for file path, got nil")
	}
}
