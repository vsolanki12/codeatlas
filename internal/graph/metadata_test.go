package graph

import (
	"testing"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestBuildGraph_Metadata(t *testing.T) {
	entities := []domain.Entity{
		{ID: "function:pkg.Hello", Name: "Hello", Kind: domain.KindFunction},
	}
	rels := []domain.Relationship{
		{ID: "rel1", From: "function:pkg.Hello", To: "test:pkg.TestHello"},
	}

	g := BuildGraph("/tmp/fake-repo", entities, rels, 2*time.Second)

	if g.Schema != "codeatlas" {
		t.Errorf("Schema = %q, want %q", g.Schema, "codeatlas")
	}
	if g.SchemaVersion != "1.3.0" {
		t.Errorf("SchemaVersion = %q, want %q", g.SchemaVersion, "1.3.0")
	}
	if g.GeneratedAt == "" {
		t.Error("GeneratedAt should not be empty")
	}
	if g.Repository != "/tmp/fake-repo" {
		t.Errorf("Repository = %q, want %q", g.Repository, "/tmp/fake-repo")
	}
	if g.ScanDuration != "2s" {
		t.Errorf("ScanDuration = %q, want %q", g.ScanDuration, "2s")
	}
	if len(g.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(g.Entities))
	}
	if len(g.Relationship) != 1 {
		t.Errorf("expected 1 relationship, got %d", len(g.Relationship))
	}
}

func TestGetGitInfo_InvalidDir(t *testing.T) {
	info := GetGitInfo("/tmp/nonexistent-dir-xyz")

	if info.Commit != "" || info.Branch != "" {
		t.Errorf("expected empty git info for invalid dir, got commit=%q branch=%q", info.Commit, info.Branch)
	}
}
