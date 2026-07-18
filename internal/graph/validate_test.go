package graph

import (
	"strings"
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestValidateGraph_Valid(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A"},
			{ID: "function:pkg.B"},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B"},
		},
	}

	if err := ValidateGraph(g); err != nil {
		t.Errorf("expected valid graph, got error: %v", err)
	}
}

func TestValidateGraph_DuplicateEntity(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A"},
			{ID: "function:pkg.A"},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for duplicate entity ID, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate entity ID") {
		t.Errorf("error should mention duplicate: %v", err)
	}
}

func TestValidateGraph_OrphanFrom(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.B"},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.MISSING", To: "function:pkg.B"},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for orphan From, got nil")
	}
	if !strings.Contains(err.Error(), "unknown From") {
		t.Errorf("error should mention unknown From: %v", err)
	}
}

func TestValidateGraph_OrphanTo(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A"},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.MISSING"},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for orphan To, got nil")
	}
	if !strings.Contains(err.Error(), "unknown To") {
		t.Errorf("error should mention unknown To: %v", err)
	}
}

func TestValidateGraph_EmptyGraph(t *testing.T) {
	g := domain.Graph{}

	if err := ValidateGraph(g); err != nil {
		t.Errorf("empty graph should be valid, got error: %v", err)
	}
}

func TestValidateGraph_MultipleProblems(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A"},
			{ID: "function:pkg.A"},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.GHOST", To: "function:pkg.A"},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for multiple problems, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "duplicate") || !strings.Contains(msg, "unknown From") {
		t.Errorf("error should report both problems: %v", err)
	}
}
