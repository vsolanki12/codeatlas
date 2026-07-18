package graph

import (
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestBuild_ReconcilesAndCreates(t *testing.T) {
	// 4. Set up fake entities by hand
	entities := []domain.Entity{
		{
			ID:      "controller:fake.MyController",
			Name:    "MyController",
			Kind:    domain.KindController,
			Watches: []string{"HostedCluster", "Secret"},
			Source:  domain.Source{Parser: "go", File: "controller.go", Line: 10},
		},
		{
			ID:   "crd:hypershift.HostedCluster",
			Name: "HostedCluster",
			Kind: domain.KindCRD,
		},
		{
			ID:   "resource:Secret.my-secret",
			Name: "Secret",
			Kind: domain.KindResource,
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	// Expect exactly 2 relationships: one reconciles, one creates
	if len(rels) != 2 {
		t.Fatalf("Expected exactly 2 generated relationships, got %d", len(rels))
	}

	// 5. Verify the Reconciles relationship metadata mapping (Index 0)
	t.Run("Verify RelReconciles Entry", func(t *testing.T) {
		r := rels[0]
		if r.Type != domain.RelReconciles {
			t.Errorf("Expected type %s, got %s", domain.RelReconciles, r.Type)
		}
		if r.From != "controller:fake.MyController" || r.To != "crd:hypershift.HostedCluster" {
			t.Errorf("Path mapping discrepancy: From=%q To=%q", r.From, r.To)
		}
		if r.Evidence.Reason != "controller has Reconcile() method" {
			t.Errorf("Unexpected verification evidence text: %q", r.Evidence.Reason)
		}
		if r.Evidence.Line != 10 || r.Evidence.File != "controller.go" {
			t.Errorf("Source coordinate tracing failed, line coordinate: %d", r.Evidence.Line)
		}
	})

	// 6. Verify the Creates relationship metadata mapping (Index 1)
	t.Run("Verify RelCreates Entry", func(t *testing.T) {
		r := rels[1]
		if r.Type != domain.RelCreates {
			t.Errorf("Expected type %s, got %s", domain.RelCreates, r.Type)
		}
		if r.From != "controller:fake.MyController" || r.To != "resource:Secret.my-secret" {
			t.Errorf("Path mapping discrepancy: From=%q To=%q", r.From, r.To)
		}
		if r.Evidence.Reason != "controller declares resource ownership chain segment" {
			t.Errorf("Unexpected verification evidence text: %q", r.Evidence.Reason)
		}
	})
}

func TestBuild_TestedBy(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "function:auth.Login",
			Name:    "Login",
			Kind:    domain.KindFunction,
			Package: "auth",
			Source: domain.Source{
				Parser: "go",
				File:   "auth.go",
				Line:   5,
			},
		},
		{
			ID:      "test:auth.TestLogin",
			Name:    "TestLogin",
			Kind:    domain.KindTest,
			Package: "auth",
			Source: domain.Source{
				Parser: "test",
				File:   "auth_test.go",
				Line:   10,
			},
		},
		{
			ID:      "test:auth.TestSomethingRandom",
			Name:    "TestSomethingRandom",
			Kind:    domain.KindTest,
			Package: "auth",
			Source: domain.Source{
				Parser: "test",
				File:   "auth_test.go",
				Line:   20,
			},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	if len(rels) != 1 {
		t.Fatalf("Expected 1 relationship, got %d", len(rels))
	}

	r := rels[0]
	if r.Type != domain.RelTestedBy {
		t.Errorf("Expected type %s, got %s", domain.RelTestedBy, r.Type)
	}

	if r.From != "function:auth.Login" || r.To != "test:auth.TestLogin" {
		t.Errorf("Wrong endpoints: From=%q To=%q", r.From, r.To)
	}
	if r.Confidence != domain.ConfidenceInferred {
		t.Errorf("Expected Inferred confidence, got %s", r.Confidence)
	}
}

func TestBuild_Deterministic(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "controller:fake.MyController",
			Name:    "MyController",
			Kind:    domain.KindController,
			Watches: []string{"HostedCluster", "Secret"},
			Source:  domain.Source{Parser: "go", File: "controller.go", Line: 10},
		},
		{
			ID:   "crd:hypershift.HostedCluster",
			Name: "HostedCluster",
			Kind: domain.KindCRD,
		},
		{
			ID:   "resource:Secret.my-secret",
			Name: "Secret",
			Kind: domain.KindResource,
		},
		{
			ID:      "function:auth.Login",
			Name:    "Login",
			Kind:    domain.KindFunction,
			Package: "auth",
			Source:  domain.Source{Parser: "go", File: "auth.go", Line: 5},
		},
		{
			ID:      "test:auth.TestLogin",
			Name:    "TestLogin",
			Kind:    domain.KindTest,
			Package: "auth",
			Source:  domain.Source{Parser: "test", File: "auth_test.go", Line: 10},
		},
	}

	b := NewRelationshipBuilder("")
	first := b.Build(entities)
	for i := 1; i < 10; i++ {
		run := b.Build(entities)
		if len(run) != len(first) {
			t.Fatalf("Run %d: got %d relationships, want %d", i, len(run), len(first))
		}
		for j := range first {
			if first[j].ID != run[j].ID {
				t.Errorf("Run %d, rel %d: ID %q != %q", i, j, run[j].ID, first[j].ID)
			}
		}
	}
}

func TestNewRelationshipID(t *testing.T) {
	tests := []struct {
		from    string
		relType domain.RelationshipType
		to      string
		want    string
	}{
		{"controller:pkg.A", domain.RelReconciles, "crd:g.B", "controller:pkg.A--reconciles--crd:g.B"},
		{"controller:pkg.A", domain.RelCreates, "resource:Secret.s", "controller:pkg.A--creates--resource:Secret.s"},
		{"function:pkg.Fn", domain.RelTestedBy, "test:pkg.TestFn", "function:pkg.Fn--tested_by--test:pkg.TestFn"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := domain.NewRelationshipID(tc.from, tc.relType, tc.to)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuild_NoMatchingTarget(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "controller:fake.OrphanController",
			Name:    "OrphanController",
			Kind:    domain.KindController,
			Watches: []string{"NonExistentCRD"},
			Source:  domain.Source{Parser: "go", File: "orphan.go", Line: 1},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	if len(rels) != 0 {
		t.Errorf("Expected 0 relationships for unmatched watches, got %d", len(rels))
	}
}

func TestBuild_EmptyWatches(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:   "controller:fake.EmptyController",
			Name: "EmptyController",
			Kind: domain.KindController,
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	if len(rels) != 0 {
		t.Errorf("Expected 0 relationships for empty watches, got %d", len(rels))
	}
}

func TestBuild_MultipleControllers(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "controller:a.ControllerA",
			Name:    "ControllerA",
			Kind:    domain.KindController,
			Watches: []string{"Foo"},
			Source:  domain.Source{Parser: "go", File: "a.go", Line: 1},
		},
		{
			ID:      "controller:b.ControllerB",
			Name:    "ControllerB",
			Kind:    domain.KindController,
			Watches: []string{"Bar"},
			Source:  domain.Source{Parser: "go", File: "b.go", Line: 1},
		},
		{
			ID:   "crd:group.Foo",
			Name: "Foo",
			Kind: domain.KindCRD,
		},
		{
			ID:   "crd:group.Bar",
			Name: "Bar",
			Kind: domain.KindCRD,
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	if len(rels) != 2 {
		t.Fatalf("Expected 2 relationships, got %d", len(rels))
	}
	if rels[0].From != "controller:a.ControllerA" {
		t.Errorf("First rel from wrong controller: %s", rels[0].From)
	}
	if rels[1].From != "controller:b.ControllerB" {
		t.Errorf("Second rel from wrong controller: %s", rels[1].From)
	}
}

func TestBuild_NoEntities(t *testing.T) {
	b := NewRelationshipBuilder("")
	rels := b.Build(nil)

	if len(rels) != 0 {
		t.Errorf("Expected 0 relationships for nil input, got %d", len(rels))
	}
}
