// Package storage reads and writes Atlas graphs as JSON files.
package storage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
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

// Header is the scalar provenance of a graph: everything needed to identify
// and select a repository, with none of its facts.
type Header struct {
	SchemaVersion    string
	Repository       string
	RepositoryID     string
	RepositoryRemote string
	GoModule         string
	Commit           string
	Branch           string
}

var headerFields = map[string]func(*Header) *string{
	"schemaVersion":    func(h *Header) *string { return &h.SchemaVersion },
	"repository":       func(h *Header) *string { return &h.Repository },
	"repositoryId":     func(h *Header) *string { return &h.RepositoryID },
	"repositoryRemote": func(h *Header) *string { return &h.RepositoryRemote },
	"goModule":         func(h *Header) *string { return &h.GoModule },
	"commit":           func(h *Header) *string { return &h.Commit },
	"branch":           func(h *Header) *string { return &h.Branch },
}

// ReadGraphHeader returns a graph's identity without decoding its entities,
// relationships, or views. It streams tokens and skips every value it does
// not need, so identifying a graph costs constant memory instead of the
// hundreds of megabytes a large repository's full graph occupies.
func ReadGraphHeader(path string) (Header, error) {
	file, err := os.Open(path)
	if err != nil {
		return Header{}, fmt.Errorf("failed to read graph file from %q: %w", path, err)
	}
	defer file.Close()

	decoder := json.NewDecoder(bufio.NewReaderSize(file, 1<<20))
	opening, err := decoder.Token()
	if err != nil {
		return Header{}, fmt.Errorf("failed to read graph header from %q: %w", path, err)
	}
	if delimiter, ok := opening.(json.Delim); !ok || delimiter != '{' {
		return Header{}, fmt.Errorf("graph %q is not a JSON object", path)
	}

	var header Header
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return Header{}, fmt.Errorf("failed to read graph header from %q: %w", path, err)
		}
		key, _ := keyToken.(string)
		field, wanted := headerFields[key]
		if !wanted {
			if err := skipJSONValue(decoder); err != nil {
				return Header{}, fmt.Errorf("failed to read graph header from %q: %w", path, err)
			}
			continue
		}
		if err := decoder.Decode(field(&header)); err != nil {
			return Header{}, fmt.Errorf("failed to read graph header from %q: %w", path, err)
		}
	}
	return header, nil
}

// skipJSONValue consumes exactly one value, descending through nested arrays
// and objects without materializing them.
func skipJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok || delimiter == '}' || delimiter == ']' {
		return nil
	}
	for depth := 1; depth > 0; {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if delimiter, ok := token.(json.Delim); ok {
			if delimiter == '{' || delimiter == '[' {
				depth++
			} else {
				depth--
			}
		}
	}
	return nil
}

// ReadGraph reads a JSON file at path and unmarshals it into a Graph.
func ReadGraph(path string) (domain.Graph, error) {
	g, _, err := ReadGraphWithDigest(path)
	return g, err
}

// ReadGraphWithDigest also returns the SHA-256 of the file as it was read.
// Consumers that need to identify the loaded snapshot take the digest from
// here rather than re-serializing the decoded graph, which on a large
// repository means marshalling a second copy of the whole graph into memory.
func ReadGraphWithDigest(path string) (domain.Graph, string, error) {
	// Measured alternative: decoding straight from the file with a
	// json.Decoder removes this byte slice but raises peak memory, because
	// the streaming decoder keeps its own buffers and cannot alias the input
	// the way Unmarshal does. Reading the file whole is the cheaper of the two.
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return domain.Graph{}, "", fmt.Errorf("failed to read graph file from %q: %w", path, err)
	}
	digest := sha256.Sum256(fileBytes)

	var g domain.Graph
	if err := json.Unmarshal(fileBytes, &g); err != nil {
		return domain.Graph{}, "", fmt.Errorf("failed to unmarshal JSON into graph structure: %w", err)
	}
	if err := g.Validate(); err != nil {
		return domain.Graph{}, "", fmt.Errorf("invalid graph: %w", err)
	}
	return g, hex.EncodeToString(digest[:]), nil
}
