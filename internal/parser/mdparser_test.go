package parser

import (
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestMarkdownParser_Parse(t *testing.T) {
	mp := NewMarkdownParser()
	targetFile := domain.File{
		RelativePath: "testdata/README.md",
	}

	entities, err := mp.Parse(targetFile)
	if err != nil {
		t.Fatalf("MarkdownParser.Parse failed unexpectedly: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected exactly 1 document entity inside slice, got %d", len(entities))
	}

	doc := entities[0]

	// 1. Assert Identity and structural constants match configurations
	expectedID := "document:README.md"
	if doc.ID != expectedID {
		t.Errorf("Document ID mismatch.\nExpected: %q\nGot: %q", expectedID, doc.ID)
	}

	if doc.Kind != domain.KindDocument {
		t.Errorf("Expected KindDocument, got structural value: %v", doc.Kind)
	}

	// 2. Assert text descriptions joined correctly using semicolons
	expectedSummary := "Getting Started; Installation; Usage"
	if doc.Description != expectedSummary {
		t.Errorf("Extracted summary details mismatch.\nExpected: %q\nGot: %q", expectedSummary, doc.Description)
	}

	// 3. Assert content captures first paragraph after title
	expectedContent := "Welcome to the HyperShift Atlas project interface. Run the standard toolchain scripts to compile targets. Execute core scans using CLI argument hooks."
	if doc.Content != expectedContent {
		t.Errorf("Content mismatch.\nExpected: %q\nGot: %q", expectedContent, doc.Content)
	}
}
