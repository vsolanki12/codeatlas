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
		Versions []struct {
			Schema struct {
				OpenAPIV3Schema struct {
					Description string `json:"description,omitempty"`
				} `json:"openAPIV3Schema,omitempty"`
			} `json:"schema,omitempty"`
		} `json:"versions,omitempty"`
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

		var description string
		for _, v := range manifest.Spec.Versions {
			if v.Schema.OpenAPIV3Schema.Description != "" {
				description = v.Schema.OpenAPIV3Schema.Description
				break
			}
		}

		entities = append(entities, domain.Entity{
			ID:          fmt.Sprintf("crd:%s.%s", crdGroup, strings.ToLower(crdKind)),
			Name:        crdKind,
			Kind:        domain.KindCRD,
			Description: description,
			Package:     crdGroup,
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   1,
			},
		})
	} else {
		var props []string
		var raw map[string]interface{}
		if err := yaml.Unmarshal(data, &raw); err == nil {
			flattenYAML("", raw, &props, 0)
		}

		entities = append(entities, domain.Entity{
			ID:         fmt.Sprintf("resource:%s.%s", strings.ToLower(manifest.Kind), manifest.Metadata.Name),
			Name:       manifest.Metadata.Name,
			Kind:       domain.KindResource,
			Package:    manifest.Metadata.Namespace,
			Properties: props,
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   1,
			},
		})
	}
	return entities, nil
}

var skipYAMLKeys = map[string]bool{
	"managedFields": true, "annotations": true, "resourceVersion": true,
	"creationTimestamp": true, "uid": true, "generation": true,
	"selfLink": true, "status": true,
}

func flattenYAML(prefix string, data interface{}, result *[]string, depth int) {
	if depth > 5 || len(*result) >= 100 {
		return
	}
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			if skipYAMLKeys[key] {
				continue
			}
			p := key
			if prefix != "" {
				p = prefix + "." + key
			}
			flattenYAML(p, val, result, depth+1)
		}
	case []interface{}:
		for i, val := range v {
			p := fmt.Sprintf("%s.%d", prefix, i)
			flattenYAML(p, val, result, depth+1)
		}
	default:
		if prefix != "" && v != nil {
			*result = append(*result, fmt.Sprintf("%s=%v", prefix, v))
		}
	}
}
