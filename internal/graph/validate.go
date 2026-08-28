package graph

import "github.com/vsolanki12/codeatlas/internal/domain"

// ValidateGraph checks graph integrity: duplicate IDs, orphan references,
// evidence contract, and confidence values.
func ValidateGraph(g domain.Graph) error {
	return g.Validate()
}
