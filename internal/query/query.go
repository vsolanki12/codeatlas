package query

import (
	"sort"
	"strings"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

type Subgraph struct {
	Entities      []*domain.Entity
	Relationships []*domain.Relationship
}

type GraphStats struct {
	TotalEntities int
	TotalRels     int
	EntityCounts  map[string]int
	RelCounts     map[string]int
}

func (idx *Index) GetEntity(id string) *domain.Entity {
	return idx.byID[id]
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
	frontier := []string{entityID}

	for d := 0; d < depth; d++ {
		var next []string
		for _, eid := range frontier {
			for _, r := range idx.fromEntity[eid] {
				rels = append(rels, r)
				if !visited[r.To] {
					visited[r.To] = true
					next = append(next, r.To)
				}
			}
			for _, r := range idx.toEntity[eid] {
				rels = append(rels, r)
				if !visited[r.From] {
					visited[r.From] = true
					next = append(next, r.From)
				}
			}
		}
		frontier = next
	}

	const maxEntities = 50
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
	}

	sort.Slice(rels, func(i, j int) bool {
		return rels[i].ID < rels[j].ID
	})

	return &Subgraph{Entities: entities, Relationships: rels}
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

func (idx *Index) Stats() *GraphStats {
	s := &GraphStats{
		TotalEntities: len(idx.graph.Entities),
		TotalRels:     len(idx.graph.Relationship),
		EntityCounts:  make(map[string]int),
		RelCounts:     make(map[string]int),
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
			return results[i].LastModified < results[j].LastModified
		})
	} else {
		sort.Slice(results, func(i, j int) bool {
			return results[i].ChangeCount > results[j].ChangeCount
		})
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
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
	Rel    *domain.Relationship
	Target *domain.Entity
}

type InvestigateResult struct {
	Entity   *domain.Entity
	OutRels  map[domain.RelationshipType][]ResolvedRel
	InRels   map[domain.RelationshipType][]ResolvedRel
	Callers  []*domain.Entity
	Tests    []*domain.Entity
	Siblings []*domain.Entity
}

type ExplainNode struct {
	Entity   *domain.Entity
	EdgeType domain.RelationshipType
	Children []*ExplainNode
}

type ExplainResult struct {
	Root       *ExplainNode
	TotalNodes int
	Capped     bool
}

func (idx *Index) Investigate(entityID string) *InvestigateResult {
	e := idx.byID[entityID]
	if e == nil {
		return nil
	}

	const maxPerType = 20

	outRels := make(map[domain.RelationshipType][]ResolvedRel)
	for _, r := range idx.fromEntity[entityID] {
		if len(outRels[r.Type]) >= maxPerType {
			continue
		}
		if target := idx.byID[r.To]; target != nil {
			outRels[r.Type] = append(outRels[r.Type], ResolvedRel{r, target})
		}
	}

	inRels := make(map[domain.RelationshipType][]ResolvedRel)
	for _, r := range idx.toEntity[entityID] {
		if len(inRels[r.Type]) >= maxPerType {
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

	siblings := idx.Where(e.Source.File, 21)
	filtered := make([]*domain.Entity, 0, len(siblings))
	for _, s := range siblings {
		if s.ID != entityID {
			filtered = append(filtered, s)
		}
	}
	if len(filtered) > 20 {
		filtered = filtered[:20]
	}

	return &InvestigateResult{
		Entity:   e,
		OutRels:  outRels,
		InRels:   inRels,
		Callers:  callers,
		Tests:    tests,
		Siblings: filtered,
	}
}

var explainEdgeOrder = []domain.RelationshipType{
	domain.RelReconciles,
	domain.RelCreates,
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
					Entity:   target,
					EdgeType: edgeType,
					Children: build(r.To, d-1),
				}
				children = append(children, node)
			}
		}
		return children
	}

	root := &ExplainNode{Entity: e}
	root.Children = build(entityID, depth)

	return &ExplainResult{
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
		return results[i].LastModified > results[j].LastModified
	})

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results
}

type ImpactResult struct {
	Entity        *domain.Entity
	CallChain     []*domain.Entity
	Controllers   []*domain.Entity
	Tests         []*domain.Entity
	Resources     []*domain.Entity
	Files         []string
	RecentChanges []*domain.Entity
	Owners        []string
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

		if !fileSet[e.Source.File] {
			fileSet[e.Source.File] = true
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
			case domain.RelReconciles, domain.RelCreates:
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
	sort.Slice(recentChanges, func(i, j int) bool { return recentChanges[i].LastModified > recentChanges[j].LastModified })

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

	if len(tests) > 30 {
		tests = tests[:30]
	}
	if len(resources) > 30 {
		resources = resources[:30]
	}

	return &ImpactResult{
		Entity:        root,
		CallChain:     callers,
		Controllers:   controllers,
		Tests:         tests,
		Resources:     resources,
		Files:         files,
		RecentChanges: recentChanges,
		Owners:        owners,
	}
}
