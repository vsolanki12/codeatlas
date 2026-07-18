package graph

import (
	"path/filepath"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

// RelationshipBuilder analyzes extracted entities and maps architectural dependencies.
type RelationshipBuilder struct {
	RootDir string
}

// NewRelationshipBuilder returns a fresh instance of the builder.
func NewRelationshipBuilder(rootDir string) *RelationshipBuilder {
	return &RelationshipBuilder{RootDir: rootDir}
}

// Build processes a collection of entities and derives structural relationship vectors.
func (b *RelationshipBuilder) Build(entities []domain.Entity) []domain.Relationship {
	// 2. Build an index of entities by name for quick reference lookups
	byName := make(map[string]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindCRD || e.Kind == domain.KindResource {
			byName[e.Name] = e
		}
	}

	var relationships []domain.Relationship

	// 3. Walk controllers, emit relationships
	for _, e := range entities {
		if e.Kind != domain.KindController {
			continue
		}
		if len(e.Watches) == 0 {
			continue
		}

		// First watch = reconciles (index 0 corresponds to .For())
		if target, ok := byName[e.Watches[0]]; ok {
			snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
			relationships = append(relationships, domain.Relationship{
				ID:         domain.NewRelationshipID(e.ID, domain.RelReconciles, target.ID),
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelReconciles,
				Confidence: domain.ConfidenceProven,
				Evidence: domain.Evidence{
					Parser:  "graph",
					File:    e.Source.File,
					Line:    e.Source.Line,
					Snippet: snippet,
					Reason:  "controller has Reconcile() method",
				},
			})
		}

		// Remaining watches = creates (indices 1: correspond to .Owns() / .Watches())
		for _, watchName := range e.Watches[1:] {
			if target, ok := byName[watchName]; ok {
				snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
				relationships = append(relationships, domain.Relationship{
					ID:         domain.NewRelationshipID(e.ID, domain.RelCreates, target.ID),
					From:       e.ID,
					To:         target.ID,
					Type:       domain.RelCreates,
					Confidence: domain.ConfidenceProven,
					Evidence: domain.Evidence{
						Parser:  "graph",
						File:    e.Source.File,
						Line:    e.Source.Line,
						Snippet: snippet,
						Reason:  "controller declares resource ownership chain segment",
					},
				})
			}
		}
	}

	funcByPkgName := make(map[string]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindFunction {
			key := e.Package + "." + e.Name
			funcByPkgName[key] = e
		}
	}

	for _, e := range entities {
		if e.Kind != domain.KindTest {
			continue
		}

		subject := strings.TrimPrefix(e.Name, "Test")
		key := e.Package + "." + subject
		target, ok := funcByPkgName[key]
		if !ok {
			continue
		}

		snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
		relationships = append(relationships, domain.Relationship{
			ID:         domain.NewRelationshipID(target.ID, domain.RelTestedBy, e.ID),
			From:       target.ID,
			To:         e.ID,
			Type:       domain.RelTestedBy,
			Confidence: domain.ConfidenceInferred,
			Evidence: domain.Evidence{
				Parser:  "graph",
				File:    e.Source.File,
				Line:    e.Source.Line,
				Snippet: snippet,
				Reason:  "test function name matches function by convention",
			},
		})
	}
	return relationships
}
