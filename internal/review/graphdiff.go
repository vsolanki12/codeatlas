package review

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

// BaseEntity is a source-backed entity observed in the base graph and absent
// from the head graph. It describes graph identity changes, not runtime impact.
type BaseEntity struct {
	ID     string        `json:"id"`
	Name   string        `json:"name"`
	Kind   string        `json:"kind"`
	Source domain.Source `json:"source"`
}

type GraphChanges struct {
	BaseGraph            query.GraphMetadata   `json:"baseGraph"`
	RemovedEntities      []BaseEntity          `json:"removedEntities,omitempty"`
	RemovedRelationships []domain.Relationship `json:"removedRelationships,omitempty"`
	Truncated            bool                  `json:"truncated,omitempty"`
}

// RunWithBaseGraph verifies the current checkout through Run and binds the
// historical graph to the merge base used by the three-dot git diff. Historical
// file state is not claimed to have been verified against the head checkout.
func RunWithBaseGraph(base, head, repo, graphPath, baseGraphPath string, expectedBuild ...string) (*ReviewResult, error) {
	headIdx, err := query.LoadGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("load head graph: %w", err)
	}
	result, err := runWithIndex(base, head, repo, graphPath, headIdx)
	if err != nil {
		return nil, err
	}
	if head == "" {
		head = "HEAD"
	}
	commit, err := exec.Command("git", "-C", repo, "merge-base", base, head).Output()
	if err != nil {
		return nil, fmt.Errorf("resolve review merge base: %w", err)
	}
	baseIdx, err := query.LoadGraph(baseGraphPath)
	if err != nil {
		return nil, fmt.Errorf("load base graph: %w", err)
	}
	baseMeta := baseIdx.GraphMetadata()
	baseGraph, headGraph := baseIdx.Graph(), headIdx.Graph()
	if headGraph.SchemaVersion != domain.CurrentSchemaVersion || headGraph.EntityIdentity != domain.CurrentEntityIdentity || !headGraph.ScanComplete || headGraph.ExtractorVersion != domain.CurrentExtractorVersion || headGraph.ExtractorBuild == "" {
		return nil, fmt.Errorf("head graph requires a complete scan with the current schema, entity identity, and extractor")
	}
	if len(expectedBuild) > 0 && expectedBuild[0] != "" && headGraph.ExtractorBuild != expectedBuild[0] {
		return nil, fmt.Errorf("head graph extractor build differs from the current executable; rescan with the current build")
	}
	if baseMeta.Commit != strings.TrimSpace(string(commit)) {
		return nil, fmt.Errorf("base graph must describe review merge base %s (graph=%s)", strings.TrimSpace(string(commit)), baseMeta.Commit)
	}
	if baseMeta.EntityIdentity != domain.CurrentEntityIdentity || baseMeta.SchemaVersion != domain.CurrentSchemaVersion || !baseMeta.ScanComplete || baseGraph.ExtractorVersion != domain.CurrentExtractorVersion || baseGraph.ExtractorBuild == "" {
		return nil, fmt.Errorf("base graph requires a complete scan with current schema and entity identity")
	}
	if existingRepositoryPath(baseMeta.Repository) != existingRepositoryPath(repo) {
		return nil, fmt.Errorf("base graph repository differs from the review checkout")
	}
	if baseGraph.ExtractionSignature == "" || baseGraph.ExtractionSignature != headGraph.ExtractionSignature {
		return nil, fmt.Errorf("base and head graphs require matching extraction signatures; rescan with the same extractor and build options")
	}
	result.GraphChanges = AnalyzeGraphChanges(baseIdx, headIdx, result.ChangedFiles)
	result.Limitations = append(result.Limitations, "Base graph commit matches the review merge base; historical file state was not independently verified. Graph removals describe stored evidence changes, not runtime behavior.")
	return result, nil
}

// AnalyzeGraphChanges restricts absent graph identities to changed paths. Both
// graph compatibility and commit binding are verified by the calling service.
func AnalyzeGraphChanges(baseIdx, headIdx *query.Index, diffs []FileDiff) *GraphChanges {
	changed := make(map[string]bool)
	for _, diff := range diffs {
		changed[diff.Path] = true
		if diff.OldPath != "" {
			changed[diff.OldPath] = true
		}
	}
	result := &GraphChanges{BaseGraph: baseIdx.GraphMetadata()}
	headRels := make(map[string]bool)
	for _, rel := range headIdx.Graph().Relationship {
		headRels[rel.ID] = true
	}
	baseGraph := baseIdx.Graph()
	for _, entity := range baseGraph.Entities {
		if changed[entity.Source.File] && headIdx.GetEntity(entity.ID) == nil {
			result.RemovedEntities = append(result.RemovedEntities, BaseEntity{ID: entity.ID, Name: entity.Name, Kind: entity.Kind.String(), Source: entity.Source})
		}
	}
	for _, rel := range baseGraph.Relationship {
		if changed[rel.Evidence.File] && !headRels[rel.ID] {
			result.RemovedRelationships = append(result.RemovedRelationships, rel)
		}
	}
	sort.Slice(result.RemovedEntities, func(i, j int) bool { return result.RemovedEntities[i].ID < result.RemovedEntities[j].ID })
	sort.Slice(result.RemovedRelationships, func(i, j int) bool { return result.RemovedRelationships[i].ID < result.RemovedRelationships[j].ID })
	return result
}

func compactGraphChanges(result *GraphChanges) *GraphChanges {
	if result == nil {
		return nil
	}
	copy := *result
	var metadataTruncated bool
	copy.BaseGraph, metadataTruncated = compactGraphMetadata(result.BaseGraph)
	copy.Truncated = result.Truncated || metadataTruncated
	copy.RemovedEntities = append([]BaseEntity(nil), result.RemovedEntities...)
	copy.RemovedRelationships = append([]domain.Relationship(nil), result.RemovedRelationships...)
	if len(copy.RemovedEntities) > maxCompactReviewFunctions {
		copy.RemovedEntities = copy.RemovedEntities[:maxCompactReviewFunctions]
		copy.Truncated = true
	}
	if len(copy.RemovedRelationships) > 100 {
		copy.RemovedRelationships = copy.RemovedRelationships[:100]
		copy.Truncated = true
	}
	for i := range copy.RemovedRelationships {
		var truncated bool
		copy.RemovedRelationships[i].Evidence.Snippet, truncated = compactText(copy.RemovedRelationships[i].Evidence.Snippet, maxCompactReviewStringBytes)
		copy.Truncated = copy.Truncated || truncated
		copy.RemovedRelationships[i].Evidence.Reason, truncated = compactText(copy.RemovedRelationships[i].Evidence.Reason, maxCompactReviewStringBytes)
		copy.Truncated = copy.Truncated || truncated
	}
	return &copy
}

func formatGraphChanges(changes *GraphChanges) string {
	if changes == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "\nBase graph evidence (%s)\n", changes.BaseGraph.Commit)
	for _, entity := range changes.RemovedEntities {
		fmt.Fprintf(&b, "- Absent from head graph: %s | %s:%d\n", entity.ID, entity.Source.File, entity.Source.Line)
	}
	for _, rel := range changes.RemovedRelationships {
		fmt.Fprintf(&b, "- Relationship absent from head graph: %s | %s:%d (%s)\n", rel.ID, rel.Evidence.File, rel.Evidence.Line, rel.Confidence)
	}
	if changes.Truncated {
		b.WriteString("Base evidence truncated; omitted facts are not evidence of absence.\n")
	}
	return b.String()
}
