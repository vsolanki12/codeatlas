package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

type Subgraph struct {
	Graph         GraphMetadata          `json:"graph"`
	Entities      []*domain.Entity       `json:"entities"`
	Relationships []*domain.Relationship `json:"relationships"`
	Truncated     bool                   `json:"truncated,omitempty"`
}

// EntityListResult is the bounded, machine-readable envelope shared by
// search/where/query consumers. Keeping graph metadata beside the entities
// prevents callers from treating a stale or incomplete result as current.
type EntityListResult struct {
	Graph                  GraphMetadata          `json:"graph"`
	Entities               []*domain.Entity       `json:"entities"`
	Relationships          []*domain.Relationship `json:"relationships,omitempty"`
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

func (idx *Index) EntityListResult(entities []*domain.Entity, includeRelationships bool, maxRelationships int) *EntityListResult {
	return idx.EntityListPageResult(EntityPage{
		Entities: entities,
		Total:    len(entities),
		Limit:    len(entities),
	}, includeRelationships, maxRelationships)
}

// EntityListPageResult builds the full entity-list envelope for a bounded
// page. The page metadata makes a truncated result resumable instead of
// forcing consumers to guess which entities were omitted.
func (idx *Index) EntityListPageResult(page EntityPage, includeRelationships bool, maxRelationships int) *EntityListResult {
	return idx.EntityListRelationshipPageResult(page, includeRelationships, 0, maxRelationships)
}

// EntityListRelationshipPageResult builds an entity page and an optional
// relationship page over the same stable entity selection. The separate
// offsets let a consumer continue relationship evidence without changing the
// entity query that produced it.
func (idx *Index) EntityListRelationshipPageResult(page EntityPage, includeRelationships bool, relationshipOffset, relationshipLimit int) *EntityListResult {
	result := &EntityListResult{
		Graph:      idx.GraphMetadata(),
		Entities:   page.Entities,
		Total:      page.Total,
		Offset:     page.Offset,
		Limit:      page.Limit,
		NextOffset: page.NextOffset(),
		Truncated:  page.HasMore,
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
	result.Relationships = relationshipPage.Relationships
	if relationshipPage.HasMore {
		result.NextRelationshipOffset = relationshipPage.NextOffset()
		result.RelationshipsTruncated = true
		result.Truncated = true
	}
	return result
}

// EntityPage is a deterministic page of entity matches. Total is calculated
// before bounding, while HasMore and NextOffset describe continuation.
type EntityPage struct {
	Entities []*domain.Entity
	Offset   int
	Limit    int
	Total    int
	HasMore  bool
}

func (p EntityPage) NextOffset() int {
	if !p.HasMore {
		return 0
	}
	return p.Offset + len(p.Entities)
}

// RelationshipPage is the deterministic, resumable page returned for the
// relationship neighborhood of one entity. Relationship pagination is kept
// separate from entity pagination because a single high-degree entity can
// otherwise make an otherwise small entity response silently incomplete.
type RelationshipPage struct {
	Relationships []*domain.Relationship
	Offset        int
	Limit         int
	Total         int
	HasMore       bool
}

func (p RelationshipPage) NextOffset() int {
	if !p.HasMore {
		return 0
	}
	return p.Offset + len(p.Relationships)
}

// NormalizePage applies the shared consumer contract: offsets cannot be
// negative, zero/negative limits use the safe default, and callers cannot
// request an unbounded response through a CLI or MCP boundary.
func NormalizePage(offset, limit int) (int, int, error) {
	if offset < 0 {
		return 0, 0, fmt.Errorf("offset must be non-negative")
	}
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	return offset, limit, nil
}

const (
	DefaultPageLimit = 20
	MaxPageLimit     = 100
)

func paginateEntities(entities []*domain.Entity, offset, limit int) EntityPage {
	if offset < 0 {
		offset = 0
	}
	total := len(entities)
	if offset > total {
		offset = total
	}
	if limit <= 0 {
		limit = total - offset
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := EntityPage{
		Entities: entities[offset:end],
		Offset:   offset,
		Limit:    limit,
		Total:    total,
	}
	page.HasMore = end < total
	return page
}

func paginateRelationships(relationships []*domain.Relationship, offset, limit int) RelationshipPage {
	if offset < 0 {
		offset = 0
	}
	total := len(relationships)
	if offset > total {
		offset = total
	}
	if limit <= 0 {
		limit = total - offset
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := RelationshipPage{
		Relationships: relationships[offset:end],
		Offset:        offset,
		Limit:         limit,
		Total:         total,
	}
	page.HasMore = end < total
	return page
}

type GraphStats struct {
	SchemaVersion  string               `json:"schemaVersion"`
	EntityIdentity string               `json:"entityIdentity,omitempty"`
	Repository     string               `json:"repository"`
	Commit         string               `json:"commit"`
	Branch         string               `json:"branch"`
	GeneratedAt    string               `json:"generatedAt"`
	ScanComplete   bool                 `json:"scanComplete"`
	ScanWarnings   []string             `json:"scanWarnings,omitempty"`
	ScanCoverage   *domain.ScanCoverage `json:"scanCoverage,omitempty"`

	TotalEntities int            `json:"totalEntities"`
	TotalRels     int            `json:"totalRelationships"`
	EntityCounts  map[string]int `json:"entityCounts"`
	RelCounts     map[string]int `json:"relationshipCounts"`
}

// Graph returns a copy of the loaded graph for verification-oriented
// consumers such as PR review. Query operations continue to expose bounded
// results rather than requiring those consumers to inspect storage directly.
func (idx *Index) Graph() domain.Graph {
	return idx.graph
}

func (idx *Index) GetEntity(id string) *domain.Entity {
	return idx.byID[id]
}

// EntitiesByID returns existing entities in the caller-provided order. It is
// a small query boundary helper for consumers that should not depend on the
// domain package just to assemble a response.
func (idx *Index) EntitiesByID(ids []string) []*domain.Entity {
	entities := make([]*domain.Entity, 0, len(ids))
	for _, id := range ids {
		if entity := idx.GetEntity(id); entity != nil {
			entities = append(entities, entity)
		}
	}
	return entities
}

// Resolve accepts an exact ID or an unambiguous exact name. It never chooses
// an arbitrary entity from a fuzzy match, because doing so can ground an LLM
// answer in the wrong package or controller.
func (idx *Index) Resolve(value string) (*domain.Entity, []*domain.Entity) {
	if entity := idx.GetEntity(value); entity != nil {
		return entity, nil
	}
	candidates := append([]*domain.Entity(nil), idx.byName[strings.ToLower(value)]...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	return nil, candidates
}

func (idx *Index) Lookup(kind string, name string, maxResults int) []*domain.Entity {
	var candidates []*domain.Entity

	if kind != "" {
		k, ok := ParseKind(kind)
		if !ok {
			return nil
		}
		candidates = idx.byKind[k]
	} else {
		candidates = make([]*domain.Entity, 0, len(idx.byID))
		for _, e := range idx.byID {
			candidates = append(candidates, e)
		}
	}

	if name != "" {
		lower := strings.ToLower(name)
		filtered := make([]*domain.Entity, 0)
		for _, e := range candidates {
			if strings.Contains(strings.ToLower(e.Name), lower) {
				filtered = append(filtered, e)
			}
		}
		candidates = filtered
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].ID < candidates[j].ID
	})

	if maxResults > 0 && len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	return candidates
}

func (idx *Index) LookupWithStatus(kind string, name string, maxResults int) ([]*domain.Entity, bool) {
	page := idx.LookupPage(kind, name, 0, maxResults)
	return page.Entities, page.HasMore
}

// LookupPage returns a deterministic, resumable page of kind/name matches.
func (idx *Index) LookupPage(kind string, name string, offset, limit int) EntityPage {
	return paginateEntities(idx.Lookup(kind, name, 0), offset, limit)
}

func (idx *Index) GetRelationships(entityID string, direction string, relType string) []*domain.Relationship {
	var results []*domain.Relationship

	if direction == "" {
		direction = "both"
	}

	if direction == "from" || direction == "both" {
		for _, r := range idx.fromEntity[entityID] {
			if relType == "" || string(r.Type) == relType {
				results = append(results, r)
			}
		}
	}

	if direction == "to" || direction == "both" {
		for _, r := range idx.toEntity[entityID] {
			if relType == "" || string(r.Type) == relType {
				results = append(results, r)
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results
}

// RelationshipPage returns a deterministic page of an entity's relationships.
// Callers must treat HasMore/NextOffset as part of the evidence contract;
// omitted relationships are not evidence that no further relationship exists.
func (idx *Index) RelationshipPage(entityID string, direction string, relType string, offset, limit int) RelationshipPage {
	return paginateRelationships(idx.GetRelationships(entityID, direction, relType), offset, limit)
}

func (idx *Index) Neighbors(entityID string, depth int) *Subgraph {
	root := idx.byID[entityID]
	if root == nil {
		return &Subgraph{}
	}

	if depth > 3 {
		depth = 3
	}

	visited := map[string]bool{entityID: true}
	var rels []*domain.Relationship
	seenRels := make(map[string]bool)
	frontier := []string{entityID}

	for d := 0; d < depth; d++ {
		var next []string
		for _, eid := range frontier {
			for _, r := range idx.fromEntity[eid] {
				if !seenRels[r.ID] {
					seenRels[r.ID] = true
					rels = append(rels, r)
				}
				if !visited[r.To] {
					visited[r.To] = true
					next = append(next, r.To)
				}
			}
			for _, r := range idx.toEntity[eid] {
				if !seenRels[r.ID] {
					seenRels[r.ID] = true
					rels = append(rels, r)
				}
				if !visited[r.From] {
					visited[r.From] = true
					next = append(next, r.From)
				}
			}
		}
		frontier = next
	}

	const maxEntities = 50
	const maxRelationships = 120
	truncated := false
	entities := make([]*domain.Entity, 0, len(visited))
	for id := range visited {
		if e := idx.byID[id]; e != nil {
			entities = append(entities, e)
		}
	}

	sort.Slice(entities, func(i, j int) bool {
		return entities[i].ID < entities[j].ID
	})
	if len(entities) > maxEntities {
		entities = entities[:maxEntities]
		truncated = true
	}
	rootIncluded := false
	for _, entity := range entities {
		if entity.ID == entityID {
			rootIncluded = true
			break
		}
	}
	if !rootIncluded {
		// The root is the subject of the query and must remain present even
		// when lexical ordering would otherwise evict it from a bounded result.
		entities[maxEntities-1] = root
		sort.Slice(entities, func(i, j int) bool {
			return entities[i].ID < entities[j].ID
		})
	}
	included := make(map[string]bool, len(entities))
	for _, e := range entities {
		included[e.ID] = true
	}
	filteredRels := rels[:0]
	for _, r := range rels {
		if included[r.From] && included[r.To] {
			filteredRels = append(filteredRels, r)
		}
	}
	rels = filteredRels

	sort.Slice(rels, func(i, j int) bool {
		return rels[i].ID < rels[j].ID
	})
	if len(rels) > maxRelationships {
		rels = rels[:maxRelationships]
		truncated = true
	}

	return &Subgraph{Graph: idx.GraphMetadata(), Entities: entities, Relationships: rels, Truncated: truncated}
}

func (idx *Index) Search(query string, maxResults int) []*domain.Entity {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}

	type scored struct {
		entity *domain.Entity
		score  int
	}
	var results []scored
	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		s := scoreEntity(e, terms)
		if s > 0 {
			results = append(results, scored{e, s})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].entity.ID < results[j].entity.ID
	})

	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	entities := make([]*domain.Entity, len(results))
	for i, r := range results {
		entities[i] = r.entity
	}
	return entities
}

// SearchWithStatus is the bounded retrieval form for consumers that need to
// tell an LLM whether the result is complete. The legacy Search API remains
// available for callers that only need the slice.
func (idx *Index) SearchWithStatus(query string, maxResults int) ([]*domain.Entity, bool) {
	page := idx.SearchPage(query, 0, maxResults)
	return page.Entities, page.HasMore
}

// SearchPage returns a deterministic, resumable page of full-text matches.
func (idx *Index) SearchPage(query string, offset, limit int) EntityPage {
	return paginateEntities(idx.Search(query, 0), offset, limit)
}

func scoreEntity(e *domain.Entity, terms []string) int {
	total := 0
	for _, term := range terms {
		s := scoreTerm(e, term)
		if s == 0 {
			return 0
		}
		total += s
	}
	return total
}

func scoreTerm(e *domain.Entity, term string) int {
	score := 0
	if strings.Contains(strings.ToLower(e.Name), term) {
		score = 100
	}
	if strings.Contains(strings.ToLower(e.ID), term) && score < 90 {
		score = 90
	}
	if strings.Contains(strings.ToLower(e.Description), term) && score < 70 {
		score = 70
	}
	if strings.Contains(strings.ToLower(e.Package), term) && score < 60 {
		score = 60
	}
	if score > 0 {
		return score
	}
	for _, imp := range e.Imports {
		if strings.Contains(strings.ToLower(imp), term) {
			return 40
		}
	}
	for _, lit := range e.Literals {
		if strings.Contains(strings.ToLower(lit), term) {
			return 30
		}
	}
	for _, prop := range e.Properties {
		if strings.Contains(strings.ToLower(prop), term) {
			return 20
		}
	}
	return 0
}

func (idx *Index) Where(path string, maxResults int) []*domain.Entity {
	lower := strings.ToLower(path)
	var results []*domain.Entity

	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		if strings.Contains(strings.ToLower(e.Source.File), lower) {
			results = append(results, e)
			continue
		}
		for _, f := range e.Files {
			if strings.Contains(strings.ToLower(f), lower) {
				results = append(results, e)
				break
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

func (idx *Index) WhereWithStatus(path string, maxResults int) ([]*domain.Entity, bool) {
	page := idx.WherePage(path, 0, maxResults)
	return page.Entities, page.HasMore
}

// WherePage returns a deterministic, resumable page of file/path matches.
func (idx *Index) WherePage(path string, offset, limit int) EntityPage {
	return paginateEntities(idx.Where(path, 0), offset, limit)
}

func (idx *Index) Stats() *GraphStats {
	s := &GraphStats{
		SchemaVersion:  idx.graph.SchemaVersion,
		EntityIdentity: idx.graph.EntityIdentity,
		Repository:     idx.graph.Repository,
		Commit:         idx.graph.Commit,
		Branch:         idx.graph.Branch,
		GeneratedAt:    idx.graph.GeneratedAt,
		ScanComplete:   idx.graph.ScanComplete,
		ScanWarnings:   append([]string(nil), idx.graph.ScanWarnings...),
		ScanCoverage:   copyScanCoverage(idx.graph.ScanCoverage),
		TotalEntities:  len(idx.graph.Entities),
		TotalRels:      len(idx.graph.Relationship),
		EntityCounts:   make(map[string]int),
		RelCounts:      make(map[string]int),
	}

	for _, e := range idx.graph.Entities {
		s.EntityCounts[e.Kind.String()]++
	}

	for _, r := range idx.graph.Relationship {
		s.RelCounts[string(r.Type)]++
	}

	return s
}

func (idx *Index) Hotspots(kind string, stale bool, limit int) []*domain.Entity {
	var results []*domain.Entity
	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		if e.ChangeCount == 0 {
			continue
		}
		if kind != "" {
			k, ok := ParseKind(kind)
			if !ok || e.Kind != k {
				continue
			}
		}
		results = append(results, e)
	}

	if stale {
		sort.Slice(results, func(i, j int) bool {
			if results[i].LastModified != results[j].LastModified {
				return results[i].LastModified < results[j].LastModified
			}
			return results[i].ID < results[j].ID
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			if results[i].ChangeCount != results[j].ChangeCount {
				return results[i].ChangeCount > results[j].ChangeCount
			}
			return results[i].ID < results[j].ID
		})
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func (idx *Index) Temporal(kind string, name string, since string, author string, stale bool, limit int) []*domain.Entity {
	nameLower := strings.ToLower(name)
	authorLower := strings.ToLower(author)
	var results []*domain.Entity

	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		if e.ChangeCount == 0 && e.LastModified == "" {
			continue
		}
		if kind != "" {
			k, ok := ParseKind(kind)
			if !ok || e.Kind != k {
				continue
			}
		}
		if name != "" && !strings.Contains(strings.ToLower(e.Name), nameLower) {
			continue
		}
		if since != "" && e.LastModified < since {
			continue
		}
		if author != "" && !strings.Contains(strings.ToLower(e.LastAuthor), authorLower) {
			continue
		}
		results = append(results, e)
	}

	if stale {
		sort.Slice(results, func(i, j int) bool {
			if results[i].LastModified != results[j].LastModified {
				return results[i].LastModified < results[j].LastModified
			}
			return results[i].ID < results[j].ID
		})
	} else if since != "" || author != "" {
		sort.Slice(results, func(i, j int) bool {
			if results[i].LastModified != results[j].LastModified {
				return results[i].LastModified > results[j].LastModified
			}
			return results[i].ID < results[j].ID
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			if results[i].ChangeCount != results[j].ChangeCount {
				return results[i].ChangeCount > results[j].ChangeCount
			}
			return results[i].ID < results[j].ID
		})
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

func (idx *Index) TemporalWithStatus(kind string, name string, since string, author string, stale bool, limit int) ([]*domain.Entity, bool) {
	page := idx.TemporalPage(kind, name, since, author, stale, 0, limit)
	return page.Entities, page.HasMore
}

// TemporalPage returns a deterministic, resumable page of git-history
// matches. The sort order is the same as Temporal.
func (idx *Index) TemporalPage(kind string, name string, since string, author string, stale bool, offset, limit int) EntityPage {
	return paginateEntities(idx.Temporal(kind, name, since, author, stale, 0), offset, limit)
}

func boundEntities(entities []*domain.Entity, maxResults int) ([]*domain.Entity, bool) {
	if maxResults <= 0 || len(entities) <= maxResults {
		return entities, false
	}
	return entities[:maxResults], true
}

func (idx *Index) Callers(entityID string) []*domain.Entity {
	var results []*domain.Entity
	for _, r := range idx.toEntity[entityID] {
		if r.Type == domain.RelCalls {
			if e := idx.byID[r.From]; e != nil {
				results = append(results, e)
			}
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})
	return results
}

type ResolvedRel struct {
	Rel    *domain.Relationship `json:"relationship"`
	Target *domain.Entity       `json:"target"`
}

type InvestigateResult struct {
	Graph     GraphMetadata                             `json:"graph"`
	Entity    *domain.Entity                            `json:"entity"`
	OutRels   map[domain.RelationshipType][]ResolvedRel `json:"outgoing"`
	InRels    map[domain.RelationshipType][]ResolvedRel `json:"incoming"`
	Callers   []*domain.Entity                          `json:"callers,omitempty"`
	Tests     []*domain.Entity                          `json:"tests,omitempty"`
	Siblings  []*domain.Entity                          `json:"siblings,omitempty"`
	Truncated bool                                      `json:"truncated,omitempty"`
}

type ExplainNode struct {
	Entity       *domain.Entity          `json:"entity"`
	EdgeType     domain.RelationshipType `json:"edgeType,omitempty"`
	Relationship *domain.Relationship    `json:"relationship,omitempty"`
	Children     []*ExplainNode          `json:"children,omitempty"`
}

type ExplainResult struct {
	Graph      GraphMetadata `json:"graph"`
	Root       *ExplainNode  `json:"root"`
	TotalNodes int           `json:"totalNodes"`
	Capped     bool          `json:"truncated,omitempty"`
}

func (idx *Index) Investigate(entityID string) *InvestigateResult {
	e := idx.byID[entityID]
	if e == nil {
		return nil
	}

	const maxPerType = 20
	truncated := false

	outRels := make(map[domain.RelationshipType][]ResolvedRel)
	for _, r := range idx.fromEntity[entityID] {
		if len(outRels[r.Type]) >= maxPerType {
			truncated = true
			continue
		}
		if target := idx.byID[r.To]; target != nil {
			outRels[r.Type] = append(outRels[r.Type], ResolvedRel{r, target})
		}
	}

	inRels := make(map[domain.RelationshipType][]ResolvedRel)
	for _, r := range idx.toEntity[entityID] {
		if len(inRels[r.Type]) >= maxPerType {
			truncated = true
			continue
		}
		if source := idx.byID[r.From]; source != nil {
			inRels[r.Type] = append(inRels[r.Type], ResolvedRel{r, source})
		}
	}

	var tests []*domain.Entity
	for _, rr := range outRels[domain.RelTestedBy] {
		tests = append(tests, rr.Target)
	}

	callers := idx.Callers(entityID)
	if len(callers) > maxPerType {
		callers = callers[:maxPerType]
		truncated = true
	}

	siblings := idx.Where(e.Source.File, 21)
	filtered := make([]*domain.Entity, 0, len(siblings))
	for _, s := range siblings {
		if s.ID != entityID {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) > 20 {
		filtered = filtered[:20]
		truncated = true
	}

	return &InvestigateResult{
		Graph:     idx.GraphMetadata(),
		Entity:    e,
		OutRels:   outRels,
		InRels:    inRels,
		Callers:   callers,
		Tests:     tests,
		Siblings:  filtered,
		Truncated: truncated,
	}
}

var explainEdgeOrder = []domain.RelationshipType{
	domain.RelReconciles,
	domain.RelCreates,
	domain.RelContains,
	domain.RelCalls,
	domain.RelTestedBy,
}

func (idx *Index) Explain(entityID string, depth int) *ExplainResult {
	e := idx.byID[entityID]
	if e == nil {
		return &ExplainResult{}
	}
	if depth <= 0 {
		depth = 2
	}
	if depth > 3 {
		depth = 3
	}

	visited := map[string]bool{entityID: true}
	total := 1
	capped := false

	var build func(eid string, d int) []*ExplainNode
	build = func(eid string, d int) []*ExplainNode {
		if d <= 0 || capped {
			return nil
		}
		var children []*ExplainNode
		for _, edgeType := range explainEdgeOrder {
			maxEdges := 20
			if edgeType == domain.RelCalls {
				maxEdges = 10
			}
			count := 0
			var targets []*domain.Relationship
			for _, r := range idx.fromEntity[eid] {
				if r.Type == edgeType {
					targets = append(targets, r)
				}
			}
			sort.Slice(targets, func(i, j int) bool {
				return targets[i].To < targets[j].To
			})
			if len(targets) > maxEdges {
				capped = true
			}
			for _, r := range targets {
				if count >= maxEdges || total >= 100 {
					if total >= 100 {
						capped = true
					}
					break
				}
				if visited[r.To] {
					continue
				}
				target := idx.byID[r.To]
				if target == nil {
					continue
				}
				visited[r.To] = true
				total++
				count++
				node := &ExplainNode{
					Entity:       target,
					EdgeType:     edgeType,
					Relationship: r,
					Children:     build(r.To, d-1),
				}
				children = append(children, node)
			}
		}
		return children
	}

	root := &ExplainNode{Entity: e}
	root.Children = build(entityID, depth)

	return &ExplainResult{
		Graph:      idx.GraphMetadata(),
		Root:       root,
		TotalNodes: total,
		Capped:     capped,
	}
}

func (idx *Index) Commits(name string, since string, author string, limit int) []*domain.Entity {
	nameLower := strings.ToLower(name)
	authorLower := strings.ToLower(author)
	var results []*domain.Entity

	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		if e.LastModified == "" {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(e.Name), nameLower) {
			continue
		}
		if since != "" && e.LastModified < since {
			continue
		}
		if author != "" && !strings.Contains(strings.ToLower(e.LastAuthor), authorLower) {
			continue
		}
		results = append(results, e)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].LastModified != results[j].LastModified {
			return results[i].LastModified > results[j].LastModified
		}
		return results[i].ID < results[j].ID
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

type ImpactResult struct {
	Graph         GraphMetadata          `json:"graph"`
	Entity        *domain.Entity         `json:"entity"`
	CallChain     []*domain.Entity       `json:"callChain,omitempty"`
	Relationships []*domain.Relationship `json:"relationships,omitempty"`
	Controllers   []*domain.Entity       `json:"controllers,omitempty"`
	Tests         []*domain.Entity       `json:"tests,omitempty"`
	Resources     []*domain.Entity       `json:"resources,omitempty"`
	Files         []string               `json:"files,omitempty"`
	RecentChanges []*domain.Entity       `json:"recentChanges,omitempty"`
	Owners        []string               `json:"owners,omitempty"`
	Truncated     bool                   `json:"truncated,omitempty"`
}

func (idx *Index) Impact(entityID string) *ImpactResult {
	root := idx.byID[entityID]
	if root == nil {
		return nil
	}

	visited := map[string]bool{entityID: true}
	chain := []*domain.Entity{root}
	frontier := []string{entityID}

	for depth := 0; depth < 5 && len(frontier) > 0; depth++ {
		var next []string
		for _, eid := range frontier {
			for _, r := range idx.toEntity[eid] {
				if r.Type != domain.RelCalls {
					continue
				}
				if visited[r.From] {
					continue
				}
				caller := idx.byID[r.From]
				if caller == nil {
					continue
				}
				visited[r.From] = true
				chain = append(chain, caller)
				next = append(next, r.From)
				if len(chain) >= 50 {
					break
				}
			}
			if len(chain) >= 50 {
				break
			}
		}
		frontier = next
	}

	controllerSet := map[string]bool{}
	testSet := map[string]bool{}
	resourceSet := map[string]bool{}
	fileSet := map[string]bool{}
	ownerSet := map[string]bool{}

	var controllers, tests, resources, recentChanges []*domain.Entity

	for _, e := range chain {
		if e.Kind == domain.KindController && !controllerSet[e.ID] {
			controllerSet[e.ID] = true
			controllers = append(controllers, e)
		}

		if e.Source.File != "" {
			fileSet[e.Source.File] = true
		}
		for _, file := range e.Files {
			if file != "" {
				fileSet[file] = true
			}
		}

		if e.LastAuthor != "" && !ownerSet[e.LastAuthor] {
			ownerSet[e.LastAuthor] = true
		}
		if e.LastModified != "" {
			recentChanges = append(recentChanges, e)
		}

		for _, r := range idx.fromEntity[e.ID] {
			target := idx.byID[r.To]
			if target == nil {
				continue
			}
			switch r.Type {
			case domain.RelTestedBy:
				if !testSet[target.ID] {
					testSet[target.ID] = true
					tests = append(tests, target)
				}
			case domain.RelReconciles, domain.RelCreates, domain.RelOwns, domain.RelWatches:
				if !resourceSet[target.ID] {
					resourceSet[target.ID] = true
					resources = append(resources, target)
				}
			}
		}
	}

	sort.Slice(controllers, func(i, j int) bool { return controllers[i].ID < controllers[j].ID })
	sort.Slice(tests, func(i, j int) bool { return tests[i].ID < tests[j].ID })
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	sort.Slice(recentChanges, func(i, j int) bool {
		if recentChanges[i].LastModified != recentChanges[j].LastModified {
			return recentChanges[i].LastModified > recentChanges[j].LastModified
		}
		return recentChanges[i].ID < recentChanges[j].ID
	})

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	sort.Strings(files)

	owners := make([]string, 0, len(ownerSet))
	for o := range ownerSet {
		owners = append(owners, o)
	}
	sort.Strings(owners)

	callers := chain[1:]
	truncated := len(chain) >= 50

	if len(tests) > 30 {
		tests = tests[:30]
		truncated = true
	}
	if len(resources) > 30 {
		resources = resources[:30]
		truncated = true
	}

	// Impact categories are derived from graph edges. Return the supporting
	// edges as well so consumers never have to treat a bare category list as
	// an unsupported architectural claim.
	returnedEntities := map[string]bool{root.ID: true}
	for _, e := range callers {
		returnedEntities[e.ID] = true
	}
	for _, e := range tests {
		returnedEntities[e.ID] = true
	}
	for _, e := range resources {
		returnedEntities[e.ID] = true
	}
	relationshipSet := make(map[string]*domain.Relationship)
	for _, e := range chain {
		for _, relationship := range idx.toEntity[e.ID] {
			if relationship.Type == domain.RelCalls && returnedEntities[relationship.From] && returnedEntities[relationship.To] {
				relationshipSet[relationship.ID] = relationship
			}
		}
		for _, relationship := range idx.fromEntity[e.ID] {
			if !returnedEntities[relationship.To] {
				continue
			}
			switch relationship.Type {
			case domain.RelTestedBy, domain.RelReconciles, domain.RelCreates, domain.RelOwns, domain.RelWatches:
				relationshipSet[relationship.ID] = relationship
			}
		}
	}
	relationships := make([]*domain.Relationship, 0, len(relationshipSet))
	for _, relationship := range relationshipSet {
		relationships = append(relationships, relationship)
	}
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].ID < relationships[j].ID })
	if len(relationships) > 80 {
		relationships = relationships[:80]
		truncated = true
	}

	return &ImpactResult{
		Graph:         idx.GraphMetadata(),
		Entity:        root,
		CallChain:     callers,
		Relationships: relationships,
		Controllers:   controllers,
		Tests:         tests,
		Resources:     resources,
		Files:         files,
		RecentChanges: recentChanges,
		Owners:        owners,
		Truncated:     truncated,
	}
}
