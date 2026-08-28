package query

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestFormatEntity(t *testing.T) {
	e := &domain.Entity{
		ID:          "controller:pkg.MyController",
		Name:        "MyController",
		Kind:        domain.KindController,
		Description: "Reconciles HostedCluster resources",
		Source:      domain.Source{File: "pkg/controller.go", Line: 10},
	}

	got := FormatEntity(e)
	if !strings.Contains(got, "controller:pkg.MyController") {
		t.Fatalf("missing ID in: %q", got)
	}
	if !strings.Contains(got, "pkg/controller.go:10") {
		t.Fatalf("missing file:line in: %q", got)
	}
	if !strings.Contains(got, "Reconciles HostedCluster") {
		t.Fatalf("missing description in: %q", got)
	}
}

func TestFormatEntity_TruncatesDescription(t *testing.T) {
	e := &domain.Entity{
		ID:          "test:id",
		Description: strings.Repeat("a", 100),
		Source:      domain.Source{File: "f.go", Line: 1},
	}

	got := FormatEntity(e)
	if !strings.HasSuffix(strings.TrimSpace(got), "...") {
		t.Fatalf("expected truncated description, got: %q", got)
	}
}

func TestFormatEntityFullDoesNotMutateGraphSlices(t *testing.T) {
	calls := make([]string, 12, 16)
	for i := range calls {
		calls[i] = "call"
	}
	e := &domain.Entity{
		ID:     "function:pkg.Reconcile",
		Name:   "Reconcile",
		Kind:   domain.KindFunction,
		Calls:  calls,
		Source: domain.Source{File: "pkg/reconcile.go", Line: 1},
	}

	FormatEntityFull(e)
	if len(e.Calls) != 12 || e.Calls[10] != "call" || e.Calls[11] != "call" {
		t.Fatalf("formatter mutated entity calls: %#v", e.Calls)
	}
}

func TestFormatEntityFullIncludesBoundedDocumentContent(t *testing.T) {
	e := &domain.Entity{
		ID:      "document:docs/design.md",
		Name:    "design.md",
		Kind:    domain.KindDocument,
		Content: strings.Repeat("x", 700),
		Source:  domain.Source{File: "docs/design.md", Line: 1},
	}

	got := FormatEntityFull(e)
	if !strings.Contains(got, "Content:") || !strings.Contains(got, "[TRUNCATED:") {
		t.Fatalf("document content was not exposed with a bound: %q", got)
	}
}

func TestFormatRelationship(t *testing.T) {
	r := &domain.Relationship{
		From:       "controller:pkg.MyController",
		To:         "crd:group.HostedCluster",
		Type:       domain.RelReconciles,
		Confidence: domain.ConfidenceProven,
		Evidence:   domain.Evidence{File: "pkg/controller.go", Line: 10},
	}

	got := FormatRelationship(r)
	if !strings.Contains(got, "--reconciles-->") {
		t.Fatalf("missing arrow in: %q", got)
	}
	if !strings.Contains(got, "proven") {
		t.Fatalf("missing confidence in: %q", got)
	}
}

func TestFormatEntityList(t *testing.T) {
	entities := []*domain.Entity{
		{ID: "a", Source: domain.Source{File: "a.go", Line: 1}},
		{ID: "b", Source: domain.Source{File: "b.go", Line: 2}},
	}

	got := FormatEntityList(entities)
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestFormatEntityList_Empty(t *testing.T) {
	got := FormatEntityList(nil)
	if !strings.Contains(got, "No matching") {
		t.Fatalf("expected empty message, got: %q", got)
	}
}

func TestFormatSubgraph(t *testing.T) {
	sg := &Subgraph{
		Entities: []*domain.Entity{
			{ID: "controller:pkg.X", Source: domain.Source{File: "x.go", Line: 1}, Description: "Does stuff"},
			{ID: "crd:group.Y", Source: domain.Source{File: "y.go", Line: 2}},
		},
		Relationships: []*domain.Relationship{
			{From: "controller:pkg.X", To: "crd:group.Y", Type: domain.RelReconciles},
		},
	}

	got := FormatSubgraph(sg)
	if !strings.Contains(got, "--reconciles-->") {
		t.Fatalf("missing relationship arrow in: %q", got)
	}
	if !strings.Contains(got, "Does stuff") {
		t.Fatalf("missing description in: %q", got)
	}
}

func TestFormatStats(t *testing.T) {
	s := &GraphStats{
		Commit:        "abc1234",
		Branch:        "feature/review",
		GeneratedAt:   "2026-08-21T06:53:25Z",
		TotalEntities: 100,
		TotalRels:     50,
		EntityCounts:  map[string]int{"controller": 10, "crd": 20},
		RelCounts:     map[string]int{"calls": 30, "reconciles": 5},
	}

	got := FormatStats(s)
	if !strings.Contains(got, "entities: 100") {
		t.Fatalf("missing entity total in: %q", got)
	}
	if !strings.Contains(got, "controller: 10") {
		t.Fatalf("missing controller count in: %q", got)
	}
	if !strings.Contains(got, "relationships: 50") {
		t.Fatalf("missing rel total in: %q", got)
	}
	for _, want := range []string{
		"commit: abc1234",
		"branch: feature/review",
		"generated: 2026-08-21T06:53:25Z",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing graph metadata %q in: %q", want, got)
		}
	}
}

func TestFormatInvestigation(t *testing.T) {
	entity := &domain.Entity{
		ID:      "controller:pkg.MyController",
		Name:    "MyController",
		Kind:    domain.KindController,
		Package: "pkg",
		Source:  domain.Source{File: "pkg/controller.go", Line: 10},
	}
	target := &domain.Entity{
		ID:     "crd:group.HostedCluster",
		Name:   "HostedCluster",
		Kind:   domain.KindCRD,
		Source: domain.Source{File: "api/types.go", Line: 5},
	}
	caller := &domain.Entity{
		ID:     "function:pkg.setup",
		Name:   "setup",
		Kind:   domain.KindFunction,
		Source: domain.Source{File: "pkg/setup.go", Line: 1},
	}
	test := &domain.Entity{
		ID:     "test:pkg.TestMyController",
		Name:   "TestMyController",
		Kind:   domain.KindTest,
		Source: domain.Source{File: "pkg/controller_test.go", Line: 1},
	}

	r := &InvestigateResult{
		Entity: entity,
		OutRels: map[domain.RelationshipType][]ResolvedRel{
			domain.RelReconciles: {{
				Rel:    &domain.Relationship{Type: domain.RelReconciles},
				Target: target,
			}},
		},
		InRels:   map[domain.RelationshipType][]ResolvedRel{},
		Callers:  []*domain.Entity{caller},
		Tests:    []*domain.Entity{test},
		Siblings: []*domain.Entity{},
	}

	got := FormatInvestigation(r)

	checks := []string{
		"=== Entity ===",
		"controller:pkg.MyController",
		"=== Relationships (1 outgoing, 0 incoming) ===",
		"reconciles (1 outgoing)",
		"-> crd:group.HostedCluster",
		"=== Callers (1) ===",
		"function:pkg.setup",
		"=== Tests (1) ===",
		"test:pkg.TestMyController",
		"=== Same File (0 others) ===",
		"(none)",
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("missing %q in:\n%s", check, got)
		}
	}
}

func TestFormatExplanation(t *testing.T) {
	root := &ExplainNode{
		Entity: &domain.Entity{
			ID:          "controller:pkg.X",
			Source:      domain.Source{File: "x.go", Line: 1},
			Description: "Root controller",
		},
		Children: []*ExplainNode{
			{
				Entity: &domain.Entity{
					ID:     "crd:group.Y",
					Source: domain.Source{File: "y.go", Line: 2},
				},
				EdgeType: domain.RelReconciles,
			},
			{
				Entity: &domain.Entity{
					ID:          "function:pkg.doStuff",
					Source:      domain.Source{File: "x.go", Line: 20},
					Description: "Does important work",
				},
				EdgeType: domain.RelCalls,
			},
		},
	}

	r := &ExplainResult{Root: root, TotalNodes: 3, Capped: false}
	got := FormatExplanation(r)

	checks := []string{
		"controller:pkg.X | x.go:1",
		"Root controller",
		"reconciles:",
		"crd:group.Y | y.go:2",
		"calls:",
		"function:pkg.doStuff | x.go:20",
		"3 nodes explored",
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("missing %q in:\n%s", check, got)
		}
	}

	if strings.Contains(got, "capped") {
		t.Error("should not contain 'capped' when not capped")
	}
}

func TestFormatExplanation_NotFound(t *testing.T) {
	r := &ExplainResult{Root: nil}
	got := FormatExplanation(r)
	if !strings.Contains(got, "Entity not found") {
		t.Fatalf("expected 'Entity not found', got: %q", got)
	}
}

func TestFormatImpact(t *testing.T) {
	entity := &domain.Entity{
		ID:          "function:pkg.reconcileEtcd",
		Name:        "reconcileEtcd",
		Kind:        domain.KindFunction,
		Source:      domain.Source{File: "pkg/etcd.go", Line: 20},
		Description: "Handles etcd reconciliation",
	}
	controller := &domain.Entity{
		ID:          "controller:pkg.MyController",
		Name:        "MyController",
		Kind:        domain.KindController,
		Source:      domain.Source{File: "pkg/controller.go", Line: 10},
		Description: "Reconciles HostedCluster resources",
	}
	test := &domain.Entity{
		ID:     "test:pkg.TestReconcileEtcd",
		Name:   "TestReconcileEtcd",
		Kind:   domain.KindTest,
		Source: domain.Source{File: "pkg/etcd_test.go", Line: 1},
	}
	crd := &domain.Entity{
		ID:     "crd:group.HostedCluster",
		Name:   "HostedCluster",
		Kind:   domain.KindCRD,
		Source: domain.Source{File: "api/types.go", Line: 5},
	}

	r := &ImpactResult{
		Entity:    entity,
		CallChain: []*domain.Entity{controller},
		Relationships: []*domain.Relationship{{
			ID:         domain.NewRelationshipID(controller.ID, domain.RelCalls, entity.ID),
			From:       controller.ID,
			To:         entity.ID,
			Type:       domain.RelCalls,
			Confidence: domain.ConfidenceInferred,
			Evidence:   domain.Evidence{Parser: "go-ast", File: "pkg/controller.go", Line: 25, Reason: "call site"},
		}},
		Controllers:   []*domain.Entity{controller},
		Tests:         []*domain.Entity{test},
		Resources:     []*domain.Entity{crd},
		Files:         []string{"pkg/controller.go", "pkg/etcd.go"},
		RecentChanges: []*domain.Entity{},
		Owners:        []string{},
	}

	got := FormatImpact(r)

	checks := []string{
		"=== Impact: function:pkg.reconcileEtcd ===",
		"=== Call Chain (1 callers) ===",
		"controller:pkg.MyController",
		"=== Relationship Evidence (1) ===",
		"controller:pkg.MyController --calls--> function:pkg.reconcileEtcd",
		"=== Controllers (1) ===",
		"=== Tests (1) ===",
		"test:pkg.TestReconcileEtcd",
		"=== Resources (1) ===",
		"crd:group.HostedCluster",
		"=== Files Affected (2) ===",
		"pkg/controller.go",
		"pkg/etcd.go",
	}
	absent := []string{
		"=== Recent Changes",
		"=== Owners",
	}
	for _, check := range checks {
		if !strings.Contains(got, check) {
			t.Errorf("missing %q in:\n%s", check, got)
		}
	}
	for _, a := range absent {
		if strings.Contains(got, a) {
			t.Errorf("should not contain %q when empty, got:\n%s", a, got)
		}
	}
}
