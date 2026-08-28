package query

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func testGraph() domain.Graph {
	source := func(parser, file string, line int) domain.Source {
		return domain.Source{Parser: parser, File: file, Line: line}
	}
	callEvidence := func(file string, line int) domain.Evidence {
		return domain.Evidence{Parser: "go-ast", File: file, Line: line, Reason: "test relationship fixture"}
	}
	return domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.0.0",
		Repository:    "/test/repo",
		Entities: []domain.Entity{
			{ID: "controller:pkg.MyController", Name: "MyController", Kind: domain.KindController, Package: "pkg", Source: source("go", "pkg/controller.go", 10), Description: "Reconciles HostedCluster resources"},
			{ID: "crd:group.HostedCluster", Name: "HostedCluster", Kind: domain.KindCRD, Package: "group", Source: source("go", "api/types.go", 5)},
			{ID: "function:pkg.reconcileEtcd", Name: "reconcileEtcd", Kind: domain.KindFunction, Package: "pkg", Source: source("go", "pkg/etcd.go", 20), Description: "Handles etcd reconciliation"},
			{ID: "function:pkg.reconcileNetwork", Name: "reconcileNetwork", Kind: domain.KindFunction, Package: "pkg", Source: source("go", "pkg/network.go", 15)},
			{ID: "test:pkg.TestReconcileEtcd", Name: "TestReconcileEtcd", Kind: domain.KindTest, Package: "pkg", Source: source("test", "pkg/etcd_test.go", 1)},
			{ID: "package:pkg", Name: "pkg", Kind: domain.KindPackage, Package: "pkg", Files: []string{"pkg/controller.go", "pkg/etcd.go", "pkg/network.go"}, Source: source("go", "pkg", 1)},
			{ID: "document:README", Name: "README", Kind: domain.KindDocument, Source: source("document", "README.md", 1), Description: "Project documentation"},
			{ID: "resource:Deployment.my-deploy", Name: "my-deploy", Kind: domain.KindResource, Source: source("yaml", "deploy.yaml", 1), Properties: []string{"kind=Deployment", "metadata.name=my-deploy", "spec.selector.app=etcd"}},
			{ID: "function:pki.ReconcileEtcdPeer", Name: "ReconcileEtcdPeer", Kind: domain.KindFunction, Package: "pki", Source: source("go", "pki/etcd.go", 44), Description: "Reconciles etcd peer certificates", Literals: []string{"etcd-discovery.%s.svc", "etcd-client.%s.svc"}, LastModified: "2026-07-15", LastAuthor: "alice@redhat.com", ChangeCount: 5},
			{ID: "function:pkg.helperFunc", Name: "helperFunc", Kind: domain.KindFunction, Package: "pkg", Source: source("go", "pkg/helper.go", 1), LastModified: "2026-06-01", LastAuthor: "bob@redhat.com", ChangeCount: 2},
		},
		Relationship: []domain.Relationship{
			{ID: domain.NewRelationshipID("controller:pkg.MyController", domain.RelReconciles, "crd:group.HostedCluster"), From: "controller:pkg.MyController", To: "crd:group.HostedCluster", Type: domain.RelReconciles, Confidence: domain.ConfidenceProven, Evidence: callEvidence("pkg/controller.go", 10)},
			{ID: domain.NewRelationshipID("controller:pkg.MyController", domain.RelCalls, "function:pkg.reconcileEtcd"), From: "controller:pkg.MyController", To: "function:pkg.reconcileEtcd", Type: domain.RelCalls, Confidence: domain.ConfidenceInferred, Evidence: callEvidence("pkg/controller.go", 25)},
			{ID: domain.NewRelationshipID("function:pkg.reconcileEtcd", domain.RelTestedBy, "test:pkg.TestReconcileEtcd"), From: "function:pkg.reconcileEtcd", To: "test:pkg.TestReconcileEtcd", Type: domain.RelTestedBy, Confidence: domain.ConfidenceInferred, Evidence: callEvidence("pkg/etcd_test.go", 1)},
			{ID: domain.NewRelationshipID("controller:pkg.MyController", domain.RelCreates, "resource:Deployment.my-deploy"), From: "controller:pkg.MyController", To: "resource:Deployment.my-deploy", Type: domain.RelCreates, Confidence: domain.ConfidenceProven, Evidence: callEvidence("pkg/controller.go", 30)},
		},
	}
}

func TestNewIndex_PopulatesAllMaps(t *testing.T) {
	idx := newIndex(testGraph())

	if len(idx.byID) != 10 {
		t.Fatalf("byID: got %d, want 10", len(idx.byID))
	}
	if len(idx.byKind[domain.KindController]) != 1 {
		t.Fatalf("byKind[controller]: got %d, want 1", len(idx.byKind[domain.KindController]))
	}
	if len(idx.byKind[domain.KindFunction]) != 4 {
		t.Fatalf("byKind[function]: got %d, want 4", len(idx.byKind[domain.KindFunction]))
	}
	if len(idx.byName["mycontroller"]) != 1 {
		t.Fatalf("byName[mycontroller]: got %d, want 1", len(idx.byName["mycontroller"]))
	}
	if len(idx.byPackage["pkg"]) != 6 {
		t.Fatalf("byPackage[pkg]: got %d, want 6", len(idx.byPackage["pkg"]))
	}
	if len(idx.fromEntity["controller:pkg.MyController"]) != 3 {
		t.Fatalf("fromEntity[controller]: got %d, want 3", len(idx.fromEntity["controller:pkg.MyController"]))
	}
	if len(idx.toEntity["crd:group.HostedCluster"]) != 1 {
		t.Fatalf("toEntity[crd]: got %d, want 1", len(idx.toEntity["crd:group.HostedCluster"]))
	}
	if len(idx.byRelType[domain.RelCalls]) != 1 {
		t.Fatalf("byRelType[calls]: got %d, want 1", len(idx.byRelType[domain.RelCalls]))
	}
}

func TestNewIndex_Empty(t *testing.T) {
	idx := newIndex(domain.Graph{})

	if idx.byID == nil {
		t.Fatal("byID should be non-nil even for empty graph")
	}
	if len(idx.byID) != 0 {
		t.Fatalf("byID should be empty, got %d", len(idx.byID))
	}
}

func TestResolveViewRequiresUnambiguousEntityName(t *testing.T) {
	graph := testGraph()
	graph.Views = map[string]domain.View{
		"controller:pkg.MyController": {
			EntityID:   "controller:pkg.MyController",
			EntityName: "MyController",
		},
	}
	graph.Entities = append(graph.Entities, domain.Entity{
		ID:   "function:other.MyController",
		Name: "MyController",
		Kind: domain.KindFunction,
		Source: domain.Source{
			Parser: "go",
			File:   "other.go",
			Line:   1,
		},
	})
	idx := newIndex(graph)

	if idx.ResolveView("controller:pkg.MyController") == nil {
		t.Fatal("exact view ID should resolve")
	}
	if idx.ResolveView("MyController") != nil {
		t.Fatal("same-named non-view entity should make name lookup ambiguous")
	}
}

func TestEntityListResult_DeduplicatesAndCapsRelationships(t *testing.T) {
	idx := newIndex(testGraph())
	entities := idx.Lookup("controller", "", 1)
	result := idx.EntityListResult(entities, true, 2)

	if result.Graph.SchemaVersion != "1.0.0" {
		t.Fatalf("graph metadata was not included: %+v", result.Graph)
	}
	if len(result.Entities) != 1 || len(result.Relationships) != 2 {
		t.Fatalf("unexpected result sizes: entities=%d relationships=%d", len(result.Entities), len(result.Relationships))
	}
	if !result.Truncated {
		t.Fatal("expected relationship cap to be marked truncated")
	}
	if result.Relationships[0].ID == result.Relationships[1].ID {
		t.Fatal("relationships were not deduplicated")
	}
}

func TestLoadGraph_FileNotFound(t *testing.T) {
	_, err := LoadGraph("/nonexistent/path.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadGraph_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	g := testGraph()
	if err := storage.WriteGraph(path, g); err != nil {
		t.Fatalf("write: %v", err)
	}

	idx, err := LoadGraph(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(idx.byID) != len(g.Entities) {
		t.Fatalf("entity count: got %d, want %d", len(idx.byID), len(g.Entities))
	}
}

func TestLoadGraph_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid json"), 0644)

	_, err := LoadGraph(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseKind(t *testing.T) {
	tests := []struct {
		input string
		want  domain.EntityKind
		ok    bool
	}{
		{"controller", domain.KindController, true},
		{"Controller", domain.KindController, true},
		{"CONTROLLER", domain.KindController, true},
		{"crd", domain.KindCRD, true},
		{"function", domain.KindFunction, true},
		{"package", domain.KindPackage, true},
		{"test", domain.KindTest, true},
		{"document", domain.KindDocument, true},
		{"resource", domain.KindResource, true},
		{"operator", domain.KindOperator, true},
		{"invalid", domain.KindUnknown, false},
		{"", domain.KindUnknown, false},
	}

	for _, tt := range tests {
		got, ok := ParseKind(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Errorf("ParseKind(%q) = (%v, %v), want (%v, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}
