package query

import (
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

type AskResult struct {
	Graph         GraphMetadata          `json:"graph"`
	Status        string                 `json:"status"`
	Entity        *domain.Entity         `json:"entity"`
	Candidates    []*domain.Entity       `json:"candidates,omitempty"`
	Match         string                 `json:"match"`
	Ambiguous     bool                   `json:"ambiguous,omitempty"`
	View          *domain.View           `json:"view,omitempty"`
	QAHit         string                 `json:"quickAnswer,omitempty"`
	QAEvidence    []*domain.Relationship `json:"quickAnswerEvidence,omitempty"`
	Explanation   *ExplainResult         `json:"explanation,omitempty"`
	Impact        *ImpactResult          `json:"impact,omitempty"`
	Investigation *InvestigateResult     `json:"investigation,omitempty"`
	Detail        bool                   `json:"detail,omitempty"`
}

func (idx *Index) Ask(entity string, intent string) *AskResult {
	e, candidates := idx.Resolve(entity)
	if e == nil {
		status := "no_match"
		if len(candidates) > 0 {
			status = "ambiguous"
		}
		return &AskResult{
			Graph:      idx.GraphMetadata(),
			Status:     status,
			Candidates: candidates,
			Match:      status,
			Ambiguous:  len(candidates) > 0,
		}
	}
	match := "id"
	if entity != e.ID {
		match = "name"
	}

	r := &AskResult{Graph: idx.GraphMetadata(), Status: "ok", Entity: e, Match: match}
	r.View = idx.ResolveView(e.ID)

	switch strings.ToLower(intent) {
	case "understand":
		if ans, ok := idx.LookupQuestion("reconciles", e.Name); ok {
			if evidence := idx.questionEvidence(e.ID, "reconciles"); len(evidence) > 0 {
				r.QAHit = ans
				r.QAEvidence = evidence
			}
		}
		r.Explanation = idx.Explain(e.ID, 2)
	case "impact":
		r.Impact = idx.Impact(e.ID)
	case "debug":
		r.Investigation = idx.Investigate(e.ID)
	default:
		for _, verb := range []string{"reconciles", "creates", "tests", "watches"} {
			if ans, ok := idx.LookupQuestion(verb, e.Name); ok {
				if evidence := idx.questionEvidence(e.ID, verb); len(evidence) > 0 {
					r.QAHit = verb + ": " + ans
					r.QAEvidence = evidence
					break
				}
			}
		}
	}

	return r
}

func (idx *Index) questionEvidence(entityID, verb string) []*domain.Relationship {
	typeName := domain.RelationshipType(verb)
	direction := "from"
	switch verb {
	case "reconciled-by":
		typeName = domain.RelReconciles
		direction = "to"
	case "created-by":
		typeName = domain.RelCreates
		direction = "to"
	case "tests":
		typeName = domain.RelTestedBy
	case "watches":
		typeName = domain.RelWatches
	case "owns":
		typeName = domain.RelOwns
	}
	return idx.GetRelationships(entityID, direction, string(typeName))
}
