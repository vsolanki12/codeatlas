package domain

import (
	"testing"
)

func TestRelationshipID(t *testing.T) {
	got := NewRelationshipID(
		"controller:hosted-cluster-reconciler",
		RelReconciles,
		"crd:hypershift.openshift.io/v1beta1.HostedCluster",
	)
	want := "controller:hosted-cluster-reconciler--reconciles--crd:hypershift.openshift.io/v1beta1.HostedCluster"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRelationshipCreation(t *testing.T) {
	r := Relationship{
		ID:         NewRelationshipID("controller:hc", RelCreates, "crd:hcp"),
		From:       "controller:hc",
		To:         "crd:hcp",
		Type:       RelCreates,
		Confidence: ConfidenceProven,
		Evidence: Evidence{
			Parser:  "go-ast",
			File:    "pkg/controllers/hostedcluster/hostedcluster_controller.go",
			Line:    213,
			Snippet: "controllerutil.CreateOrUpdate(ctx, r.Client, hcp, func() error {",
			Reason:  "HostedClusterReconciler calls CreateOrUpdate to create a HostedControlPlane",
		},
	}
	if r.Type != RelCreates {
		t.Errorf("Type = %q, want %q", r.Type, RelCreates)
	}
	if r.Confidence != ConfidenceProven {
		t.Errorf("Confidence = %q, want %q", r.Confidence, ConfidenceProven)
	}

	if r.Evidence.Line != 213 {
		t.Errorf("Evidence.Line = %d, want 213", r.Evidence.Line)
	}
}
