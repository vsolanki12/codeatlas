package parser

import (
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestParseFixtures(t *testing.T) {
	tests := []struct {
		name      string
		file      string
		wantTotal int
		wantFunc  int
		wantCntrl int
	}{
		{"empty file", "testdata/empty.go", 1, 0, 0},
		{"standalone functions", "testdata/functions.go", 4, 3, 0},
		{"methods", "testdata/methods.go", 3, 2, 0},
		{"controller with watches", "testdata/controller.go", 8, 6, 1},
	}

	p := NewGoParser()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entities, err := p.Parse(domain.File{RelativePath: tc.file})
			if err != nil {
				t.Fatalf("Parse() returned unexpected error: %v", err)
			}
			if len(entities) != tc.wantTotal {
				t.Errorf("Total entity count mismatch. Got %d, want %d", len(entities), tc.wantTotal)
			}

			gotFuncs := 0
			gotCntrl := 0
			for _, ent := range entities {
				switch ent.Kind {
				case domain.KindFunction:
					gotFuncs++
				case domain.KindController:
					gotCntrl++
				}
			}

			if gotFuncs != tc.wantFunc {
				t.Errorf("KindFunction count mismatch. Got %d, want %d", gotFuncs, tc.wantFunc)
			}

			if gotCntrl != tc.wantCntrl {
				t.Errorf("KindController count mismatch. Got %d, want %d", gotCntrl, tc.wantCntrl)
			}
		})
	}
}

func TestParseEntityDetails(t *testing.T) {
	p := NewGoParser()

	t.Run("function IDs and descriptions", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/functions.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		byName := make(map[string]domain.Entity)
		for _, ent := range entities {
			byName[ent.Name] = ent
		}

		if byName["utils"].ID != "package:utils" {
			t.Errorf("Package ID = %q, want %q", byName["utils"].ID, "package:utils")
		}
		if byName["Add"].ID != "function:utils.Add" {
			t.Errorf("Add ID = %q, want %q", byName["Add"].ID, "function:utils.Add")
		}
		if byName["Add"].Description == "" {
			t.Error("Add should have a doc comment description")
		}
		if byName["multiply"].Description != "" {
			t.Errorf("multiply should have no description, got %q", byName["multiply"].Description)
		}
	})

	t.Run("method IDs with receiver", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/methods.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		byName := make(map[string]domain.Entity)
		for _, ent := range entities {
			byName[ent.Name] = ent
		}

		if byName["Get"].ID != "function:cache.Store.Get" {
			t.Errorf("Get ID = %q, want %q", byName["Get"].ID, "function:cache.Store.Get")
		}
		if byName["Size"].ID != "function:cache.Store.Size" {
			t.Errorf("Size ID = %q, want %q", byName["Size"].ID, "function:cache.Store.Size")
		}
	})

	t.Run("controller watches", func(t *testing.T) {
		entities, err := p.Parse(domain.File{RelativePath: "testdata/controller.go"})
		if err != nil {
			t.Fatalf("Parse failed: %v", err)
		}

		var controller *domain.Entity
		for i := range entities {
			if entities[i].Kind == domain.KindController {
				controller = &entities[i]
				break
			}
		}

		if controller == nil {
			t.Fatal("Expected a KindController entity, found none")
		}
		if controller.ID != "controller:fake.FakeReconciler" {
			t.Errorf("Controller ID = %q, want %q", controller.ID, "controller:fake.FakeReconciler")
		}

		expectedWatches := []string{"HostedCluster", "Secret"}
		if len(controller.Watches) != len(expectedWatches) {
			t.Fatalf("Watches count mismatch. Got %v, want %v", controller.Watches, expectedWatches)
		}
		for i, w := range controller.Watches {
			if w != expectedWatches[i] {
				t.Errorf("Watch[%d] = %q, want %q", i, w, expectedWatches[i])
			}
		}
	})
}
