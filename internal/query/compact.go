package query

import (
	"sort"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// CompactEntity is the entity shape intended for machine consumers that need
// routing and architectural context without repeating large entity payloads.
// Relationship evidence remains the authority for relationship claims.
type CompactEntity struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	Kind                string        `json:"kind"`
	Description         string        `json:"description,omitempty"`
	Content             string        `json:"content,omitempty"`
	Package             string        `json:"package,omitempty"`
	Files               []string      `json:"files,omitempty"`
	Watches             []string      `json:"watches,omitempty"`
	WatchMethods        []string      `json:"watchMethods,omitempty"`
	WatchSites          []domain.Site `json:"watchSites,omitempty"`
	Creates             []string      `json:"creates,omitempty"`
	CreateSites         []domain.Site `json:"createSites,omitempty"`
	Calls               []string      `json:"calls,omitempty"`
	CallSites           []domain.Site `json:"callSites,omitempty"`
	EnvVars             []string      `json:"envVars,omitempty"`
	Implements          []string      `json:"implements,omitempty"`
	ImplementationSites []domain.Site `json:"implementationSites,omitempty"`
	Imports             []string      `json:"imports,omitempty"`
	ImportSites         []domain.Site `json:"importSites,omitempty"`
	Literals            []string      `json:"literals,omitempty"`
	Properties          []string      `json:"properties,omitempty"`
	Embeds              []string      `json:"embeds,omitempty"`
	EmbedSites          []domain.Site `json:"embedSites,omitempty"`
	Source              domain.Source `json:"source"`
	Truncated           bool          `json:"truncated,omitempty"`
}

// CompactRelationship preserves the complete relationship identity and
// evidence while avoiding repeated target/source entities.
type CompactRelationship struct {
	ID         string                  `json:"id"`
	From       string                  `json:"from"`
	To         string                  `json:"to"`
	Type       domain.RelationshipType `json:"type"`
	Confidence domain.Confidence       `json:"confidence"`
	Evidence   domain.Evidence         `json:"evidence"`
}

type CompactEntityListResult struct {
	Graph                  GraphMetadata          `json:"graph"`
	Entities               []*CompactEntity       `json:"entities"`
	Relationships          []*CompactRelationship `json:"relationships,omitempty"`
	Total                  int                    `json:"total"`
	Offset                 int                    `json:"offset"`
	Limit                  int                    `json:"limit"`
	NextOffset             int                    `json:"nextOffset,omitempty"`
	RelationshipTotal      int                    `json:"relationshipTotal,omitempty"`
	RelationshipOffset     int                    `json:"relationshipOffset,omitempty"`
	RelationshipLimit      int                    `json:"relationshipLimit,omitempty"`
	NextRelationshipOffset int                    `json:"nextRelationshipOffset,omitempty"`
	RelationshipsTruncated bool                   `json:"relationshipsTruncated,omitempty"`
	Truncated              bool                   `json:"truncated,omitempty"`
}

type CompactResolvedRel struct {
	Relationship *CompactRelationship `json:"relationship"`
	Target       *CompactEntity       `json:"target"`
}

type CompactInvestigateResult struct {
	Graph     GraphMetadata                                    `json:"graph"`
	Entity    *CompactEntity                                   `json:"entity"`
	OutRels   map[domain.RelationshipType][]CompactResolvedRel `json:"outgoing"`
	InRels    map[domain.RelationshipType][]CompactResolvedRel `json:"incoming"`
	Callers   []*CompactEntity                                 `json:"callers,omitempty"`
	Tests     []*CompactEntity                                 `json:"tests,omitempty"`
	Siblings  []*CompactEntity                                 `json:"siblings,omitempty"`
	Truncated bool                                             `json:"truncated,omitempty"`
}

type CompactExplainNode struct {
	Entity       *CompactEntity          `json:"entity"`
	EdgeType     domain.RelationshipType `json:"edgeType,omitempty"`
	Relationship *CompactRelationship    `json:"relationship,omitempty"`
	Children     []*CompactExplainNode   `json:"children,omitempty"`
}

type CompactExplainResult struct {
	Graph      GraphMetadata       `json:"graph"`
	Root       *CompactExplainNode `json:"root"`
	TotalNodes int                 `json:"totalNodes"`
	Capped     bool                `json:"truncated,omitempty"`
}

type CompactImpactResult struct {
	Graph         GraphMetadata          `json:"graph"`
	Entity        *CompactEntity         `json:"entity"`
	CallChain     []*CompactEntity       `json:"callChain,omitempty"`
	Relationships []*CompactRelationship `json:"relationships,omitempty"`
	Controllers   []*CompactEntity       `json:"controllers,omitempty"`
	Tests         []*CompactEntity       `json:"tests,omitempty"`
	Resources     []*CompactEntity       `json:"resources,omitempty"`
	Files         []string               `json:"files,omitempty"`
	RecentChanges []*CompactEntity       `json:"recentChanges,omitempty"`
	Owners        []string               `json:"owners,omitempty"`
	Truncated     bool                   `json:"truncated,omitempty"`
}

type CompactView struct {
	EntityID      string                    `json:"entityID"`
	EntityName    string                    `json:"entityName"`
	Kind          string                    `json:"kind"`
	Package       string                    `json:"package,omitempty"`
	File          string                    `json:"file"`
	Description   string                    `json:"description,omitempty"`
	Reconciles    string                    `json:"reconciles,omitempty"`
	Creates       []string                  `json:"creates,omitempty"`
	Watches       []string                  `json:"watches,omitempty"`
	Calls         []string                  `json:"calls,omitempty"`
	ReconciledBy  string                    `json:"reconciledBy,omitempty"`
	CreatedBy     []string                  `json:"createdBy,omitempty"`
	CalledBy      []string                  `json:"calledBy,omitempty"`
	Tests         []string                  `json:"tests,omitempty"`
	TestCount     int                       `json:"testCount"`
	Files         []string                  `json:"files,omitempty"`
	Owners        []string                  `json:"owners,omitempty"`
	ChangeCount   int                       `json:"changeCount,omitempty"`
	LastModified  string                    `json:"lastModified,omitempty"`
	LastAuthor    string                    `json:"lastAuthor,omitempty"`
	Relationships []domain.ViewRelationship `json:"relationships,omitempty"`
	Truncated     bool                      `json:"truncated,omitempty"`
}

type CompactAskResult struct {
	Graph         GraphMetadata             `json:"graph"`
	Entity        *CompactEntity            `json:"entity"`
	Candidates    []*CompactEntity          `json:"candidates,omitempty"`
	Match         string                    `json:"match"`
	Ambiguous     bool                      `json:"ambiguous,omitempty"`
	View          *CompactView              `json:"view,omitempty"`
	QAHit         string                    `json:"quickAnswer,omitempty"`
	QAEvidence    []*CompactRelationship    `json:"quickAnswerEvidence,omitempty"`
	Explanation   *CompactExplainResult     `json:"explanation,omitempty"`
	Impact        *CompactImpactResult      `json:"impact,omitempty"`
	Investigation *CompactInvestigateResult `json:"investigation,omitempty"`
	Truncated     bool                      `json:"truncated,omitempty"`
}

type CompactSubgraphResult struct {
	Graph         GraphMetadata          `json:"graph"`
	Entities      []*CompactEntity       `json:"entities"`
	Relationships []*CompactRelationship `json:"relationships"`
	Truncated     bool                   `json:"truncated,omitempty"`
}

func CompactEntities(entities []*domain.Entity) []*CompactEntity {
	result := make([]*CompactEntity, 0, len(entities))
	for _, entity := range entities {
		if entity != nil {
			result = append(result, compactEntity(entity))
		}
	}
	return result
}

// CompactEntity exposes the bounded entity representation without requiring
// consumers to import the domain package.
func CompactEntitySummary(entity *domain.Entity) *CompactEntity {
	return compactEntity(entity)
}

func (idx *Index) CompactEntityListResult(entities []*domain.Entity, includeRelationships bool, maxRelationships int) *CompactEntityListResult {
	return idx.CompactEntityListPageResult(EntityPage{
		Entities: entities,
		Total:    len(entities),
		Limit:    len(entities),
	}, includeRelationships, maxRelationships)
}

// CompactEntityListPageResult is the machine-oriented equivalent of
// EntityListPageResult. It retains page metadata so an assistant can request
// the next deterministic slice without broadening the original query.
func (idx *Index) CompactEntityListPageResult(page EntityPage, includeRelationships bool, maxRelationships int) *CompactEntityListResult {
	return idx.CompactEntityListRelationshipPageResult(page, includeRelationships, 0, maxRelationships)
}

// CompactEntityListRelationshipPageResult is the machine-oriented equivalent
// of EntityListRelationshipPageResult. It makes relationship truncation
// resumable while preserving the compact token budget.
func (idx *Index) CompactEntityListRelationshipPageResult(page EntityPage, includeRelationships bool, relationshipOffset, relationshipLimit int) *CompactEntityListResult {
	result := &CompactEntityListResult{
		Graph:      idx.GraphMetadata(),
		Entities:   CompactEntities(page.Entities),
		Total:      page.Total,
		Offset:     page.Offset,
		Limit:      page.Limit,
		NextOffset: page.NextOffset(),
		Truncated:  page.HasMore,
	}
	for _, entity := range result.Entities {
		if entity.Truncated {
			result.Truncated = true
		}
	}
	if !includeRelationships {
		return result
	}

	seen := make(map[string]bool)
	var allRelationships []*domain.Relationship
	for _, entity := range page.Entities {
		for _, relationship := range idx.GetRelationships(entity.ID, "both", "") {
			if seen[relationship.ID] {
				continue
			}
			seen[relationship.ID] = true
			allRelationships = append(allRelationships, relationship)
		}
	}
	sort.Slice(allRelationships, func(i, j int) bool {
		return allRelationships[i].ID < allRelationships[j].ID
	})
	relationshipPage := paginateRelationships(allRelationships, relationshipOffset, relationshipLimit)
	result.RelationshipTotal = relationshipPage.Total
	result.RelationshipOffset = relationshipPage.Offset
	result.RelationshipLimit = relationshipPage.Limit
	result.Relationships = make([]*CompactRelationship, 0, len(relationshipPage.Relationships))
	for _, relationship := range relationshipPage.Relationships {
		result.Relationships = append(result.Relationships, compactRelationship(relationship))
	}
	if relationshipPage.HasMore {
		result.NextRelationshipOffset = relationshipPage.NextOffset()
		result.RelationshipsTruncated = true
		result.Truncated = true
	}
	return result
}

// CompactEntityRelationshipPageResult creates a bounded entity response for
// an explicit relationship page. It is used when a caller needs to continue a
// high-degree entity query without repeating the entity payload.
func (idx *Index) CompactEntityRelationshipPageResult(entity *domain.Entity, page RelationshipPage) *CompactEntityListResult {
	result := &CompactEntityListResult{
		Graph:                  idx.GraphMetadata(),
		Entities:               CompactEntities([]*domain.Entity{entity}),
		Relationships:          make([]*CompactRelationship, 0, len(page.Relationships)),
		Total:                  1,
		Limit:                  1,
		RelationshipTotal:      page.Total,
		RelationshipOffset:     page.Offset,
		RelationshipLimit:      page.Limit,
		NextRelationshipOffset: page.NextOffset(),
		RelationshipsTruncated: page.HasMore,
		Truncated:              page.HasMore,
	}
	for _, relationship := range page.Relationships {
		result.Relationships = append(result.Relationships, compactRelationship(relationship))
	}
	return result
}

// CompactRelationships preserves relationship IDs and evidence for callers
// that already have a separately paginated relationship slice.
func CompactRelationships(relationships []*domain.Relationship) []*CompactRelationship {
	result := make([]*CompactRelationship, 0, len(relationships))
	for _, relationship := range relationships {
		result = append(result, compactRelationship(relationship))
	}
	return result
}

// CompactAsk converts a full compound query to a bounded, non-repeating
// representation suitable for an LLM prompt. It deliberately keeps graph
// status and relationship evidence in the result.
func CompactAsk(result *AskResult) *CompactAskResult {
	if result == nil {
		return nil
	}
	compact := &CompactAskResult{
		Graph:     result.Graph,
		Entity:    compactEntity(result.Entity),
		Match:     result.Match,
		Ambiguous: result.Ambiguous,
		QAHit:     result.QAHit,
		View:      compactView(result.View),
	}
	compact.Candidates = CompactEntities(result.Candidates)
	for _, candidate := range compact.Candidates {
		compact.Truncated = compact.Truncated || candidate.Truncated
	}
	if compact.Entity != nil && compact.Entity.Truncated {
		compact.Truncated = true
	}

	if result.Explanation != nil {
		compact.Explanation = compactExplain(result.Explanation)
		compact.Truncated = compact.Truncated || compact.Explanation.Capped
	}
	for _, relationship := range result.QAEvidence {
		compact.QAEvidence = append(compact.QAEvidence, compactRelationship(relationship))
	}
	if result.Impact != nil {
		compact.Impact = compactImpact(result.Impact)
		compact.Truncated = compact.Truncated || compact.Impact.Truncated
	}
	if result.Investigation != nil {
		compact.Investigation = compactInvestigate(result.Investigation)
		compact.Truncated = compact.Truncated || compact.Investigation.Truncated
	}
	return compact
}

func CompactSubgraph(result *Subgraph) *CompactSubgraphResult {
	if result == nil {
		return nil
	}
	compact := &CompactSubgraphResult{
		Graph:     result.Graph,
		Entities:  CompactEntities(result.Entities),
		Truncated: result.Truncated,
	}
	for _, entity := range compact.Entities {
		compact.Truncated = compact.Truncated || entity.Truncated
	}
	for _, relationship := range result.Relationships {
		compact.Relationships = append(compact.Relationships, compactRelationship(relationship))
	}
	return compact
}

func CompactViewResult(view *domain.View) *CompactView {
	return compactView(view)
}

func CompactInvestigate(result *InvestigateResult) *CompactInvestigateResult {
	if result == nil {
		return nil
	}
	return compactInvestigate(result)
}

func CompactExplain(result *ExplainResult) *CompactExplainResult {
	if result == nil {
		return nil
	}
	return compactExplain(result)
}

func CompactImpact(result *ImpactResult) *CompactImpactResult {
	if result == nil {
		return nil
	}
	return compactImpact(result)
}

func compactEntity(entity *domain.Entity) *CompactEntity {
	if entity == nil {
		return nil
	}
	result := &CompactEntity{
		ID:          entity.ID,
		Name:        entity.Name,
		Kind:        entity.Kind.String(),
		Description: entity.Description,
		Package:     entity.Package,
		Source:      entity.Source,
	}
	truncated := false
	result.Files, truncated = cappedStrings(entity.Files, 6)
	result.Watches, truncated = cappedStringsWithFlag(entity.Watches, 8, truncated)
	result.WatchMethods, truncated = cappedStringsWithFlag(entity.WatchMethods, 8, truncated)
	result.WatchSites, truncated = cappedSitesWithFlag(entity.WatchSites, 8, truncated)
	result.Creates, truncated = cappedStringsWithFlag(entity.Creates, 8, truncated)
	result.CreateSites, truncated = cappedSitesWithFlag(entity.CreateSites, 8, truncated)
	result.Calls, truncated = cappedStringsWithFlag(entity.Calls, 12, truncated)
	result.CallSites, truncated = cappedSitesWithFlag(entity.CallSites, 12, truncated)
	result.EnvVars, truncated = cappedStringsWithFlag(entity.EnvVars, 8, truncated)
	result.Implements, truncated = cappedStringsWithFlag(entity.Implements, 6, truncated)
	result.ImplementationSites, truncated = cappedSitesWithFlag(entity.ImplementationSites, 6, truncated)
	result.Imports, truncated = cappedStringsWithFlag(entity.Imports, 8, truncated)
	result.ImportSites, truncated = cappedSitesWithFlag(entity.ImportSites, 8, truncated)
	result.Literals, truncated = cappedStringsWithFlag(entity.Literals, 6, truncated)
	result.Properties, truncated = cappedStringsWithFlag(entity.Properties, 10, truncated)
	result.Embeds, truncated = cappedStringsWithFlag(entity.Embeds, 6, truncated)
	result.EmbedSites, truncated = cappedSitesWithFlag(entity.EmbedSites, 6, truncated)
	if len(result.Description) > 240 {
		result.Description = result.Description[:240] + "..."
		truncated = true
	}
	if entity.Kind == domain.KindDocument && entity.Content != "" {
		result.Content = entity.Content
		if len(result.Content) > 600 {
			result.Content = result.Content[:597] + "..."
			truncated = true
		}
	}
	result.Truncated = truncated
	return result
}

func compactView(view *domain.View) *CompactView {
	if view == nil {
		return nil
	}
	result := &CompactView{
		EntityID:     view.EntityID,
		EntityName:   view.EntityName,
		Kind:         view.Kind,
		Package:      view.Package,
		File:         view.File,
		Description:  view.Description,
		Reconciles:   view.Reconciles,
		ReconciledBy: view.ReconciledBy,
		TestCount:    view.TestCount,
		ChangeCount:  view.ChangeCount,
		LastModified: view.LastModified,
		LastAuthor:   view.LastAuthor,
	}
	truncated := false
	result.Creates, truncated = cappedStrings(view.Creates, 12)
	result.Watches, truncated = cappedStringsWithFlag(view.Watches, 12, truncated)
	result.Calls, truncated = cappedStringsWithFlag(view.Calls, 12, truncated)
	result.CreatedBy, truncated = cappedStringsWithFlag(view.CreatedBy, 12, truncated)
	result.CalledBy, truncated = cappedStringsWithFlag(view.CalledBy, 12, truncated)
	result.Tests, truncated = cappedStringsWithFlag(view.Tests, 20, truncated)
	result.Files, truncated = cappedStringsWithFlag(view.Files, 8, truncated)
	result.Owners, truncated = cappedStringsWithFlag(view.Owners, 8, truncated)
	if len(view.Relationships) > 24 {
		result.Relationships = append([]domain.ViewRelationship(nil), view.Relationships[:24]...)
		truncated = true
	} else {
		result.Relationships = append([]domain.ViewRelationship(nil), view.Relationships...)
	}
	if len(result.Description) > 240 {
		result.Description = result.Description[:240] + "..."
		truncated = true
	}
	result.Truncated = truncated
	return result
}

func compactRelationship(relationship *domain.Relationship) *CompactRelationship {
	if relationship == nil {
		return nil
	}
	return &CompactRelationship{
		ID:         relationship.ID,
		From:       relationship.From,
		To:         relationship.To,
		Type:       relationship.Type,
		Confidence: relationship.Confidence,
		Evidence:   relationship.Evidence,
	}
}

func compactInvestigate(result *InvestigateResult) *CompactInvestigateResult {
	compact := &CompactInvestigateResult{
		Graph:     result.Graph,
		Entity:    compactEntity(result.Entity),
		OutRels:   make(map[domain.RelationshipType][]CompactResolvedRel),
		InRels:    make(map[domain.RelationshipType][]CompactResolvedRel),
		Truncated: result.Truncated,
	}
	if compact.Entity != nil && compact.Entity.Truncated {
		compact.Truncated = true
	}
	for relType, relationships := range result.OutRels {
		values, truncated := compactResolvedRelationships(relationships, 8)
		compact.OutRels[relType] = values
		compact.Truncated = compact.Truncated || truncated
	}
	for relType, relationships := range result.InRels {
		values, truncated := compactResolvedRelationships(relationships, 8)
		compact.InRels[relType] = values
		compact.Truncated = compact.Truncated || truncated
	}
	compact.Callers, compact.Truncated = compactEntityList(result.Callers, 20, compact.Truncated)
	compact.Tests, compact.Truncated = compactEntityList(result.Tests, 20, compact.Truncated)
	compact.Siblings, compact.Truncated = compactEntityList(result.Siblings, 12, compact.Truncated)
	return compact
}

func compactResolvedRelationships(relationships []ResolvedRel, max int) ([]CompactResolvedRel, bool) {
	truncated := len(relationships) > max
	if truncated {
		relationships = relationships[:max]
	}
	result := make([]CompactResolvedRel, 0, len(relationships))
	for _, relationship := range relationships {
		result = append(result, CompactResolvedRel{
			Relationship: compactRelationship(relationship.Rel),
			Target:       compactEntity(relationship.Target),
		})
	}
	return result, truncated
}

func compactExplain(result *ExplainResult) *CompactExplainResult {
	budget := 60
	root, truncated := compactExplainNode(result.Root, &budget)
	return &CompactExplainResult{
		Graph:      result.Graph,
		Root:       root,
		TotalNodes: result.TotalNodes,
		Capped:     result.Capped || truncated,
	}
}

func compactExplainNode(node *ExplainNode, budget *int) (*CompactExplainNode, bool) {
	if node == nil {
		return nil, false
	}
	if *budget == 0 {
		return nil, true
	}
	*budget--
	result := &CompactExplainNode{
		Entity:       compactEntity(node.Entity),
		EdgeType:     node.EdgeType,
		Relationship: compactRelationship(node.Relationship),
	}
	truncated := result.Entity != nil && result.Entity.Truncated
	for _, child := range node.Children {
		compactChild, childTruncated := compactExplainNode(child, budget)
		if compactChild != nil {
			result.Children = append(result.Children, compactChild)
		}
		if childTruncated {
			truncated = true
			break
		}
	}
	return result, truncated
}

func compactImpact(result *ImpactResult) *CompactImpactResult {
	compact := &CompactImpactResult{
		Graph:     result.Graph,
		Entity:    compactEntity(result.Entity),
		Owners:    result.Owners,
		Truncated: result.Truncated,
	}
	if compact.Entity != nil && compact.Entity.Truncated {
		compact.Truncated = true
	}
	compact.CallChain, compact.Truncated = compactEntityList(result.CallChain, 30, compact.Truncated)
	compact.Relationships, compact.Truncated = compactRelationshipList(result.Relationships, 40, compact.Truncated)
	compact.Controllers, compact.Truncated = compactEntityList(result.Controllers, 20, compact.Truncated)
	compact.Tests, compact.Truncated = compactEntityList(result.Tests, 20, compact.Truncated)
	compact.Resources, compact.Truncated = compactEntityList(result.Resources, 20, compact.Truncated)
	compact.RecentChanges, compact.Truncated = compactEntityList(result.RecentChanges, 20, compact.Truncated)
	compact.Files, compact.Truncated = cappedStringsWithFlag(result.Files, 30, compact.Truncated)
	compact.Owners, compact.Truncated = cappedStringsWithFlag(result.Owners, 12, compact.Truncated)
	return compact
}

func compactRelationshipList(relationships []*domain.Relationship, max int, truncated bool) ([]*CompactRelationship, bool) {
	if len(relationships) > max {
		relationships = relationships[:max]
		truncated = true
	}
	result := make([]*CompactRelationship, 0, len(relationships))
	for _, relationship := range relationships {
		result = append(result, compactRelationship(relationship))
	}
	return result, truncated
}

func compactEntityList(entities []*domain.Entity, max int, truncated bool) ([]*CompactEntity, bool) {
	if len(entities) > max {
		entities = entities[:max]
		truncated = true
	}
	result := CompactEntities(entities)
	for _, entity := range result {
		truncated = truncated || entity.Truncated
	}
	return result, truncated
}

func cappedStrings(values []string, max int) ([]string, bool) {
	if len(values) > max {
		return append([]string(nil), values[:max]...), true
	}
	return append([]string(nil), values...), false
}

func cappedStringsWithFlag(values []string, max int, truncated bool) ([]string, bool) {
	result, capped := cappedStrings(values, max)
	return result, truncated || capped
}

func cappedSites(values []domain.Site, max int) ([]domain.Site, bool) {
	if len(values) > max {
		return append([]domain.Site(nil), values[:max]...), true
	}
	return append([]domain.Site(nil), values...), false
}

func cappedSitesWithFlag(values []domain.Site, max int, truncated bool) ([]domain.Site, bool) {
	result, capped := cappedSites(values, max)
	return result, truncated || capped
}
