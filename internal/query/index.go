package query

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

type Index struct {
	graph              domain.Graph
	graphFingerprint   string
	evidenceVocabulary map[string]bool

	byID      map[string]*domain.Entity
	byKind    map[domain.EntityKind][]*domain.Entity
	byName    map[string][]*domain.Entity
	byPackage map[string][]*domain.Entity

	fromEntity map[string][]*domain.Relationship
	toEntity   map[string][]*domain.Relationship
	byRelType  map[domain.RelationshipType][]*domain.Relationship

	viewByID   map[string]*domain.View
	viewByName map[string]*domain.View
	questions  map[string]string
}

func LoadGraph(path string) (*Index, error) {
	g, err := storage.ReadGraph(path)
	if err != nil {
		return nil, fmt.Errorf("load graph: %w", err)
	}
	return newIndex(g), nil
}

// EntitiesInFile returns entities whose declared source file or implementation
// file list contains path. Unlike Where, it cannot accidentally mix similarly
// named directories and is therefore suitable for diff-to-entity mapping.
func (idx *Index) EntitiesInFile(path string, maxResults int) []*domain.Entity {
	var result []*domain.Entity
	for _, e := range idx.byID {
		if e.Source.File == path || containsString(e.Files, path) {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Source.Line == result[j].Source.Line {
			return result[i].ID < result[j].ID
		}
		return result[i].Source.Line < result[j].Source.Line
	})
	if maxResults > 0 && len(result) > maxResults {
		result = result[:maxResults]
	}
	return result
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// GraphMetadata returns the small immutable portion of the graph that
// consumers need to report freshness and scan completeness.
type GraphMetadata struct {
	SchemaVersion               string                       `json:"schemaVersion"`
	EntityIdentity              string                       `json:"entityIdentity,omitempty"`
	ExtractorVersion            string                       `json:"extractorVersion,omitempty"`
	ExtractorBuild              string                       `json:"extractorBuild,omitempty"`
	ExtractionSignature         string                       `json:"extractionSignature,omitempty"`
	BuildContext                *domain.BuildContext         `json:"buildContext,omitempty"`
	TypeAnalysis                *domain.TypeAnalysisCoverage `json:"typeAnalysis,omitempty"`
	TypeAnalysisDiagnosticCount int                          `json:"typeAnalysisDiagnosticCount,omitempty"`
	Repository                  string                       `json:"repository"`
	Commit                      string                       `json:"commit"`
	Branch                      string                       `json:"branch"`
	GeneratedAt                 string                       `json:"generatedAt"`
	ScanComplete                bool                         `json:"scanComplete"`
	ScanWarnings                []string                     `json:"scanWarnings,omitempty"`
	ScanCoverage                *domain.ScanCoverage         `json:"scanCoverage,omitempty"`
}

func (idx *Index) GraphMetadata() GraphMetadata {
	var analysis *domain.TypeAnalysisCoverage
	diagnosticCount := 0
	if idx.graph.TypeAnalysis != nil {
		copy := *idx.graph.TypeAnalysis
		diagnosticCount = copy.DiagnosticCount
		copy.Diagnostics = nil
		analysis = &copy
	}
	return GraphMetadata{
		SchemaVersion:               idx.graph.SchemaVersion,
		EntityIdentity:              idx.graph.EntityIdentity,
		ExtractorVersion:            idx.graph.ExtractorVersion,
		ExtractorBuild:              idx.graph.ExtractorBuild,
		ExtractionSignature:         idx.graph.ExtractionSignature,
		BuildContext:                idx.graph.BuildContext,
		TypeAnalysis:                analysis,
		TypeAnalysisDiagnosticCount: diagnosticCount,
		Repository:                  idx.graph.Repository,
		Commit:                      idx.graph.Commit,
		Branch:                      idx.graph.Branch,
		GeneratedAt:                 idx.graph.GeneratedAt,
		ScanComplete:                idx.graph.ScanComplete,
		ScanWarnings:                append([]string(nil), idx.graph.ScanWarnings...),
		ScanCoverage:                copyScanCoverage(idx.graph.ScanCoverage),
	}
}

func copyScanCoverage(coverage *domain.ScanCoverage) *domain.ScanCoverage {
	if coverage == nil {
		return nil
	}
	copy := *coverage
	return &copy
}

func newIndex(g domain.Graph) *Index {
	encoded, _ := json.Marshal(g)
	digest := sha256.Sum256(encoded)
	idx := &Index{
		graph:              g,
		graphFingerprint:   hex.EncodeToString(digest[:]),
		evidenceVocabulary: buildEvidenceVocabulary(g.Entities),
		byID:               make(map[string]*domain.Entity, len(g.Entities)),
		byKind:             make(map[domain.EntityKind][]*domain.Entity),
		byName:             make(map[string][]*domain.Entity),
		byPackage:          make(map[string][]*domain.Entity),
		fromEntity:         make(map[string][]*domain.Relationship),
		toEntity:           make(map[string][]*domain.Relationship),
		byRelType:          make(map[domain.RelationshipType][]*domain.Relationship),
	}

	for i := range g.Entities {
		e := &g.Entities[i]
		idx.byID[e.ID] = e
		idx.byKind[e.Kind] = append(idx.byKind[e.Kind], e)
		lower := strings.ToLower(e.Name)
		idx.byName[lower] = append(idx.byName[lower], e)
		if e.Package != "" {
			idx.byPackage[e.Package] = append(idx.byPackage[e.Package], e)
		}
	}

	for i := range g.Relationship {
		r := &g.Relationship[i]
		idx.fromEntity[r.From] = append(idx.fromEntity[r.From], r)
		idx.toEntity[r.To] = append(idx.toEntity[r.To], r)
		idx.byRelType[r.Type] = append(idx.byRelType[r.Type], r)
	}
	for _, rels := range idx.fromEntity {
		sort.Slice(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	}
	for _, rels := range idx.toEntity {
		sort.Slice(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	}
	for _, rels := range idx.byRelType {
		sort.Slice(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	}

	idx.viewByID = make(map[string]*domain.View, len(g.Views))
	idx.viewByName = make(map[string]*domain.View, len(g.Views))
	for id, v := range g.Views {
		v := v
		idx.viewByID[id] = &v
		name := strings.ToLower(v.EntityName)
		if _, exists := idx.viewByName[name]; exists {
			// A view lookup by name is only safe when the name is unique. Keep a
			// nil sentinel so map iteration order cannot select one arbitrarily.
			idx.viewByName[name] = nil
			continue
		}
		idx.viewByName[name] = &v
	}

	idx.questions = g.Questions

	return idx
}

// GraphFingerprint identifies the entire loaded graph snapshot, including its
// extraction provenance. Offsets are only meaningful within this snapshot.
func (idx *Index) GraphFingerprint() string { return idx.graphFingerprint }

func (idx *Index) GetView(entityID string) *domain.View {
	return idx.viewByID[entityID]
}

func (idx *Index) SearchView(name string) *domain.View {
	return idx.viewByName[strings.ToLower(name)]
}

// ResolveView accepts an exact view/entity ID or an exact entity name only
// when that name identifies one graph entity. A view must never be selected
// merely because a different entity happens to share its display name.
func (idx *Index) ResolveView(value string) *domain.View {
	if view, ok := idx.viewByID[value]; ok {
		return view
	}
	candidates := idx.byName[strings.ToLower(value)]
	if len(candidates) != 1 {
		return nil
	}
	return idx.viewByID[candidates[0].ID]
}

func (idx *Index) LookupQuestion(verb, subject string) (string, bool) {
	if idx.questions == nil {
		return "", false
	}
	key := verb + ":" + subject
	ans, ok := idx.questions[key]
	return ans, ok
}

func ParseKind(s string) (domain.EntityKind, bool) {
	switch strings.ToLower(s) {
	case "operator":
		return domain.KindOperator, true
	case "controller":
		return domain.KindController, true
	case "crd":
		return domain.KindCRD, true
	case "function":
		return domain.KindFunction, true
	case "package":
		return domain.KindPackage, true
	case "test":
		return domain.KindTest, true
	case "document":
		return domain.KindDocument, true
	case "resource":
		return domain.KindResource, true
	case "template":
		return domain.KindTemplate, true
	case "field":
		return domain.KindField, true
	default:
		return domain.KindUnknown, false
	}
}
