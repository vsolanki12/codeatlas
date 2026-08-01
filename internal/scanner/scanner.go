package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/discovery"
	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/graph"
	"github.com/vsolanki12/codeatlas/internal/parser"
	"github.com/vsolanki12/codeatlas/internal/storage"
	"github.com/vsolanki12/codeatlas/internal/temporal"
	"github.com/vsolanki12/codeatlas/internal/views"
)

// Result holds the output of a scan — the graph and summary stats.
type Result struct {
	Graph        domain.Graph
	EntityCount  int
	RelCount     int
	Duration     time.Duration
	Warnings     []string
	Incremental  bool
	ChangedFiles int
	ReusedFiles  int
	DeletedFiles int
}

type ScanOptions struct {
	Temporal      bool
	PreviousGraph string
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

	// Step 1b: Incremental detection — skip unchanged files
	allFiles := files
	var incremental bool
	var changedCount, reusedCount, deletedCount int
	var oldEntities []domain.Entity

	if opts.PreviousGraph != "" {
		prev, err := storage.ReadGraph(opts.PreviousGraph)
		if err == nil && len(prev.FileTimestamps) > 0 {
			changed, unchanged, deleted := changedFiles(files, prev.FileTimestamps)
			if len(changed) < len(files) {
				incremental = true
				changedCount = len(changed)
				reusedCount = len(unchanged)
				deletedCount = len(deleted)
				files = changed

				unchangedSet := make(map[string]bool, len(unchanged))
				for _, f := range unchanged {
					unchangedSet[f.RelativePath] = true
				}
				oldEntities = entitiesFromFiles(prev.Entities, unchangedSet)
				_ = deleted
			}
		}
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

	// Step 3b: Merge old entities from unchanged files
	if incremental {
		entities = append(oldEntities, entities...)
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

	// Step 7b: Store file timestamps for incremental scanning
	timestamps := make(map[string]string, len(allFiles))
	for _, f := range allFiles {
		timestamps[f.RelativePath] = f.ModifiedTime.UTC().Format(time.RFC3339)
	}
	g.FileTimestamps = timestamps

	// Step 8: Validate
	if err := graph.ValidateGraph(g); err != nil {
		return nil, fmt.Errorf("graph validation: %w", err)
	}

	// Step 8b: Compile knowledge views and question index
	g.Views = views.Compile(g.Entities, g.Relationship)
	g.Questions = views.CompileQuestions(g.Views)

	// Step 9: Write JSON
	if err := storage.WriteGraph(outputPath, g); err != nil {
		return nil, fmt.Errorf("write graph: %w", err)
	}

	return &Result{
		Graph:        g,
		EntityCount:  len(entities),
		RelCount:     len(relationships),
		Duration:     duration,
		Warnings:     warnings,
		Incremental:  incremental,
		ChangedFiles: changedCount,
		ReusedFiles:  reusedCount,
		DeletedFiles: deletedCount,
	}, nil
}

func changedFiles(files []domain.File, oldTimestamps map[string]string) (changed, unchanged []domain.File, deleted []string) {
	currentPaths := make(map[string]bool, len(files))
	for _, f := range files {
		currentPaths[f.RelativePath] = true
		oldTS, exists := oldTimestamps[f.RelativePath]
		if !exists {
			changed = append(changed, f)
			continue
		}
		if f.ModifiedTime.UTC().Format(time.RFC3339) != oldTS {
			changed = append(changed, f)
		} else {
			unchanged = append(unchanged, f)
		}
	}
	for path := range oldTimestamps {
		if !currentPaths[path] {
			deleted = append(deleted, path)
		}
	}
	return
}

func entitiesFromFiles(entities []domain.Entity, unchangedPaths map[string]bool) []domain.Entity {
	var result []domain.Entity
	for _, e := range entities {
		if unchangedPaths[e.Source.File] {
			result = append(result, e)
			continue
		}
		for p := range unchangedPaths {
			if strings.HasPrefix(p, e.Source.File+"/") {
				result = append(result, e)
				break
			}
		}
	}
	return result
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
