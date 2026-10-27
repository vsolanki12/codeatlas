package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestModuleDirsReturnsEveryModuleSorted(t *testing.T) {
	files := []domain.File{
		{RelativePath: "tests/go.mod"},
		{RelativePath: "main.go"},
		{RelativePath: "go.mod"},
		{RelativePath: "api/go.mod"},
		{RelativePath: "api/types.go"},
	}
	got := moduleDirs(files)
	want := []string{".", "api", "tests"}
	if len(got) != len(want) {
		t.Fatalf("module dirs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("module dirs = %v, want %v", got, want)
		}
	}
}

func TestModuleDirsFallsBackToRepositoryRoot(t *testing.T) {
	got := moduleDirs([]domain.File{{RelativePath: "main.go"}})
	if len(got) != 1 || got[0] != "." {
		t.Fatalf("module dirs = %v, want [.]", got)
	}
}

func TestExportIndexResolvesStandardLibrary(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "go.mod"), "module example.com/repo\n\ngo 1.21\n")
	write(t, filepath.Join(repo, "p", "p.go"), "package p\n\nimport \"strings\"\n\nfunc Upper(s string) string { return strings.ToUpper(s) }\n")

	index, diagnostics := newExportIndex(repo, []domain.File{{RelativePath: "go.mod"}}, domain.BuildContext{})
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diagnostics)
	}
	if index["strings"] == "" {
		t.Fatalf("no export data for strings; index has %d entries", len(index))
	}
}

// A repository the toolchain cannot list must leave type enrichment on the
// previous GOROOT/GOPATH importer rather than failing the scan.
func TestExportIndexDegradesWhenToolchainCannotList(t *testing.T) {
	repo := t.TempDir()
	write(t, filepath.Join(repo, "go.mod"), "this is not a go.mod\n")

	index, diagnostics := newExportIndex(repo, []domain.File{{RelativePath: "go.mod"}}, domain.BuildContext{})
	if len(index) != 0 {
		t.Fatalf("expected empty index, got %d entries", len(index))
	}
	if len(diagnostics) == 0 {
		t.Fatal("expected a diagnostic describing the go list failure")
	}
	if index.importerFor(nil) == nil {
		t.Fatal("empty index must still supply a fallback importer")
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
