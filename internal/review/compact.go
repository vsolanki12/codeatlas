package review

import (
	"fmt"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

const (
	maxCompactReviewFiles       = 100
	maxCompactReviewFunctions   = 60
	maxCompactReviewTests       = 60
	maxCompactReviewAssessments = 60
	maxCompactReviewUnmapped    = 100
	maxCompactReviewStringBytes = 2048
	maxCompactReviewTextBytes   = 96 * 1024
)

// CompactReviewResult is the bounded machine-facing review contract. It keeps
// entity IDs, source locations, relationship evidence, and review status while
// avoiding repeated full domain entities in every nested section.
type CompactReviewResult struct {
	Base                 string                  `json:"base"`
	Head                 string                  `json:"head"`
	PR                   *CompactPRMetadata      `json:"pr,omitempty"`
	Graph                query.GraphMetadata     `json:"graph"`
	GraphFreshness       string                  `json:"graphFreshness"`
	ChangedFiles         []FileDiff              `json:"changedFiles"`
	Functions            []CompactEntityReview   `json:"functions,omitempty"`
	Tests                []CompactTestLink       `json:"tests,omitempty"`
	TestAssessments      []CompactTestAssessment `json:"testAssessments,omitempty"`
	ReviewLeads          []ReviewLead            `json:"reviewLeads,omitempty"`
	UnmappedFiles        []string                `json:"unmappedFiles,omitempty"`
	Limitations          []string                `json:"limitations"`
	Heuristics           []string                `json:"heuristics"`
	LLMInterpretation    []string                `json:"llmInterpretation"`
	DiffExcerpt          string                  `json:"diffExcerpt,omitempty"`
	DiffExcerptTruncated bool                    `json:"diffExcerptTruncated,omitempty"`
	DiffOmitted          bool                    `json:"diffOmitted,omitempty"`
	Truncated            bool                    `json:"truncated,omitempty"`
}

// CompactPRMetadata keeps PR context bounded and visibly separate from graph
// facts. The body remains untrusted user-provided context.
type CompactPRMetadata struct {
	Repository       string   `json:"repository"`
	Number           int      `json:"number"`
	URL              string   `json:"url,omitempty"`
	Title            string   `json:"title,omitempty"`
	Body             string   `json:"body,omitempty"`
	BodyTruncated    bool     `json:"bodyTruncated,omitempty"`
	Author           string   `json:"author,omitempty"`
	BaseRef          string   `json:"baseRef,omitempty"`
	BaseSHA          string   `json:"baseSHA,omitempty"`
	HeadRef          string   `json:"headRef,omitempty"`
	HeadSHA          string   `json:"headSHA,omitempty"`
	ChangedFileCount int      `json:"changedFileCount,omitempty"`
	Additions        int      `json:"additions,omitempty"`
	Deletions        int      `json:"deletions,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	Files            []PRFile `json:"files,omitempty"`
	FilesTruncated   bool     `json:"filesTruncated,omitempty"`
	Truncated        bool     `json:"truncated,omitempty"`
}

type CompactEntityReview struct {
	Entity                 *query.CompactEntity   `json:"entity"`
	Added                  bool                   `json:"added,omitempty"`
	Approximate            bool                   `json:"approximate"`
	ChangedFiles           []string               `json:"changedFiles,omitempty"`
	Hunks                  []Hunk                 `json:"hunks,omitempty"`
	Relationships          []*domain.Relationship `json:"relationships,omitempty"`
	RelationshipsTruncated bool                   `json:"relationshipsTruncated,omitempty"`
	Callers                []*query.CompactEntity `json:"callers,omitempty"`
	Callees                []string               `json:"callees,omitempty"`
	Controllers            []*query.CompactEntity `json:"controllers,omitempty"`
	Tests                  []*query.CompactEntity `json:"tests,omitempty"`
	Resources              []*query.CompactEntity `json:"resources,omitempty"`
	View                   *query.CompactView     `json:"view,omitempty"`
	Patterns               []PatternFinding       `json:"patterns,omitempty"`
	Truncated              bool                   `json:"truncated,omitempty"`
}

type CompactTestLink struct {
	Test          *query.CompactEntity   `json:"test"`
	Targets       []*query.CompactEntity `json:"targets,omitempty"`
	Relationships []*domain.Relationship `json:"relationships,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
	Confidence    domain.Confidence      `json:"confidence,omitempty"`
	IsAdded       bool                   `json:"added"`
	Truncated     bool                   `json:"truncated,omitempty"`
}

type CompactTestReference struct {
	Test         *query.CompactEntity `json:"test"`
	Relationship *domain.Relationship `json:"relationship,omitempty"`
	Confidence   domain.Confidence    `json:"confidence,omitempty"`
	Reason       string               `json:"reason"`
}

type CompactTestAssessment struct {
	EntityID               string                 `json:"entityID"`
	EntityName             string                 `json:"entityName"`
	Added                  bool                   `json:"added,omitempty"`
	Status                 string                 `json:"status"`
	LinkedTests            []CompactTestReference `json:"linkedTests,omitempty"`
	InferredTests          []CompactTestReference `json:"inferredTests,omitempty"`
	LinkedTestsTruncated   bool                   `json:"linkedTestsTruncated,omitempty"`
	InferredTestsTruncated bool                   `json:"inferredTestsTruncated,omitempty"`
	Coverage               string                 `json:"coverage"`
	Reason                 string                 `json:"reason"`
	Truncated              bool                   `json:"truncated,omitempty"`
}

// CompactReview converts a deterministic review to a bounded representation
// suitable for an MCP response. Relationship evidence is kept in full for the
// retained edges; omitted entries are marked through Truncated.
func CompactReview(result *ReviewResult, includeDiff bool) *CompactReviewResult {
	if result == nil {
		return nil
	}

	base, baseTruncated := compactText(result.Base, 512)
	head, headTruncated := compactText(result.Head, 512)
	graph, graphTruncated := compactGraphMetadata(result.Graph)
	graphFreshness, freshnessTruncated := compactText(result.GraphFreshness, 128)
	limitations, limitationsTruncated := compactMessages(result.Limitations, 24, 800)
	heuristics, heuristicsTruncated := compactMessages(result.Heuristics, 12, 800)
	llmInterpretation, interpretationTruncated := compactMessages(result.LLMInterpretation, 12, 800)
	compact := &CompactReviewResult{
		Base:              base,
		Head:              head,
		Graph:             graph,
		GraphFreshness:    graphFreshness,
		Limitations:       limitations,
		Heuristics:        heuristics,
		LLMInterpretation: llmInterpretation,
	}

	truncated := baseTruncated || headTruncated || graphTruncated || freshnessTruncated || limitationsTruncated || heuristicsTruncated || interpretationTruncated
	var sectionTruncated bool
	compact.PR, sectionTruncated = compactPRMetadata(result.PR)
	truncated = truncated || sectionTruncated
	compact.Truncated = truncated
	compact.ChangedFiles, truncated = compactFileDiffs(result.ChangedFiles, maxCompactReviewFiles, truncated)
	compact.Truncated = truncated
	compact.Functions, truncated = compactEntityReviews(result.Functions, maxCompactReviewFunctions, truncated)
	compact.Truncated = truncated
	compact.Tests, truncated = compactTestLinks(result.Tests, maxCompactReviewTests, truncated)
	compact.Truncated = truncated
	compact.TestAssessments, truncated = compactTestAssessments(result.TestAssessments, maxCompactReviewAssessments, truncated)
	compact.Truncated = truncated
	compact.ReviewLeads, truncated = compactReviewLeads(result.ReviewLeads, maxReviewLeads, truncated)
	compact.Truncated = truncated
	compact.UnmappedFiles, truncated = compactStrings(result.UnmappedFiles, maxCompactReviewUnmapped, truncated)
	compact.Truncated = truncated

	if includeDiff {
		var diffTruncated bool
		compact.DiffExcerpt, diffTruncated = compactText(result.DiffExcerpt, maxDiffExcerptBytes)
		compact.DiffExcerptTruncated = result.DiffExcerptTruncated || diffTruncated
		compact.Truncated = compact.Truncated || diffTruncated
	} else if result.DiffExcerpt != "" {
		compact.DiffOmitted = true
	}
	return compact
}

func compactPRMetadata(metadata *PRMetadata) (*CompactPRMetadata, bool) {
	if metadata == nil {
		return nil, false
	}
	repository, repositoryTruncated := compactText(metadata.Repository, maxCompactReviewStringBytes)
	url, urlTruncated := compactText(metadata.URL, maxCompactReviewStringBytes)
	title, titleTruncated := compactText(metadata.Title, maxCompactReviewStringBytes)
	body, bodyTruncated := compactText(metadata.Body, 4000)
	labels, truncated := compactStrings(metadata.Labels, 20, false)
	files, filesTruncated := compactPRFiles(metadata.Files, 40)
	author, authorTruncated := compactText(metadata.Author, 512)
	baseRef, baseRefTruncated := compactText(metadata.BaseRef, 512)
	baseSHA, baseSHATruncated := compactText(metadata.BaseSHA, 256)
	headRef, headRefTruncated := compactText(metadata.HeadRef, 512)
	headSHA, headSHATruncated := compactText(metadata.HeadSHA, 256)
	truncated = truncated || repositoryTruncated || urlTruncated || titleTruncated || bodyTruncated || authorTruncated || baseRefTruncated || baseSHATruncated || headRefTruncated || headSHATruncated
	return &CompactPRMetadata{
		Repository:       repository,
		Number:           metadata.Number,
		URL:              url,
		Title:            title,
		Body:             body,
		BodyTruncated:    metadata.BodyTruncated || bodyTruncated,
		Author:           author,
		BaseRef:          baseRef,
		BaseSHA:          baseSHA,
		HeadRef:          headRef,
		HeadSHA:          headSHA,
		ChangedFileCount: metadata.ChangedFileCount,
		Additions:        metadata.Additions,
		Deletions:        metadata.Deletions,
		Labels:           labels,
		Files:            files,
		FilesTruncated:   metadata.FilesTruncated || filesTruncated,
		Truncated:        truncated || filesTruncated,
	}, truncated || filesTruncated
}

func compactPRFiles(files []PRFile, max int) ([]PRFile, bool) {
	truncated := len(files) > max
	if truncated {
		files = files[:max]
	}
	result := make([]PRFile, 0, len(files))
	for _, file := range files {
		copyFile := file
		var fieldTruncated bool
		copyFile.Path, fieldTruncated = compactText(copyFile.Path, maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
		copyFile.OldPath, fieldTruncated = compactText(copyFile.OldPath, maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
		copyFile.Status, fieldTruncated = compactText(copyFile.Status, 128)
		truncated = truncated || fieldTruncated
		result = append(result, copyFile)
	}
	return result, truncated
}

func compactFileDiffs(files []FileDiff, max int, truncated bool) ([]FileDiff, bool) {
	if len(files) > max {
		files = files[:max]
		truncated = true
	}
	result := make([]FileDiff, 0, len(files))
	for _, file := range files {
		copyFile := file
		var fieldTruncated bool
		copyFile.Path, fieldTruncated = compactText(copyFile.Path, maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
		copyFile.OldPath, fieldTruncated = compactText(copyFile.OldPath, maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
		copyFile.Hunks, truncated = compactHunks(copyFile.Hunks, 20, truncated)
		result = append(result, copyFile)
	}
	return result, truncated
}

func compactEntityReviews(reviews []EntityReview, max int, truncated bool) ([]CompactEntityReview, bool) {
	if len(reviews) > max {
		reviews = reviews[:max]
		truncated = true
	}
	result := make([]CompactEntityReview, 0, len(reviews))
	for _, review := range reviews {
		compactReview := CompactEntityReview{
			Entity:                 query.CompactEntitySummary(review.Entity),
			Added:                  review.Added,
			Approximate:            review.Approximate,
			RelationshipsTruncated: review.RelationshipsTruncated,
			View:                   query.CompactViewResult(review.View),
		}
		compactReview.ChangedFiles, compactReview.Truncated = compactStrings(review.ChangedFiles, 12, compactReview.Truncated)
		compactReview.Hunks, compactReview.Truncated = compactHunks(review.Hunks, 20, compactReview.Truncated)
		compactReview.Relationships, compactReview.Truncated = compactRelationships(review.Relationships, 24, compactReview.Truncated)
		compactReview.Callers, compactReview.Truncated = compactEntities(review.Callers, 20, compactReview.Truncated)
		compactReview.Callees, compactReview.Truncated = compactStrings(review.Callees, 24, compactReview.Truncated)
		compactReview.Controllers, compactReview.Truncated = compactEntities(review.Controllers, 20, compactReview.Truncated)
		compactReview.Tests, compactReview.Truncated = compactEntities(review.Tests, 20, compactReview.Truncated)
		compactReview.Resources, compactReview.Truncated = compactEntities(review.Resources, 24, compactReview.Truncated)
		compactReview.Patterns, compactReview.Truncated = compactPatterns(review.Patterns, 12, compactReview.Truncated)
		if compactReview.Entity != nil {
			compactReview.Truncated = compactReview.Truncated || compactReview.Entity.Truncated
		}
		truncated = truncated || compactReview.Truncated
		result = append(result, compactReview)
	}
	return result, truncated
}

func compactTestLinks(links []TestLink, max int, truncated bool) ([]CompactTestLink, bool) {
	if len(links) > max {
		links = links[:max]
		truncated = true
	}
	result := make([]CompactTestLink, 0, len(links))
	for _, link := range links {
		compactLink := CompactTestLink{
			Test:       query.CompactEntitySummary(link.Test),
			Confidence: link.Confidence,
			IsAdded:    link.IsAdded,
		}
		compactLink.Reason, compactLink.Truncated = compactText(link.Reason, 800)
		compactLink.Targets, compactLink.Truncated = compactEntities(link.Targets, 12, compactLink.Truncated)
		compactLink.Relationships, compactLink.Truncated = compactRelationships(link.Relationships, 12, compactLink.Truncated)
		truncated = truncated || compactLink.Truncated
		result = append(result, compactLink)
	}
	return result, truncated
}

func compactTestAssessments(assessments []TestAssessment, max int, truncated bool) ([]CompactTestAssessment, bool) {
	if len(assessments) > max {
		assessments = assessments[:max]
		truncated = true
	}
	result := make([]CompactTestAssessment, 0, len(assessments))
	for _, assessment := range assessments {
		compactAssessment := CompactTestAssessment{
			Added:                  assessment.Added,
			LinkedTestsTruncated:   assessment.LinkedTestsTruncated,
			InferredTestsTruncated: assessment.InferredTestsTruncated,
		}
		var fieldTruncated bool
		compactAssessment.EntityID, fieldTruncated = compactText(assessment.EntityID, maxCompactReviewStringBytes)
		compactAssessment.Truncated = compactAssessment.Truncated || fieldTruncated
		compactAssessment.EntityName, fieldTruncated = compactText(assessment.EntityName, maxCompactReviewStringBytes)
		compactAssessment.Truncated = compactAssessment.Truncated || fieldTruncated
		compactAssessment.Status, fieldTruncated = compactText(assessment.Status, 256)
		compactAssessment.Truncated = compactAssessment.Truncated || fieldTruncated
		compactAssessment.Coverage, fieldTruncated = compactText(assessment.Coverage, 512)
		compactAssessment.Truncated = compactAssessment.Truncated || fieldTruncated
		compactAssessment.Reason, fieldTruncated = compactText(assessment.Reason, 800)
		compactAssessment.Truncated = compactAssessment.Truncated || fieldTruncated
		compactAssessment.LinkedTests, compactAssessment.Truncated = compactTestReferences(assessment.LinkedTests, 12, compactAssessment.Truncated)
		compactAssessment.InferredTests, compactAssessment.Truncated = compactTestReferences(assessment.InferredTests, 12, compactAssessment.Truncated)
		truncated = truncated || compactAssessment.Truncated
		result = append(result, compactAssessment)
	}
	return result, truncated
}

func compactTestReferences(references []TestReference, max int, truncated bool) ([]CompactTestReference, bool) {
	if len(references) > max {
		references = references[:max]
		truncated = true
	}
	result := make([]CompactTestReference, 0, len(references))
	for _, reference := range references {
		relationship, relationshipTruncated := compactRelationshipWithStatus(reference.Relationship)
		compactReference := CompactTestReference{
			Test:         query.CompactEntitySummary(reference.Test),
			Relationship: relationship,
			Confidence:   reference.Confidence,
		}
		var fieldTruncated bool
		compactReference.Reason, fieldTruncated = compactText(reference.Reason, 800)
		truncated = truncated || relationshipTruncated || fieldTruncated
		result = append(result, compactReference)
	}
	return result, truncated
}

func compactReviewLeads(leads []ReviewLead, max int, truncated bool) ([]ReviewLead, bool) {
	if len(leads) > max {
		leads = leads[:max]
		truncated = true
	}
	result := make([]ReviewLead, 0, len(leads))
	for _, lead := range leads {
		copyLead := lead
		var fieldTruncated bool
		copyLead.Kind, fieldTruncated = compactText(copyLead.Kind, 128)
		truncated = truncated || fieldTruncated
		copyLead.Status, fieldTruncated = compactText(copyLead.Status, 128)
		truncated = truncated || fieldTruncated
		copyLead.Summary, fieldTruncated = compactText(copyLead.Summary, 1000)
		truncated = truncated || fieldTruncated
		copyLead.File, fieldTruncated = compactText(copyLead.File, maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
		copyLead.Confidence, fieldTruncated = compactText(copyLead.Confidence, 128)
		truncated = truncated || fieldTruncated
		copyLead.RelatedEntities, truncated = compactStrings(copyLead.RelatedEntities, 12, truncated)
		copyLead.TestEntities, truncated = compactStrings(copyLead.TestEntities, 12, truncated)
		copyLead.SuggestedChecks, truncated = compactStrings(copyLead.SuggestedChecks, 12, truncated)
		if len(copyLead.Evidence) > maxReviewLeadEvidence {
			copyLead.Evidence = append([]ReviewLeadEvidence(nil), copyLead.Evidence[:maxReviewLeadEvidence]...)
			truncated = true
		}
		for i := range copyLead.Evidence {
			copyLead.Evidence[i].File, fieldTruncated = compactText(copyLead.Evidence[i].File, maxCompactReviewStringBytes)
			truncated = truncated || fieldTruncated
			copyLead.Evidence[i].Kind, fieldTruncated = compactText(copyLead.Evidence[i].Kind, 128)
			truncated = truncated || fieldTruncated
			copyLead.Evidence[i].Detail, fieldTruncated = compactText(copyLead.Evidence[i].Detail, 800)
			truncated = truncated || fieldTruncated
		}
		result = append(result, copyLead)
	}
	return result, truncated
}

func compactEntities(entities []*domain.Entity, max int, truncated bool) ([]*query.CompactEntity, bool) {
	if len(entities) > max {
		entities = entities[:max]
		truncated = true
	}
	result := query.CompactEntities(entities)
	for _, entity := range result {
		truncated = truncated || entity.Truncated
	}
	return result, truncated
}

func compactRelationships(relationships []*domain.Relationship, max int, truncated bool) ([]*domain.Relationship, bool) {
	if len(relationships) > max {
		relationships = relationships[:max]
		truncated = true
	}
	result := make([]*domain.Relationship, 0, len(relationships))
	for _, relationship := range relationships {
		compact, relationshipTruncated := compactRelationshipWithStatus(relationship)
		truncated = truncated || relationshipTruncated
		result = append(result, compact)
	}
	return result, truncated
}

func compactRelationship(relationship *domain.Relationship) *domain.Relationship {
	compact, _ := compactRelationshipWithStatus(relationship)
	return compact
}

func compactRelationshipWithStatus(relationship *domain.Relationship) (*domain.Relationship, bool) {
	if relationship == nil {
		return nil, false
	}
	copyRelationship := *relationship
	var truncated bool
	var fieldTruncated bool
	copyRelationship.Evidence.Parser, fieldTruncated = compactText(copyRelationship.Evidence.Parser, 128)
	truncated = truncated || fieldTruncated
	copyRelationship.Evidence.File, fieldTruncated = compactText(copyRelationship.Evidence.File, maxCompactReviewStringBytes)
	truncated = truncated || fieldTruncated
	copyRelationship.Evidence.Snippet, fieldTruncated = compactText(copyRelationship.Evidence.Snippet, 600)
	truncated = truncated || fieldTruncated
	copyRelationship.Evidence.Reason, fieldTruncated = compactText(copyRelationship.Evidence.Reason, 500)
	truncated = truncated || fieldTruncated
	return &copyRelationship, truncated
}

func compactHunks(hunks []Hunk, max int, truncated bool) ([]Hunk, bool) {
	if len(hunks) > max {
		hunks = hunks[:max]
		truncated = true
	}
	result := make([]Hunk, 0, len(hunks))
	for _, hunk := range hunks {
		copyHunk := hunk
		var fieldTruncated bool
		copyHunk.Header, fieldTruncated = compactText(copyHunk.Header, 512)
		truncated = truncated || fieldTruncated
		result = append(result, copyHunk)
	}
	return result, truncated
}

func compactPatterns(patterns []PatternFinding, max int, truncated bool) ([]PatternFinding, bool) {
	if len(patterns) > max {
		patterns = patterns[:max]
		truncated = true
	}
	result := make([]PatternFinding, 0, len(patterns))
	for _, pattern := range patterns {
		copyPattern := pattern
		var summaryTruncated bool
		copyPattern.Area, summaryTruncated = compactText(copyPattern.Area, 256)
		truncated = truncated || summaryTruncated
		copyPattern.Status, summaryTruncated = compactText(copyPattern.Status, 256)
		truncated = truncated || summaryTruncated
		copyPattern.Summary, summaryTruncated = compactText(copyPattern.Summary, 600)
		truncated = truncated || summaryTruncated
		if len(copyPattern.Evidence) > 8 {
			copyPattern.Evidence = append([]PatternEvidence(nil), copyPattern.Evidence[:8]...)
			copyPattern.EvidenceTruncated = true
			truncated = true
		} else {
			copyPattern.Evidence = append([]PatternEvidence(nil), copyPattern.Evidence...)
		}
		for i := range copyPattern.Evidence {
			copyPattern.Evidence[i].EntityID, summaryTruncated = compactText(copyPattern.Evidence[i].EntityID, maxCompactReviewStringBytes)
			truncated = truncated || summaryTruncated
			copyPattern.Evidence[i].File, summaryTruncated = compactText(copyPattern.Evidence[i].File, maxCompactReviewStringBytes)
			truncated = truncated || summaryTruncated
			copyPattern.Evidence[i].Detail, summaryTruncated = compactText(copyPattern.Evidence[i].Detail, 500)
			truncated = truncated || summaryTruncated
		}
		result = append(result, copyPattern)
	}
	return result, truncated
}

func compactStrings(values []string, max int, truncated bool) ([]string, bool) {
	if len(values) > max {
		values = values[:max]
		truncated = true
	}
	result := append([]string(nil), values...)
	for i := range result {
		var fieldTruncated bool
		result[i], fieldTruncated = compactText(result[i], maxCompactReviewStringBytes)
		truncated = truncated || fieldTruncated
	}
	return result, truncated
}

func compactMessages(values []string, max, maxBytes int) ([]string, bool) {
	values, truncated := compactStrings(values, max, false)
	for i := range values {
		var fieldTruncated bool
		values[i], fieldTruncated = compactText(values[i], maxBytes)
		truncated = truncated || fieldTruncated
	}
	return values, truncated
}

func compactGraphMetadata(metadata query.GraphMetadata) (query.GraphMetadata, bool) {
	result := metadata
	var truncated bool
	var fieldTruncated bool
	result.SchemaVersion, fieldTruncated = compactText(result.SchemaVersion, 128)
	truncated = truncated || fieldTruncated
	result.EntityIdentity, fieldTruncated = compactText(result.EntityIdentity, 256)
	truncated = truncated || fieldTruncated
	result.Repository, fieldTruncated = compactText(result.Repository, maxCompactReviewStringBytes)
	truncated = truncated || fieldTruncated
	result.Commit, fieldTruncated = compactText(result.Commit, 256)
	truncated = truncated || fieldTruncated
	result.Branch, fieldTruncated = compactText(result.Branch, 512)
	truncated = truncated || fieldTruncated
	result.GeneratedAt, fieldTruncated = compactText(result.GeneratedAt, 128)
	truncated = truncated || fieldTruncated
	result.ScanWarnings, fieldTruncated = compactMessages(result.ScanWarnings, 12, 800)
	truncated = truncated || fieldTruncated
	return result, truncated
}

func compactText(value string, max int) (string, bool) {
	if len(value) <= max {
		return value, false
	}
	const marker = "... [TRUNCATED]"
	limit := max - len(marker)
	if limit <= 0 {
		return marker, true
	}
	return value[:limit] + marker, true
}

// FormatReviewCompact is the default MCP text representation. It makes the
// evidence/heuristic boundary visible without repeating every full entity.
func FormatReviewCompact(result *ReviewResult, includeDiff bool) string {
	compact := CompactReview(result, includeDiff)
	if compact == nil {
		return "No review result.\n"
	}

	var b strings.Builder
	b.WriteString("CODEATLAS PR REVIEW (COMPACT)\n")
	b.WriteString("=============================\n")
	fmt.Fprintf(&b, "Base: %s | Head: %s\n", compact.Base, compact.Head)
	fmt.Fprintf(&b, "Graph: %s (%s, %s)\n", compact.Graph.Commit, compact.GraphFreshness, scanStatus(compact.Graph.ScanComplete))
	if compact.Graph.Repository != "" {
		fmt.Fprintf(&b, "Graph repository: %s\n", compact.Graph.Repository)
	}

	if compact.PR != nil {
		b.WriteString("\nPull request metadata (user-provided, untrusted)\n")
		fmt.Fprintf(&b, "Repository: %s | #%d\n", compact.PR.Repository, compact.PR.Number)
		if compact.PR.Title != "" {
			fmt.Fprintf(&b, "Title: %s\n", compact.PR.Title)
		}
		if compact.PR.Author != "" {
			fmt.Fprintf(&b, "Author: %s\n", compact.PR.Author)
		}
		if compact.PR.BaseRef != "" || compact.PR.HeadRef != "" {
			fmt.Fprintf(&b, "Refs: %s (%s) -> %s (%s)\n", compact.PR.BaseRef, shortSHA(compact.PR.BaseSHA), compact.PR.HeadRef, shortSHA(compact.PR.HeadSHA))
		}
		if compact.PR.ChangedFileCount > 0 || compact.PR.Additions > 0 || compact.PR.Deletions > 0 {
			fmt.Fprintf(&b, "GitHub totals: %d files, +%d/-%d\n", compact.PR.ChangedFileCount, compact.PR.Additions, compact.PR.Deletions)
		}
		if compact.PR.Body != "" {
			b.WriteString("PR description (untrusted): ")
			b.WriteString(compact.PR.Body)
			b.WriteByte('\n')
		}
		if compact.PR.BodyTruncated || compact.PR.FilesTruncated {
			b.WriteString("PR metadata is truncated; omitted text/files are not evidence of absence.\n")
		}
	}

	b.WriteString("\nChanged files\n")
	for _, file := range compact.ChangedFiles {
		fmt.Fprintf(&b, "- %s [%s] +%d/-%d\n", file.Path, file.Status, file.AddedLines, file.DeletedLines)
	}
	if len(compact.ChangedFiles) == 0 {
		b.WriteString("- none mapped from supplied diff\n")
	}

	if compact.DiffExcerpt != "" {
		b.WriteString("\nChanged diff evidence (bounded)\n```diff\n")
		b.WriteString(compact.DiffExcerpt)
		if !strings.HasSuffix(compact.DiffExcerpt, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
		if compact.DiffExcerptTruncated {
			b.WriteString("Diff excerpt truncated; omitted changed lines are not available for exact-line review.\n")
		}
	} else if compact.DiffOmitted {
		b.WriteString("\nDiff evidence omitted by request; file/hunk mapping remains available.\n")
	}

	if len(compact.Functions) > 0 {
		b.WriteString("\nChanged entities and graph evidence\n")
		for _, function := range compact.Functions {
			b.WriteString("- ")
			b.WriteString(compactEntityLabel(function.Entity))
			if function.Added {
				b.WriteString(" [ADDED]")
			}
			if function.Approximate {
				b.WriteString(" [APPROXIMATE MAPPING]")
			}
			b.WriteByte('\n')
			if len(function.Hunks) > 0 {
				fmt.Fprintf(&b, "  hunks: %s\n", formatHunks(function.Hunks))
			}
			if len(function.Callers) > 0 {
				fmt.Fprintf(&b, "  callers: %s\n", formatEntities(function.Callers))
			}
			if len(function.Callees) > 0 {
				fmt.Fprintf(&b, "  callees: %s\n", strings.Join(function.Callees, ", "))
			}
			if len(function.Relationships) > 0 {
				b.WriteString("  relationships (evidence):\n")
				for _, relationship := range function.Relationships {
					fmt.Fprintf(&b, "    - %s -> %s [%s] evidence=%s:%d %s\n", relationship.From, relationship.To, relationship.Confidence, relationship.Evidence.File, relationship.Evidence.Line, relationship.Evidence.Reason)
				}
			}
			if len(function.Controllers) > 0 || len(function.Resources) > 0 {
				fmt.Fprintf(&b, "  blast radius: controllers=%s resources=%s\n", formatEntities(function.Controllers), formatEntities(function.Resources))
			}
			if len(function.Tests) > 0 {
				fmt.Fprintf(&b, "  graph-linked tests: %s\n", formatEntities(function.Tests))
			}
			for _, pattern := range function.Patterns {
				fmt.Fprintf(&b, "  pattern observation [%s] %s: %s\n", pattern.Status, pattern.Area, pattern.Summary)
				for _, evidence := range pattern.Evidence {
					fmt.Fprintf(&b, "    evidence: %s:%d (%s)\n", evidence.File, evidence.Line, evidence.Detail)
				}
			}
			if function.RelationshipsTruncated || function.Truncated {
				b.WriteString("  [TRUNCATED: omitted entity context is not evidence of absence]\n")
			}
		}
	} else {
		b.WriteString("\nChanged entities and graph evidence\n- none mapped\n")
	}

	if len(compact.ReviewLeads) > 0 {
		b.WriteString("\nDeterministic review leads\n")
		b.WriteString("These are evidence-backed prompts, not confirmed defects.\n")
		for _, lead := range compact.ReviewLeads {
			fmt.Fprintf(&b, "- [%s] %s (%s): %s\n", lead.Status, lead.Kind, reviewLeadLocation(lead), lead.Summary)
			for _, evidence := range lead.Evidence {
				fmt.Fprintf(&b, "  evidence: %s\n", reviewLeadEvidenceLabel(evidence))
			}
			if len(lead.SuggestedChecks) > 0 {
				fmt.Fprintf(&b, "  checks: %s\n", strings.Join(lead.SuggestedChecks, "; "))
			}
		}
	}

	if len(compact.Tests) > 0 || len(compact.TestAssessments) > 0 {
		b.WriteString("\nTest evidence\n")
		for _, test := range compact.Tests {
			fmt.Fprintf(&b, "- %s test: %s\n", addedStatus(test.IsAdded), compactEntityLabel(test.Test))
			if len(test.Targets) > 0 {
				fmt.Fprintf(&b, "  targets: %s [%s, %s]\n", formatEntities(test.Targets), test.Confidence, test.Reason)
			} else {
				b.WriteString("  targets: unknown\n")
			}
		}
		for _, assessment := range compact.TestAssessments {
			fmt.Fprintf(&b, "- %s | behavior coverage: %s | %s\n", assessment.EntityID, assessment.Coverage, assessment.Reason)
			if len(assessment.LinkedTests) > 0 {
				b.WriteString("  stored links: ")
				b.WriteString(formatTestReferences(assessment.LinkedTests))
				b.WriteByte('\n')
			}
			if len(assessment.InferredTests) > 0 {
				b.WriteString("  inferred links (not proof): ")
				b.WriteString(formatTestReferences(assessment.InferredTests))
				b.WriteByte('\n')
			}
		}
	}

	if len(compact.UnmappedFiles) > 0 {
		fmt.Fprintf(&b, "\nUnmapped files: %s\n", strings.Join(compact.UnmappedFiles, ", "))
	}

	b.WriteString("\nEvidence limitations\n")
	for _, limitation := range compact.Limitations {
		fmt.Fprintf(&b, "- %s\n", limitation)
	}
	b.WriteString("Heuristics\n")
	for _, heuristic := range compact.Heuristics {
		fmt.Fprintf(&b, "- %s\n", heuristic)
	}
	b.WriteString("LLM interpretation\n")
	if len(compact.LLMInterpretation) == 0 {
		b.WriteString("- none; this deterministic review does not invoke an LLM\n")
	} else {
		for _, interpretation := range compact.LLMInterpretation {
			fmt.Fprintf(&b, "- %s\n", interpretation)
		}
	}
	if compact.Truncated {
		b.WriteString("[TRUNCATED: one or more review sections were capped; omitted entries are not evidence of absence.]\n")
	}
	return boundCompactReviewText(b.String())
}

func compactEntityLabel(entity *query.CompactEntity) string {
	if entity == nil {
		return "unknown entity"
	}
	return fmt.Sprintf("%s (%s:%d)", entity.ID, entity.Source.File, entity.Source.Line)
}

func formatEntities(entities []*query.CompactEntity) string {
	values := make([]string, 0, len(entities))
	for _, entity := range entities {
		values = append(values, compactEntityLabel(entity))
	}
	return strings.Join(values, ", ")
}

func formatHunks(hunks []Hunk) string {
	values := make([]string, 0, len(hunks))
	for _, hunk := range hunks {
		values = append(values, fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s", hunk.OldStart, hunk.OldCount, hunk.NewStart, hunk.NewCount, hunk.Header))
	}
	return strings.Join(values, "; ")
}

func formatTestReferences(references []CompactTestReference) string {
	values := make([]string, 0, len(references))
	for _, reference := range references {
		value := compactEntityLabel(reference.Test) + " [" + string(reference.Confidence) + "]"
		if evidence := formatTestRelationshipEvidence(reference.Relationship); evidence != "" {
			value += " — " + evidence
		} else if reference.Reason != "" {
			value += " — " + reference.Reason
		}
		values = append(values, value)
	}
	return strings.Join(values, ", ")
}

func addedStatus(added bool) string {
	if added {
		return "ADDED"
	}
	return "MODIFIED"
}

func boundCompactReviewText(value string) string {
	if len(value) <= maxCompactReviewTextBytes {
		return value
	}
	const marker = "\n[TRUNCATED: compact MCP review text limit reached; omitted sections are not evidence of absence.]\n"
	limit := maxCompactReviewTextBytes - len(marker)
	if lineEnd := strings.LastIndex(value[:limit], "\n"); lineEnd > 0 {
		limit = lineEnd + 1
	}
	return value[:limit] + marker
}
