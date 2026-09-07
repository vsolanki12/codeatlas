package review

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

const (
	reviewLeadStatus           = "REVIEW_LEAD"
	reviewLeadInsufficient     = "INSUFFICIENT_EVIDENCE"
	reviewLeadConfidenceHigh   = "high"
	reviewLeadConfidenceMedium = "medium"
	reviewLeadConfidenceLow    = "low"
	maxReviewLeads             = 40
	maxReviewLeadEvidence      = 5
)

// ReviewLead is a deterministic prompt for a deeper reviewer. It is not a
// defect finding: consumers must validate the lead against the diff, graph
// evidence, repository conventions, and tests before presenting it as a
// review comment.
type ReviewLead struct {
	Kind            string               `json:"kind"`
	Status          string               `json:"status"`
	Summary         string               `json:"summary"`
	File            string               `json:"file,omitempty"`
	Line            int                  `json:"line,omitempty"`
	StartLine       int                  `json:"startLine,omitempty"`
	EndLine         int                  `json:"endLine,omitempty"`
	Side            string               `json:"side,omitempty"`
	InDiff          bool                 `json:"inDiff"`
	Confidence      string               `json:"confidence"`
	Evidence        []ReviewLeadEvidence `json:"evidence,omitempty"`
	RelatedEntities []string             `json:"relatedEntities,omitempty"`
	TestEntities    []string             `json:"testEntities,omitempty"`
	SuggestedChecks []string             `json:"suggestedChecks,omitempty"`
}

type ReviewLeadEvidence struct {
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Detail string `json:"detail"`
}

var lifecycleLeadTokens = []string{
	"finalizer",
	"deletiontimestamp",
	"requeue",
	"requeueafter",
	"removefinalizer",
	"addfinalizer",
	"status().patch",
	"status().update",
}

func analyzeReviewLeads(functions []EntityReview, assessments []TestAssessment) []ReviewLead {
	assessmentByEntity := make(map[string]TestAssessment, len(assessments))
	for _, assessment := range assessments {
		assessmentByEntity[assessment.EntityID] = assessment
	}

	var leads []ReviewLead
	for _, function := range functions {
		if function.Entity == nil || len(function.addedLines) == 0 {
			continue
		}

		if line, ok := firstAddedLineMatching(function.addedLines, hasLifecycleSignal); ok {
			leads = append(leads, ReviewLead{
				Kind:            "lifecycle_change",
				Status:          reviewLeadStatus,
				Summary:         "Changed code contains lifecycle, retry, or finalizer behavior; verify failure paths and follow-up reconciliation.",
				Confidence:      reviewLeadConfidenceHigh,
				RelatedEntities: []string{function.Entity.ID},
				SuggestedChecks: []string{"reconcile after transient failure", "reconcile after restart", "finalizer cleanup and removal", "requeue and retry behavior"},
				Evidence:        []ReviewLeadEvidence{addedLineEvidence(line, "changed line contains a lifecycle/retry signal")},
				File:            line.File,
				Line:            line.Line,
				StartLine:       line.Line,
				EndLine:         line.Line,
				Side:            "RIGHT",
				InDiff:          true,
			})
		}

		if line, ok := firstAddedLineMatching(function.addedLines, hasStateComparisonSignal); ok {
			leads = append(leads, ReviewLead{
				Kind:            "state_machine_change",
				Status:          reviewLeadStatus,
				Summary:         "State-machine comparison changed; verify wire representations, all valid states, and unknown-state behavior.",
				Confidence:      reviewLeadConfidenceHigh,
				RelatedEntities: []string{function.Entity.ID},
				SuggestedChecks: []string{"raw and typed state representations", "every documented terminal and transient state", "unknown or newly added state behavior", "case and serialization differences"},
				Evidence:        []ReviewLeadEvidence{addedLineEvidence(line, "changed line compares or dispatches on state")},
				File:            line.File,
				Line:            line.Line,
				StartLine:       line.Line,
				EndLine:         line.Line,
				Side:            "RIGHT",
				InDiff:          true,
			})
		}

		for _, pattern := range function.Patterns {
			if pattern.Status != patternDiffersObserved {
				continue
			}
			lead := ReviewLead{
				Kind:            "pattern_deviation",
				Status:          reviewLeadStatus,
				Summary:         fmt.Sprintf("Observed %s differs from comparable repository patterns; verify whether the difference is intentional.", pattern.Area),
				Confidence:      reviewLeadConfidenceMedium,
				Evidence:        patternLeadEvidence(pattern),
				RelatedEntities: []string{function.Entity.ID},
				SuggestedChecks: []string{"compare the changed behavior with the cited peers", "confirm the difference is intentional", "add a focused regression test if the difference is required"},
			}
			setLeadAnchor(&lead, function, pattern)
			leads = append(leads, lead)
		}

		if shouldSuggestTestEvidence(function) {
			assessment, ok := assessmentByEntity[function.Entity.ID]
			if ok && len(assessment.LinkedTests) == 0 && len(assessment.InferredTests) == 0 {
				line, hasLine := firstAddedLine(function.addedLines)
				lead := ReviewLead{
					Kind:            "test_evidence",
					Status:          reviewLeadInsufficient,
					Summary:         "Atlas found no proven or inferred test link for this changed high-impact entity; inspect behavior coverage.",
					Confidence:      reviewLeadConfidenceLow,
					RelatedEntities: []string{function.Entity.ID},
					SuggestedChecks: []string{"exercise the changed success and failure paths", "cover retry, restart, and cleanup behavior when applicable", "verify the assertion observes the changed outcome"},
					Evidence: []ReviewLeadEvidence{{
						File:   function.Entity.Source.File,
						Line:   function.Entity.Source.Line,
						Kind:   "test",
						Detail: assessment.Reason,
					}},
				}
				if hasLine {
					lead.File = line.File
					lead.Line = line.Line
					lead.StartLine = line.Line
					lead.EndLine = line.Line
					lead.Side = "RIGHT"
					lead.InDiff = true
				} else {
					lead.File = function.Entity.Source.File
					lead.Line = function.Entity.Source.Line
				}
				leads = append(leads, lead)
			}
		}

		if len(function.Controllers) >= 3 || len(function.Resources) >= 5 {
			line, hasLine := firstAddedLine(function.addedLines)
			lead := ReviewLead{
				Kind:            "blast_radius",
				Status:          reviewLeadStatus,
				Summary:         "Changed entity has a broad Atlas impact surface; verify ownership, rollout, and partial-failure effects across dependents.",
				Confidence:      reviewLeadConfidenceMedium,
				RelatedEntities: []string{function.Entity.ID},
				SuggestedChecks: []string{"dependent controller behavior", "resource ownership and cleanup", "partial rollout or retry behavior", "backward compatibility for existing resources"},
				Evidence: []ReviewLeadEvidence{{
					File:   function.Entity.Source.File,
					Line:   function.Entity.Source.Line,
					Kind:   "graph",
					Detail: fmt.Sprintf("Atlas impact includes %d controller(s) and %d resource(s).", len(function.Controllers), len(function.Resources)),
				}},
			}
			if hasLine {
				lead.File = line.File
				lead.Line = line.Line
				lead.StartLine = line.Line
				lead.EndLine = line.Line
				lead.Side = "RIGHT"
				lead.InDiff = true
			} else {
				lead.File = function.Entity.Source.File
				lead.Line = function.Entity.Source.Line
			}
			leads = append(leads, lead)
		}
	}

	return normalizeReviewLeads(leads)
}

func hasLifecycleSignal(text string) bool {
	text = strings.ToLower(text)
	for _, token := range lifecycleLeadTokens {
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func hasStateComparisonSignal(text string) bool {
	text = strings.ToLower(text)
	if !strings.Contains(text, "state") {
		return false
	}
	return strings.Contains(text, "==") || strings.Contains(text, "!=") ||
		strings.Contains(text, "switch") || strings.Contains(text, "strings.tolower") ||
		strings.Contains(text, "strings.equalfold") || strings.Contains(text, "equalfold")
}

func shouldSuggestTestEvidence(function EntityReview) bool {
	if function.Entity == nil || function.Entity.Kind != domain.KindFunction {
		return false
	}
	if len(function.Controllers) > 0 || len(function.Resources) > 0 {
		return true
	}
	for _, line := range function.addedLines {
		if hasLifecycleSignal(line.Text) || hasStateComparisonSignal(line.Text) {
			return true
		}
	}
	return false
}

func firstAddedLine(lines []AddedLine) (AddedLine, bool) {
	if len(lines) == 0 {
		return AddedLine{}, false
	}
	return lines[0], true
}

func firstAddedLineMatching(lines []AddedLine, predicate func(string) bool) (AddedLine, bool) {
	for _, line := range lines {
		if predicate(line.Text) {
			return line, true
		}
	}
	return AddedLine{}, false
}

func addedLineEvidence(line AddedLine, detail string) ReviewLeadEvidence {
	return ReviewLeadEvidence{File: line.File, Line: line.Line, Kind: "diff", Detail: detail + ": " + strings.TrimSpace(line.Text)}
}

func patternLeadEvidence(pattern PatternFinding) []ReviewLeadEvidence {
	evidence := make([]ReviewLeadEvidence, 0, len(pattern.Evidence))
	for _, item := range pattern.Evidence {
		evidence = append(evidence, ReviewLeadEvidence{File: item.File, Line: item.Line, Kind: "pattern", Detail: item.Detail})
	}
	return evidence
}

func setLeadAnchor(lead *ReviewLead, function EntityReview, pattern PatternFinding) {
	for _, evidence := range pattern.Evidence {
		for _, line := range function.addedLines {
			if evidence.File != "" && evidence.File == line.File && evidence.Line == line.Line {
				lead.File = line.File
				lead.Line = line.Line
				lead.StartLine = line.Line
				lead.EndLine = line.Line
				lead.Side = "RIGHT"
				lead.InDiff = true
				return
			}
		}
	}
	if line, ok := firstAddedLine(function.addedLines); ok {
		lead.File = line.File
		lead.Line = line.Line
		lead.StartLine = line.Line
		lead.EndLine = line.Line
		lead.Side = "RIGHT"
		lead.InDiff = true
		return
	}
	lead.File = function.Entity.Source.File
	lead.Line = function.Entity.Source.Line
}

func normalizeReviewLeads(leads []ReviewLead) []ReviewLead {
	seen := make(map[string]bool, len(leads))
	result := make([]ReviewLead, 0, len(leads))
	for _, lead := range leads {
		if lead.Kind == "" || lead.Summary == "" {
			continue
		}
		if len(lead.Evidence) > maxReviewLeadEvidence {
			lead.Evidence = append([]ReviewLeadEvidence(nil), lead.Evidence[:maxReviewLeadEvidence]...)
		}
		key := fmt.Sprintf("%s|%s|%d|%s", lead.Kind, lead.File, lead.Line, lead.Summary)
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, lead)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Summary < result[j].Summary
	})
	if len(result) > maxReviewLeads {
		result = result[:maxReviewLeads]
	}
	return result
}

func reviewLeadLocation(lead ReviewLead) string {
	if lead.File == "" {
		return "no source anchor"
	}
	line := lead.Line
	if lead.StartLine > 0 {
		line = lead.StartLine
	}
	location := lead.File
	if line > 0 {
		location = fmt.Sprintf("%s:%d", lead.File, line)
	}
	if line > 0 && lead.EndLine > line {
		location = fmt.Sprintf("%s:%d-%d", lead.File, line, lead.EndLine)
	}
	if lead.InDiff {
		side := lead.Side
		if side == "" {
			side = "RIGHT"
		}
		return location + " inline " + side
	}
	return location + " body context"
}

func reviewLeadEvidenceLabel(evidence ReviewLeadEvidence) string {
	location := ""
	if evidence.File != "" {
		location = evidence.File
		if evidence.Line > 0 {
			location += fmt.Sprintf(":%d", evidence.Line)
		}
	}
	if evidence.Kind != "" {
		if location != "" {
			location += " "
		}
		location += "[" + evidence.Kind + "]"
	}
	if location == "" {
		location = "evidence"
	}
	if evidence.Detail == "" {
		return location
	}
	return location + " — " + evidence.Detail
}
