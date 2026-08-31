package review

import (
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func TestMapToEntities_MapsAdditionalImplementationFileApproximately(t *testing.T) {
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities: []domain.Entity{
			{
				ID:    "controller:example.com/controllers.WidgetReconciler",
				Name:  "WidgetReconciler",
				Kind:  domain.KindController,
				Files: []string{"controllers/reconcile.go", "controllers/setup.go"},
				Source: domain.Source{
					Parser:  "go",
					File:    "controllers/reconcile.go",
					Line:    10,
					EndLine: 30,
				},
			},
		},
	}

	changed, unmapped := MapToEntities([]FileDiff{{
		Path:   "controllers/setup.go",
		Status: FileModified,
		Hunks:  []Hunk{{NewStart: 50, NewCount: 2}},
	}}, queryTestIndex(t, graph))

	if len(unmapped) != 0 {
		t.Fatalf("unexpected unmapped files: %v", unmapped)
	}
	if len(changed) != 1 || changed[0].Entity.ID != "controller:example.com/controllers.WidgetReconciler" {
		t.Fatalf("unexpected changed entities: %+v", changed)
	}
	if !changed[0].Approximate {
		t.Fatal("additional implementation file should be marked approximate")
	}
}

func TestMapToEntitiesReportsUnmappedNonGoFiles(t *testing.T) {
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities: []domain.Entity{{
			ID:   "function:example.com/controllers.Reconcile",
			Name: "Reconcile",
			Kind: domain.KindFunction,
			Source: domain.Source{
				Parser: "go",
				File:   "controllers/reconcile.go",
				Line:   10,
			},
		}},
	}
	_, unmapped := MapToEntities([]FileDiff{{
		Path:   "config/policy.json",
		Status: FileModified,
	}}, queryTestIndex(t, graph))
	if len(unmapped) != 1 || unmapped[0] != "config/policy.json" {
		t.Fatalf("unmapped files = %v, want the unsupported changed file", unmapped)
	}
}

func TestAnalyzeAggregatesMergedEntityChangesAcrossFiles(t *testing.T) {
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities: []domain.Entity{
			{
				ID:    "function:example.com/controllers.Reconcile",
				Name:  "Reconcile",
				Kind:  domain.KindFunction,
				Files: []string{"controllers/reconcile.go", "controllers/helpers.go"},
				Source: domain.Source{
					Parser:  "go",
					File:    "controllers/reconcile.go",
					Line:    10,
					EndLine: 30,
				},
			},
		},
	}
	result := Analyze([]FileDiff{
		{
			Path:   "controllers/reconcile.go",
			Status: FileModified,
			Hunks:  []Hunk{{NewStart: 12, NewCount: 2}},
		},
		{
			Path:   "controllers/helpers.go",
			Status: FileModified,
			Hunks:  []Hunk{{NewStart: 50, NewCount: 2}},
		},
	}, queryTestIndex(t, graph), "base", "head")

	if len(result.Functions) != 1 {
		t.Fatalf("changed entities = %d, want 1: %+v", len(result.Functions), result.Functions)
	}
	entity := result.Functions[0]
	if len(entity.ChangedFiles) != 2 || entity.ChangedFiles[0] != "controllers/reconcile.go" || entity.ChangedFiles[1] != "controllers/helpers.go" {
		t.Fatalf("changed files = %v, want both implementation files", entity.ChangedFiles)
	}
	if len(entity.Hunks) != 2 || !entity.Approximate {
		t.Fatalf("aggregated hunks/approximation = %d/%v, want 2/true", len(entity.Hunks), entity.Approximate)
	}
}

func TestMapToEntitiesIgnoresForeignFileEntriesWhenFindingNextSpan(t *testing.T) {
	graph := domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities: []domain.Entity{
			{
				ID:    "package:example.com/controllers",
				Name:  "controllers",
				Kind:  domain.KindPackage,
				Files: []string{"controllers/reconcile.go"},
				Source: domain.Source{
					Parser: "go",
					File:   "controllers",
					Line:   1,
				},
			},
			{
				ID:   "function:example.com/controllers.Reconcile",
				Name: "Reconcile",
				Kind: domain.KindFunction,
				Source: domain.Source{
					Parser:  "go",
					File:    "controllers/reconcile.go",
					Line:    10,
					EndLine: 18,
				},
			},
			{
				ID:   "function:example.com/controllers.Helper",
				Name: "Helper",
				Kind: domain.KindFunction,
				Source: domain.Source{
					Parser:  "go",
					File:    "controllers/reconcile.go",
					Line:    25,
					EndLine: 30,
				},
			},
		},
	}

	changed, _ := MapToEntities([]FileDiff{{
		Path:   "controllers/reconcile.go",
		Status: FileModified,
		Hunks:  []Hunk{{NewStart: 12, NewCount: 2}},
	}}, queryTestIndex(t, graph))
	if len(changed) != 1 || changed[0].Entity.ID != "function:example.com/controllers.Reconcile" {
		t.Fatalf("unexpected span mapping: %+v", changed)
	}
}

func TestMapToEntitiesScopesAddedLinesToMappedEntity(t *testing.T) {
	first := domain.Entity{
		ID:   "function:example.com/controllers.First",
		Name: "First",
		Kind: domain.KindFunction,
		Source: domain.Source{
			Parser:  "go",
			File:    "controllers/reconcile.go",
			Line:    10,
			EndLine: 20,
		},
	}
	second := domain.Entity{
		ID:   "function:example.com/controllers.Second",
		Name: "Second",
		Kind: domain.KindFunction,
		Source: domain.Source{
			Parser:  "go",
			File:    "controllers/reconcile.go",
			Line:    30,
			EndLine: 40,
		},
	}
	changed, _ := MapToEntities([]FileDiff{{
		Path:   "controllers/reconcile.go",
		Status: FileModified,
		Hunks:  []Hunk{{NewStart: 10, NewCount: 31}},
		AddedContent: []AddedLine{
			{File: "controllers/reconcile.go", Line: 12, Text: "fmt.Errorf(\"first\")"},
			{File: "controllers/reconcile.go", Line: 32, Text: "fmt.Errorf(\"second\")"},
		},
	}}, queryTestIndex(t, domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		Entities:      []domain.Entity{first, second},
	}))
	if len(changed) != 2 {
		t.Fatalf("changed entities = %d, want 2: %+v", len(changed), changed)
	}
	if len(changed[0].AddedLines) != 1 || changed[0].AddedLines[0].Line != 12 {
		t.Fatalf("first entity added lines = %+v", changed[0].AddedLines)
	}
	if len(changed[1].AddedLines) != 1 || changed[1].AddedLines[0].Line != 32 {
		t.Fatalf("second entity added lines = %+v", changed[1].AddedLines)
	}
}

func TestIsAddedEntityRecognizesNewFunctionDeclaration(t *testing.T) {
	entity := reviewTestEntity("function:example.com/pkg.NewFunction", "NewFunction", domain.KindFunction, "pkg/file.go", 10)
	if !isAddedEntity(ChangedEntity{
		Entity:     entity,
		Path:       "pkg/file.go",
		Hunks:      []Hunk{{OldStart: 7, OldCount: 3, NewStart: 7, NewCount: 4}},
		AddedLines: []AddedLine{{File: "pkg/file.go", Line: 10, Text: "func NewFunction() {}"}},
	}) {
		t.Fatal("new function declaration should be marked added")
	}

	existing := reviewTestEntity("function:example.com/pkg.Reconcile", "Reconcile", domain.KindFunction, "pkg/file.go", 10)
	if isAddedEntity(ChangedEntity{
		Entity:       existing,
		Path:         "pkg/file.go",
		Hunks:        []Hunk{{OldStart: 10, OldCount: 1, NewStart: 10, NewCount: 1}},
		AddedLines:   []AddedLine{{File: "pkg/file.go", Line: 10, Text: "func Reconcile() {}"}},
		DeletedLines: []DeletedLine{{File: "pkg/file.go", Line: 10, Text: "func Reconcile() {"}},
	}) {
		t.Fatal("existing function signature should not be marked added")
	}
}

func queryTestIndex(t *testing.T, graph domain.Graph) *query.Index {
	t.Helper()
	path := filepath.Join(t.TempDir(), "graph.json")
	if err := storage.WriteGraph(path, graph); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	idx, err := query.LoadGraph(path)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}
	return idx
}
