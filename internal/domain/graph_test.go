package domain

import (
	"encoding/json"
	"testing"
)

func TestGraphMarshal(t *testing.T) {
	g := Graph{
		Schema:        "atlas-graph",
		SchemaVersion: "1.0.0",
		GeneratedAt:   "2026-07-16T14:00:00Z",
		Repository:    "github.com/openshift/hypershift",
		Commit:        "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		Branch:        "main",
		ScanDuration:  "1.5s",
		Entities: []Entity{{
			ID:   "controller:hosted-cluster-reconciler",
			Name: "HostedClusterReconciler",
			Kind: KindOperator,
		}},
		Relationship: []Relationship{{
			ID:   "controller:hosted-cluster-reconciler",
			From: "controller:hc",
			To:   "crd:hcp",
		}},
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Graph
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.Repository != g.Repository {
		t.Errorf("Repository = %q, want %q", decoded.Repository, g.Repository)
	}
	if len(decoded.Entities) != len(g.Entities) {
		t.Errorf("Entities length = %d, want %d", len(decoded.Entities), len(g.Entities))
	}
	if decoded.Entities[0].Kind != g.Entities[0].Kind {
		t.Errorf("Entity kind = %v, want %v", decoded.Entities[0].Kind, g.Entities[0].Kind)
	}
}
