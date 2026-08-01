package query

import (
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

type AskResult struct {
	Entity        *domain.Entity
	View          *domain.View
	QAHit         string
	Explanation   *ExplainResult
	Impact        *ImpactResult
	Investigation *InvestigateResult
}

func (idx *Index) Ask(entity string, intent string) *AskResult {
	e := idx.GetEntity(entity)
	if e == nil {
		results := idx.Search(entity, 1)
		if len(results) > 0 {
			e = results[0]
		}
	}
	if e == nil {
		return nil
	}

	r := &AskResult{Entity: e}
	r.View = idx.GetView(e.ID)
	if r.View == nil {
		r.View = idx.SearchView(e.Name)
	}

	switch strings.ToLower(intent) {
	case "understand":
		r.Explanation = idx.Explain(e.ID, 2)
	case "impact":
		r.Impact = idx.Impact(e.ID)
	case "debug":
		r.Investigation = idx.Investigate(e.ID)
	}

	return r
}
