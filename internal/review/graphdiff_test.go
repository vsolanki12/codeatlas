package review

import (
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestGraphChangesKeepBaseEvidenceAndRestrictToDiffPaths(t *testing.T) {
	removed := domain.Entity{ID: "function:example/pkg.Removed", Name: "Removed", Kind: domain.KindFunction, Source: domain.Source{Parser: "go", File: "pkg/deleted.go", Line: 4, EndLine: 9}}
	kept := domain.Entity{ID: "function:example/pkg.Kept", Name: "Kept", Kind: domain.KindFunction, Source: domain.Source{Parser: "go", File: "pkg/kept.go", Line: 1}}
	unrelated := domain.Entity{ID: "function:example/other.Unrelated", Name: "Unrelated", Kind: domain.KindFunction, Source: domain.Source{Parser: "go", File: "other/file.go", Line: 1}}
	rel := domain.Relationship{ID: domain.NewRelationshipID(removed.ID, domain.RelCalls, kept.ID), From: removed.ID, To: kept.ID, Type: domain.RelCalls, Confidence: domain.ConfidenceProven, Evidence: domain.Evidence{Parser: "go", File: "pkg/deleted.go", Line: 5, Snippet: "Kept()", Reason: "resolved call"}}
	base := queryTestIndex(t, domain.Graph{Schema: "codeatlas", SchemaVersion: domain.CurrentSchemaVersion, Entities: []domain.Entity{removed, kept, unrelated}, Relationship: []domain.Relationship{rel}})
	head := queryTestIndex(t, domain.Graph{Schema: "codeatlas", SchemaVersion: domain.CurrentSchemaVersion, Entities: []domain.Entity{kept}})
	changes := AnalyzeGraphChanges(base, head, []FileDiff{{Path: "pkg/deleted.go", OldPath: "pkg/deleted.go", Status: FileDeleted}})
	if len(changes.RemovedEntities) != 1 || changes.RemovedEntities[0].ID != removed.ID {
		t.Fatalf("base entities = %+v; expected only the changed-path entity", changes.RemovedEntities)
	}
	if len(changes.RemovedRelationships) != 1 || changes.RemovedRelationships[0].Evidence != rel.Evidence {
		t.Fatalf("lost base relationship evidence: %+v", changes.RemovedRelationships)
	}
	compact := CompactReview(&ReviewResult{GraphChanges: changes}, false)
	if compact.GraphChanges == nil || len(compact.GraphChanges.RemovedEntities) != 1 {
		t.Fatal("compact review omitted base graph evidence")
	}
}

func TestGraphChangesDoNotCallSurvivingIdentitiesRemoved(t *testing.T) {
	entity := domain.Entity{ID: "function:example/pkg.Kept", Name: "Kept", Kind: domain.KindFunction, Source: domain.Source{Parser: "go", File: "pkg/file.go", Line: 1}}
	idx := queryTestIndex(t, domain.Graph{Schema: "codeatlas", SchemaVersion: domain.CurrentSchemaVersion, Entities: []domain.Entity{entity}})
	changes := AnalyzeGraphChanges(idx, idx, []FileDiff{{Path: "pkg/file.go", Status: FileModified}})
	if len(changes.RemovedEntities) != 0 || len(changes.RemovedRelationships) != 0 {
		t.Fatalf("identical graphs produce removals: %+v", changes)
	}
}
