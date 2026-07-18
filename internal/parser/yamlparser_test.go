package parser

import (
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func TestYAMLParser_Parse_Resource(t *testing.T) {
	yp := NewYAMLParser()
	target := domain.File{RelativePath: "testdata/deployment.yaml"}

	entities, err := yp.Parse(target)
	if err != nil {
		t.Fatalf("YAMLParser failed unexpectedly: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected exactly 1 entity, got %d", len(entities))
	}

	ent := entities[0]
	if ent.Kind != domain.KindResource {
		t.Errorf("Expected KindResource, got %v", ent.Kind)
	}

	expectedID := "resource:deployment.atlas-api"
	if ent.ID != expectedID {
		t.Errorf("Resource ID mismatch.\nExpected: %q\nGot: %q", expectedID, ent.ID)
	}

	if ent.Package != "hyper-system" {
		t.Errorf("Expected package to represent namespace 'hyper-system', got %q", ent.Package)
	}
}

func TestYAMLParser_Parse_CRD(t *testing.T) {
	yp := NewYAMLParser()
	target := domain.File{RelativePath: "testdata/crd.yaml"}

	entities, err := yp.Parse(target)
	if err != nil {
		t.Fatalf("YAMLParser failed unexpectedly: %v", err)
	}

	if len(entities) != 1 {
		t.Fatalf("Expected exactly 1 entity, got %d", len(entities))
	}

	ent := entities[0]
	if ent.Kind != domain.KindCRD {
		t.Errorf("Expected KindCRD, got %v", ent.Kind)
	}

	expectedID := "crd:hypershift.openshift.io.hostedcluster"
	if ent.ID != expectedID {
		t.Errorf("CRD ID mismatch.\nExpected: %q\nGot: %q", expectedID, ent.ID)
	}

	if ent.Name != "HostedCluster" {
		t.Errorf("Expected Name to be 'HostedCluster', got %q", ent.Name)
	}
}
