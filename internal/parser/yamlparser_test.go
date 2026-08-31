package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestYAMLParser_IgnoresNonKubernetesDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "playbook.yml")
	content := []byte("- name: cleanup\n  hosts: localhost\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	entities, err := NewYAMLParser().Parse(domain.File{RelativePath: "playbook.yml"})
	if err != nil {
		t.Fatalf("non-Kubernetes YAML should be ignored, got error: %v", err)
	}
	if len(entities) != 0 {
		t.Fatalf("got %d entities for non-Kubernetes YAML, want 0", len(entities))
	}
}

func TestYAMLParser_ParseMultipleDocuments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "resources.yaml")
	content := []byte("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: first\n---\napiVersion: v1\nkind: Secret\nmetadata:\n  name: second\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	t.Chdir(dir)
	entities, err := NewYAMLParser().Parse(domain.File{RelativePath: "resources.yaml"})
	if err != nil {
		t.Fatalf("multi-document YAML should parse, got error: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("got %d entities, want 2", len(entities))
	}
	if entities[0].Source.Line != 2 || entities[1].Source.Line != 7 {
		t.Fatalf("unexpected document source lines: %d, %d", entities[0].Source.Line, entities[1].Source.Line)
	}
}

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

func TestYAMLParser_Properties(t *testing.T) {
	yp := NewYAMLParser()
	entities, err := yp.Parse(domain.File{RelativePath: "testdata/service.yaml"})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("Expected 1 entity, got %d", len(entities))
	}

	ent := entities[0]
	if len(ent.Properties) == 0 {
		t.Fatal("Expected Properties to be populated for Service resource")
	}

	propSet := make(map[string]bool)
	for _, p := range ent.Properties {
		propSet[p] = true
	}

	if !propSet["spec.selector.app=atlas-api"] {
		t.Errorf("Expected property 'spec.selector.app=atlas-api', got %v", ent.Properties)
	}
	if !propSet["kind=Service"] {
		t.Errorf("Expected property 'kind=Service', got %v", ent.Properties)
	}
	if !propSet["metadata.name=atlas-routing-service"] {
		t.Errorf("Expected property 'metadata.name=atlas-routing-service', got %v", ent.Properties)
	}
	// annotations should be skipped
	for _, p := range ent.Properties {
		if len(p) > 11 && p[:11] == "annotations" {
			t.Errorf("annotations should be skipped, found %q", p)
		}
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

	expectedDesc := "HostedCluster is the primary resource for a HyperShift-managed cluster."
	if ent.Description != expectedDesc {
		t.Errorf("CRD description mismatch.\nExpected: %q\nGot: %q", expectedDesc, ent.Description)
	}
}

func TestYAMLParser_TemplatePreservesShapeWithoutInventingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deployment.yaml")
	content := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
spec:
  replicas: {{ .Replicas }}
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	entities, err := NewYAMLParserForRepo(dir).Parse(domain.File{RelativePath: "deployment.yaml"})
	if err != nil {
		t.Fatalf("templated Kubernetes YAML should parse statically, got error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want one template entity", len(entities))
	}
	entity := entities[0]
	if entity.Kind != domain.KindTemplate {
		t.Fatalf("kind = %s, want template", entity.Kind)
	}
	if entity.ID != "template:kubernetes.deployment@deployment.yaml#1" {
		t.Fatalf("template ID = %q, want deterministic file-based ID", entity.ID)
	}
	if entity.Name != "Deployment template" || entity.Source.Parser != "yaml-template" {
		t.Fatalf("template entity = %+v", entity)
	}
	if entity.Source.File != "deployment.yaml" || entity.Source.Line != 1 {
		t.Fatalf("template source = %+v", entity.Source)
	}
}

func TestYAMLParser_TemplateControlLinesAreNotParseFailures(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.yaml")
	content := []byte(`{{- if .Enabled }}
apiVersion: v1
kind: Service
metadata:
  name: service
{{- end }}
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	entities, err := NewYAMLParserForRepo(dir).Parse(domain.File{RelativePath: "service.yaml"})
	if err != nil {
		t.Fatalf("template control lines should be removed before static parsing, got error: %v", err)
	}
	if len(entities) != 1 || entities[0].Kind != domain.KindResource || entities[0].Name != "service" {
		t.Fatalf("entities = %+v, want one concrete resource with static name", entities)
	}
	if entities[0].Source.Parser != "yaml-template" {
		t.Fatalf("parser = %q, want yaml-template", entities[0].Source.Parser)
	}
}

func TestYAMLParser_TemplateKindDoesNotBecomeResourceIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "object.yaml")
	content := []byte(`apiVersion: apps/v1
kind: {{ .Kind }}
metadata:
  name: stable-name
  namespace: {{ .Namespace }}
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	entities, err := NewYAMLParserForRepo(dir).Parse(domain.File{RelativePath: "object.yaml"})
	if err != nil {
		t.Fatalf("templated kind should parse as an opaque template, got error: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("got %d entities, want one template entity", len(entities))
	}
	entity := entities[0]
	if entity.Kind != domain.KindTemplate || entity.ID != "template:kubernetes.unknown@object.yaml#1" {
		t.Fatalf("template entity = %+v", entity)
	}
	if entity.Name != "Kubernetes manifest template" || entity.Package != "" {
		t.Fatalf("template identity leaked unresolved values: %+v", entity)
	}
}
