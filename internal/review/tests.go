package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

const (
	testStatusStructuralLink = "STRUCTURAL_LINK_FOUND"
	testStatusInsufficient   = "INSUFFICIENT_EVIDENCE"
	testCoverageUnproven     = "INSUFFICIENT_EVIDENCE"
	maxTestReferences        = 20
)

// analyzeTests reports graph relationships and clearly labels naming or
// same-file matches as inference. It never turns a relationship into a
// claim about runtime execution, branch coverage, or changed behavior.
func analyzeTests(functions []EntityReview, changedTests []TestLink, idx *query.Index) []TestAssessment {
	if idx == nil {
		return nil
	}

	assessments := make([]TestAssessment, 0, len(functions))
	for _, function := range functions {
		if function.Entity == nil || function.Entity.Kind != domain.KindFunction {
			continue
		}

		assessment := TestAssessment{
			EntityID:   function.Entity.ID,
			EntityName: function.Entity.Name,
			Added:      function.Added,
			Status:     testStatusInsufficient,
			Coverage:   testCoverageUnproven,
		}
		directTestIDs := make(map[string]bool)
		for _, relationship := range idx.GetRelationships(function.Entity.ID, "from", string(domain.RelTestedBy)) {
			test := idx.GetEntity(relationship.To)
			if test == nil {
				continue
			}
			directTestIDs[test.ID] = true
			reason := "CodeAtlas tested_by relationship (stored edge)"
			if relationship.Evidence.Reason != "" {
				reason = relationship.Evidence.Reason
			}
			assessment.LinkedTests = append(assessment.LinkedTests, TestReference{
				Test:         test,
				Relationship: relationship,
				Confidence:   relationship.Confidence,
				Reason:       reason,
			})
		}

		for _, changedTest := range changedTests {
			if changedTest.Test == nil || changedTest.Confidence != domain.ConfidenceInferred || directTestIDs[changedTest.Test.ID] {
				continue
			}
			if !containsEntityID(changedTest.Targets, function.Entity.ID) {
				continue
			}
			assessment.InferredTests = append(assessment.InferredTests, TestReference{
				Test:       changedTest.Test,
				Confidence: domain.ConfidenceInferred,
				Reason:     changedTest.Reason,
			})
		}

		sort.Slice(assessment.LinkedTests, func(i, j int) bool {
			return assessment.LinkedTests[i].Test.ID < assessment.LinkedTests[j].Test.ID
		})
		sort.Slice(assessment.InferredTests, func(i, j int) bool {
			return assessment.InferredTests[i].Test.ID < assessment.InferredTests[j].Test.ID
		})
		if len(assessment.LinkedTests) > maxTestReferences {
			assessment.LinkedTestsTruncated = true
			assessment.LinkedTests = assessment.LinkedTests[:maxTestReferences]
		}
		if len(assessment.InferredTests) > maxTestReferences {
			assessment.InferredTestsTruncated = true
			assessment.InferredTests = assessment.InferredTests[:maxTestReferences]
		}

		switch {
		case len(assessment.LinkedTests) > 0:
			assessment.Status = testStatusStructuralLink
			assessment.Reason = "A stored tested_by relationship links this function to a test, but the graph cannot prove that the changed behavior or any runtime branch is covered."
			if len(assessment.InferredTests) > 0 {
				assessment.Reason += " Additional changed-test links are inferred."
			}
		case len(assessment.InferredTests) > 0:
			assessment.Reason = "A changed test is linked by a naming or same-file heuristic, but no stored tested_by relationship proves that it exercises this function."
		default:
			assessment.Reason = "No stored tested_by relationship establishes a related test; this is not proof that no test exists."
		}
		if function.Added && len(assessment.LinkedTests) == 0 {
			assessment.Reason = "New function has no stored tested_by relationship establishing a related test; this is not proof that no test exists."
			if len(assessment.InferredTests) > 0 {
				assessment.Reason = "New function has only an inferred changed-test link; no stored tested_by relationship proves that the test exercises it."
			}
		}
		if !idx.GraphMetadata().ScanComplete {
			assessment.Reason += " The graph scan is incomplete, so related facts may be absent."
		}

		assessments = append(assessments, assessment)
	}

	sort.Slice(assessments, func(i, j int) bool { return assessments[i].EntityID < assessments[j].EntityID })
	return assessments
}

func containsEntityID(entities []*domain.Entity, id string) bool {
	for _, entity := range entities {
		if entity != nil && entity.ID == id {
			return true
		}
	}
	return false
}

func formatTestAssessmentSummary(assessment TestAssessment) string {
	parts := []string{fmt.Sprintf("%s: %s", assessment.EntityName, assessment.Status)}
	if len(assessment.LinkedTests) > 0 {
		parts = append(parts, fmt.Sprintf("%d stored link(s)", len(assessment.LinkedTests)))
	}
	if len(assessment.InferredTests) > 0 {
		parts = append(parts, fmt.Sprintf("%d inferred link(s)", len(assessment.InferredTests)))
	}
	return strings.Join(parts, " | ")
}
