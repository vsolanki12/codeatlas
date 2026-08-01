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

	from := make(map[string][]domain.Relationship, len(rels)/4)
	to := make(map[string][]domain.Relationship, len(rels)/4)
	for _, r := range rels {
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
		}
	}

	return views
}

func compileController(e *domain.Entity, outRels, inRels []domain.Relationship, byID map[string]*domain.Entity) domain.View {
	v := baseView(e)

	v.Watches = e.Watches

	for _, r := range outRels {
		switch r.Type {
		case domain.RelReconciles:
			if t := byID[r.To]; t != nil {
				v.Reconciles = t.Name
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

	existingCreates := make(map[string]bool, len(v.Creates))
	for _, c := range v.Creates {
		existingCreates[c] = true
	}
	for _, p := range e.Properties {
		if strings.HasPrefix(p, "creates:") {
			name := strings.TrimPrefix(p, "creates:")
			if !existingCreates[name] {
				existingCreates[name] = true
				v.Creates = append(v.Creates, name)
			}
		}
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

	for _, r := range outRels {
		if r.Type == domain.RelTestedBy {
			if t := byID[r.To]; t != nil {
				v.Tests = append(v.Tests, t.Name)
			}
		}
	}

	for _, r := range inRels {
		switch r.Type {
		case domain.RelReconciles:
			if t := byID[r.From]; t != nil {
				v.ReconciledBy = t.Name
			}
		case domain.RelCreates:
			if t := byID[r.From]; t != nil {
				v.CreatedBy = append(v.CreatedBy, t.Name)
			}
		}
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
		Files:       e.Files,
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
	for _, v := range views {
		name := v.EntityName
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
	}
	return qa
}

func sortAll(v *domain.View) {
	sort.Strings(v.Creates)
	sort.Strings(v.Calls)
	sort.Strings(v.Tests)
	sort.Strings(v.CreatedBy)
	sort.Strings(v.CalledBy)
	sort.Strings(v.Files)
	sort.Strings(v.Owners)
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
