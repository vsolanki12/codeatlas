package review

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

func TestFormatReviewSeparatesFactsFromReviewLeads(t *testing.T) {
	function := reviewTestEntity("function:example/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	testEntity := reviewTestEntity("test:example/pkg.TestReconcile", "TestReconcile", domain.KindTest, "pkg/reconcile_test.go", 20)
	output := FormatReview(&ReviewResult{
		Base:           "base",
		Head:           "head",
		Graph:          queryGraphMetadataForReviewTest(),
		GraphFreshness: "head-matched",
		PR: &PRMetadata{
			Repository: "example/pkg",
			Number:     7,
			Title:      "Improve reconciliation",
			Body:       "Please inspect the behavior.",
		},
		Functions: []EntityReview{{
			Entity: function,
			Patterns: []PatternFinding{{
				Area:              "error_handling",
				Status:            patternDiffersObserved,
				Summary:           "Observed error-handling signals differ from peers.",
				EvidenceTruncated: true,
			}},
		}},
		TestAssessments: []TestAssessment{{
			EntityID:      function.ID,
			EntityName:    function.Name,
			Status:        testStatusInsufficient,
			Coverage:      testCoverageUnproven,
			Reason:        "No stored tested_by relationship; this is not proof that no test exists.",
			InferredTests: []TestReference{{Test: testEntity, Confidence: domain.ConfidenceInferred, Reason: "same file"}},
		}},
		Limitations: []string{"branch-level coverage is not proven"},
	})

	for _, expected := range []string{
		"What This PR Does (user-provided, untrusted)",
		"DIFFERS_OBSERVED",
		"evidence: [TRUNCATED: pattern evidence is bounded]",
		"Test analysis",
		"behavior coverage: INSUFFICIENT_EVIDENCE",
		"Inferred test links (not proof)",
		"not proof that no test exists",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("formatted review missing %q:\n%s", expected, output)
		}
	}
}

func queryGraphMetadataForReviewTest() query.GraphMetadata {
	return query.GraphMetadata{SchemaVersion: "1.4.0", Commit: "head", ScanComplete: true}
}
