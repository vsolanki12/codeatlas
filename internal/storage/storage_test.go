package storage

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestRoundTrip(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatalf("failed to create the temp directory: %v", err)
	}

	defer os.RemoveAll(tempDir)
	testFilePath := filepath.Join(tempDir, "graph.json")

	originalGraph := domain.Graph{
		Entities: []domain.Entity{
			{
				ID:   "ent-1",
				Name: "Auth-Service",
			},
			{
				ID:   "ent-2",
				Name: "User Database",
			},
		},
		Relationship: []domain.Relationship{
			{
				ID:   "ent-1--queries--ent-2",
				From: "ent-1",
				To:   "ent-2",
				Type: "queries",
			},
		},
	}

	if err := WriteGraph(testFilePath, originalGraph); err != nil {
		t.Fatalf("WriteGraph failed: %v", err)
	}

	recoveredGraph, err := ReadGraph(testFilePath)

	if err != nil {
		t.Fatalf("Reading graph failed: %v", err)
	}
	if !reflect.DeepEqual(originalGraph, recoveredGraph) {
		t.Errorf("Data mismatch after round-trip!\nExpected: %+v\nGot:      %+v", originalGraph, recoveredGraph)
	}
}
