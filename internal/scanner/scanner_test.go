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
	if g.SchemaVersion != domain.CurrentSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", g.SchemaVersion, domain.CurrentSchemaVersion)
	}
	if len(g.Entities) == 0 {
		t.Error("graph has no entities")
	}
	if len(g.FileTimestamps) == 0 {
		t.Error("expected FileTimestamps to be populated")
	}
	if len(g.FileFingerprints) == 0 {
		t.Error("expected FileFingerprints to be populated")
	}
	if !g.ScanComplete {
		t.Errorf("test repository scan should be complete, warnings: %v", g.ScanWarnings)
	}
	if g.EntityIdentity != domain.CurrentEntityIdentity {
		t.Errorf("entity identity = %q, want %q", g.EntityIdentity, domain.CurrentEntityIdentity)
	}
	if g.ScanCoverage == nil || g.ScanCoverage.Discovered != len(g.ScanFiles) || g.ScanCoverage.Failed != 0 {
		t.Fatalf("unexpected scan coverage: %+v (files=%d)", g.ScanCoverage, len(g.ScanFiles))
	}
}

func TestScan_RecordsCoverageForTemplatesAndIgnoredFiles(t *testing.T) {
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Join(repoDir, "config")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
spec:
  replicas: {{ .Replicas }}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "notes.txt"), []byte("not parsed by the current scanner\n"), 0644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "atlas.json")
	result, err := Scan(repoDir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Graph.ScanCoverage == nil {
		t.Fatal("scan coverage is missing")
	}
	coverage := *result.Graph.ScanCoverage
	if coverage.Discovered != 3 || coverage.Parsed != 2 || coverage.Ignored != 1 || coverage.Reused != 0 || coverage.Failed != 0 {
		t.Fatalf("coverage = %+v, want discovered=3 parsed=2 ignored=1", coverage)
	}
	if !result.Graph.ScanComplete {
		t.Fatalf("ignored files should not be reported as parser failures: %v", result.Graph.ScanWarnings)
	}
	statuses := make(map[string]domain.ScanFileStatus)
	for _, file := range result.Graph.ScanFiles {
		statuses[file.Path] = file.Status
	}
	if statuses["main.go"] != domain.ScanFileParsed || statuses["config/deployment.yaml"] != domain.ScanFileParsed || statuses["notes.txt"] != domain.ScanFileIgnored {
		t.Fatalf("scan file statuses = %+v", statuses)
	}
	foundTemplate := false
	for _, entity := range result.Graph.Entities {
		if entity.Kind == domain.KindTemplate {
			foundTemplate = true
			if entity.Source.Parser != "yaml-template" || entity.Name != "Deployment template" {
				t.Fatalf("template entity = %+v", entity)
			}
		}
	}
	if !foundTemplate {
		t.Fatal("expected a template entity for the unresolved deployment name")
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
	firstBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read first graph: %v", err)
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
	secondBytes, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read incremental graph: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("unchanged full and incremental scans produced different graph bytes")
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

func TestScan_IncrementalClearsPreviousTemporalDataWhenNotRequested(t *testing.T) {
	repoDir := setupTestRepo(t)
	outPath := filepath.Join(t.TempDir(), "atlas.json")

	first, err := Scan(repoDir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if len(first.Graph.Entities) == 0 {
		t.Fatal("initial scan produced no entities")
	}
	first.Graph.Entities[0].LastAuthor = "stale@example.com"
	first.Graph.Entities[0].LastModified = "2026-08-01T00:00:00Z"
	first.Graph.Entities[0].ChangeCount = 99
	if err := storage.WriteGraph(outPath, first.Graph); err != nil {
		t.Fatalf("write temporal fixture graph: %v", err)
	}

	second, err := Scan(repoDir, outPath, ScanOptions{PreviousGraph: outPath})
	if err != nil {
		t.Fatalf("incremental scan: %v", err)
	}
	if !second.Incremental {
		t.Fatal("expected incremental scan")
	}
	for _, entity := range second.Graph.Entities {
		if entity.LastAuthor != "" || entity.LastModified != "" || entity.ChangeCount != 0 {
			t.Fatalf("stale temporal data survived non-temporal scan for %s: %+v", entity.ID, entity)
		}
	}
}

func TestScan_IncompleteGraphForcesFullRescan(t *testing.T) {
	repoDir := setupTestRepo(t)
	if err := os.WriteFile(filepath.Join(repoDir, "broken.yaml"), []byte("kind: ["), 0644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "atlas.json")

	first, err := Scan(repoDir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	if first.Graph.ScanComplete || len(first.Graph.ScanWarnings) == 0 {
		t.Fatalf("expected incomplete initial graph, got complete=%v warnings=%v", first.Graph.ScanComplete, first.Graph.ScanWarnings)
	}
	if first.Graph.ScanCoverage == nil || first.Graph.ScanCoverage.Failed != 1 {
		t.Fatalf("expected one failed scan file, got coverage=%+v", first.Graph.ScanCoverage)
	}

	second, err := Scan(repoDir, outPath, ScanOptions{PreviousGraph: outPath})
	if err != nil {
		t.Fatalf("rescan: %v", err)
	}
	if second.Incremental {
		t.Fatal("incomplete graph must force a full rescan")
	}
	if second.Graph.ScanComplete || len(second.Graph.ScanWarnings) == 0 {
		t.Fatalf("expected incomplete rescan result, got complete=%v warnings=%v", second.Graph.ScanComplete, second.Graph.ScanWarnings)
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
		"a.go": fileFingerprint(files[0]),
		"b.go": fileFingerprint(files[1]),
		"c.go": fileFingerprint(domain.File{RelativePath: "c.go", ModifiedTime: now, Size: 0}),
		"d.go": fileFingerprint(domain.File{RelativePath: "d.go", ModifiedTime: now}),
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

func TestChangedFiles_ContentHash(t *testing.T) {
	files := []domain.File{
		{RelativePath: "same.go", ContentHash: "sha256:same"},
		{RelativePath: "changed.go", ContentHash: "sha256:new"},
	}
	previous := map[string]string{
		"same.go":    "sha256:same",
		"changed.go": "sha256:old",
	}

	changed, unchanged, _ := changedFiles(files, previous)
	if len(changed) != 1 || changed[0].RelativePath != "changed.go" {
		t.Errorf("changed = %v, want changed.go", changed)
	}
	if len(unchanged) != 1 || unchanged[0].RelativePath != "same.go" {
		t.Errorf("unchanged = %v, want same.go", unchanged)
	}
}

func TestClearTemporalFields(t *testing.T) {
	entities := []domain.Entity{
		{ID: "function:pkg.changed", LastAuthor: "alice@example.com", LastModified: "2026-08-01T00:00:00Z", ChangeCount: 4},
		{ID: "function:pkg.empty"},
	}

	clearTemporalFields(entities)
	for _, entity := range entities {
		if entity.LastAuthor != "" || entity.LastModified != "" || entity.ChangeCount != 0 {
			t.Fatalf("temporal fields were not cleared for %s: %+v", entity.ID, entity)
		}
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

func TestScan_MergesControllerFactsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/project\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatal(err)
	}
	controllerDir := filepath.Join(dir, "controllers")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controllerDir, "reconcile.go"), []byte(`package controllers

type WidgetReconciler struct{}

func (r *WidgetReconciler) Reconcile() {}
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(controllerDir, "setup.go"), []byte(`package controllers

func (r *WidgetReconciler) SetupWithManager(mgr interface{}) error {
	return builder.For(&Widget{}).Complete(r)
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	manifestDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manifestDir, "widget-crd.yaml"), []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
`), 0644); err != nil {
		t.Fatal(err)
	}

	outPath := filepath.Join(t.TempDir(), "atlas.json")
	result, err := Scan(dir, outPath, ScanOptions{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(result.Graph.ScanWarnings) != 0 {
		t.Fatalf("unexpected warnings: %v", result.Graph.ScanWarnings)
	}

	var controller *domain.Entity
	for i := range result.Graph.Entities {
		if result.Graph.Entities[i].Kind == domain.KindController {
			controller = &result.Graph.Entities[i]
			break
		}
	}
	if controller == nil {
		t.Fatal("expected merged controller entity")
	}
	if len(controller.Files) != 2 || controller.Files[0] != "controllers/reconcile.go" || controller.Files[1] != "controllers/setup.go" {
		t.Fatalf("controller files = %v, want both implementation files", controller.Files)
	}
	if len(controller.Watches) != 1 || controller.Watches[0] != "Widget" || controller.WatchMethods[0] != "For" {
		t.Fatalf("controller watch facts = %v/%v, want Widget/For", controller.Watches, controller.WatchMethods)
	}

	wantTarget := "crd:example.com.widget"
	wantRelation := domain.NewRelationshipID(controller.ID, domain.RelReconciles, wantTarget)
	found := false
	for _, relation := range result.Graph.Relationship {
		if relation.ID == wantRelation {
			found = true
			if relation.Evidence.File != "controllers/setup.go" {
				t.Errorf("relationship evidence file = %q, want controllers/setup.go", relation.Evidence.File)
			}
		}
	}
	if !found {
		t.Fatalf("expected reconciles relationship %s, got %v", wantRelation, result.Graph.Relationship)
	}
}
