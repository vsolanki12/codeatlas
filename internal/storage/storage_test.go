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

func TestReadGraphHeaderSkipsFactsAndReadsIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	source := domain.Source{Parser: "go", File: "main.go", Line: 1}
	g := domain.Graph{
		Schema: "codeatlas", SchemaVersion: domain.CurrentSchemaVersion, EntityIdentity: domain.CurrentEntityIdentity,
		Repository: "/repo", RepositoryID: "git:example.com/repo", RepositoryRemote: "example.com/repo",
		GoModule: "example.com/repo", Commit: "abc123", Branch: "main",
		GeneratedAt: "2026-01-01T00:00:00Z",
		Entities: []domain.Entity{
			{ID: "function:p.Run", Name: "Run", Kind: domain.KindFunction, Package: "p", Source: source},
		},
	}
	if err := WriteGraph(path, g); err != nil {
		t.Fatalf("WriteGraph: %v", err)
	}

	header, err := ReadGraphHeader(path)
	if err != nil {
		t.Fatalf("ReadGraphHeader: %v", err)
	}
	if header.RepositoryID != "git:example.com/repo" || header.Repository != "/repo" ||
		header.GoModule != "example.com/repo" || header.Commit != "abc123" ||
		header.Branch != "main" || header.SchemaVersion != domain.CurrentSchemaVersion {
		t.Fatalf("header = %+v", header)
	}
}

func TestReadGraphHeaderRejectsNonObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := os.WriteFile(path, []byte("[1,2,3]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadGraphHeader(path); err == nil {
		t.Fatal("expected an error for a graph file that is not a JSON object")
	}
}
