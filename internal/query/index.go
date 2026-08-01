package query

import (
	"fmt"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

type Index struct {
	graph domain.Graph

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

func newIndex(g domain.Graph) *Index {
	idx := &Index{
		graph:      g,
		byID:       make(map[string]*domain.Entity, len(g.Entities)),
		byKind:     make(map[domain.EntityKind][]*domain.Entity),
		byName:     make(map[string][]*domain.Entity),
		byPackage:  make(map[string][]*domain.Entity),
		fromEntity: make(map[string][]*domain.Relationship),
		toEntity:   make(map[string][]*domain.Relationship),
		byRelType:  make(map[domain.RelationshipType][]*domain.Relationship),
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

	idx.viewByID = make(map[string]*domain.View, len(g.Views))
	idx.viewByName = make(map[string]*domain.View, len(g.Views))
	for id, v := range g.Views {
		v := v
		idx.viewByID[id] = &v
		idx.viewByName[strings.ToLower(v.EntityName)] = &v
	}

	idx.questions = g.Questions

	return idx
}

func (idx *Index) GetView(entityID string) *domain.View {
	return idx.viewByID[entityID]
}

func (idx *Index) SearchView(name string) *domain.View {
	lower := strings.ToLower(name)
	if v := idx.viewByName[lower]; v != nil {
		return v
	}
	for k, v := range idx.viewByName {
		if strings.Contains(k, lower) {
			return v
		}
	}
	return nil
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
	default:
		return domain.KindUnknown, false
	}
}
