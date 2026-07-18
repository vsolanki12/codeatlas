package graph

import (
	"fmt"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

// ValidateGraph checks a graph for duplicate entityIDs and orphan
// relationship references. Return nil if graph is valid, or an error
// listing all problem found.
func ValidateGraph(g domain.Graph) error {
	var problems []string

	// 1. check for duplicate entity IDs.
	// Build a set of seenIDs. If we  see the same ID twice record it.
	seen := make(map[string]bool)
	for _, e := range g.Entities {
		if seen[e.ID] {
			problems = append(problems, fmt.Sprintf("duplicate entity ID: %s", e.ID))
		}
		seen[e.ID] = true
	}

	// 2. Check that every relationship's From and To point to existing entities.
	// The 'seen' map now contains all entity IDs.
	for _, r := range g.Relationship {
		if !seen[r.From] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown From: %s", r.ID, r.From))
		}
		if !seen[r.To] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown To: %s", r.ID, r.To))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("graph validation failed:\n%s", strings.Join(problems, "\n"))
}
