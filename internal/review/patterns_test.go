package review

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestAnalyzePatternsUsesDeterministicPeerEvidence(t *testing.T) {
	changed := reviewTestEntity("function:example/pkg.reconcile", "reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	changed.Package = "example/pkg"
	changed.Calls = []string{"fmt.Errorf"}
	peers := []*domain.Entity{
		reviewTestEntity("function:example/pkg.helper", "helper", domain.KindFunction, "pkg/reconcile.go", 30),
		reviewTestEntity("function:example/pkg.worker", "worker", domain.KindFunction, "pkg/reconcile.go", 40),
		reviewTestEntity("function:example/pkg.cleanup", "cleanup", domain.KindFunction, "pkg/reconcile.go", 50),
	}
	for _, peer := range peers {
		peer.Package = changed.Package
		peer.Calls = []string{"fmt.Errorf"}
	}
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities:      append([]domain.Entity{*changed}, dereferenceEntities(peers)...),
	}
	idx := queryTestIndex(t, graph)

	findings := analyzePatterns(EntityReview{Entity: changed}, idx)
	if len(findings) != 2 {
		t.Fatalf("findings = %+v, want naming and error handling", findings)
	}
	if findings[0].Area != "function_naming" || findings[0].Status != patternMatchesObserved {
		t.Fatalf("naming finding = %+v", findings[0])
	}
	if findings[1].Area != "error_handling" || findings[1].Status != patternMatchesObserved {
		t.Fatalf("error finding = %+v", findings[1])
	}
	if len(findings[1].Evidence) == 0 || findings[1].Evidence[0].EntityID != changed.ID {
		t.Fatalf("error evidence = %+v", findings[1].Evidence)
	}
}

func TestAnalyzePatternsLabelsInsufficientEvidence(t *testing.T) {
	changed := reviewTestEntity("function:example/pkg.reconcile", "reconcile", domain.KindFunction, "pkg/reconcile.go", 10)
	changed.Package = "example/pkg"
	changed.Calls = []string{"fmt.Errorf"}
	peer := reviewTestEntity("function:example/pkg.helper", "helper", domain.KindFunction, "pkg/reconcile.go", 30)
	peer.Package = changed.Package
	graph := domain.Graph{Schema: "codeatlas", SchemaVersion: "1.4.0", Entities: []domain.Entity{*changed, *peer}}
	findings := analyzePatterns(EntityReview{Entity: changed}, queryTestIndex(t, graph))

	for _, finding := range findings {
		if finding.Area == "error_handling" {
			if finding.Status != patternInsufficientEvidence || !strings.Contains(finding.Summary, "insufficient") {
				t.Fatalf("error finding = %+v", finding)
			}
			return
		}
	}
	t.Fatalf("error-handling finding not found: %+v", findings)
}

func reviewTestEntity(id, name string, kind domain.EntityKind, file string, line int) *domain.Entity {
	return &domain.Entity{
		ID:   id,
		Name: name,
		Kind: kind,
		Source: domain.Source{
			Parser: "go",
			File:   file,
			Line:   line,
		},
	}
}

func dereferenceEntities(entities []*domain.Entity) []domain.Entity {
	result := make([]domain.Entity, 0, len(entities))
	for _, entity := range entities {
		result = append(result, *entity)
	}
	return result
}
