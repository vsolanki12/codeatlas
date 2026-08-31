package graph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/parser"
)

func TestBuildTypedCallRelationshipsResolvesLocalPackageCalls(t *testing.T) {
	repo := t.TempDir()
	writeTypedFixture(t, repo, map[string]string{
		"one/one.go": `package one

import "example.com/repo/two"

func Caller() { Callee(); two.External() }
func Callee() {}
`,
		"two/two.go": `package two

func External() {}
`,
	})

	files := []domain.File{
		{RelativePath: "one/one.go"},
		{RelativePath: "two/two.go"},
	}
	entities := parseTypedFixture(t, repo, files)
	relationships := BuildTypedCallRelationships(repo, files, entities)

	wantID := domain.NewRelationshipID(
		"function:example.com/repo/one.Caller",
		domain.RelCalls,
		"function:example.com/repo/one.Callee",
	)
	for _, relationship := range relationships {
		if relationship.ID != wantID {
			continue
		}
		if relationship.Confidence != domain.ConfidenceProven || relationship.Evidence.Parser != "go-types" || relationship.Evidence.File != "one/one.go" || relationship.Evidence.Line != 5 {
			t.Fatalf("typed relationship = %+v, want proven go-types evidence at one/one.go:5", relationship)
		}
		return
	}
	t.Fatalf("typed relationships did not contain %s: %+v", wantID, relationships)
}

func TestMergeRelationshipsPrefersTypedEvidence(t *testing.T) {
	from := "function:example/pkg.Caller"
	to := "function:example/pkg.Callee"
	id := domain.NewRelationshipID(from, domain.RelCalls, to)
	base := domain.Relationship{
		ID: id, From: from, To: to, Type: domain.RelCalls,
		Confidence: domain.ConfidenceInferred,
		Evidence:   domain.Evidence{Parser: "go-ast", File: "pkg/a.go", Line: 10, Reason: "name match"},
	}
	typed := base
	typed.Confidence = domain.ConfidenceProven
	typed.Evidence = domain.Evidence{Parser: "go-types", File: "pkg/a.go", Line: 10, Reason: "Go type checker resolved the call target"}
	merged := MergeRelationships([]domain.Relationship{base}, []domain.Relationship{typed})
	if len(merged) != 1 || merged[0].Confidence != domain.ConfidenceProven || merged[0].Evidence.Parser != "go-types" {
		t.Fatalf("merged relationships = %+v, want typed evidence", merged)
	}
}

func parseTypedFixture(t *testing.T, repo string, files []domain.File) []domain.Entity {
	t.Helper()
	p := parser.NewGoParserForRepo(repo)
	var entities []domain.Entity
	for _, file := range files {
		parsed, err := p.Parse(file)
		if err != nil {
			t.Fatalf("parse %s: %v", file.RelativePath, err)
		}
		entities = append(entities, parsed...)
	}
	return entities
}

func writeTypedFixture(t *testing.T, repo string, files map[string]string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/repo\n\ngo 1.22\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for relative, content := range files {
		path := filepath.Join(repo, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
