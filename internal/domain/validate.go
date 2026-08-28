package domain

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

var validRelationshipTypes = map[RelationshipType]bool{
	RelReconciles:   true,
	RelCreates:      true,
	RelOwns:         true,
	RelWatches:      true,
	RelCalls:        true,
	RelTestedBy:     true,
	RelDocumentedIn: true,
	RelDependsOn:    true,
	RelImports:      true,
	RelImplements:   true,
	RelEmits:        true,
	RelContains:     true,
	RelPartOf:       true,
	RelEmbeds:       true,
}

// Validate checks the graph contract before the graph is persisted or
// exposed to a consumer. It deliberately lives in domain so storage, query,
// and the scanner all enforce the same rules without creating package cycles.
func (g Graph) Validate() error {
	// Preserve the useful zero-value behavior for callers constructing an empty
	// graph in memory. Any non-empty graph must be a current CodeAtlas graph.
	if len(g.Entities) == 0 && len(g.Relationship) == 0 && g.Schema == "" && g.SchemaVersion == "" {
		return nil
	}

	var problems []string
	if g.Schema != "codeatlas" {
		problems = append(problems, fmt.Sprintf("unsupported schema: %q", g.Schema))
	}
	if strings.TrimSpace(g.SchemaVersion) == "" {
		problems = append(problems, "missing schema version")
	}
	if g.EntityIdentity != "" && g.EntityIdentity != CurrentEntityIdentity {
		problems = append(problems, fmt.Sprintf("unsupported entity identity: %q", g.EntityIdentity))
	}
	if g.ScanComplete && len(g.ScanWarnings) > 0 {
		problems = append(problems, "complete graph contains scan warnings")
	}

	entityIDs := make(map[string]bool, len(g.Entities))
	for _, e := range g.Entities {
		if strings.TrimSpace(e.ID) == "" {
			problems = append(problems, "entity has empty ID")
		} else if entityIDs[e.ID] {
			problems = append(problems, fmt.Sprintf("duplicate entity ID: %s", e.ID))
		}
		entityIDs[e.ID] = true

		if strings.TrimSpace(e.Name) == "" {
			problems = append(problems, fmt.Sprintf("entity %s has empty name", e.ID))
		}
		if e.Kind < KindOperator || e.Kind > KindUnknown {
			problems = append(problems, fmt.Sprintf("entity %s has invalid kind: %d", e.ID, e.Kind))
		} else if e.Kind == KindUnknown {
			problems = append(problems, fmt.Sprintf("entity %s has unknown kind", e.ID))
		}
		if e.Kind >= KindOperator && e.Kind < KindUnknown {
			expectedPrefix := e.Kind.String() + ":"
			if !strings.HasPrefix(e.ID, expectedPrefix) {
				problems = append(problems, fmt.Sprintf("entity %s does not match kind %s", e.ID, e.Kind))
			}
		}
		if err := validateSource(e.Source, fmt.Sprintf("entity %s", e.ID)); err != nil {
			problems = append(problems, err.Error())
		}
		for _, file := range e.Files {
			if err := validateRelativePath(file, fmt.Sprintf("entity %s file", e.ID)); err != nil {
				problems = append(problems, err.Error())
			}
		}
		sites := append([]Site{}, e.WatchSites...)
		sites = append(sites, e.CreateSites...)
		sites = append(sites, e.CallSites...)
		sites = append(sites, e.ImportSites...)
		sites = append(sites, e.ImplementationSites...)
		sites = append(sites, e.EmbedSites...)
		for _, site := range sites {
			if strings.TrimSpace(site.Name) == "" {
				problems = append(problems, fmt.Sprintf("entity %s has a site with empty name", e.ID))
			}
			if err := validateSource(site.Source, fmt.Sprintf("entity %s site %s", e.ID, site.Name)); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}

	relationIDs := make(map[string]bool, len(g.Relationship))
	relationshipsByID := make(map[string]Relationship, len(g.Relationship))
	for _, r := range g.Relationship {
		if !entityIDs[r.From] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown From: %s", r.ID, r.From))
		}
		if !entityIDs[r.To] {
			problems = append(problems, fmt.Sprintf("relationship %s references unknown To: %s", r.ID, r.To))
		}
		if strings.TrimSpace(r.ID) == "" {
			problems = append(problems, "relationship has empty ID")
		} else if relationIDs[r.ID] {
			problems = append(problems, fmt.Sprintf("duplicate relationship ID: %s", r.ID))
		}
		relationIDs[r.ID] = true
		relationshipsByID[r.ID] = r
		if r.ID != NewRelationshipID(r.From, r.Type, r.To) {
			problems = append(problems, fmt.Sprintf("relationship %s has non-deterministic ID", r.ID))
		}
		if !validRelationshipTypes[r.Type] {
			problems = append(problems, fmt.Sprintf("relationship %s has invalid type: %q", r.ID, r.Type))
		}
		if r.Confidence != ConfidenceProven && r.Confidence != ConfidenceInferred {
			problems = append(problems, fmt.Sprintf("relationship %s has invalid confidence: %q", r.ID, r.Confidence))
		}
		if strings.TrimSpace(r.Evidence.Parser) == "" {
			problems = append(problems, fmt.Sprintf("relationship %s has no evidence parser", r.ID))
		}
		if strings.TrimSpace(r.Evidence.File) == "" {
			problems = append(problems, fmt.Sprintf("relationship %s has no evidence file", r.ID))
		} else if err := validateSource(r.EvidenceSource(), fmt.Sprintf("relationship %s evidence", r.ID)); err != nil {
			problems = append(problems, err.Error())
		}
		if strings.TrimSpace(r.Evidence.Reason) == "" {
			problems = append(problems, fmt.Sprintf("relationship %s has no evidence reason", r.ID))
		}
	}

	viewIDs := make([]string, 0, len(g.Views))
	for id := range g.Views {
		viewIDs = append(viewIDs, id)
	}
	sort.Strings(viewIDs)
	for _, id := range viewIDs {
		view := g.Views[id]
		if id != view.EntityID {
			problems = append(problems, fmt.Sprintf("view key %s does not match entity ID %s", id, view.EntityID))
		}
		if !entityIDs[view.EntityID] {
			problems = append(problems, fmt.Sprintf("view %s references unknown entity: %s", id, view.EntityID))
		} else if entity := findEntity(g.Entities, view.EntityID); entity != nil && view.EntityName != entity.Name {
			problems = append(problems, fmt.Sprintf("view %s has entity name %q, want %q", id, view.EntityName, entity.Name))
		}
		for _, relation := range view.Relationships {
			stored, ok := relationshipsByID[relation.ID]
			if !ok {
				problems = append(problems, fmt.Sprintf("view %s references unknown relationship: %s", id, relation.ID))
				continue
			}
			var expectedEntity string
			switch relation.Direction {
			case "outgoing":
				if stored.From != view.EntityID {
					problems = append(problems, fmt.Sprintf("view %s has invalid outgoing relationship: %s", id, relation.ID))
				}
				expectedEntity = stored.To
			case "incoming":
				if stored.To != view.EntityID {
					problems = append(problems, fmt.Sprintf("view %s has invalid incoming relationship: %s", id, relation.ID))
				}
				expectedEntity = stored.From
			default:
				problems = append(problems, fmt.Sprintf("view %s has invalid relationship direction: %s", id, relation.Direction))
			}
			if expectedEntity != "" && relation.EntityID != expectedEntity {
				problems = append(problems, fmt.Sprintf("view %s relationship %s points to %s, want %s", id, relation.ID, relation.EntityID, expectedEntity))
			}
			if relation.Type != stored.Type || relation.Confidence != stored.Confidence {
				problems = append(problems, fmt.Sprintf("view %s relationship %s does not preserve relationship metadata", id, relation.ID))
			}
			if target := findEntity(g.Entities, relation.EntityID); target != nil && relation.EntityName != target.Name {
				problems = append(problems, fmt.Sprintf("view %s relationship %s has entity name %q, want %q", id, relation.ID, relation.EntityName, target.Name))
			}
			if relation.Evidence != stored.Evidence {
				problems = append(problems, fmt.Sprintf("view %s relationship %s does not preserve relationship evidence", id, relation.ID))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("graph validation failed:\n%s", strings.Join(problems, "\n"))
	}
	return nil
}

func findEntity(entities []Entity, id string) *Entity {
	for i := range entities {
		if entities[i].ID == id {
			return &entities[i]
		}
	}
	return nil
}

// EvidenceSource adapts relationship evidence to the common source checks.
func (r Relationship) EvidenceSource() Source {
	return Source{Parser: r.Evidence.Parser, File: r.Evidence.File, Line: r.Evidence.Line}
}

func validateSource(source Source, owner string) error {
	if strings.TrimSpace(source.File) == "" {
		return fmt.Errorf("%s has no source file", owner)
	}
	if strings.TrimSpace(source.Parser) == "" {
		return fmt.Errorf("%s has no source parser", owner)
	}
	if source.Line <= 0 {
		return fmt.Errorf("%s has invalid source line: %d", owner, source.Line)
	}
	if source.EndLine != 0 && source.EndLine < source.Line {
		return fmt.Errorf("%s has end line before start line: %d < %d", owner, source.EndLine, source.Line)
	}
	return validateRelativePath(source.File, owner)
}

func validateRelativePath(file, owner string) error {
	clean := path.Clean(strings.ReplaceAll(file, "\\", "/"))
	if path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("%s has non-relative source file: %s", owner, file)
	}
	return nil
}
