package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/storage"
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

	if g.Schema != "hypershift-atlas" {
		t.Errorf("schema = %q, want %q", g.Schema, "hypershift-atlas")
	}
	if g.SchemaVersion != "1.2.0" {
		t.Errorf("schemaVersion = %q, want %q", g.SchemaVersion, "1.2.0")
	}
	if len(g.Entities) == 0 {
		t.Error("graph has no entities")
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
