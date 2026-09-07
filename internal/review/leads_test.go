package review

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestAnalyzeReviewLeadsAnchorsStateLifecycleAndTestSignals(t *testing.T) {
	entity := reviewTestEntity("function:example/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	function := EntityReview{
		Entity: entity,
		addedLines: []AddedLine{{
			File: "pkg/reconcile.go",
			Line: 42,
			Text: `if state == "deleting" { addFinalizer(obj) }`,
		}},
	}
	assessment := TestAssessment{
		EntityID: entity.ID,
		Reason:   "No stored tested_by relationship establishes a related test; this is not proof that no test exists.",
	}

	leads := analyzeReviewLeads([]EntityReview{function}, []TestAssessment{assessment})
	if len(leads) != 3 {
		t.Fatalf("lead count = %d, want 3: %+v", len(leads), leads)
	}
	seen := make(map[string]ReviewLead)
	for _, lead := range leads {
		seen[lead.Kind] = lead
		if !lead.InDiff || lead.Side != "RIGHT" || lead.File != "pkg/reconcile.go" || lead.Line != 42 {
			t.Errorf("lead anchor = %+v, want right-side pkg/reconcile.go:42 diff anchor", lead)
		}
	}
	for _, kind := range []string{"lifecycle_change", "state_machine_change", "test_evidence"} {
		if _, ok := seen[kind]; !ok {
			t.Errorf("missing %s lead: %+v", kind, leads)
		}
	}
	if seen["test_evidence"].Status != reviewLeadInsufficient {
		t.Errorf("test lead status = %q, want %q", seen["test_evidence"].Status, reviewLeadInsufficient)
	}
}

func TestReviewLeadFormattingAndCompactContract(t *testing.T) {
	lead := ReviewLead{
		Kind:       "state_machine_change",
		Status:     reviewLeadStatus,
		Summary:    "Verify raw and typed state representations.",
		File:       "pkg/reconcile.go",
		Line:       42,
		StartLine:  42,
		EndLine:    42,
		Side:       "RIGHT",
		InDiff:     true,
		Confidence: reviewLeadConfidenceHigh,
		Evidence: []ReviewLeadEvidence{{
			File:   "pkg/reconcile.go",
			Line:   42,
			Kind:   "diff",
			Detail: `state == "deleting"`,
		}},
		SuggestedChecks: []string{"unknown state behavior"},
	}
	result := &ReviewResult{
		ReviewLeads: leadSlice(lead),
		Limitations: []string{"lead is not proof"},
	}

	verbose := FormatReview(result)
	compact := FormatReviewCompact(result, false)
	for _, output := range []string{verbose, compact} {
		for _, expected := range []string{"state_machine_change", "pkg/reconcile.go:42 inline RIGHT", "not confirmed defects", "state ==", "unknown state behavior"} {
			if !strings.Contains(output, expected) {
				t.Errorf("formatted output missing %q:\n%s", expected, output)
			}
		}
	}

	compactResult := CompactReview(result, false)
	encoded, err := json.Marshal(compactResult)
	if err != nil {
		t.Fatalf("marshal compact review: %v", err)
	}
	if !strings.Contains(string(encoded), `"reviewLeads"`) {
		t.Fatalf("compact contract omitted review leads: %s", encoded)
	}
}

func leadSlice(lead ReviewLead) []ReviewLead {
	return []ReviewLead{lead}
}
