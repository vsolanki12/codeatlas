package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

var _ Parser = (*YAMLParser)(nil)

type YAMLParser struct {
	rootDir          string
	includeNamespace bool
}

func NewYAMLParser() *YAMLParser {
	return &YAMLParser{}
}

func NewYAMLParserForRepo(repoPath string) *YAMLParser {
	abs, err := filepath.Abs(repoPath)
	if err != nil {
		abs = repoPath
	}
	return &YAMLParser{rootDir: filepath.Clean(abs), includeNamespace: true}
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
	readPath := file.RelativePath
	if p.rootDir != "" {
		readPath = filepath.Join(p.rootDir, filepath.FromSlash(file.RelativePath))
	}
	data, err := os.ReadFile(readPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read yaml file %s: %w", file.RelativePath, err)
	}

	var entities []domain.Entity
	for _, document := range splitYAMLDocuments(string(data)) {
		parsed, err := p.parseDocument(file.RelativePath, []byte(document.Content), document.Line)
		if err != nil {
			return nil, err
		}
		entities = append(entities, parsed...)
	}
	return entities, nil
}

func (p *YAMLParser) parseDocument(filePath string, data []byte, documentLine int) ([]domain.Entity, error) {

	// Repositories commonly contain Ansible, CI, and configuration YAML next
	// to Kubernetes manifests. Parse the document shape first so unsupported
	// top-level arrays/maps are ignored rather than reported as broken
	// Kubernetes resources.
	var document interface{}
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml content: %w", err)
	}
	object, ok := document.(map[string]interface{})
	if !ok {
		return nil, nil
	}
	kind, _ := object["kind"].(string)
	metadata, _ := object["metadata"].(map[string]interface{})
	metadataName, _ := metadata["name"].(string)
	if kind == "" || metadataName == "" {
		return nil, nil
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
			Files:       []string{filePath},
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   documentLine,
			},
		})
	} else {
		var props []string
		var raw map[string]interface{}
		if err := yaml.Unmarshal(data, &raw); err == nil {
			flattenYAML("", raw, &props, 0)
		}

		entities = append(entities, domain.Entity{
			ID:         resourceID(manifest.Kind, manifest.Metadata.Namespace, manifest.Metadata.Name, filePath, p.includeNamespace),
			Name:       manifest.Metadata.Name,
			Kind:       domain.KindResource,
			Package:    manifest.Metadata.Namespace,
			Properties: props,
			Source: domain.Source{
				Parser: "yaml",
				File:   filePath,
				Line:   documentLine,
			},
		})
	}
	return entities, nil
}

type yamlDocument struct {
	Content string
	Line    int
}

// splitYAMLDocuments keeps the parser independent of a Kubernetes runtime
// while supporting the document streams commonly used for generated
// manifests. A separator is recognized only when it occupies a complete YAML
// line, so an indented "---" inside a literal block is not split.
func splitYAMLDocuments(content string) []yamlDocument {
	var documents []yamlDocument
	start := 0
	line := 1
	startLine := 1
	for offset := 0; offset < len(content); {
		next := strings.IndexByte(content[offset:], '\n')
		end := len(content)
		if next >= 0 {
			end = offset + next
		}
		if strings.TrimSpace(content[offset:end]) == "---" {
			if strings.TrimSpace(content[start:offset]) != "" {
				documents = append(documents, yamlDocument{Content: content[start:offset], Line: startLine})
			}
			if next < 0 {
				start = len(content)
				startLine = line + 1
				break
			}
			start = end + 1
			startLine = line + 1
		}
		line++
		if next < 0 {
			break
		}
		offset = end + 1
	}
	if strings.TrimSpace(content[start:]) != "" {
		documents = append(documents, yamlDocument{Content: content[start:], Line: startLine})
	}
	if len(documents) == 0 && strings.TrimSpace(content) != "" {
		return []yamlDocument{{Content: content, Line: 1}}
	}
	return documents
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
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			val := v[key]
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

func resourceID(kind, namespace, name, filePath string, includeNamespace bool) string {
	if !includeNamespace {
		return fmt.Sprintf("resource:%s.%s", strings.ToLower(kind), name)
	}
	// Namespace is part of a Kubernetes object identity. Keep a stable marker
	// for cluster-scoped objects instead of allowing two scopes to collapse.
	if namespace == "" {
		namespace = "_cluster"
	}
	// Multiple overlays can intentionally declare the same Kubernetes object
	// identity. Keep each source declaration distinct in the repository graph.
	return fmt.Sprintf("resource:%s.%s/%s@%s", strings.ToLower(kind), namespace, name, filepath.ToSlash(filePath))
}
