package mcpserver

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/freshness"
)

func TestCompactFreshnessBoundsFileLists(t *testing.T) {
	files := make([]string, maxMCPFreshnessFiles+1)
	for i := range files {
		files[i] = "file.go"
	}
	compact := compactFreshness(freshness.Result{
		Available:    true,
		ChangedFiles: files,
		NewFiles:     files,
		DeletedFiles: files,
	})
	if len(compact.ChangedFiles) != maxMCPFreshnessFiles || len(compact.NewFiles) != maxMCPFreshnessFiles || len(compact.DeletedFiles) != maxMCPFreshnessFiles {
		t.Fatalf("file list lengths = %d/%d/%d, want %d", len(compact.ChangedFiles), len(compact.NewFiles), len(compact.DeletedFiles), maxMCPFreshnessFiles)
	}
	if !compact.Truncated {
		t.Fatal("expected freshness truncation")
	}
	if !strings.Contains(formatFreshness(compact), "file lists capped") {
		t.Fatal("freshness text omitted truncation marker")
	}
}
