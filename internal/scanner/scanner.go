package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsolanki12/hypershift-atlas/internal/discovery"
	"github.com/vsolanki12/hypershift-atlas/internal/domain"
	"github.com/vsolanki12/hypershift-atlas/internal/graph"
	"github.com/vsolanki12/hypershift-atlas/internal/parser"
	"github.com/vsolanki12/hypershift-atlas/internal/storage"
	"github.com/vsolanki12/hypershift-atlas/internal/temporal"
)

// Result holds the output of a scan — the graph and summary stats.
type Result struct {
	Graph       domain.Graph
	EntityCount int
	RelCount    int
	Duration    time.Duration
	Warnings    []string
}

type ScanOptions struct {
	Temporal bool
}

func Scan(repoPath string, outputPath string, opts ScanOptions) (*Result, error) {
	start := time.Now()

	// Resolve output path before chdir changes working directory
	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}
	outputPath = absOutput

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

	// Step 4: Deduplicate entities by ID, merging Files and Imports.
	seenIdx := make(map[string]int)
	deduped := entities[:0]
	for _, e := range entities {
		if idx, ok := seenIdx[e.ID]; ok {
			deduped[idx].Files = appendUnique(deduped[idx].Files, e.Files)
			deduped[idx].Imports = appendUnique(deduped[idx].Imports, e.Imports)
			deduped[idx].Literals = appendUnique(deduped[idx].Literals, e.Literals)
			deduped[idx].Properties = appendUnique(deduped[idx].Properties, e.Properties)
			deduped[idx].Embeds = appendUnique(deduped[idx].Embeds, e.Embeds)
			continue
		}
		seenIdx[e.ID] = len(deduped)
		deduped = append(deduped, e)
	}
	entities = deduped

	// Step 5: Temporal enrichment (optional)
	if opts.Temporal {
		if err := temporal.Enrich(repoPath, entities); err != nil {
			warnings = append(warnings, fmt.Sprintf("temporal: %v", err))
		}
	}

	// Step 6: Build relationships
	builder := graph.NewRelationshipBuilder(repoPath)
	relationships := builder.Build(entities)

	// Step 7: Assemble graph with metadata
	duration := time.Since(start)
	g := graph.BuildGraph(repoPath, entities, relationships, duration)

	// Step 8: Validate
	if err := graph.ValidateGraph(g); err != nil {
		return nil, fmt.Errorf("graph validation: %w", err)
	}

	// Step 9: Write JSON
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

func appendUnique(existing, additions []string) []string {
	set := make(map[string]bool, len(existing))
	for _, s := range existing {
		set[s] = true
	}
	for _, s := range additions {
		if !set[s] {
			set[s] = true
			existing = append(existing, s)
		}
	}
	return existing
}
