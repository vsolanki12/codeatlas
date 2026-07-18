package scanner

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vsolanki12/hypershift-atlas/internal/discovery"
	"github.com/vsolanki12/hypershift-atlas/internal/domain"
	"github.com/vsolanki12/hypershift-atlas/internal/graph"
	"github.com/vsolanki12/hypershift-atlas/internal/parser"
	"github.com/vsolanki12/hypershift-atlas/internal/storage"
)

// Result holds the output of a scan — the graph and summary stats.
type Result struct {
	Graph       domain.Graph
	EntityCount int
	RelCount    int
	Duration    time.Duration
	Warnings    []string
}

// Scan runs the full Atlas pipeline: discover files, classify by type,
// parse with the appropriate parser, build relationships, validate the
// graph, and write it to outputPath as JSON.
func Scan(repoPath string, outputPath string) (*Result, error) {
	start := time.Now()

	// Step 1: Validate repo and discover files
	disc, err := discovery.New(domain.Repository{RootPath: repoPath})
	if err != nil {
		return nil, fmt.Errorf("discovery init: %w", err)
	}

	files, err := disc.Scan()
	if err != nil {
		return nil, fmt.Errorf("file discovery: %w", err)
	}

	// Step 2: Classify files by extension
	groups := parser.Classify(files)

	// Step 3: Parse each file group
	// Parsers read files using relative paths, so we temporarily change
	// to the repo directory. Restored via defer.
	oldDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	if err := os.Chdir(repoPath); err != nil {
		return nil, fmt.Errorf("chdir to repo: %w", err)
	}
	defer os.Chdir(oldDir)

	var entities []domain.Entity
	var warnings []string

	// Go source files (skip _test.go — those go to TestParser)
	goParser := parser.NewGoParser()
	for _, f := range groups[".go"] {
		if strings.HasSuffix(f.RelativePath, "_test.go") {
			continue
		}
		ents, err := goParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("go: %s: %v", f.RelativePath, err))
			continue
		}
		entities = append(entities, ents...)
	}

	// Test files
	testParser := parser.NewTestParser()
	for _, f := range groups[".go"] {
		if !strings.HasSuffix(f.RelativePath, "_test.go") {
			continue
		}
		ents, err := testParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("test: %s: %v", f.RelativePath, err))
			continue
		}
		entities = append(entities, ents...)
	}

	// YAML files (.yaml and .yml)
	yamlParser := parser.NewYAMLParser()
	for _, ext := range []string{".yaml", ".yml"} {
		for _, f := range groups[ext] {
			ents, err := yamlParser.Parse(f)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("yaml: %s: %v", f.RelativePath, err))
				continue
			}
			entities = append(entities, ents...)
		}
	}

	// Markdown files
	mdParser := parser.NewMarkdownParser()
	for _, f := range groups[".md"] {
		ents, err := mdParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("md: %s: %v", f.RelativePath, err))
			continue
		}
		entities = append(entities, ents...)
	}

	// Step 4: Deduplicate entities by ID.
	// The GoParser emits a package entity per file — 5 files in the same
	// package produce 5 identical package:pkg entries. Keep the first seen.
	seen := make(map[string]bool)
	deduped := entities[:0]
	for _, e := range entities {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		deduped = append(deduped, e)
	}
	entities = deduped

	// Step 5: Build relationships
	builder := graph.NewRelationshipBuilder(repoPath)
	relationships := builder.Build(entities)

	// Step 6: Assemble graph with metadata
	duration := time.Since(start)
	g := graph.BuildGraph(repoPath, entities, relationships, duration)

	// Step 7: Validate
	if err := graph.ValidateGraph(g); err != nil {
		return nil, fmt.Errorf("graph validation: %w", err)
	}

	// Step 8: Write JSON
	if err := storage.WriteGraph(outputPath, g); err != nil {
		return nil, fmt.Errorf("write graph: %w", err)
	}

	return &Result{
		Graph:       g,
		EntityCount: len(entities),
		RelCount:    len(relationships),
		Duration:    duration,
		Warnings:    warnings,
	}, nil
}
