// Package storage reads and writes Atlas graphs as JSON files.
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// WriteGraph marshals a Graph to indented JSON and atomically replaces path.
func WriteGraph(path string, g domain.Graph) error {
	if err := g.Validate(); err != nil {
		return fmt.Errorf("refusing to write invalid graph: %w", err)
	}
	jsonBytes, err := json.MarshalIndent(g, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal graph to JSON: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directories for path %s: %w", path, err)
	}

	temporary, err := os.CreateTemp(dir, ".atlas-graph-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temporary graph file in %s: %w", dir, err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(0644); err != nil {
		return fmt.Errorf("failed to set graph file permissions: %w", err)
	}
	if _, err := temporary.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write graph JSON to temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("failed to flush graph JSON: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("failed to close temporary graph file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("failed to replace graph JSON at %s: %w", path, err)
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
	if err := g.Validate(); err != nil {
		return domain.Graph{}, fmt.Errorf("invalid graph: %w", err)
	}
	return g, nil
}
