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
	RelReferences:   true,
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
	if g.ExtractionSignature != "" {
		if g.ExtractorVersion == "" || g.BuildContext == nil {
			problems = append(problems, "extraction signature requires extractor version and build context")
		} else if g.ExtractionSignature != ExtractionSignature(g.ExtractorVersion, *g.BuildContext, g.ExtractorBuild) {
			problems = append(problems, "extraction signature does not match extractor version and build context")
		}
	}
	if coverage := g.TypeAnalysis; coverage != nil {
		if coverage.Packages < 0 || coverage.CheckedPackages < 0 || coverage.FailedPackages < 0 || coverage.Files < 0 || coverage.ExcludedFiles < 0 || coverage.ResolvedCalls < 0 || coverage.UnresolvedCalls < 0 || coverage.ResolvedReferences < 0 || coverage.ExternalImports < 0 || coverage.ExternalImportFailures < 0 || coverage.DiagnosticCount < 0 {
			problems = append(problems, "type analysis coverage counts must be non-negative")
		}
		if coverage.CheckedPackages+coverage.FailedPackages > coverage.Packages || coverage.ExcludedFiles > coverage.Files || coverage.ExternalImportFailures > coverage.ExternalImports {
			problems = append(problems, "type analysis coverage counts are inconsistent")
		}
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
		if e.Kind < KindOperator || e.Kind > KindField {
			problems = append(problems, fmt.Sprintf("entity %s has invalid kind: %d", e.ID, e.Kind))
		} else if e.Kind == KindUnknown {
			problems = append(problems, fmt.Sprintf("entity %s has unknown kind", e.ID))
		}
		if e.Kind >= KindOperator && e.Kind <= KindField && e.Kind != KindUnknown {
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
		for _, operation := range e.ResourceOperations {
			if operation.Operation == "" {
				problems = append(problems, fmt.Sprintf("entity %s has resource operation with empty name", e.ID))
			}
			if operation.Confidence != ConfidenceProven && operation.Confidence != ConfidenceInferred {
				problems = append(problems, fmt.Sprintf("entity %s has resource operation with invalid confidence", e.ID))
			}
			if err := validateSource(operation.Source, fmt.Sprintf("entity %s resource operation", e.ID)); err != nil {
				problems = append(problems, err.Error())
			}
		}
		for _, reference := range e.ReferenceSites {
			if strings.TrimSpace(reference.Target) == "" {
				problems = append(problems, fmt.Sprintf("entity %s has a reference site with empty target", e.ID))
			}
			if err := validateSource(reference.Source, fmt.Sprintf("entity %s reference site", e.ID)); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}
	for _, entity := range g.Entities {
		for _, reference := range entity.ReferenceSites {
			if !entityIDs[reference.Target] {
				problems = append(problems, fmt.Sprintf("entity %s references unknown field target: %s", entity.ID, reference.Target))
			}
		}
	}

	if len(g.ScanFiles) > 0 && g.ScanCoverage == nil {
		problems = append(problems, "scan files require scan coverage")
	}
	if g.ScanCoverage != nil {
		coverage := g.ScanCoverage
		if coverage.Discovered < 0 || coverage.Parsed < 0 || coverage.Reused < 0 || coverage.Ignored < 0 || coverage.Failed < 0 {
			problems = append(problems, "scan coverage contains a negative count")
		}
		if coverage.Discovered != len(g.ScanFiles) {
			problems = append(problems, fmt.Sprintf("scan coverage discovered=%d but scan files=%d", coverage.Discovered, len(g.ScanFiles)))
		}
		if coverage.Discovered != coverage.Parsed+coverage.Reused+coverage.Ignored+coverage.Failed {
			problems = append(problems, "scan coverage counts do not add up to discovered files")
		}
		if g.ScanComplete && coverage.Failed > 0 {
			problems = append(problems, "complete graph contains failed scan files")
		}
	}
	scanFiles := make(map[string]bool, len(g.ScanFiles))
	scanStatuses := make(map[string]ScanFileStatus, len(g.ScanFiles))
	scanCounts := make(map[ScanFileStatus]int)
	for _, file := range g.ScanFiles {
		if file.EntityCount < 0 {
			problems = append(problems, fmt.Sprintf("scan file %s has a negative entity count", file.Path))
		}
		if strings.TrimSpace(file.Path) == "" {
			problems = append(problems, "scan file has empty path")
		} else if err := validateRelativePath(file.Path, "scan file"); err != nil {
			problems = append(problems, err.Error())
		}
		if scanFiles[file.Path] {
			problems = append(problems, fmt.Sprintf("duplicate scan file: %s", file.Path))
		}
		scanFiles[file.Path] = true
		scanStatuses[file.Path] = file.Status
		scanCounts[file.Status]++
		switch file.Status {
		case ScanFileParsed, ScanFileReused:
			if strings.TrimSpace(file.Parser) == "" {
				problems = append(problems, fmt.Sprintf("scan file %s has no parser", file.Path))
			}
		case ScanFileIgnored, ScanFileFailed:
			if strings.TrimSpace(file.Reason) == "" {
				problems = append(problems, fmt.Sprintf("scan file %s has no reason", file.Path))
			}
		default:
			problems = append(problems, fmt.Sprintf("scan file %s has invalid status: %q", file.Path, file.Status))
		}
	}
	if g.ScanCoverage != nil {
		coverage := g.ScanCoverage
		for _, status := range []ScanFileStatus{ScanFileParsed, ScanFileReused, ScanFileIgnored, ScanFileFailed} {
			var expected int
			switch status {
			case ScanFileParsed:
				expected = coverage.Parsed
			case ScanFileReused:
				expected = coverage.Reused
			case ScanFileIgnored:
				expected = coverage.Ignored
			case ScanFileFailed:
				expected = coverage.Failed
			}
			if scanCounts[status] != expected {
				problems = append(problems, fmt.Sprintf("scan coverage %s=%d but scan files=%d", status, expected, scanCounts[status]))
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
		if len(g.ScanFiles) > 0 {
			status, ok := scanStatuses[r.Evidence.File]
			if !ok {
				problems = append(problems, fmt.Sprintf("relationship %s evidence file is not in scan inventory: %s", r.ID, r.Evidence.File))
			} else if status != ScanFileParsed && status != ScanFileReused {
				problems = append(problems, fmt.Sprintf("relationship %s evidence file is not parsed: %s (%s)", r.ID, r.Evidence.File, status))
			}
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
