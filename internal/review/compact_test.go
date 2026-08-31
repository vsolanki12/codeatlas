package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func TestRunFromDiffTextMapsDiffWithoutTemporaryFile(t *testing.T) {
	graphPath := t.TempDir() + "/graph.json"
	if err := storage.WriteGraph(graphPath, domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  "1.4.0",
		EntityIdentity: domain.CurrentEntityIdentity,
		Entities: []domain.Entity{{
			ID:   "function:example/pkg.Reconcile",
			Name: "Reconcile",
			Kind: domain.KindFunction,
			Source: domain.Source{
				Parser:  "go",
				File:    "pkg/controller.go",
				Line:    10,
				EndLine: 14,
			},
		}},
	}); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	diff := `diff --git a/pkg/controller.go b/pkg/controller.go
index abc..def 100644
--- a/pkg/controller.go
+++ b/pkg/controller.go
@@ -10,3 +10,4 @@ func Reconcile() {
 context
+new behavior
 context
`
	result, err := RunFromDiffText(diff, graphPath, "base", "head")
	if err != nil {
		t.Fatalf("RunFromDiffText returned error: %v", err)
	}
	if result.Base != "base" || result.Head != "head" {
		t.Fatalf("refs = %s/%s, want base/head", result.Base, result.Head)
	}
	if len(result.Functions) != 1 || result.Functions[0].Entity.ID != "function:example/pkg.Reconcile" {
		t.Fatalf("mapped functions = %+v", result.Functions)
	}
	if result.DiffExcerpt != diff {
		t.Fatalf("diff excerpt was not preserved")
	}
}

func TestCompactReviewBoundsNestedEvidence(t *testing.T) {
	entity := reviewTestEntity("function:example/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/controller.go", 10)
	entity.Calls = make([]string, 100)
	for i := range entity.Calls {
		entity.Calls[i] = "function:example/pkg.Callee"
	}
	relationship := &domain.Relationship{
		ID:         "function:example/pkg.Reconcile--calls--function:example/pkg.Callee",
		From:       entity.ID,
		To:         "function:example/pkg.Callee",
		Type:       domain.RelCalls,
		Confidence: domain.ConfidenceProven,
		Evidence: domain.Evidence{
			Parser:  "go",
			File:    "pkg/controller.go",
			Line:    12,
			Snippet: strings.Repeat("snippet", 200),
			Reason:  strings.Repeat("reason", 200),
		},
	}

	functions := make([]EntityReview, maxCompactReviewFunctions+1)
	for i := range functions {
		functions[i] = EntityReview{
			Entity:        entity,
			Relationships: []*domain.Relationship{relationship},
			Callees:       entity.Calls,
		}
	}
	compact := CompactReview(&ReviewResult{
		Graph:       queryGraphMetadataForReviewTest(),
		Functions:   functions,
		DiffExcerpt: strings.Repeat("diff\n", maxDiffExcerptBytes),
		Limitations: []string{"limitation"},
		Heuristics:  []string{"heuristic"},
	}, true)

	if len(compact.Functions) != maxCompactReviewFunctions {
		t.Fatalf("compact functions = %d, want %d", len(compact.Functions), maxCompactReviewFunctions)
	}
	if !compact.Truncated {
		t.Fatal("expected compact review truncation")
	}
	if len(compact.DiffExcerpt) > maxDiffExcerptBytes {
		t.Fatalf("diff excerpt length = %d, want <= %d", len(compact.DiffExcerpt), maxDiffExcerptBytes)
	}
	if got := len(compact.Functions[0].Relationships[0].Evidence.Snippet); got > 600 {
		t.Fatalf("relationship snippet length = %d, want <= 600", got)
	}
	if got := len(compact.Functions[0].Relationships[0].Evidence.Reason); got > 500 {
		t.Fatalf("relationship reason length = %d, want <= 500", got)
	}
	encoded, err := json.Marshal(compact)
	if err != nil {
		t.Fatalf("marshal compact review: %v", err)
	}
	if !strings.Contains(string(encoded), `"truncated":true`) {
		t.Fatalf("encoded compact review omitted truncation marker: %s", encoded[:min(len(encoded), 500)])
	}
}

func TestFormatReviewCompactSeparatesEvidenceAndHeuristics(t *testing.T) {
	entity := reviewTestEntity("function:example/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/controller.go", 10)
	output := FormatReviewCompact(&ReviewResult{
		Base:           "base",
		Head:           "head",
		Graph:          queryGraphMetadataForReviewTest(),
		GraphFreshness: "unverified",
		DiffExcerpt:    "diff --git a/pkg/controller.go b/pkg/controller.go\n",
		Functions: []EntityReview{{
			Entity: entity,
			Relationships: []*domain.Relationship{{
				From:       entity.ID,
				To:         "resource:example/Widget",
				Type:       domain.RelCreates,
				Confidence: domain.ConfidenceProven,
				Evidence:   domain.Evidence{File: "pkg/controller.go", Line: 12, Reason: "explicit create call"},
			}},
		}},
		Limitations: []string{"behavior coverage is not proven"},
		Heuristics:  []string{"test naming fallback"},
	}, false)

	for _, expected := range []string{
		"relationships (evidence)",
		"explicit create call",
		"Evidence limitations",
		"behavior coverage is not proven",
		"Heuristics",
		"test naming fallback",
		"Diff evidence omitted by request",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("compact review missing %q:\n%s", expected, output)
		}
	}
}

func TestCompactReviewBoundsDiffMetadata(t *testing.T) {
	large := strings.Repeat("x", maxCompactReviewStringBytes*2)
	compact := CompactReview(&ReviewResult{
		ChangedFiles: []FileDiff{{
			Path:    large,
			OldPath: large,
			Hunks:   []Hunk{{Header: large}},
		}},
		UnmappedFiles: []string{large},
	}, true)

	if len(compact.ChangedFiles[0].Path) > maxCompactReviewStringBytes || len(compact.ChangedFiles[0].OldPath) > maxCompactReviewStringBytes {
		t.Fatal("compact review did not bound diff paths")
	}
	if len(compact.ChangedFiles[0].Hunks[0].Header) > 512 {
		t.Fatal("compact review did not bound hunk headers")
	}
	if !compact.Truncated {
		t.Fatal("expected diff metadata truncation marker")
	}
}
