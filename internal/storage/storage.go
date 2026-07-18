// Package storage reads and writes Atlas graphs as JSON files.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

// WriteGraph marshals a Graph to indented JSON and writes it to path.
func WriteGraph(path string, g domain.Graph) error {
	jsonBytes, err := json.MarshalIndent(g, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal graph to JSON: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories for path %s: %w", path, err)
	}

	if err := os.WriteFile(path, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write graph JSON to file %s: %w", path, err)
	}

	return nil
}

// ReadGraph reads a JSON file at path and unmarshals it into a Graph.
func ReadGraph(path string) (domain.Graph, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return domain.Graph{}, fmt.Errorf("failed to read graph file from %q: %w", path, err)
	}

	var g domain.Graph
	if err := json.Unmarshal(fileBytes, &g); err != nil {
		return domain.Graph{}, fmt.Errorf("failed to unmarshal JSON into graph structure: %w", err)
	}
	return g, nil
}
