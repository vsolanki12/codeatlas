package domain

import (
	"encoding/json"
	"strings"
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

func TestGraphRelationshipWireKey(t *testing.T) {
	g := Graph{Schema: "codeatlas", SchemaVersion: "1.4.0", Relationship: []Relationship{{ID: "r"}}}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if string(data) == "" || !containsJSONKey(string(data), `"relationships"`) {
		t.Fatalf("canonical relationships key missing: %s", data)
	}
	if containsJSONKey(string(data), `"relationship"`) {
		t.Fatalf("legacy singular relationships key emitted: %s", data)
	}

	var legacy Graph
	if err := json.Unmarshal([]byte(`{"schema":"codeatlas","schemaVersion":"1.4.0","relationship":[{"id":"legacy"}]}`), &legacy); err != nil {
		t.Fatalf("legacy graph should remain readable: %v", err)
	}
	if len(legacy.Relationship) != 1 || legacy.Relationship[0].ID != "legacy" {
		t.Fatalf("legacy relationships were not loaded: %+v", legacy.Relationship)
	}
}

func TestGraphScanCoverageRoundTrip(t *testing.T) {
	g := Graph{
		Schema:        "codeatlas",
		SchemaVersion: CurrentSchemaVersion,
		ScanFiles: []ScanFile{{Path: "main.go", Status: ScanFileParsed, Parser: "go", EntityCount: 2}, {
			Path: "README.rst", Status: ScanFileIgnored, Reason: "no parser registered for extension .rst",
		}},
		ScanCoverage: &ScanCoverage{Discovered: 2, Parsed: 1, Ignored: 1},
	}
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded Graph
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded.ScanCoverage == nil || *decoded.ScanCoverage != *g.ScanCoverage {
		t.Fatalf("scan coverage = %+v, want %+v", decoded.ScanCoverage, g.ScanCoverage)
	}
	if len(decoded.ScanFiles) != 2 || decoded.ScanFiles[1].Status != ScanFileIgnored {
		t.Fatalf("scan files = %+v, want parsed and ignored entries", decoded.ScanFiles)
	}
}

func containsJSONKey(data, key string) bool {
	return len(data) >= len(key) && strings.Contains(data, key+":")
}
