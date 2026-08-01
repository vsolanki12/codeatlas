package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

// setupTestRepo creates a minimal Go project with a controller file and
// a test file so the scanner has something to parse end-to-end.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// A controller file with Reconcile + SetupWithManager
	controllerSrc := `package mycontroller

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
)

type MyReconciler struct{}

func (r *MyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *MyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hyperv1.HostedCluster{}).
		Complete(r)
}
`
	ctrlDir := filepath.Join(dir, "control-plane", "hostedcluster")
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ctrlDir, "reconciler.go"), []byte(controllerSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// A test file in the same package
	testSrc := `package mycontroller

import "testing"

func TestReconcile(t *testing.T) {
	t.Log("placeholder test")
}
`
	if err := os.WriteFile(filepath.Join(ctrlDir, "reconciler_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestScan_EndToEnd(t *testing.T) {
	repoDir := setupTestRepo(t)
	outPath := filepath.Join(t.TempDir(), "atlas.json")

	result, err := Scan(repoDir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Should find entities: at minimum a package, functions, a controller, and a test
	if result.EntityCount == 0 {
		t.Error("expected entities, got 0")
	}

	// Duration should be non-zero
	if result.Duration == 0 {
		t.Error("expected non-zero duration")
	}

	// Output file should exist and be valid JSON
	g, err := storage.ReadGraph(outPath)
	if err != nil {
		t.Fatalf("failed to read output graph: %v", err)
	}

	if g.Schema != "codeatlas" {
		t.Errorf("schema = %q, want %q", g.Schema, "codeatlas")
	}
	if g.SchemaVersion != "1.3.0" {
		t.Errorf("schemaVersion = %q, want %q", g.SchemaVersion, "1.3.0")
	}
	if len(g.Entities) == 0 {
		t.Error("graph has no entities")
	}
	if len(g.FileTimestamps) == 0 {
		t.Error("expected FileTimestamps to be populated")
	}
}

func TestScan_Incremental(t *testing.T) {
	repoDir := setupTestRepo(t)
	outPath := filepath.Join(t.TempDir(), "atlas.json")

	// Full scan first
	r1, err := Scan(repoDir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if r1.Incremental {
		t.Error("first scan should not be incremental")
	}

	// Second scan with same output — should be incremental with 0 changes
	r2, err := Scan(repoDir, outPath, ScanOptions{PreviousGraph: outPath})
	if err != nil {
		t.Fatalf("incremental scan: %v", err)
	}
	if !r2.Incremental {
		t.Error("second scan should be incremental")
	}
	if r2.ChangedFiles != 0 {
		t.Errorf("expected 0 changed files, got %d", r2.ChangedFiles)
	}
	if r2.EntityCount != r1.EntityCount {
		t.Errorf("entity count mismatch: full=%d incremental=%d", r1.EntityCount, r2.EntityCount)
	}

	// Modify a file and re-scan
	newContent := []byte(`package mycontroller
func ExtraFunc() {}
`)
	ctrlFile := filepath.Join(repoDir, "control-plane", "hostedcluster", "extra.go")
	if err := os.WriteFile(ctrlFile, newContent, 0644); err != nil {
		t.Fatal(err)
	}

	r3, err := Scan(repoDir, outPath, ScanOptions{PreviousGraph: outPath})
	if err != nil {
		t.Fatalf("incremental scan after change: %v", err)
	}
	if !r3.Incremental {
		t.Error("third scan should be incremental")
	}
	if r3.ChangedFiles != 1 {
		t.Errorf("expected 1 changed file, got %d", r3.ChangedFiles)
	}
	if r3.EntityCount < r2.EntityCount {
		t.Errorf("entity count should not decrease after adding a file: before=%d after=%d", r2.EntityCount, r3.EntityCount)
	}
}

func TestChangedFiles(t *testing.T) {
	now := time.Now().UTC()
	files := []domain.File{
		{RelativePath: "a.go", ModifiedTime: now},
		{RelativePath: "b.go", ModifiedTime: now},
		{RelativePath: "c.go", ModifiedTime: now.Add(time.Hour)},
	}
	oldTS := map[string]string{
		"a.go": now.Format(time.RFC3339),
		"b.go": now.Format(time.RFC3339),
		"c.go": now.Format(time.RFC3339),
		"d.go": now.Format(time.RFC3339),
	}

	changed, unchanged, deleted := changedFiles(files, oldTS)
	if len(changed) != 1 || changed[0].RelativePath != "c.go" {
		t.Errorf("expected 1 changed (c.go), got %v", changed)
	}
	if len(unchanged) != 2 {
		t.Errorf("expected 2 unchanged, got %d", len(unchanged))
	}
	if len(deleted) != 1 || deleted[0] != "d.go" {
		t.Errorf("expected 1 deleted (d.go), got %v", deleted)
	}
}

func TestScan_InvalidRepo(t *testing.T) {
	_, err := Scan("/nonexistent/path", "/tmp/out.json", ScanOptions{})
	if err == nil {
		t.Error("expected error for invalid repo path")
	}
}

func TestScan_EmptyRepo(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.json")

	result, err := Scan(dir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("empty repo should not error: %v", err)
	}

	if result.EntityCount != 0 {
		t.Errorf("expected 0 entities in empty repo, got %d", result.EntityCount)
	}
}

func TestScan_WarningsOnBadFile(t *testing.T) {
	dir := t.TempDir()

	// Write an invalid Go file
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte("not valid go"), 0644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "out.json")
	result, err := Scan(dir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("scan should succeed with warnings, got error: %v", err)
	}

	if len(result.Warnings) == 0 {
		t.Error("expected warnings for invalid Go file, got none")
	}
}
