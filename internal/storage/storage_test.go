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
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities: []domain.Entity{
			{
				ID:     "function:auth.Service",
				Name:   "Service",
				Kind:   domain.KindFunction,
				Source: domain.Source{Parser: "go", File: "auth.go", Line: 1},
			},
			{
				ID:     "resource:User.Database",
				Name:   "Database",
				Kind:   domain.KindResource,
				Source: domain.Source{Parser: "yaml", File: "database.yaml", Line: 1},
			},
		},
		Relationship: []domain.Relationship{
			{
				ID:         domain.NewRelationshipID("function:auth.Service", domain.RelCalls, "resource:User.Database"),
				From:       "function:auth.Service",
				To:         "resource:User.Database",
				Type:       domain.RelCalls,
				Confidence: domain.ConfidenceInferred,
				Evidence:   domain.Evidence{Parser: "go-ast", File: "auth.go", Line: 2, Reason: "call detected"},
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
