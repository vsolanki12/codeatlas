package parser

import (
	"fmt"
	"os"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

var _ Parser = (*YAMLParser)(nil)

type YAMLParser struct{}

func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

type k8sManifest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace,omitempty"`
	} `json:"metadata"`
	Spec struct {
		Group string `json:"group,omitempty"`
		Names struct {
			Kind string `json:"kind,omitempty"`
		} `json:"names,omitempty"`
	} `json:"spec,omitempty"`
}

func (p *YAMLParser) Parse(file domain.File) ([]domain.Entity, error) {
	filePath := file.RelativePath
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read yaml file %s: %w", filePath, err)
	}

	var manifest k8sManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml content: %w", err)
	}

	if manifest.Kind == "" || manifest.Metadata.Name == "" {
		return nil, nil
	}

	var entities []domain.Entity

	if manifest.Kind == "CustomResourceDefinition" {
		crdGroup := manifest.Spec.Group
		crdKind := manifest.Spec.Names.Kind

		if crdKind == "" {
			crdKind = manifest.Metadata.Name
		}

		entities = append(entities, domain.Entity{
			ID:      fmt.Sprintf("crd:%s.%s", crdGroup, strings.ToLower(crdKind)),
			Name:    crdKind,
			Kind:    domain.KindCRD,
			Package: crdGroup,
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   1,
			},
		})
	} else {
		entities = append(entities, domain.Entity{
			ID:      fmt.Sprintf("resource:%s.%s", strings.ToLower(manifest.Kind), manifest.Metadata.Name),
			Name:    manifest.Metadata.Name,
			Kind:    domain.KindResource,
			Package: manifest.Metadata.Namespace,
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   1,
			},
		})
	}
	return entities, nil
}
