package graph

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
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

	// 6. Verify the Watches relationship metadata mapping (Index 1)
	t.Run("Verify RelWatches Entry", func(t *testing.T) {
		r := rels[1]
		if r.Type != domain.RelWatches {
			t.Errorf("Expected type %s, got %s", domain.RelWatches, r.Type)
		}
		if r.From != "controller:fake.MyController" || r.To != "resource:Secret.my-secret" {
			t.Errorf("Path mapping discrepancy: From=%q To=%q", r.From, r.To)
		}
		if r.Confidence != domain.ConfidenceInferred {
			t.Errorf("Expected inferred confidence, got %s", r.Confidence)
		}
		if r.Evidence.Reason != "controller watches this resource via SetupWithManager" {
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

func TestBuild_FunctionCalls(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "function:helpers.helperA",
			Name:    "helperA",
			Kind:    domain.KindFunction,
			Package: "helpers",
			Calls:   []string{"helperB", "processItem"},
			Source:  domain.Source{Parser: "go", File: "helpers.go", Line: 3},
		},
		{
			ID:      "function:helpers.helperB",
			Name:    "helperB",
			Kind:    domain.KindFunction,
			Package: "helpers",
			Source:  domain.Source{Parser: "go", File: "helpers.go", Line: 8},
		},
		{
			ID:      "function:helpers.processItem",
			Name:    "processItem",
			Kind:    domain.KindFunction,
			Package: "helpers",
			Source:  domain.Source{Parser: "go", File: "helpers.go", Line: 12},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	callRels := 0
	for _, r := range rels {
		if r.Type == domain.RelCalls {
			callRels++
		}
	}
	if callRels != 2 {
		t.Errorf("Expected 2 function call relationships, got %d", callRels)
		for _, r := range rels {
			t.Logf("  %s", r.ID)
		}
	}
}

func TestBuild_SamePackageDisambiguation(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "controller:hostedcluster.HostedClusterReconciler",
			Name:    "HostedClusterReconciler",
			Kind:    domain.KindController,
			Package: "hostedcluster",
			Watches: []string{"HostedCluster"},
			Calls:   []string{"r.reconcile", "r.reconcileLegacy"},
			Source:  domain.Source{Parser: "go", File: "controller.go", Line: 10},
		},
		{
			ID:   "crd:HostedCluster",
			Name: "HostedCluster",
			Kind: domain.KindCRD,
		},
		{
			ID:      "function:hostedcluster.HostedClusterReconciler.reconcile",
			Name:    "reconcile",
			Kind:    domain.KindFunction,
			Package: "hostedcluster",
			Source:  domain.Source{Parser: "go", File: "controller.go", Line: 100},
		},
		{
			ID:      "function:hostedcluster.HostedClusterReconciler.reconcileLegacy",
			Name:    "reconcileLegacy",
			Kind:    domain.KindFunction,
			Package: "hostedcluster",
			Source:  domain.Source{Parser: "go", File: "reconcile_legacy.go", Line: 50},
		},
		{
			ID:      "function:nodepool.NodePoolReconciler.reconcile",
			Name:    "reconcile",
			Kind:    domain.KindFunction,
			Package: "nodepool",
			Source:  domain.Source{Parser: "go", File: "nodepool.go", Line: 200},
		},
		{
			ID:      "function:scheduler.reconcile",
			Name:    "reconcile",
			Kind:    domain.KindFunction,
			Package: "scheduler",
			Source:  domain.Source{Parser: "go", File: "scheduler.go", Line: 300},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	var callTargets []string
	for _, r := range rels {
		if r.Type == domain.RelCalls && r.From == "controller:hostedcluster.HostedClusterReconciler" {
			callTargets = append(callTargets, r.To)
		}
	}

	// reconcileLegacy: unique bare name, should resolve
	found := false
	for _, t := range callTargets {
		if t == "function:hostedcluster.HostedClusterReconciler.reconcileLegacy" {
			found = true
		}
	}
	if !found {
		t.Errorf("missing call to reconcileLegacy, got targets: %v", callTargets)
	}

	// reconcile: ambiguous (3 packages), same-package should resolve to hostedcluster
	found = false
	for _, t := range callTargets {
		if t == "function:hostedcluster.HostedClusterReconciler.reconcile" {
			found = true
		}
	}
	if !found {
		t.Errorf("same-package disambiguation failed for reconcile, got targets: %v", callTargets)
	}

	// Should NOT resolve to nodepool or scheduler
	for _, tgt := range callTargets {
		if strings.Contains(tgt, "nodepool") || strings.Contains(tgt, "scheduler") {
			t.Errorf("wrong package resolved: %s", tgt)
		}
	}
}

func TestBuild_SkipsGenericNames(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:      "function:pkg.doWork",
			Name:    "doWork",
			Kind:    domain.KindFunction,
			Package: "pkg",
			Calls:   []string{"Error", "String", "helperB"},
			Source:  domain.Source{Parser: "go", File: "work.go", Line: 1},
		},
		{
			ID:      "function:pkg.Error",
			Name:    "Error",
			Kind:    domain.KindFunction,
			Package: "pkg",
			Source:  domain.Source{Parser: "go", File: "work.go", Line: 10},
		},
		{
			ID:      "function:pkg.helperB",
			Name:    "helperB",
			Kind:    domain.KindFunction,
			Package: "pkg",
			Source:  domain.Source{Parser: "go", File: "work.go", Line: 15},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	callRels := 0
	for _, r := range rels {
		if r.Type == domain.RelCalls {
			callRels++
			if r.To == "function:pkg.Error" {
				t.Error("Should not create call edge for generic name 'Error'")
			}
		}
	}
	if callRels != 1 {
		t.Errorf("Expected 1 call relationship (helperB only), got %d", callRels)
	}
}

func TestBuild_Embeds(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:     "package:assets",
			Name:   "assets",
			Kind:   domain.KindPackage,
			Embeds: []string{"*/*.yaml"},
			Source: domain.Source{Parser: "go", File: "v2/assets", Line: 1},
		},
		{
			ID:   "resource:service.etcd-client",
			Name: "etcd-client",
			Kind: domain.KindResource,
			Source: domain.Source{Parser: "yaml", File: "v2/assets/etcd/service.yaml", Line: 1},
		},
		{
			ID:   "resource:service.etcd-discovery",
			Name: "etcd-discovery",
			Kind: domain.KindResource,
			Source: domain.Source{Parser: "yaml", File: "v2/assets/etcd/discovery-service.yaml", Line: 1},
		},
		{
			ID:   "resource:deployment.other",
			Name: "other",
			Kind: domain.KindResource,
			Source: domain.Source{Parser: "yaml", File: "somewhere-else/deploy.yaml", Line: 1},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	embedRels := 0
	for _, r := range rels {
		if r.Type == domain.RelEmbeds {
			embedRels++
			if r.From != "package:assets" {
				t.Errorf("Embed from wrong entity: %s", r.From)
			}
		}
	}
	if embedRels != 2 {
		t.Errorf("Expected 2 embeds relationships (etcd-client + etcd-discovery), got %d", embedRels)
		for _, r := range rels {
			t.Logf("  %s --%s--> %s", r.From, r.Type, r.To)
		}
	}
}

func TestBuild_Implements(t *testing.T) {
	entities := []domain.Entity{
		{
			ID:         "function:mycomp.myComponent.IsRequestServing",
			Name:       "IsRequestServing",
			Kind:       domain.KindFunction,
			Package:    "mycomp",
			Implements: []string{"ComponentOptions"},
			Source:     domain.Source{Parser: "go", File: "component.go", Line: 16},
		},
		{
			ID:      "function:mycomp.ComponentOptions",
			Name:    "ComponentOptions",
			Kind:    domain.KindFunction,
			Package: "mycomp",
			Source:  domain.Source{Parser: "go", File: "component.go", Line: 3},
		},
	}

	b := NewRelationshipBuilder("")
	rels := b.Build(entities)

	implRels := 0
	for _, r := range rels {
		if r.Type == domain.RelImplements {
			implRels++
			if r.Evidence.Reason != "var _ assertion detected, target resolved by name match" {
				t.Errorf("Wrong reason: %q", r.Evidence.Reason)
			}
		}
	}
	if implRels != 1 {
		t.Errorf("Expected 1 implements relationship, got %d", implRels)
	}
}
