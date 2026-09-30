package query

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

const (
	EvidenceVersion            = "1.0"
	DefaultEvidenceBudgetBytes = 16 * 1024
	MinEvidenceBudgetBytes     = 2048
	MaxEvidenceBudgetBytes     = 1024 * 1024
)

// EvidenceRequest retains the complete question and optional exact subject.
// Offsets refer to the ranked entity selection, never to serialized bytes.
type EvidenceRequest struct {
	Question         string `json:"question,omitempty"`
	Entity           string `json:"entity,omitempty"`
	Scope            string `json:"scope,omitempty"`
	Intent           string `json:"intent,omitempty"`
	BudgetBytes      int    `json:"budgetBytes,omitempty"`
	Offset           int    `json:"offset,omitempty"`
	GraphFingerprint string `json:"graphFingerprint,omitempty"`
}

// EvidenceEntity is a routing record. Facts about relationships live only in
// Relationships; selection reasons explain relevance rather than assert facts.
type EvidenceEntity struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Kind        string        `json:"kind"`
	Package     string        `json:"package,omitempty"`
	Description string        `json:"description,omitempty"`
	Source      domain.Source `json:"source"`
	Score       int           `json:"score"`
	Reasons     []string      `json:"reasons"`
}

type EvidenceSource struct {
	EntityID  string        `json:"entityID"`
	Role      string        `json:"role"`
	Source    domain.Source `json:"source"`
	FocusLine int           `json:"focusLine,omitempty"`
	Reason    string        `json:"reason"`
}

type EvidenceOmission struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// EvidencePacket is the versioned graph-only manifest shared by consumers.
// Its budget counts compact JSON bytes, not tokenizer-specific input tokens or
// subsequently materialized source. Entities and relationships are unique by ID.
type EvidencePacket struct {
	Version          string                 `json:"version"`
	Status           string                 `json:"status"`
	Graph            GraphMetadata          `json:"graph"`
	GraphFingerprint string                 `json:"graphFingerprint"`
	Entities         []*EvidenceEntity      `json:"entities"`
	Relationships    []*CompactRelationship `json:"relationships"`
	Sources          []EvidenceSource       `json:"sources"`
	Candidates       []*EvidenceEntity      `json:"candidates,omitempty"`
	Omissions        []EvidenceOmission     `json:"omissions,omitempty"`
	BudgetBytes      int                    `json:"budgetBytes"`
	Total            int                    `json:"total"`
	NextOffset       int                    `json:"nextOffset,omitempty"`
	Truncated        bool                   `json:"truncated"`
}

type evidenceSelection struct {
	entity       *domain.Entity
	score        int
	reasons      []string
	parent       *domain.Relationship
	distance     int
	routingBoost int
}

var evidenceIDPattern = regexp.MustCompile(`(?:operator|controller|crd|function|package|test|document|resource|template|field):[^\s\x60"'<>]+`)
var evidenceWordPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*`)
var evidenceConditionalPattern = regexp.MustCompile(`\b(?:if|switch)\b`)

// Evidence selects evidence without parsing files or calling another producer.
// A consumer may materialize Sources only after verifying graph/check-out state.
func (idx *Index) Evidence(req EvidenceRequest) (*EvidencePacket, error) {
	if req.BudgetBytes == 0 {
		req.BudgetBytes = DefaultEvidenceBudgetBytes
	}
	if req.BudgetBytes < MinEvidenceBudgetBytes || req.BudgetBytes > MaxEvidenceBudgetBytes {
		return nil, fmt.Errorf("budgetBytes must be between %d and %d", MinEvidenceBudgetBytes, MaxEvidenceBudgetBytes)
	}
	if req.Offset < 0 {
		return nil, fmt.Errorf("offset must be non-negative")
	}
	if req.Offset > 0 && req.GraphFingerprint == "" {
		return nil, fmt.Errorf("continuation requires graphFingerprint from the first packet")
	}
	if req.GraphFingerprint != "" && req.GraphFingerprint != idx.GraphFingerprint() {
		return nil, fmt.Errorf("graphFingerprint mismatch: continuation belongs to a different graph snapshot")
	}
	if strings.TrimSpace(req.Question) == "" && strings.TrimSpace(req.Entity) == "" {
		return nil, fmt.Errorf("question or entity is required")
	}
	if len(req.Question) > 64*1024 || len(req.Scope) > 4096 || len(req.Entity) > 4096 {
		return nil, fmt.Errorf("evidence request exceeds supported input length")
	}
	req.Intent = strings.ToLower(strings.TrimSpace(req.Intent))
	if req.Intent == "" {
		req.Intent = "understand"
	}
	if req.Intent != "understand" && req.Intent != "debug" && req.Intent != "impact" {
		return nil, fmt.Errorf("unsupported evidence intent %q", req.Intent)
	}
	packet := &EvidencePacket{Version: EvidenceVersion, Status: "ok", Graph: idx.GraphMetadata(),
		GraphFingerprint: idx.GraphFingerprint(), BudgetBytes: req.BudgetBytes,
		Entities: []*EvidenceEntity{}, Relationships: []*CompactRelationship{}, Sources: []EvidenceSource{}}
	if len(packet.Graph.ScanWarnings) > 8 {
		addEvidenceOmission(packet, "graph_warnings", len(packet.Graph.ScanWarnings)-8)
		packet.Graph.ScanWarnings = packet.Graph.ScanWarnings[:8]
	}
	for i, warning := range packet.Graph.ScanWarnings {
		if len(warning) > 240 {
			packet.Graph.ScanWarnings[i] = truncateEvidenceString(warning, 240)
			addEvidenceOmission(packet, "graph_warning_text", 1)
		}
	}
	if !evidenceFitsWithReserve(packet) {
		return nil, fmt.Errorf("budgetBytes is insufficient for graph provenance and omission diagnostics")
	}

	selections, ambiguous, terms := idx.evidenceSeeds(req)
	focusTerms := evidenceFocusTerms(terms, req.Scope)
	if len(ambiguous) > 0 {
		packet.Status = "ambiguous"
		packet.Total = len(ambiguous)
		if req.Offset > len(ambiguous) {
			return nil, fmt.Errorf("offset exceeds candidate total")
		}
		for i := req.Offset; i < len(ambiguous); i++ {
			candidate := evidenceEntity(ambiguous[i])
			next := cloneEvidencePacket(packet)
			next.Candidates = append(next.Candidates, candidate)
			setEvidenceContinuation(next, i+1, len(ambiguous), "budget")
			if !evidenceFitsWithReserve(next) {
				setEvidenceContinuation(packet, i, len(ambiguous), "budget")
				break
			}
			packet = next
		}
		return fitEvidenceEnvelope(packet)
	}
	if len(selections) == 0 {
		packet.Status = "no_match"
		return packet, nil
	}
	selections, depthOmitted := idx.expandEvidence(selections, req.Intent, focusTerms)
	packet.Total = len(selections)
	if req.Offset > packet.Total {
		return nil, fmt.Errorf("offset exceeds entity total")
	}
	if depthOmitted > 0 {
		addEvidenceOmission(packet, "depth", depthOmitted)
	}
	for i := req.Offset; i < len(selections); i++ {
		next := cloneEvidencePacket(packet)
		idx.addEvidenceSelection(next, selections[i], focusTerms)
		setEvidenceContinuation(next, i+1, len(selections), "budget")
		if !evidenceFitsWithReserve(next) {
			setEvidenceContinuation(packet, i, len(selections), "budget")
			if len(packet.Entities) == 0 {
				packet.Status = "budget_exhausted"
			}
			break
		}
		packet = next
	}
	// Retain additional edges between selected entities, including converging
	// paths that a tree's visited set would lose. Each edge has one evidence copy.
	seenEntities := make(map[string]bool, len(packet.Entities))
	seenRels := make(map[string]bool, len(packet.Relationships))
	for _, e := range packet.Entities {
		seenEntities[e.ID] = true
	}
	for _, r := range packet.Relationships {
		seenRels[r.ID] = true
	}
	var extra []*domain.Relationship
	for id := range seenEntities {
		for _, r := range idx.fromEntity[id] {
			if seenEntities[r.To] && !seenRels[r.ID] && evidenceEdgeType(r.Type) {
				seenRels[r.ID] = true
				extra = append(extra, r)
			}
		}
	}
	sort.Slice(extra, func(i, j int) bool { return extra[i].ID < extra[j].ID })
	for _, r := range extra {
		next := cloneEvidencePacket(packet)
		next.Relationships = append(next.Relationships, compactRelationship(r))
		idx.addEvidenceEdgeSource(next, r, focusTerms)
		if !evidenceFitsWithReserve(next) {
			addEvidenceOmission(packet, "relationship_budget", 1)
			continue
		}
		packet = next
	}
	return fitEvidenceEnvelope(packet)
}

func (idx *Index) evidenceSeeds(req EvidenceRequest) ([]evidenceSelection, []evidenceSelection, []string) {
	question := req.Question
	subject := strings.TrimSpace(req.Entity)
	if subject == "" {
		ids := evidenceIDPattern.FindAllString(question, -1)
		if len(ids) == 1 {
			subject = strings.TrimRight(ids[0], ".,;?!)]}")
		}
	}
	question = evidenceIDPattern.ReplaceAllString(question, " ")
	terms := idx.evidenceTerms(question)
	var anchor *domain.Entity
	if subject != "" {
		e, candidates := idx.Resolve(subject)
		if e == nil && len(candidates) == 0 {
			return nil, nil, terms
		}
		if e != nil && !evidenceScopeMatches(e, req.Scope) {
			return nil, nil, terms
		}
		if e == nil {
			var scoped []evidenceSelection
			for _, candidate := range candidates {
				if evidenceScopeMatches(candidate, req.Scope) {
					scoped = append(scoped, evidenceSelection{entity: candidate, score: 1000, reasons: []string{"exact_name"}})
				}
			}
			if len(scoped) != 1 {
				return nil, scoped, terms
			}
			e = scoped[0].entity
		}
		anchor = e
	}
	if anchor == nil && len(terms) == 1 {
		_, candidates := idx.Resolve(terms[0])
		var scoped []evidenceSelection
		for _, e := range candidates {
			if evidenceScopeMatches(e, req.Scope) {
				scoped = append(scoped, evidenceSelection{entity: e, score: 1000, reasons: []string{"exact_name"}})
			}
		}
		if len(scoped) > 1 {
			return nil, scoped, terms
		}
	}
	if len(terms) == 0 && anchor == nil {
		return nil, nil, terms
	}
	var allowed map[string]bool
	if anchor != nil {
		allowed = idx.evidenceReachable(anchor.ID, 2)
	}
	var selections []evidenceSelection
	for i := range idx.graph.Entities {
		e := &idx.graph.Entities[i]
		if allowed != nil && !allowed[e.ID] {
			continue
		}
		if !evidenceScopeMatches(e, req.Scope) {
			continue
		}
		score, reasons := evidenceScore(e, terms)
		if len(terms) == 0 {
			if e.ID != anchor.ID {
				continue
			}
			score, reasons = 1000, []string{"exact_subject"}
		}
		if score == 0 {
			continue
		}
		if len(terms) > 0 && e.Kind == domain.KindFunction && !e.Generated && strings.Contains(strings.ToLower(e.Name), "reconcile") {
			score += 80
			reasons = append(reasons, "reconcile_name_pattern")
		}
		for _, r := range idx.fromEntity[e.ID] {
			if r.Type != domain.RelReferences || !matchesEvidenceTerms(r.To, evidenceFocusTerms(terms, req.Scope)) {
				continue
			}
			if strings.Contains(r.Evidence.Reason, "keyed struct field") {
				score += 30
				reasons = append(reasons, "typed_field_initialization")
			} else {
				score += 80
				reasons = append(reasons, "typed_field_reference")
			}
			if evidenceConditionalPattern.MatchString(r.Evidence.Snippet) {
				score += 160
				reasons = append(reasons, "conditional_site_pattern")
			}
			break
		}
		if anchor != nil && e.ID == anchor.ID {
			score += 1000
			reasons = append(reasons, "exact_subject")
		}
		selections = append(selections, evidenceSelection{entity: e, score: score, reasons: reasons})
	}
	sortEvidenceSelections(selections)
	return selections, nil, terms
}

func (idx *Index) evidenceTerms(question string) []string {
	words := evidenceWordPattern.FindAllString(question, -1)
	var technical, vocabulary []string
	for _, word := range words {
		lower := strings.ToLower(word)
		if evidenceStopWords[lower] {
			continue
		}
		signaled := strings.ContainsAny(word, "_.")
		for i, r := range word {
			if i > 0 && unicode.IsUpper(r) {
				signaled = true
			}
		}
		if signaled {
			technical = append(technical, lower)
			continue
		}
		if idx.evidenceVocabularyTerm(lower) {
			vocabulary = append(vocabulary, lower)
		}
	}
	if len(technical) > 0 {
		// Lowercase scope/entity spellings remain available alongside CamelCase
		// concepts. Arbitrary prose that happens to appear in descriptions does not.
		return uniqueStrings(append(technical, vocabulary...))
	}
	return uniqueStrings(vocabulary)
}

func (idx *Index) evidenceVocabularyTerm(term string) bool {
	if len(term) < 3 {
		return len(idx.byName[term]) > 0
	}
	return idx.evidenceVocabulary[term]
}

func buildEvidenceVocabulary(entities []domain.Entity) map[string]bool {
	result := make(map[string]bool)
	for i := range entities {
		e := &entities[i]
		for _, value := range append(append([]string{e.Name}, e.Properties...), e.Literals...) {
			for _, part := range strings.FieldsFunc(value, func(r rune) bool { return !(unicode.IsLetter(r) || unicode.IsDigit(r)) }) {
				result[strings.ToLower(part)] = true
			}
		}
	}
	return result
}

var evidenceStopWords = func() map[string]bool {
	result := make(map[string]bool)
	for _, word := range strings.Fields("a an and are as at be been being by can could do does done for from had has have how i if in into is it its me my of on or our should so that the their them there these they this those to under us was we what when where which who why will with work works working implement implements implementation implemented behavior behaviour feature explain explanation code repository project please show tell find relevant related about actually happens happen fix change changes expected actual observed result results bug issue description summary steps reproduce reproduction problem need needs want wants would must may might not no yes") {
		result[word] = true
	}
	return result
}()

func evidenceScopeMatches(e *domain.Entity, scope string) bool {
	haystack := strings.ToLower(strings.Join([]string{e.ID, e.Name, e.Package, e.Source.File}, " "))
	for _, term := range strings.Fields(strings.ToLower(scope)) {
		if !strings.Contains(haystack, term) {
			return false
		}
	}
	return true
}

func evidenceScore(e *domain.Entity, terms []string) (int, []string) {
	score := 0
	var reasons []string
	for _, term := range terms {
		best := 0
		reason := ""
		for _, field := range []struct {
			name, value string
			weight      int
		}{
			{"name", e.Name, 100}, {"identity", e.ID, 60}, {"description", e.Description, 35}, {"path", e.Source.File, 40}, {"package", e.Package, 30},
		} {
			if strings.Contains(strings.ToLower(field.value), term) && field.weight > best {
				best, reason = field.weight, field.name+":"+term
			}
		}
		for _, value := range append(append([]string(nil), e.Properties...), e.Literals...) {
			if strings.Contains(strings.ToLower(value), term) && best < 120 {
				best, reason = 120, "usage:"+term
			}
		}
		for _, site := range e.CallSites {
			if strings.Contains(strings.ToLower(site.Name), term) && best < 110 {
				best, reason = 110, "call_site:"+term
			}
		}
		if best == 0 {
			return 0, nil
		}
		score += best
		reasons = append(reasons, reason)
	}
	if e.Kind == domain.KindField {
		score += 150
		reasons = append(reasons, "field_definition")
	}
	if e.Generated {
		score -= 250
		reasons = append(reasons, "generated_file")
	}
	if strings.HasPrefix(strings.ToLower(e.Name), "with") && (e.Generated || strings.Contains(strings.ToLower(e.Source.File), "applyconfiguration")) {
		score -= 100
		reasons = append(reasons, "setter_pattern")
	}
	if score < 1 {
		score = 1
	}
	return score, reasons
}

func (idx *Index) evidenceReachable(id string, depth int) map[string]bool {
	seen := map[string]bool{id: true}
	frontier := []string{id}
	for d := 0; d < depth; d++ {
		var next []string
		for _, from := range frontier {
			for _, r := range idx.GetRelationships(from, "both", "") {
				if !evidenceEdgeType(r.Type) {
					continue
				}
				to := r.To
				if to == from {
					to = r.From
				}
				if !seen[to] {
					seen[to] = true
					next = append(next, to)
				}
			}
		}
		frontier = next
	}
	return seen
}

func evidenceEdgeType(t domain.RelationshipType) bool {
	switch t {
	case domain.RelContains, domain.RelCalls, domain.RelTestedBy, domain.RelReferences, domain.RelCreates, domain.RelReconciles, domain.RelWatches, domain.RelDocumentedIn, domain.RelImplements:
		return true
	}
	return false
}

func (idx *Index) expandEvidence(seeds []evidenceSelection, intent string, terms []string) ([]evidenceSelection, int) {
	selected := make(map[string]evidenceSelection, len(seeds))
	for _, seed := range seeds {
		selected[seed.entity.ID] = seed
	}
	frontier := append([]evidenceSelection(nil), seeds...)
	for depth := 1; depth <= 2; depth++ {
		var next []evidenceSelection
		for _, parent := range frontier {
			for _, r := range idx.GetRelationships(parent.entity.ID, "both", "") {
				if !evidenceEdgeType(r.Type) {
					continue
				}
				incoming := r.To == parent.entity.ID
				if r.Type == domain.RelReferences && !incoming && len(terms) > 0 && !matchesEvidenceTerms(r.To, terms) {
					continue
				}
				if intent == "understand" && incoming && parent.entity.Kind != domain.KindField && r.Type != domain.RelTestedBy && r.Type != domain.RelContains {
					continue
				}
				if intent == "impact" && !incoming && r.Type != domain.RelTestedBy && r.Type != domain.RelReferences {
					continue
				}
				id := r.To
				if incoming {
					id = r.From
				}
				if _, exists := selected[id]; exists {
					continue
				}
				e := idx.GetEntity(id)
				if e == nil {
					continue
				}
				// A contextual call-site bonus ranks that helper. It must not inflate
				// every test and dependency reachable from the helper as well.
				score := parent.score - parent.routingBoost - 40*depth
				reasons := []string{fmt.Sprintf("graph:%s:%s", r.Type, parent.entity.ID)}
				routingBoost := 0
				if r.Type == domain.RelCalls && !incoming {
					if distance, ok := idx.callDistanceToConcept(parent.entity.ID, r, terms); ok && distance <= 24 {
						routingBoost = 180 - distance*2
						score += routingBoost
						reasons = append(reasons, "call_near_concept_reference")
					}
				}
				if e.Kind == domain.KindTest {
					score += 20
				}
				if e.Generated {
					score -= 250
				}
				selection := evidenceSelection{entity: e, score: score, reasons: reasons, parent: r, distance: depth, routingBoost: routingBoost}
				selected[id] = selection
				next = append(next, selection)
			}
		}
		frontier = next
	}
	omitted := make(map[string]bool)
	for _, current := range frontier {
		for _, r := range idx.GetRelationships(current.entity.ID, "both", "") {
			if !evidenceEdgeType(r.Type) {
				continue
			}
			incoming := r.To == current.entity.ID
			if r.Type == domain.RelReferences && !incoming && len(terms) > 0 && !matchesEvidenceTerms(r.To, terms) {
				continue
			}
			if intent == "understand" && incoming && current.entity.Kind != domain.KindField && r.Type != domain.RelTestedBy && r.Type != domain.RelContains {
				continue
			}
			if intent == "impact" && !incoming && r.Type != domain.RelTestedBy && r.Type != domain.RelReferences {
				continue
			}
			id := r.To
			if id == current.entity.ID {
				id = r.From
			}
			if _, exists := selected[id]; !exists {
				omitted[id] = true
			}
		}
	}
	result := make([]evidenceSelection, 0, len(selected))
	for _, s := range selected {
		result = append(result, s)
	}
	sortEvidenceSelections(result)
	// Reserve the question's declaration and primary implementation before
	// contextual helpers. Prefer a test of that implementation over tests of
	// indirectly reached utilities. These are routing choices, not coverage proof.
	var prioritized []evidenceSelection
	used := make(map[string]bool)
	var implementationID string
	for _, role := range []string{"definition", "implementation"} {
		selection, ok := firstEvidenceRole(seeds, role)
		if !ok {
			selection, ok = firstEvidenceRole(result, role)
		}
		if ok {
			prioritized = append(prioritized, selection)
			used[selection.entity.ID] = true
			if role == "implementation" {
				implementationID = selection.entity.ID
			}
		}
	}
	var directTests []evidenceSelection
	for _, selection := range result {
		if selection.entity.Kind != domain.KindTest || selection.entity.Generated {
			continue
		}
		for _, r := range idx.fromEntity[implementationID] {
			if r.Type == domain.RelTestedBy && r.To == selection.entity.ID {
				directTests = append(directTests, selection)
				break
			}
		}
	}
	sortEvidenceSelections(directTests)
	selection, ok := firstEvidenceRole(directTests, "test")
	if !ok {
		selection, ok = firstEvidenceRole(seeds, "test")
	}
	if !ok {
		selection, ok = firstEvidenceRole(result, "test")
	}
	if ok {
		prioritized = append(prioritized, selection)
		used[selection.entity.ID] = true
	}
	for _, selection := range result {
		if !used[selection.entity.ID] {
			prioritized = append(prioritized, selection)
		}
	}
	return prioritized, len(omitted)
}

func firstEvidenceRole(values []evidenceSelection, role string) (evidenceSelection, bool) {
	for _, selection := range values {
		if evidenceRole(selection.entity.Kind) == role && !selection.entity.Generated {
			return selection, true
		}
	}
	return evidenceSelection{}, false
}

func sortEvidenceSelections(values []evidenceSelection) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].score != values[j].score {
			return values[i].score > values[j].score
		}
		return values[i].entity.ID < values[j].entity.ID
	})
}

func evidenceRole(k domain.EntityKind) string {
	switch k {
	case domain.KindField, domain.KindCRD, domain.KindTemplate:
		return "definition"
	case domain.KindTest:
		return "test"
	case domain.KindDocument:
		return "documentation"
	default:
		return "implementation"
	}
}

func normalizedEvidenceSource(s domain.Source) domain.Source {
	if s.EndLine < s.Line {
		s.EndLine = s.Line
	}
	return s
}

func evidenceEntity(s evidenceSelection) *EvidenceEntity {
	return &EvidenceEntity{ID: s.entity.ID, Name: s.entity.Name, Kind: s.entity.Kind.String(), Package: s.entity.Package,
		Description: truncateEvidenceString(s.entity.Description, 240), Source: normalizedEvidenceSource(s.entity.Source), Score: s.score, Reasons: uniqueStrings(s.reasons)}
}

func (idx *Index) addEvidenceSelection(packet *EvidencePacket, s evidenceSelection, terms []string) {
	if addEvidenceEntity(packet, s) {
		idx.addEvidenceEntitySources(packet, s.entity, terms)
	}
	if s.parent == nil {
		return
	}
	other := s.parent.From
	if other == s.entity.ID {
		other = s.parent.To
	}
	if anchor := idx.GetEntity(other); anchor != nil {
		anchorSelection := evidenceSelection{entity: anchor, score: s.score, reasons: []string{"relationship_endpoint"}}
		if addEvidenceEntity(packet, anchorSelection) {
			idx.addEvidenceEntitySources(packet, anchor, terms)
		}
	}
	for _, r := range packet.Relationships {
		if r.ID == s.parent.ID {
			return
		}
	}
	packet.Relationships = append(packet.Relationships, compactRelationship(s.parent))
	idx.addEvidenceEdgeSource(packet, s.parent, terms)
}

func addEvidenceEntity(packet *EvidencePacket, s evidenceSelection) bool {
	for _, e := range packet.Entities {
		if e.ID == s.entity.ID {
			return false
		}
	}
	packet.Entities = append(packet.Entities, evidenceEntity(s))
	if len(s.entity.Description) > 240 {
		addEvidenceOmission(packet, "metadata", 1)
	}
	return true
}

func (idx *Index) addEvidenceEntitySources(packet *EvidencePacket, e *domain.Entity, terms []string) {
	source := normalizedEvidenceSource(e.Source)
	if source.File == "" || source.Line <= 0 {
		addEvidenceOmission(packet, "unavailable_span", 1)
		return
	}
	focused := false
	for _, site := range e.ReferenceSites {
		if matchesEvidenceTerms(site.Target, terms) {
			addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: "usage", Source: evidenceWindow(site.Source, e.Source), FocusLine: site.Source.Line, Reason: "typed field reference occurrence"})
			focused = true
		}
	}
	for _, operation := range e.ResourceOperations {
		if distance, hasConcept := idx.callDistanceToConcept(e.ID, &domain.Relationship{Evidence: domain.Evidence{File: operation.Source.File, Line: operation.Source.Line}}, terms); hasConcept && distance > 24 {
			continue
		}
		addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: "usage", Source: evidenceWindow(operation.Source, e.Source), FocusLine: operation.Source.Line,
			Reason: fmt.Sprintf("resource operation observation: %s; object=%s; confidence=%s", operation.Operation, operation.ObjectType, operation.Confidence)})
		focused = true
	}
	for _, site := range e.CallSites {
		if matchesEvidenceTerms(site.Name, terms) {
			addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: "usage", Source: evidenceWindow(site.Source, e.Source), FocusLine: site.Source.Line, Reason: "matching call site"})
			focused = true
		}
	}
	for _, r := range idx.fromEntity[e.ID] {
		if r.Type == domain.RelReferences && matchesEvidenceTerms(r.To, terms) {
			// Older compatible fixtures may only contain the edge's first proof.
			// New scans retain all occurrences separately on the entity.
			if len(e.ReferenceSites) > 0 {
				continue
			}
			addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: "usage", Source: evidenceWindow(domain.Source{Parser: r.Evidence.Parser, File: r.Evidence.File, Line: r.Evidence.Line}, e.Source), FocusLine: r.Evidence.Line, Reason: "matching field reference"})
			focused = true
		}
	}
	if focused && source.EndLine-source.Line > 80 && (e.Kind == domain.KindFunction || e.Kind == domain.KindController) {
		source.EndLine = source.Line + 15
		addEvidenceOmission(packet, "source_window", 1)
		addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: evidenceRole(e.Kind), Source: source, Reason: "declaration context; complete span is recorded on the entity"})
		return
	}
	addEvidenceSource(packet, EvidenceSource{EntityID: e.ID, Role: evidenceRole(e.Kind), Source: source, Reason: "graph declaration span"})
}

// Source proximity is an explicit routing heuristic. It selects helpers near
// a resolved concept reference without asserting control-flow dependence.
func (idx *Index) callDistanceToConcept(entityID string, call *domain.Relationship, terms []string) (int, bool) {
	best := int(^uint(0) >> 1)
	for _, ref := range idx.fromEntity[entityID] {
		if ref.Type != domain.RelReferences || ref.Evidence.File != call.Evidence.File || !matchesEvidenceTerms(ref.To, terms) {
			continue
		}
		distance := ref.Evidence.Line - call.Evidence.Line
		if distance < 0 {
			distance = -distance
		}
		if distance < best {
			best = distance
		}
	}
	return best, best != int(^uint(0)>>1)
}

func (idx *Index) addEvidenceEdgeSource(packet *EvidencePacket, r *domain.Relationship, terms []string) {
	role := "relationship"
	reason := "relationship evidence: " + string(r.Type)
	if r.Type == domain.RelReferences {
		role = "usage"
		reason = "field reference evidence"
	}
	source := domain.Source{Parser: r.Evidence.Parser, File: r.Evidence.File, Line: r.Evidence.Line, EndLine: r.Evidence.Line}
	if role == "usage" {
		if e := idx.GetEntity(r.From); e != nil {
			source = evidenceWindow(source, e.Source)
		}
	}
	addEvidenceSource(packet, EvidenceSource{EntityID: r.From, Role: role, Source: source, FocusLine: r.Evidence.Line, Reason: reason})
}

func evidenceWindow(site, container domain.Source) domain.Source {
	site = normalizedEvidenceSource(site)
	if site.File == "" || site.Line <= 0 {
		return site
	}
	site.Column = 0
	site.Line -= 8
	if site.Line < 1 {
		site.Line = 1
	}
	site.EndLine += 10
	if site.File == container.File && container.Line > 0 {
		if site.Line < container.Line {
			site.Line = container.Line
		}
		if container.EndLine >= container.Line && site.EndLine > container.EndLine {
			site.EndLine = container.EndLine
		}
	}
	return normalizedEvidenceSource(site)
}

func matchesEvidenceTerms(value string, terms []string) bool {
	if len(terms) == 0 {
		return false
	}
	lower := strings.ToLower(value)
	for _, term := range terms {
		if !strings.Contains(lower, term) {
			return false
		}
	}
	return true
}

func evidenceFocusTerms(terms []string, scope string) []string {
	scopeTerms := make(map[string]bool)
	for _, term := range strings.Fields(strings.ToLower(scope)) {
		scopeTerms[term] = true
	}
	var focused []string
	for _, term := range terms {
		if !scopeTerms[term] {
			focused = append(focused, term)
		}
	}
	if len(focused) == 0 {
		return terms
	}
	return focused
}

func addEvidenceSource(packet *EvidencePacket, s EvidenceSource) {
	if s.Source.File == "" || s.Source.Line <= 0 {
		addEvidenceOmission(packet, "unavailable_span", 1)
		return
	}
	s.Source = normalizedEvidenceSource(s.Source)
	for _, existing := range packet.Sources {
		if existing.EntityID == s.EntityID && existing.Role == s.Role && existing.Source == s.Source && existing.FocusLine == s.FocusLine {
			return
		}
	}
	packet.Sources = append(packet.Sources, s)
}

func cloneEvidencePacket(p *EvidencePacket) *EvidencePacket {
	copy := *p
	copy.Entities = append([]*EvidenceEntity{}, p.Entities...)
	copy.Relationships = append([]*CompactRelationship{}, p.Relationships...)
	copy.Sources = append([]EvidenceSource{}, p.Sources...)
	copy.Candidates = append([]*EvidenceEntity(nil), p.Candidates...)
	copy.Omissions = append([]EvidenceOmission(nil), p.Omissions...)
	return &copy
}

func addEvidenceOmission(packet *EvidencePacket, reason string, count int) {
	if count <= 0 {
		return
	}
	for i := range packet.Omissions {
		if packet.Omissions[i].Reason == reason {
			packet.Omissions[i].Count += count
			packet.Truncated = true
			return
		}
	}
	packet.Omissions = append(packet.Omissions, EvidenceOmission{Reason: reason, Count: count})
	packet.Truncated = true
}

func setEvidenceContinuation(packet *EvidencePacket, next, total int, reason string) {
	var omissions []EvidenceOmission
	for _, omission := range packet.Omissions {
		if omission.Reason != reason {
			omissions = append(omissions, omission)
		}
	}
	packet.Omissions = omissions
	packet.NextOffset = 0
	packet.Truncated = len(omissions) > 0
	if next < total {
		packet.NextOffset = next
		addEvidenceOmission(packet, reason, total-next)
	}
}

func evidenceFits(packet *EvidencePacket) bool {
	encoded, err := json.Marshal(packet)
	return err == nil && len(encoded) <= packet.BudgetBytes
}

func evidenceFitsWithReserve(packet *EvidencePacket) bool {
	encoded, err := json.Marshal(packet)
	return err == nil && len(encoded) <= packet.BudgetBytes-192
}

func fitEvidenceEnvelope(packet *EvidencePacket) (*EvidencePacket, error) {
	if evidenceFits(packet) {
		return packet, nil
	}
	// Diagnostics themselves are budgeted. Never return an oversized packet or
	// silently remove selected facts to squeeze a final diagnostic into it.
	return nil, fmt.Errorf("budgetBytes is insufficient for evidence and omission diagnostics; increase budgetBytes")
}

func truncateEvidenceString(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	end := maxBytes - 3
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end] + "..."
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}
