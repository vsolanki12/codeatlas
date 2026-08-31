package review

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestAnalyzeTestsSeparatesStoredAndInferredLinks(t *testing.T) {
	function := reviewTestEntity("function:example/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	testEntity := reviewTestEntity("test:example/pkg.TestReconcile", "TestReconcile", domain.KindTest, "pkg/reconcile_test.go", 20)
	relationship := domain.Relationship{
		ID:         domain.NewRelationshipID(function.ID, domain.RelTestedBy, testEntity.ID),
		From:       function.ID,
		To:         testEntity.ID,
		Type:       domain.RelTestedBy,
		Confidence: domain.ConfidenceProven,
		Evidence: domain.Evidence{
			Parser: "go",
			File:   "pkg/reconcile_test.go",
			Line:   20,
			Reason: "explicit test reference",
		},
	}
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities:      []domain.Entity{*function, *testEntity},
		Relationship:  []domain.Relationship{relationship},
	}
	idx := queryTestIndex(t, graph)

	assessments := analyzeTests(
		[]EntityReview{{Entity: function, Added: true}},
		[]TestLink{{Test: testEntity, Targets: []*domain.Entity{function}, Confidence: domain.ConfidenceInferred, Reason: "naming convention"}},
		idx,
	)
	if len(assessments) != 1 {
		t.Fatalf("assessments = %+v", assessments)
	}
	assessment := assessments[0]
	if assessment.Status != testStatusStructuralLink || assessment.Coverage != testCoverageUnproven {
		t.Fatalf("assessment status/coverage = %s/%s", assessment.Status, assessment.Coverage)
	}
	if len(assessment.LinkedTests) != 1 || len(assessment.InferredTests) != 0 {
		t.Fatalf("stored/inferred links = %d/%d", len(assessment.LinkedTests), len(assessment.InferredTests))
	}
	if !strings.Contains(assessment.Reason, "cannot prove") {
		t.Fatalf("assessment overclaims coverage: %s", assessment.Reason)
	}
}

func TestAnalyzeTestsDoesNotCallInferredLinkCoverage(t *testing.T) {
	function := reviewTestEntity("function:example/pkg.reconcile", "reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	testEntity := reviewTestEntity("test:example/pkg.TestReconcile", "TestReconcile", domain.KindTest, "pkg/reconcile_test.go", 20)
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities:      []domain.Entity{*function, *testEntity},
	}
	idx := queryTestIndex(t, graph)
	assessments := analyzeTests(
		[]EntityReview{{Entity: function, Added: true}},
		[]TestLink{{Test: testEntity, Targets: []*domain.Entity{function}, Confidence: domain.ConfidenceInferred, Reason: "same file"}},
		idx,
	)
	if len(assessments) != 1 {
		t.Fatalf("assessments = %+v", assessments)
	}
	assessment := assessments[0]
	if assessment.Status != testStatusInsufficient || len(assessment.LinkedTests) != 0 || len(assessment.InferredTests) != 1 {
		t.Fatalf("assessment = %+v", assessment)
	}
	if !strings.Contains(assessment.Reason, "inferred") || !strings.Contains(assessment.Reason, "proves") {
		t.Fatalf("inferred link reason = %q", assessment.Reason)
	}
}
