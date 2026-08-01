package views

import (
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestCompile_ControllerView(t *testing.T) {
	entities := []domain.Entity{
		{ID: "controller:hc", Name: "HostedClusterReconciler", Kind: domain.KindController,
			Package: "pkg/controllers/hostedcluster", Watches: []string{"HostedCluster", "Secret"},
			Source: domain.Source{File: "hc.go", Line: 10}},
		{ID: "crd:HostedCluster", Name: "HostedCluster", Kind: domain.KindCRD,
			Source: domain.Source{File: "api/hc.go", Line: 1}},
		{ID: "crd:HCP", Name: "HostedControlPlane", Kind: domain.KindCRD,
			Source: domain.Source{File: "api/hcp.go", Line: 1}},
		{ID: "function:reconcileEtcd", Name: "reconcileEtcd", Kind: domain.KindFunction,
			Source: domain.Source{File: "etcd.go", Line: 20}},
		{ID: "test:TestHC", Name: "TestHostedCluster", Kind: domain.KindTest,
			Source: domain.Source{File: "hc_test.go", Line: 5}},
	}

	rels := []domain.Relationship{
		{From: "controller:hc", To: "crd:HostedCluster", Type: domain.RelReconciles},
		{From: "controller:hc", To: "crd:HCP", Type: domain.RelCreates},
		{From: "controller:hc", To: "function:reconcileEtcd", Type: domain.RelCalls},
		{From: "controller:hc", To: "test:TestHC", Type: domain.RelTestedBy},
	}

	views := Compile(entities, rels)

	v, ok := views["controller:hc"]
	if !ok {
		t.Fatal("expected view for controller:hc")
	}

	if v.Reconciles != "HostedCluster" {
		t.Errorf("Reconciles = %q, want HostedCluster", v.Reconciles)
	}
	if len(v.Creates) != 1 || v.Creates[0] != "HostedControlPlane" {
		t.Errorf("Creates = %v, want [HostedControlPlane]", v.Creates)
	}
	if len(v.Calls) != 1 || v.Calls[0] != "reconcileEtcd" {
		t.Errorf("Calls = %v, want [reconcileEtcd]", v.Calls)
	}
	if v.TestCount != 1 {
		t.Errorf("TestCount = %d, want 1", v.TestCount)
	}
	if len(v.Watches) != 2 {
		t.Errorf("Watches = %v, want 2 items", v.Watches)
	}
}

func TestCompile_CRDView(t *testing.T) {
	entities := []domain.Entity{
		{ID: "controller:hc", Name: "HostedClusterReconciler", Kind: domain.KindController,
			Source: domain.Source{File: "hc.go", Line: 10}},
		{ID: "crd:HostedCluster", Name: "HostedCluster", Kind: domain.KindCRD,
			Description: "A hosted cluster", Source: domain.Source{File: "api/hc.go", Line: 1}},
		{ID: "test:TestHC", Name: "TestHC", Kind: domain.KindTest,
			Source: domain.Source{File: "hc_test.go", Line: 5}},
	}

	rels := []domain.Relationship{
		{From: "controller:hc", To: "crd:HostedCluster", Type: domain.RelReconciles},
		{From: "controller:hc", To: "crd:HostedCluster", Type: domain.RelCreates},
		{From: "crd:HostedCluster", To: "test:TestHC", Type: domain.RelTestedBy},
	}

	views := Compile(entities, rels)

	v, ok := views["crd:HostedCluster"]
	if !ok {
		t.Fatal("expected view for crd:HostedCluster")
	}

	if v.ReconciledBy != "HostedClusterReconciler" {
		t.Errorf("ReconciledBy = %q, want HostedClusterReconciler", v.ReconciledBy)
	}
	if len(v.CreatedBy) != 1 || v.CreatedBy[0] != "HostedClusterReconciler" {
		t.Errorf("CreatedBy = %v, want [HostedClusterReconciler]", v.CreatedBy)
	}
	if v.TestCount != 1 {
		t.Errorf("TestCount = %d, want 1", v.TestCount)
	}
}

func TestCompile_SkipsFunctionsAndPackages(t *testing.T) {
	entities := []domain.Entity{
		{ID: "function:foo", Name: "foo", Kind: domain.KindFunction,
			Source: domain.Source{File: "foo.go", Line: 1}},
		{ID: "package:bar", Name: "bar", Kind: domain.KindPackage,
			Source: domain.Source{File: "bar", Line: 0}},
	}

	views := Compile(entities, nil)
	if len(views) != 0 {
		t.Errorf("expected 0 views for non-controller/CRD entities, got %d", len(views))
	}
}

func TestCompileQuestions(t *testing.T) {
	views := map[string]domain.View{
		"controller:hc": {
			EntityName: "HostedClusterReconciler",
			Reconciles: "HostedCluster",
			Creates:    []string{"HCP", "Etcd"},
			Tests:      []string{"TestHC"},
			Owners:     []string{"alice@example.com"},
		},
		"crd:HC": {
			EntityName: "HostedCluster",
			ReconciledBy: "HostedClusterReconciler",
		},
	}

	qa := CompileQuestions(views)

	if qa["reconciles:HostedClusterReconciler"] != "HostedCluster" {
		t.Errorf("reconciles question wrong: %q", qa["reconciles:HostedClusterReconciler"])
	}
	if qa["creates:HostedClusterReconciler"] != "HCP, Etcd" {
		t.Errorf("creates question wrong: %q", qa["creates:HostedClusterReconciler"])
	}
	if qa["reconciled-by:HostedCluster"] != "HostedClusterReconciler" {
		t.Errorf("reconciled-by question wrong: %q", qa["reconciled-by:HostedCluster"])
	}
	if qa["tests:HostedClusterReconciler"] != "TestHC" {
		t.Errorf("tests question wrong: %q", qa["tests:HostedClusterReconciler"])
	}
}
