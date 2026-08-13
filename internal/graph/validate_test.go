package graph

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestValidateGraph_Valid(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.B", Source: domain.Source{File: "b.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "a.go"}},
		},
	}

	if err := ValidateGraph(g); err != nil {
		t.Errorf("expected valid graph, got error: %v", err)
	}
}

func TestValidateGraph_DuplicateEntity(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
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
			{ID: "function:pkg.B", Source: domain.Source{File: "b.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.MISSING", To: "function:pkg.B", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "b.go"}},
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
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.MISSING", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "a.go"}},
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
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.GHOST", To: "function:pkg.A", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "a.go"}},
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

func TestValidateGraph_MissingSourceFile(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A"},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for missing source file, got nil")
	}
	if !strings.Contains(err.Error(), "no source file") {
		t.Errorf("error should mention missing source file: %v", err)
	}
}

func TestValidateGraph_InvalidConfidence(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.B", Source: domain.Source{File: "b.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B", Confidence: "maybe", Evidence: domain.Evidence{File: "a.go"}},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for invalid confidence, got nil")
	}
	if !strings.Contains(err.Error(), "invalid confidence") {
		t.Errorf("error should mention invalid confidence: %v", err)
	}
}

func TestValidateGraph_MissingEvidenceFile(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.B", Source: domain.Source{File: "b.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B", Confidence: domain.ConfidenceInferred},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for missing evidence file, got nil")
	}
	if !strings.Contains(err.Error(), "no evidence file") {
		t.Errorf("error should mention missing evidence: %v", err)
	}
}

func TestValidateGraph_DuplicateRelationship(t *testing.T) {
	g := domain.Graph{
		Entities: []domain.Entity{
			{ID: "function:pkg.A", Source: domain.Source{File: "a.go"}},
			{ID: "function:pkg.B", Source: domain.Source{File: "b.go"}},
		},
		Relationship: []domain.Relationship{
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "a.go"}},
			{ID: "rel1", From: "function:pkg.A", To: "function:pkg.B", Confidence: domain.ConfidenceInferred, Evidence: domain.Evidence{File: "a.go"}},
		},
	}

	err := ValidateGraph(g)
	if err == nil {
		t.Fatal("expected error for duplicate relationship ID, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate relationship ID") {
		t.Errorf("error should mention duplicate relationship: %v", err)
	}
}
