package graph

import (
	"fmt"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

var validConfidence = map[domain.Confidence]bool{
	domain.ConfidenceProven:   true,
	domain.ConfidenceInferred: true,
}

// ValidateGraph checks graph integrity: duplicate IDs, orphan references,
// evidence contract, and confidence values.
func ValidateGraph(g domain.Graph) error {
	var problems []string

	seen := make(map[string]bool)
	for _, e := range g.Entities {
		if seen[e.ID] {
			problems = append(problems, fmt.Sprintf("duplicate entity ID: %s", e.ID))
		}
		seen[e.ID] = true

		if e.Source.File == "" {
			problems = append(problems, fmt.Sprintf("entity %s has no source file", e.ID))
		}
	}

	relSeen := make(map[string]bool)
	for _, r := range g.Relationship {
		if !seen[r.From] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown From: %s", r.ID, r.From))
		}
		if !seen[r.To] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown To: %s", r.ID, r.To))
		}
		if relSeen[r.ID] {
			problems = append(problems, fmt.Sprintf("duplicate relationship ID: %s", r.ID))
		}
		relSeen[r.ID] = true

		if !validConfidence[r.Confidence] {
			problems = append(problems, fmt.Sprintf("relationship %s has invalid confidence: %q", r.ID, r.Confidence))
		}
		if r.Evidence.File == "" {
			problems = append(problems, fmt.Sprintf("relationship %s has no evidence file", r.ID))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("graph validation failed:\n%s", strings.Join(problems, "\n"))
}
