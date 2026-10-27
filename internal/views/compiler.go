package views

import (
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// Compile generates pre-computed views for controllers and CRDs.
func Compile(entities []domain.Entity, rels []domain.Relationship) map[string]domain.View {
	byID := make(map[string]*domain.Entity, len(entities))
	for i := range entities {
		byID[entities[i].ID] = &entities[i]
	}
	sortedRels := append([]domain.Relationship(nil), rels...)
	sort.Slice(sortedRels, func(i, j int) bool {
		return sortedRels[i].ID < sortedRels[j].ID
	})

	from := make(map[string][]domain.Relationship, len(sortedRels)/4)
	to := make(map[string][]domain.Relationship, len(sortedRels)/4)
	for _, r := range sortedRels {
		from[r.From] = append(from[r.From], r)
		to[r.To] = append(to[r.To], r)
	}

	views := make(map[string]domain.View)

	for i := range entities {
		e := &entities[i]
		switch e.Kind {
		case domain.KindController:
			views[e.ID] = compileController(e, from[e.ID], to[e.ID], byID)
		case domain.KindCRD:
			views[e.ID] = compileCRD(e, from[e.ID], to[e.ID], byID)
		case domain.KindType, domain.KindInterface, domain.KindPackage:
			views[e.ID] = compileStructure(e, from[e.ID], to[e.ID], byID)
		}
	}

	return views
}

// memberLimit bounds the denormalized name lists on a structural view. A
// generated package can declare hundreds of types; a view is a summary, and
// the complete set is always available by traversing the graph itself.
const memberLimit = 50

// compileStructure builds the view for a Go type, interface, or package.
// These are the entities a reader of an unfamiliar repository starts from,
// and before they existed the view layer produced nothing at all for a
// repository without controllers.
func compileStructure(e *domain.Entity, outRels, inRels []domain.Relationship, byID map[string]*domain.Entity) domain.View {
	v := baseView(e)
	appendViewRelationships(&v, outRels, "outgoing", byID)
	appendViewRelationships(&v, inRels, "incoming", byID)

	for _, r := range outRels {
		target := byID[r.To]
		if target == nil {
			continue
		}
		switch r.Type {
		case domain.RelImplements:
			v.Implements = appendUniqueName(v.Implements, target.Name)
		case domain.RelContains:
			switch target.Kind {
			case domain.KindFunction:
				v.Methods = appendUniqueName(v.Methods, target.Name)
			case domain.KindField:
				v.Fields = appendUniqueName(v.Fields, target.Name)
			}
		case domain.RelImports:
			v.Imports = appendUniqueName(v.Imports, target.Name)
		case domain.RelTestedBy:
			v.Tests = appendUniqueName(v.Tests, target.Name)
		case domain.RelCalls:
			v.Calls = appendUniqueName(v.Calls, target.Name)
		}
	}

	for _, r := range inRels {
		source := byID[r.From]
		if source == nil {
			continue
		}
		switch r.Type {
		case domain.RelImplements:
			v.ImplementedBy = appendUniqueName(v.ImplementedBy, source.Name)
		case domain.RelImports:
			v.ImportedBy = appendUniqueName(v.ImportedBy, source.Name)
		case domain.RelCalls:
			v.CalledBy = appendUniqueName(v.CalledBy, source.Name)
		}
	}

	for _, list := range []*[]string{&v.Implements, &v.ImplementedBy, &v.Methods, &v.Fields, &v.Imports, &v.ImportedBy, &v.Calls, &v.CalledBy} {
		if len(*list) > memberLimit {
			*list = (*list)[:memberLimit]
		}
	}
	v.TestCount = len(v.Tests)
	sortAll(&v)
	return v
}

func compileController(e *domain.Entity, outRels, inRels []domain.Relationship, byID map[string]*domain.Entity) domain.View {
	v := baseView(e)
	appendViewRelationships(&v, outRels, "outgoing", byID)
	appendViewRelationships(&v, inRels, "incoming", byID)

	// Views are sorted independently for display; copy the slice so this does
	// not reorder the entity's aligned watchMethods/watchSites facts.
	v.Watches = append([]string(nil), e.Watches...)

	var reconciles []string
	for _, r := range outRels {
		switch r.Type {
		case domain.RelReconciles:
			if t := byID[r.To]; t != nil {
				reconciles = appendUniqueName(reconciles, t.Name)
			}
		case domain.RelCreates:
			if t := byID[r.To]; t != nil {
				v.Creates = append(v.Creates, t.Name)
			}
		case domain.RelCalls:
			if t := byID[r.To]; t != nil {
				v.Calls = append(v.Calls, t.Name)
			}
		case domain.RelTestedBy:
			if t := byID[r.To]; t != nil {
				v.Tests = append(v.Tests, t.Name)
			}
		}
	}
	if len(reconciles) == 1 {
		v.Reconciles = reconciles[0]
	}

	for _, r := range inRels {
		if r.Type == domain.RelCalls {
			if t := byID[r.From]; t != nil {
				v.CalledBy = append(v.CalledBy, t.Name)
			}
		}
	}

	if len(v.Calls) > 10 {
		v.Calls = v.Calls[:10]
	}
	v.TestCount = len(v.Tests)
	sortAll(&v)
	return v
}

func compileCRD(e *domain.Entity, outRels, inRels []domain.Relationship, byID map[string]*domain.Entity) domain.View {
	v := baseView(e)
	appendViewRelationships(&v, outRels, "outgoing", byID)
	appendViewRelationships(&v, inRels, "incoming", byID)

	for _, r := range outRels {
		if r.Type == domain.RelTestedBy {
			if t := byID[r.To]; t != nil {
				v.Tests = append(v.Tests, t.Name)
			}
		}
	}

	var reconciledBy []string
	for _, r := range inRels {
		switch r.Type {
		case domain.RelReconciles:
			if t := byID[r.From]; t != nil {
				reconciledBy = appendUniqueName(reconciledBy, t.Name)
			}
		case domain.RelCreates:
			if t := byID[r.From]; t != nil {
				v.CreatedBy = append(v.CreatedBy, t.Name)
			}
		}
	}
	if len(reconciledBy) == 1 {
		v.ReconciledBy = reconciledBy[0]
	}

	v.TestCount = len(v.Tests)
	sortAll(&v)
	return v
}

func baseView(e *domain.Entity) domain.View {
	v := domain.View{
		EntityID:    e.ID,
		EntityName:  e.Name,
		Kind:        e.Kind.String(),
		Package:     e.Package,
		File:        e.Source.File,
		Description: e.Description,
		Files:       append([]string(nil), e.Files...),
	}
	if e.LastAuthor != "" {
		v.LastAuthor = e.LastAuthor
		v.Owners = []string{e.LastAuthor}
	}
	if e.LastModified != "" {
		v.LastModified = e.LastModified
	}
	v.ChangeCount = e.ChangeCount
	return v
}

// CompileQuestions generates deterministic Q&A pairs from pre-computed views.
func CompileQuestions(views map[string]domain.View) map[string]string {
	qa := make(map[string]string, len(views)*3)
	ids := make([]string, 0, len(views))
	for id := range views {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	nameCounts := make(map[string]int, len(ids))
	qualifiedCounts := make(map[string]int, len(ids))
	for _, id := range ids {
		nameCounts[strings.ToLower(views[id].EntityName)]++
		if qualified := qualifiedName(views[id]); qualified != "" {
			qualifiedCounts[strings.ToLower(qualified)]++
		}
	}
	for _, id := range ids {
		v := views[id]
		// A name-keyed Q&A is safe only when the graph proves that the
		// name identifies one view. Never let map iteration decide which
		// same-named controller/CRD wins. A repository that declares the same
		// type name in many packages still answers under the qualified name.
		var subjects []string
		if nameCounts[strings.ToLower(v.EntityName)] == 1 {
			subjects = append(subjects, v.EntityName)
		}
		if qualified := qualifiedName(v); qualified != "" && qualifiedCounts[strings.ToLower(qualified)] == 1 {
			subjects = append(subjects, qualified)
		}
		for _, name := range subjects {
			if v.Reconciles != "" {
				qa["reconciles:"+name] = v.Reconciles
			}
			if v.ReconciledBy != "" {
				qa["reconciled-by:"+name] = v.ReconciledBy
			}
			if len(v.Creates) > 0 {
				qa["creates:"+name] = join(v.Creates)
			}
			if len(v.CreatedBy) > 0 {
				qa["created-by:"+name] = join(v.CreatedBy)
			}
			if len(v.Tests) > 0 {
				qa["tests:"+name] = join(v.Tests)
			}
			if len(v.Owners) > 0 {
				qa["owns:"+name] = join(v.Owners)
			}
			if len(v.Files) > 0 {
				qa["files:"+name] = join(v.Files)
			}
			if len(v.Watches) > 0 {
				qa["watches:"+name] = join(v.Watches)
			}
			if len(v.Implements) > 0 {
				qa["implements:"+name] = join(v.Implements)
			}
			if len(v.ImplementedBy) > 0 {
				qa["implemented-by:"+name] = join(v.ImplementedBy)
			}
			if len(v.Methods) > 0 {
				qa["methods:"+name] = join(v.Methods)
			}
			if len(v.Fields) > 0 {
				qa["fields:"+name] = join(v.Fields)
			}
			if len(v.Imports) > 0 {
				qa["imports:"+name] = join(v.Imports)
			}
			if len(v.ImportedBy) > 0 {
				qa["imported-by:"+name] = join(v.ImportedBy)
			}
		}
	}
	return qa
}

func qualifiedName(v domain.View) string {
	return domain.QualifiedName(v.EntityID, v.Package, v.EntityName)
}

func sortAll(v *domain.View) {
	sort.Strings(v.Watches)
	sort.Strings(v.Creates)
	sort.Strings(v.Calls)
	sort.Strings(v.Tests)
	sort.Strings(v.CreatedBy)
	sort.Strings(v.CalledBy)
	sort.Strings(v.Files)
	sort.Strings(v.Owners)
	sort.Strings(v.Implements)
	sort.Strings(v.ImplementedBy)
	sort.Strings(v.Methods)
	sort.Strings(v.Fields)
	sort.Strings(v.Imports)
	sort.Strings(v.ImportedBy)
	sort.Slice(v.Relationships, func(i, j int) bool {
		if v.Relationships[i].ID != v.Relationships[j].ID {
			return v.Relationships[i].ID < v.Relationships[j].ID
		}
		return v.Relationships[i].Direction < v.Relationships[j].Direction
	})
}

func appendViewRelationships(view *domain.View, rels []domain.Relationship, direction string, byID map[string]*domain.Entity) {
	for _, rel := range rels {
		entityID := rel.To
		if direction == "incoming" {
			entityID = rel.From
		}
		entity := byID[entityID]
		if entity == nil {
			continue
		}
		view.Relationships = append(view.Relationships, domain.ViewRelationship{
			ID:         rel.ID,
			Direction:  direction,
			Type:       rel.Type,
			EntityID:   entity.ID,
			EntityName: entity.Name,
			Confidence: rel.Confidence,
			Evidence:   rel.Evidence,
		})
	}
}

func join(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += ", " + s
	}
	return result
}

func appendUniqueName(names []string, name string) []string {
	for _, existing := range names {
		if existing == name {
			return names
		}
	}
	return append(names, name)
}
