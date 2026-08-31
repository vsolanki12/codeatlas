package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

var (
	templateActionPattern = regexp.MustCompile(`{{[-+]?[^{}\n]*[-+]?}}`)
	templateOnlyLine      = regexp.MustCompile(`^\s*{{[-+]?[^{}\n]*[-+]?}}\s*$`)
)

const templatePlaceholderPrefix = "__CODEATLAS_TEMPLATE_"

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

// IsTemplateFile reports whether a YAML file contains Go-template actions.
// It is intentionally a lexical check used for scan diagnostics; it does not
// attempt to execute or interpret the template.
func IsTemplateFile(repoPath string, file domain.File) bool {
	readPath := file.RelativePath
	if repoPath != "" {
		readPath = filepath.Join(repoPath, filepath.FromSlash(file.RelativePath))
	}
	data, err := os.ReadFile(readPath)
	return err == nil && hasTemplateActions(string(data))
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
		documentData := document.Content
		parserName := "yaml"
		if hasTemplateActions(documentData) {
			// Go-template actions are not YAML syntax until a runtime value is
			// supplied. Remove control-only lines and replace scalar actions with
			// opaque placeholders so static kind/shape can still be extracted.
			documentData = sanitizeTemplateYAML(documentData)
			parserName = "yaml-template"
		}
		parsed, err := p.parseDocument(file.RelativePath, []byte(documentData), document.Line, parserName)
		if err != nil {
			return nil, err
		}
		entities = append(entities, parsed...)
	}
	return entities, nil
}

func (p *YAMLParser) parseDocument(filePath string, data []byte, documentLine int, parserName string) ([]domain.Entity, error) {

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

	crdGroup := ""
	crdKind := ""
	if manifest.Kind == "CustomResourceDefinition" {
		crdGroup = manifest.Spec.Group
		crdKind = manifest.Spec.Names.Kind
		if crdKind == "" {
			crdKind = manifest.Metadata.Name
		}
	}

	identityUnresolved := parserName == "yaml-template" && (containsTemplatePlaceholder(manifest.Kind) ||
		containsTemplatePlaceholder(manifest.Metadata.Name) || containsTemplatePlaceholder(manifest.Metadata.Namespace) ||
		containsTemplatePlaceholder(crdGroup) || containsTemplatePlaceholder(crdKind))
	if identityUnresolved {
		// The runtime object name is unknown, so a resource ID would falsely
		// claim a concrete object. Preserve the template as an entity with a
		// deterministic file-based identity instead. The kind can also be
		// templated, so use an opaque "unknown" marker rather than leaking a
		// placeholder into a resource ID or name.
		kindPlaceholder := containsTemplatePlaceholder(manifest.Kind)
		templateKind := strings.ToLower(manifest.Kind)
		if templateKind == "" || kindPlaceholder {
			templateKind = "unknown"
		}
		templateName := "Kubernetes manifest template"
		if templateKind != "unknown" {
			templateName = manifest.Kind + " template"
		}
		templateNamespace := manifest.Metadata.Namespace
		if containsTemplatePlaceholder(templateNamespace) {
			templateNamespace = ""
		}
		return []domain.Entity{{
			ID:          templateID(templateKind, filePath, documentLine),
			Name:        templateName,
			Kind:        domain.KindTemplate,
			Description: "Kubernetes manifest template with unresolved runtime identity.",
			Package:     templateNamespace,
			Source: domain.Source{
				Parser: parserName,
				File:   filePath,
				Line:   documentLine,
			},
		}}, nil
	}

	var entities []domain.Entity
	if manifest.Kind == "CustomResourceDefinition" {
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
				Parser: parserName,
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
		if parserName == "yaml-template" {
			props = removeTemplateProperties(props)
		}

		entities = append(entities, domain.Entity{
			ID:         resourceID(manifest.Kind, manifest.Metadata.Namespace, manifest.Metadata.Name, filePath, p.includeNamespace),
			Name:       manifest.Metadata.Name,
			Kind:       domain.KindResource,
			Package:    manifest.Metadata.Namespace,
			Properties: props,
			Source: domain.Source{
				Parser: parserName,
				File:   filePath,
				Line:   documentLine,
			},
		})
	}
	return entities, nil
}

func hasTemplateActions(content string) bool {
	return strings.Contains(content, "{{") && strings.Contains(content, "}}") && templateActionPattern.MatchString(content)
}

func sanitizeTemplateYAML(content string) string {
	var result strings.Builder
	result.Grow(len(content))
	nextPlaceholder := 0
	for _, line := range strings.SplitAfter(content, "\n") {
		withoutNewline := strings.TrimSuffix(line, "\n")
		if templateOnlyLine.MatchString(withoutNewline) {
			if strings.HasSuffix(line, "\n") {
				result.WriteByte('\n')
			}
			continue
		}
		replaced := templateActionPattern.ReplaceAllStringFunc(withoutNewline, func(string) string {
			placeholder := fmt.Sprintf("%s%d__", templatePlaceholderPrefix, nextPlaceholder)
			nextPlaceholder++
			return placeholder
		})
		result.WriteString(replaced)
		if strings.HasSuffix(line, "\n") {
			result.WriteByte('\n')
		}
	}
	return result.String()
}

func containsTemplatePlaceholder(value string) bool {
	return strings.Contains(value, templatePlaceholderPrefix)
}

func removeTemplateProperties(properties []string) []string {
	filtered := properties[:0]
	for _, property := range properties {
		if !containsTemplatePlaceholder(property) {
			filtered = append(filtered, property)
		}
	}
	return filtered
}

func templateID(kind, filePath string, line int) string {
	return fmt.Sprintf("template:kubernetes.%s@%s#%d", strings.ToLower(kind), filepath.ToSlash(filePath), line)
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
