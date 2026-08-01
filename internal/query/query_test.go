package query

import (
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestGetEntity(t *testing.T) {
	idx := newIndex(testGraph())

	e := idx.GetEntity("controller:pkg.MyController")
	if e == nil {
		t.Fatal("expected to find controller")
	}
	if e.Name != "MyController" {
		t.Fatalf("name: got %q, want MyController", e.Name)
	}

	if idx.GetEntity("nonexistent") != nil {
		t.Fatal("expected nil for nonexistent ID")
	}
}

func TestLookup_ByKind(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("controller", "", 0)
	if len(results) != 1 {
		t.Fatalf("controller lookup: got %d, want 1", len(results))
	}

	results = idx.Lookup("function", "", 0)
	if len(results) != 4 {
		t.Fatalf("function lookup: got %d, want 4", len(results))
	}
}

func TestLookup_ByName(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("", "reconcile", 0)
	if len(results) != 4 {
		t.Fatalf("name 'reconcile' lookup: got %d, want 4 (3 functions + 1 test)", len(results))
	}
}

func TestLookup_ByKindAndName(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("function", "etcd", 0)
	if len(results) != 2 {
		t.Fatalf("function+etcd: got %d, want 2", len(results))
	}
}

func TestLookup_CaseInsensitive(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("CONTROLLER", "MYCONTROLLER", 0)
	if len(results) != 1 {
		t.Fatalf("case insensitive lookup: got %d, want 1", len(results))
	}
}

func TestLookup_MaxResults(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("", "", 3)
	if len(results) != 3 {
		t.Fatalf("max results: got %d, want 3", len(results))
	}
}

func TestLookup_InvalidKind(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Lookup("nonexistent", "", 0)
	if results != nil {
		t.Fatalf("invalid kind: got %d results, want nil", len(results))
	}
}

func TestGetRelationships_From(t *testing.T) {
	idx := newIndex(testGraph())

	rels := idx.GetRelationships("controller:pkg.MyController", "from", "")
	if len(rels) != 3 {
		t.Fatalf("from controller: got %d, want 3", len(rels))
	}
}

func TestGetRelationships_To(t *testing.T) {
	idx := newIndex(testGraph())

	rels := idx.GetRelationships("crd:group.HostedCluster", "to", "")
	if len(rels) != 1 {
		t.Fatalf("to crd: got %d, want 1", len(rels))
	}
}

func TestGetRelationships_Both(t *testing.T) {
	idx := newIndex(testGraph())

	rels := idx.GetRelationships("function:pkg.reconcileEtcd", "both", "")
	if len(rels) != 2 {
		t.Fatalf("both for function: got %d, want 2 (1 incoming call + 1 outgoing tested_by)", len(rels))
	}
}

func TestGetRelationships_TypeFilter(t *testing.T) {
	idx := newIndex(testGraph())

	rels := idx.GetRelationships("controller:pkg.MyController", "from", "calls")
	if len(rels) != 1 {
		t.Fatalf("calls only: got %d, want 1", len(rels))
	}
	if rels[0].Type != domain.RelCalls {
		t.Fatalf("type: got %s, want calls", rels[0].Type)
	}
}

func TestNeighbors_Depth0(t *testing.T) {
	idx := newIndex(testGraph())

	sg := idx.Neighbors("controller:pkg.MyController", 0)
	if len(sg.Entities) != 1 {
		t.Fatalf("depth 0: got %d entities, want 1", len(sg.Entities))
	}
	if len(sg.Relationships) != 0 {
		t.Fatalf("depth 0: got %d rels, want 0", len(sg.Relationships))
	}
}

func TestNeighbors_Depth1(t *testing.T) {
	idx := newIndex(testGraph())

	sg := idx.Neighbors("controller:pkg.MyController", 1)
	if len(sg.Entities) != 4 {
		t.Fatalf("depth 1: got %d entities, want 4 (controller + crd + function + resource)", len(sg.Entities))
	}
	if len(sg.Relationships) != 3 {
		t.Fatalf("depth 1: got %d rels, want 3", len(sg.Relationships))
	}
}

func TestNeighbors_Depth2(t *testing.T) {
	idx := newIndex(testGraph())

	sg := idx.Neighbors("controller:pkg.MyController", 2)
	if len(sg.Entities) < 4 {
		t.Fatalf("depth 2: got %d entities, want >= 4", len(sg.Entities))
	}
}

func TestNeighbors_NotFound(t *testing.T) {
	idx := newIndex(testGraph())

	sg := idx.Neighbors("nonexistent", 1)
	if len(sg.Entities) != 0 {
		t.Fatalf("not found: got %d entities, want 0", len(sg.Entities))
	}
}

func TestNeighbors_Deterministic(t *testing.T) {
	idx := newIndex(testGraph())

	first := idx.Neighbors("controller:pkg.MyController", 2)
	for i := 0; i < 10; i++ {
		again := idx.Neighbors("controller:pkg.MyController", 2)
		if len(again.Entities) != len(first.Entities) {
			t.Fatalf("run %d: entity count changed %d vs %d", i, len(again.Entities), len(first.Entities))
		}
		for j, e := range again.Entities {
			if e.ID != first.Entities[j].ID {
				t.Fatalf("run %d: entity %d ID changed %q vs %q", i, j, e.ID, first.Entities[j].ID)
			}
		}
	}
}

func TestSearch(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Search("etcd", 0)
	if len(results) != 4 {
		t.Fatalf("search 'etcd': got %d, want 4 (function + test + resource via property + pki function)", len(results))
	}

	results = idx.Search("HostedCluster", 0)
	if len(results) != 2 {
		t.Fatalf("search 'HostedCluster': got %d, want 2 (crd + controller via description)", len(results))
	}
}

func TestSearch_NoResults(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Search("zzzznonexistent", 0)
	if len(results) != 0 {
		t.Fatalf("got %d, want 0", len(results))
	}
}

func TestWhere(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Where("etcd.go", 0)
	if len(results) != 3 {
		t.Fatalf("where 'etcd.go': got %d, want 3 (function + test + pki function)", len(results))
	}
}

func TestWhere_MatchesFilesArray(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Where("controller.go", 0)
	found := false
	for _, e := range results {
		if e.Kind == domain.KindPackage {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected package entity matched via Files array")
	}
}

func TestSearch_MultiTermAND(t *testing.T) {
	idx := newIndex(testGraph())

	// "etcd peer" should match only the PKI function (name has "etcd", description has "peer")
	results := idx.Search("etcd peer", 0)
	if len(results) != 1 {
		t.Fatalf("search 'etcd peer': got %d, want 1", len(results))
	}
	if results[0].ID != "function:pki.ReconcileEtcdPeer" {
		t.Fatalf("got %s, want function:pki.ReconcileEtcdPeer", results[0].ID)
	}
}

func TestSearch_MultiTermCrossField(t *testing.T) {
	idx := newIndex(testGraph())

	// "pki etcd-discovery" — "pki" matches package, "etcd-discovery" matches literals
	results := idx.Search("pki etcd-discovery", 0)
	if len(results) != 1 {
		t.Fatalf("search 'pki etcd-discovery': got %d, want 1", len(results))
	}
	if results[0].ID != "function:pki.ReconcileEtcdPeer" {
		t.Fatalf("got %s, want function:pki.ReconcileEtcdPeer", results[0].ID)
	}
}

func TestSearch_MultiTermNoMatch(t *testing.T) {
	idx := newIndex(testGraph())

	// "etcd network" — no entity matches both
	results := idx.Search("etcd network", 0)
	if len(results) != 0 {
		t.Fatalf("search 'etcd network': got %d, want 0", len(results))
	}
}

func TestSearch_SingleTermBackwardCompat(t *testing.T) {
	idx := newIndex(testGraph())

	// Single term should work exactly as before
	results := idx.Search("reconcile", 0)
	if len(results) < 3 {
		t.Fatalf("search 'reconcile': got %d, want >= 3", len(results))
	}
}

func TestStats(t *testing.T) {
	idx := newIndex(testGraph())

	s := idx.Stats()
	if s.TotalEntities != 10 {
		t.Fatalf("total entities: got %d, want 10", s.TotalEntities)
	}
	if s.TotalRels != 4 {
		t.Fatalf("total rels: got %d, want 4", s.TotalRels)
	}
	if s.EntityCounts["controller"] != 1 {
		t.Fatalf("controller count: got %d, want 1", s.EntityCounts["controller"])
	}
	if s.RelCounts["calls"] != 1 {
		t.Fatalf("calls count: got %d, want 1", s.RelCounts["calls"])
	}
}

func TestSearch_RelevanceOrder(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Search("etcd", 0)
	if len(results) < 2 {
		t.Fatalf("search 'etcd': got %d, want >= 2", len(results))
	}
	// Name-match entities (reconcileEtcd, ReconcileEtcdPeer) should rank before
	// property-match (resource with spec.selector.app=etcd)
	nameMatchIdx := -1
	propMatchIdx := -1
	for i, e := range results {
		if e.ID == "function:pkg.reconcileEtcd" {
			nameMatchIdx = i
		}
		if e.ID == "resource:Deployment.my-deploy" {
			propMatchIdx = i
		}
	}
	if nameMatchIdx == -1 || propMatchIdx == -1 {
		t.Fatalf("expected both name-match and property-match results, got %v", results)
	}
	if nameMatchIdx > propMatchIdx {
		t.Errorf("name-match (score 100) at index %d should rank before property-match (score 20) at index %d", nameMatchIdx, propMatchIdx)
	}
}

func TestCallers(t *testing.T) {
	idx := newIndex(testGraph())

	// MyController calls reconcileEtcd (via r2 relationship)
	callers := idx.Callers("function:pkg.reconcileEtcd")
	if len(callers) != 1 {
		t.Fatalf("callers of reconcileEtcd: got %d, want 1", len(callers))
	}
	if callers[0].ID != "controller:pkg.MyController" {
		t.Fatalf("caller: got %s, want controller:pkg.MyController", callers[0].ID)
	}
}

func TestCallers_NoCallers(t *testing.T) {
	idx := newIndex(testGraph())

	callers := idx.Callers("controller:pkg.MyController")
	if len(callers) != 0 {
		t.Fatalf("callers of MyController: got %d, want 0", len(callers))
	}
}

func TestCallers_NotFound(t *testing.T) {
	idx := newIndex(testGraph())

	callers := idx.Callers("nonexistent:id")
	if len(callers) != 0 {
		t.Fatalf("callers of nonexistent: got %d, want 0", len(callers))
	}
}

func TestCommits_ByName(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Commits("ReconcileEtcdPeer", "", "", 20)
	if len(results) != 1 {
		t.Fatalf("commits by name 'ReconcileEtcdPeer': got %d, want 1", len(results))
	}
	if results[0].ID != "function:pki.ReconcileEtcdPeer" {
		t.Fatalf("got %s, want function:pki.ReconcileEtcdPeer", results[0].ID)
	}
}

func TestCommits_BySince(t *testing.T) {
	idx := newIndex(testGraph())

	// Only ReconcileEtcdPeer modified 2026-07-15, helperFunc modified 2026-06-01
	results := idx.Commits("", "2026-07-01", "", 20)
	if len(results) != 1 {
		t.Fatalf("commits since 2026-07-01: got %d, want 1", len(results))
	}
	if results[0].ID != "function:pki.ReconcileEtcdPeer" {
		t.Fatalf("got %s, want function:pki.ReconcileEtcdPeer", results[0].ID)
	}
}

func TestCommits_ByAuthor(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Commits("", "", "alice", 20)
	if len(results) != 1 {
		t.Fatalf("commits by alice: got %d, want 1", len(results))
	}
	if results[0].LastAuthor != "alice@redhat.com" {
		t.Fatalf("author: got %s, want alice@redhat.com", results[0].LastAuthor)
	}
}

func TestCommits_NoTemporal(t *testing.T) {
	idx := newIndex(testGraph())

	// Search for entity without temporal data
	results := idx.Commits("MyController", "", "", 20)
	if len(results) != 0 {
		t.Fatalf("commits for MyController (no temporal): got %d, want 0", len(results))
	}
}

func TestCommits_AllWithTemporal(t *testing.T) {
	idx := newIndex(testGraph())

	results := idx.Commits("", "", "", 20)
	if len(results) != 2 {
		t.Fatalf("all commits: got %d, want 2", len(results))
	}
	// Sorted by LastModified desc: 2026-07-15 before 2026-06-01
	if results[0].LastModified != "2026-07-15" {
		t.Fatalf("first result LastModified: got %s, want 2026-07-15", results[0].LastModified)
	}
	if results[1].LastModified != "2026-06-01" {
		t.Fatalf("second result LastModified: got %s, want 2026-06-01", results[1].LastModified)
	}
}

func compoundTestGraph() domain.Graph {
	g := testGraph()
	g.Relationship = append(g.Relationship,
		domain.Relationship{
			ID: "r5", From: "function:pkg.reconcileNetwork", To: "function:pkg.reconcileEtcd",
			Type: domain.RelCalls, Confidence: domain.ConfidenceInferred,
			Evidence: domain.Evidence{File: "pkg/network.go", Line: 20},
		},
		domain.Relationship{
			ID: "r6", From: "function:pkg.reconcileEtcd", To: "function:pkg.reconcileNetwork",
			Type: domain.RelCalls, Confidence: domain.ConfidenceInferred,
			Evidence: domain.Evidence{File: "pkg/etcd.go", Line: 25},
		},
	)
	return g
}

func TestInvestigate(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Investigate("controller:pkg.MyController")
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.Entity.ID != "controller:pkg.MyController" {
		t.Fatalf("entity: got %s", r.Entity.ID)
	}

	// 3 outgoing: reconciles, calls, creates
	outCount := 0
	for _, rels := range r.OutRels {
		outCount += len(rels)
	}
	if outCount != 3 {
		t.Fatalf("outgoing rels: got %d, want 3", outCount)
	}

	// 0 incoming (nothing points to controller in testGraph)
	inCount := 0
	for _, rels := range r.InRels {
		inCount += len(rels)
	}
	if inCount != 0 {
		t.Fatalf("incoming rels: got %d, want 0", inCount)
	}

	if len(r.Callers) != 0 {
		t.Fatalf("callers: got %d, want 0", len(r.Callers))
	}
	if len(r.Tests) != 0 {
		t.Fatalf("tests: got %d, want 0", len(r.Tests))
	}

	// Siblings: package:pkg matches via Files array
	if len(r.Siblings) != 1 {
		t.Fatalf("siblings: got %d, want 1 (package:pkg)", len(r.Siblings))
	}
}

func TestInvestigate_WithCallersAndTests(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Investigate("function:pkg.reconcileEtcd")
	if r == nil {
		t.Fatal("expected non-nil result")
	}

	if len(r.Callers) != 1 {
		t.Fatalf("callers: got %d, want 1", len(r.Callers))
	}
	if r.Callers[0].ID != "controller:pkg.MyController" {
		t.Fatalf("caller: got %s", r.Callers[0].ID)
	}

	if len(r.Tests) != 1 {
		t.Fatalf("tests: got %d, want 1", len(r.Tests))
	}
	if r.Tests[0].ID != "test:pkg.TestReconcileEtcd" {
		t.Fatalf("test: got %s", r.Tests[0].ID)
	}
}

func TestInvestigate_NotFound(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Investigate("nonexistent:id")
	if r != nil {
		t.Fatal("expected nil for nonexistent entity")
	}
}

func TestExplain_Depth1(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Explain("controller:pkg.MyController", 1)
	if r.Root == nil {
		t.Fatal("expected non-nil root")
	}
	if r.Root.Entity.ID != "controller:pkg.MyController" {
		t.Fatalf("root: got %s", r.Root.Entity.ID)
	}
	// Depth 1: 3 children (crd via reconciles, resource via creates, reconcileEtcd via calls)
	if len(r.Root.Children) != 3 {
		t.Fatalf("children: got %d, want 3", len(r.Root.Children))
	}
	// Total: root + 3 children = 4
	if r.TotalNodes != 4 {
		t.Fatalf("total nodes: got %d, want 4", r.TotalNodes)
	}
}

func TestExplain_Depth2(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Explain("controller:pkg.MyController", 2)
	if r.Root == nil {
		t.Fatal("expected non-nil root")
	}
	// Depth 2: root -> (crd, resource, reconcileEtcd -> TestReconcileEtcd)
	if r.TotalNodes != 5 {
		t.Fatalf("total nodes: got %d, want 5", r.TotalNodes)
	}

	// Find reconcileEtcd child and verify it has TestReconcileEtcd
	var etcdChild *ExplainNode
	for _, c := range r.Root.Children {
		if c.Entity.ID == "function:pkg.reconcileEtcd" {
			etcdChild = c
			break
		}
	}
	if etcdChild == nil {
		t.Fatal("expected reconcileEtcd in children")
	}
	if len(etcdChild.Children) != 1 {
		t.Fatalf("reconcileEtcd children: got %d, want 1", len(etcdChild.Children))
	}
	if etcdChild.Children[0].Entity.ID != "test:pkg.TestReconcileEtcd" {
		t.Fatalf("test child: got %s", etcdChild.Children[0].Entity.ID)
	}
}

func TestExplain_NotFound(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Explain("nonexistent:id", 2)
	if r.Root != nil {
		t.Fatal("expected nil root for nonexistent entity")
	}
	if r.TotalNodes != 0 {
		t.Fatalf("total nodes: got %d, want 0", r.TotalNodes)
	}
}

func TestExplain_NoCycles(t *testing.T) {
	idx := newIndex(compoundTestGraph())

	// compoundTestGraph adds cycle: reconcileEtcd <-> reconcileNetwork
	r := idx.Explain("function:pkg.reconcileEtcd", 3)
	if r.Root == nil {
		t.Fatal("expected non-nil root")
	}
	// Should terminate without infinite loop. Visited set prevents revisiting.
	if r.TotalNodes > 10 {
		t.Fatalf("total nodes %d seems too high, possible cycle issue", r.TotalNodes)
	}
	if r.Capped {
		t.Fatal("should not be capped for small graph")
	}
}

func TestImpact_Function(t *testing.T) {
	idx := newIndex(testGraph())

	// reconcileEtcd is called by MyController
	r := idx.Impact("function:pkg.reconcileEtcd")
	if r == nil {
		t.Fatal("expected non-nil result")
	}
	if r.Entity.ID != "function:pkg.reconcileEtcd" {
		t.Fatalf("entity: got %s", r.Entity.ID)
	}

	// Call chain: MyController calls reconcileEtcd
	if len(r.CallChain) != 1 {
		t.Fatalf("call chain: got %d, want 1", len(r.CallChain))
	}
	if r.CallChain[0].ID != "controller:pkg.MyController" {
		t.Fatalf("caller: got %s", r.CallChain[0].ID)
	}

	// Controllers: MyController
	if len(r.Controllers) != 1 {
		t.Fatalf("controllers: got %d, want 1", len(r.Controllers))
	}

	// Tests: TestReconcileEtcd (from reconcileEtcd's tested_by edge)
	if len(r.Tests) != 1 {
		t.Fatalf("tests: got %d, want 1", len(r.Tests))
	}

	// Resources: MyController creates Deployment + reconciles HostedCluster
	if len(r.Resources) != 2 {
		t.Fatalf("resources: got %d, want 2", len(r.Resources))
	}

	// Files: pkg/etcd.go (root) + pkg/controller.go (caller)
	if len(r.Files) != 2 {
		t.Fatalf("files: got %d, want 2", len(r.Files))
	}
}

func TestImpact_Controller(t *testing.T) {
	idx := newIndex(testGraph())

	// Controller has no callers — it's a top-level entry point
	r := idx.Impact("controller:pkg.MyController")
	if r == nil {
		t.Fatal("expected non-nil result")
	}

	if len(r.CallChain) != 0 {
		t.Fatalf("call chain: got %d, want 0", len(r.CallChain))
	}

	// Controller itself is in the chain, so it appears in Controllers
	if len(r.Controllers) != 1 {
		t.Fatalf("controllers: got %d, want 1", len(r.Controllers))
	}

	// Resources from controller's own edges
	if len(r.Resources) != 2 {
		t.Fatalf("resources: got %d, want 2", len(r.Resources))
	}
}

func TestImpact_NotFound(t *testing.T) {
	idx := newIndex(testGraph())

	r := idx.Impact("nonexistent:id")
	if r != nil {
		t.Fatal("expected nil for nonexistent entity")
	}
}

func TestImpact_WithTemporal(t *testing.T) {
	idx := newIndex(testGraph())

	// ReconcileEtcdPeer has temporal data but no callers in testGraph
	r := idx.Impact("function:pki.ReconcileEtcdPeer")
	if r == nil {
		t.Fatal("expected non-nil result")
	}

	if len(r.RecentChanges) != 1 {
		t.Fatalf("recent changes: got %d, want 1", len(r.RecentChanges))
	}
	if len(r.Owners) != 1 {
		t.Fatalf("owners: got %d, want 1", len(r.Owners))
	}
	if r.Owners[0] != "alice@redhat.com" {
		t.Fatalf("owner: got %s", r.Owners[0])
	}
}

func TestImpact_NoCycles(t *testing.T) {
	idx := newIndex(compoundTestGraph())

	// compoundTestGraph: reconcileEtcd <-> reconcileNetwork cycle
	r := idx.Impact("function:pkg.reconcileEtcd")
	if r == nil {
		t.Fatal("expected non-nil result")
	}

	// Should terminate without infinite loop
	// Chain: reconcileEtcd + callers (MyController, reconcileNetwork)
	if len(r.CallChain) > 10 {
		t.Fatalf("call chain %d seems too high, possible cycle issue", len(r.CallChain))
	}
}
