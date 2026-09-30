package scanner

import (
	"fmt"
	"go/build"
	"path/filepath"
	"runtime"
	"sort"
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
	GOOS          string
	GOARCH        string
	BuildTags     []string
}

// EffectiveBuildContext resolves explicit target settings without mutating the
// process environment. The context is persisted and used for cache identity.
func EffectiveBuildContext(opts ScanOptions) domain.BuildContext {
	context := domain.BuildContext{GOOS: opts.GOOS, GOARCH: opts.GOARCH, GoVersion: runtime.Version(), CgoEnabled: build.Default.CgoEnabled}
	if context.GOOS == "" {
		context.GOOS = build.Default.GOOS
	}
	if context.GOARCH == "" {
		context.GOARCH = build.Default.GOARCH
	}
	seen := make(map[string]bool)
	for _, tag := range opts.BuildTags {
		if tag = strings.TrimSpace(tag); tag != "" && !seen[tag] {
			seen[tag] = true
			context.BuildTags = append(context.BuildTags, tag)
		}
	}
	sort.Strings(context.BuildTags)
	return context
}

func Scan(repoPath string, outputPath string, opts ScanOptions) (*Result, error) {
	start := time.Now()
	buildContext := EffectiveBuildContext(opts)
	extractorBuild, err := ExecutableBuildIdentity()
	if err != nil {
		return nil, fmt.Errorf("extractor build identity: %w", err)
	}
	signature := domain.ExtractionSignature(domain.CurrentExtractorVersion, buildContext, extractorBuild)
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repository path: %w", err)
	}
	repoPath = absRepo

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
	files = excludeOutputFile(files, repoPath, outputPath)

	// Step 1b: Incremental detection — skip unchanged files
	allFiles := files
	var incremental bool
	var changedCount, reusedCount, deletedCount int
	var oldEntities []domain.Entity
	reusedPaths := make(map[string]bool)

	if opts.PreviousGraph != "" {
		prev, err := storage.ReadGraph(opts.PreviousGraph)
		// An incomplete graph is not a safe source of reusable facts: a parser
		// warning may belong to an unchanged file, and reusing that file while
		// marking the new graph complete would turn an unknown into a false fact.
		// Legacy graphs also have ScanComplete=false, so they receive one full
		// migration scan before incremental mode is enabled.
		if err == nil && prev.ScanComplete && prev.SchemaVersion == domain.CurrentSchemaVersion && prev.ExtractorVersion == domain.CurrentExtractorVersion && prev.ExtractorBuild == extractorBuild && prev.ExtractionSignature == signature && prev.EntityIdentity == domain.CurrentEntityIdentity && sameRepository(prev.Repository, repoPath) && (len(prev.FileTimestamps) > 0 || len(prev.FileFingerprints) > 0) {
			previousState := prev.FileFingerprints
			if len(previousState) == 0 {
				previousState = prev.FileTimestamps
			}
			changed, unchanged, deleted := changedFiles(files, previousState)
			if len(changed) < len(files) && !packageRescanRequired(prev.Entities, changed, deleted) {
				incremental = true
				changedCount = len(changed)
				reusedCount = len(unchanged)
				deletedCount = len(deleted)
				files = changed

				unchangedSet := make(map[string]bool, len(unchanged))
				for _, f := range unchanged {
					unchangedSet[f.RelativePath] = true
					reusedPaths[f.RelativePath] = true
				}
				oldEntities = entitiesFromFiles(prev.Entities, unchangedSet)
			}
		}
	}

	// Step 2: Classify files by extension
	groups := parser.Classify(files)
	// ScanFiles describes the deterministic facts represented by the graph.
	// Whether a supported file was parsed during this invocation or reused from
	// a previous graph is operational metadata already reported by Result; it
	// must not change the persisted graph bytes for an identical repository.
	scanFiles := initializeScanFiles(allFiles, reusedPaths)

	// Step 3: Parse each file group. Parsers resolve relative paths against the
	// repository root, so scanning does not mutate the process working directory.
	var entities []domain.Entity
	var warnings []string

	// Go source files (skip _test.go — those go to TestParser)
	goParser := parser.NewGoParserForRepo(repoPath)
	for _, f := range groups[".go"] {
		if strings.HasSuffix(f.RelativePath, "_test.go") {
			continue
		}
		ents, err := goParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("go: %s: %v", f.RelativePath, err))
			markScanFile(scanFiles, f.RelativePath, domain.ScanFileFailed, "go", 0, err.Error())
			continue
		}
		entities = append(entities, ents...)
		markScanFile(scanFiles, f.RelativePath, domain.ScanFileParsed, "go", len(ents), "")
	}

	// Test files
	testParser := parser.NewTestParserForRepo(repoPath)
	for _, f := range groups[".go"] {
		if !strings.HasSuffix(f.RelativePath, "_test.go") {
			continue
		}
		ents, err := testParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("test: %s: %v", f.RelativePath, err))
			markScanFile(scanFiles, f.RelativePath, domain.ScanFileFailed, "test", 0, err.Error())
			continue
		}
		entities = append(entities, ents...)
		markScanFile(scanFiles, f.RelativePath, domain.ScanFileParsed, "test", len(ents), "")
	}

	// YAML files (.yaml and .yml)
	yamlParser := parser.NewYAMLParserForRepo(repoPath)
	for _, ext := range []string{".yaml", ".yml"} {
		for _, f := range groups[ext] {
			ents, err := yamlParser.Parse(f)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("yaml: %s: %v", f.RelativePath, err))
				parserName := "yaml"
				if parser.IsTemplateFile(repoPath, f) {
					parserName = "yaml-template"
				}
				markScanFile(scanFiles, f.RelativePath, domain.ScanFileFailed, parserName, 0, err.Error())
				continue
			}
			entities = append(entities, ents...)
			parserName := "yaml"
			if parser.IsTemplateFile(repoPath, f) {
				parserName = "yaml-template"
			}
			markScanFile(scanFiles, f.RelativePath, domain.ScanFileParsed, parserName, len(ents), "")
		}
	}

	// Markdown files
	mdParser := parser.NewMarkdownParserForRepo(repoPath)
	for _, f := range groups[".md"] {
		ents, err := mdParser.Parse(f)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("md: %s: %v", f.RelativePath, err))
			markScanFile(scanFiles, f.RelativePath, domain.ScanFileFailed, "markdown", 0, err.Error())
			continue
		}
		entities = append(entities, ents...)
		markScanFile(scanFiles, f.RelativePath, domain.ScanFileParsed, "markdown", len(ents), "")
	}

	// Step 3b: Merge old entities from unchanged files
	if incremental {
		entities = append(oldEntities, entities...)
	}

	// Step 4: Deduplicate entities by ID, merging Files and Imports.
	activeFiles := activeGoSourceFiles(repoPath, allFiles, buildContext)
	seenIdx := make(map[string]int)
	deduped := entities[:0]
	for _, e := range entities {
		if idx, ok := seenIdx[e.ID]; ok {
			existingActive, incomingActive := activeFiles[deduped[idx].Source.File], activeFiles[e.Source.File]
			if filepath.Ext(deduped[idx].Source.File) == ".go" && filepath.Ext(e.Source.File) == ".go" && deduped[idx].Source.File != e.Source.File {
				deduped[idx].Files = appendUnique(deduped[idx].Files, []string{deduped[idx].Source.File, e.Source.File})
			}
			if incomingActive && !existingActive {
				// Identity remains shared across variants; its primary source and
				// documentation describe the declaration selected by the target.
				deduped[idx].Source = e.Source
				deduped[idx].Description = e.Description
				deduped[idx].Generated = e.Generated
			}
			if deduped[idx].Description == "" && e.Description != "" && (!existingActive || incomingActive) {
				deduped[idx].Description = e.Description
			}
			deduped[idx].Files = appendUnique(deduped[idx].Files, e.Files)
			deduped[idx].Watches, deduped[idx].WatchMethods, deduped[idx].WatchSites = appendUniqueWatchFacts(
				deduped[idx].Watches, deduped[idx].WatchMethods, deduped[idx].WatchSites,
				e.Watches, e.WatchMethods, e.WatchSites,
			)
			deduped[idx].Imports = appendUnique(deduped[idx].Imports, e.Imports)
			deduped[idx].ImportSites = appendUniqueSites(deduped[idx].ImportSites, e.ImportSites)
			deduped[idx].Literals = appendUnique(deduped[idx].Literals, e.Literals)
			deduped[idx].Properties = appendUnique(deduped[idx].Properties, e.Properties)
			deduped[idx].Embeds = appendUnique(deduped[idx].Embeds, e.Embeds)
			deduped[idx].Calls = appendUnique(deduped[idx].Calls, e.Calls)
			deduped[idx].Creates = appendUnique(deduped[idx].Creates, e.Creates)
			deduped[idx].CreateSites = appendUniqueSites(deduped[idx].CreateSites, e.CreateSites)
			deduped[idx].CallSites = appendUniqueSites(deduped[idx].CallSites, e.CallSites)
			deduped[idx].ImplementationSites = appendUniqueSites(deduped[idx].ImplementationSites, e.ImplementationSites)
			deduped[idx].EmbedSites = appendUniqueSites(deduped[idx].EmbedSites, e.EmbedSites)
			deduped[idx].Implements = appendUnique(deduped[idx].Implements, e.Implements)
			deduped[idx].EnvVars = appendUnique(deduped[idx].EnvVars, e.EnvVars)
			if !existingActive || incomingActive {
				deduped[idx].Generated = deduped[idx].Generated && e.Generated
			}
			deduped[idx].ResourceOperations = append(deduped[idx].ResourceOperations, e.ResourceOperations...)
			deduped[idx].ReferenceSites = append(deduped[idx].ReferenceSites, e.ReferenceSites...)
			continue
		}
		seenIdx[e.ID] = len(deduped)
		deduped = append(deduped, e)
	}
	entities = deduped
	normalizePackageFiles(entities, allFiles)
	sortEntities(entities)

	// Step 5: Temporal enrichment (optional)
	if opts.Temporal {
		if err := temporal.Enrich(repoPath, entities); err != nil {
			warnings = append(warnings, fmt.Sprintf("temporal: %v", err))
		}
	} else {
		// A previous graph may have been scanned with --temporal. Do not carry
		// those historical facts into a graph whose current scan did not request
		// them: incremental reuse must preserve the meaning of the current scan
		// options, not silently retain stale optional metadata.
		clearTemporalFields(entities)
	}
	sort.Strings(warnings)

	// Step 6: Build relationships
	builder := graph.NewRelationshipBuilder(repoPath)
	// Upgrade only relationships whose call targets are resolved by Go's type
	// checker. The AST builder remains the fallback for repositories that do
	// not type-check cleanly; typed analysis never invents a target outside the
	// graph.
	typedRelationships, typeCoverage := graph.BuildTypedRelationships(repoPath, allFiles, entities, buildContext)
	// Typed resource resolution can add observations. Canonicalize them before
	// edge building so full and incremental scans serialize the same facts.
	sortEntities(entities)
	relationships := builder.Build(entities)
	relationships = graph.MergeRelationships(relationships, typedRelationships)

	// Step 7: Assemble graph with metadata
	duration := time.Since(start)
	g := graph.BuildGraph(repoPath, entities, relationships, duration)
	g.ExtractorVersion = domain.CurrentExtractorVersion
	g.ExtractorBuild = extractorBuild
	g.ExtractionSignature = signature
	g.BuildContext = &buildContext
	g.TypeAnalysis = &typeCoverage

	// Step 7b: Store file timestamps for incremental scanning
	timestamps := make(map[string]string, len(allFiles))
	for _, f := range allFiles {
		timestamps[f.RelativePath] = f.ModifiedTime.UTC().Format(time.RFC3339)
	}
	g.FileTimestamps = timestamps
	fingerprints := make(map[string]string, len(allFiles))
	for _, f := range allFiles {
		fingerprints[f.RelativePath] = fileFingerprint(f)
	}
	g.FileFingerprints = fingerprints
	for i := range scanFiles {
		scanFiles[i].EntityCount = countEntitiesForFile(entities, scanFiles[i].Path)
	}
	sort.Slice(scanFiles, func(i, j int) bool { return scanFiles[i].Path < scanFiles[j].Path })
	g.ScanFiles = scanFiles
	coverage := summarizeScanFiles(scanFiles)
	g.ScanCoverage = &coverage
	g.ScanComplete = coverage.Failed == 0 && len(warnings) == 0
	g.ScanWarnings = append([]string(nil), warnings...)

	// Step 8: Validate
	if err := graph.ValidateGraph(g); err != nil {
		return nil, fmt.Errorf("graph validation: %w", err)
	}

	// Step 8b: Compile knowledge views and question index
	sort.Slice(g.Relationship, func(i, j int) bool { return g.Relationship[i].ID < g.Relationship[j].ID })
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

func activeGoSourceFiles(root string, files []domain.File, context domain.BuildContext) map[string]bool {
	buildContext := build.Default
	buildContext.GOOS = context.GOOS
	buildContext.GOARCH = context.GOARCH
	buildContext.BuildTags = append([]string(nil), context.BuildTags...)
	buildContext.CgoEnabled = context.CgoEnabled
	active := make(map[string]bool)
	for _, file := range files {
		if filepath.Ext(file.RelativePath) != ".go" {
			continue
		}
		absolute := filepath.Join(root, filepath.FromSlash(file.RelativePath))
		if matches, err := buildContext.MatchFile(filepath.Dir(absolute), filepath.Base(absolute)); err == nil && matches {
			active[file.RelativePath] = true
		}
	}
	return active
}

func initializeScanFiles(files []domain.File, reused map[string]bool) []domain.ScanFile {
	result := make([]domain.ScanFile, 0, len(files))
	for _, file := range files {
		parserName, supported := parserForFile(file.RelativePath)
		if supported && reused[file.RelativePath] {
			result = append(result, domain.ScanFile{Path: file.RelativePath, Status: domain.ScanFileParsed, Parser: parserName})
			continue
		}
		if supported {
			result = append(result, domain.ScanFile{
				Path: file.RelativePath, Status: domain.ScanFileFailed, Parser: parserName,
				Reason: "parser did not complete",
			})
			continue
		}
		reason := "no parser registered for file type"
		if ext := filepath.Ext(file.RelativePath); ext != "" {
			reason = "no parser registered for extension " + ext
		}
		result = append(result, domain.ScanFile{Path: file.RelativePath, Status: domain.ScanFileIgnored, Reason: reason})
	}
	return result
}

func parserForFile(path string) (string, bool) {
	switch filepath.Ext(path) {
	case ".go":
		if strings.HasSuffix(path, "_test.go") {
			return "test", true
		}
		return "go", true
	case ".yaml", ".yml":
		return "yaml", true
	case ".md":
		return "markdown", true
	default:
		return "", false
	}
}

func markScanFile(files []domain.ScanFile, path string, status domain.ScanFileStatus, parserName string, entityCount int, reason string) {
	for i := range files {
		if files[i].Path != path {
			continue
		}
		files[i].Status = status
		files[i].Parser = parserName
		files[i].EntityCount = entityCount
		files[i].Reason = reason
		return
	}
}

func countEntitiesForFile(entities []domain.Entity, path string) int {
	count := 0
	for _, entity := range entities {
		if entity.Source.File == path || containsPath(entity.Files, path) {
			count++
		}
	}
	return count
}

func containsPath(paths []string, want string) bool {
	for _, path := range paths {
		if path == want {
			return true
		}
	}
	return false
}

func summarizeScanFiles(files []domain.ScanFile) domain.ScanCoverage {
	coverage := domain.ScanCoverage{Discovered: len(files)}
	for _, file := range files {
		switch file.Status {
		case domain.ScanFileParsed:
			coverage.Parsed++
		case domain.ScanFileReused:
			coverage.Reused++
		case domain.ScanFileIgnored:
			coverage.Ignored++
		case domain.ScanFileFailed:
			coverage.Failed++
		}
	}
	return coverage
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
		if !matchesFileState(f, oldTS) {
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

func excludeOutputFile(files []domain.File, repoPath, outputPath string) []domain.File {
	relative, err := filepath.Rel(repoPath, outputPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return files
	}
	relative = filepath.ToSlash(relative)
	filtered := make([]domain.File, 0, len(files))
	for _, file := range files {
		if file.RelativePath != relative {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func matchesFileState(file domain.File, previous string) bool {
	if timestamp, err := time.Parse(time.RFC3339, previous); err == nil {
		return file.ModifiedTime.UTC().Format(time.RFC3339) == timestamp.UTC().Format(time.RFC3339)
	}
	return fileFingerprint(file) == previous
}

func entitiesFromFiles(entities []domain.Entity, unchangedPaths map[string]bool) []domain.Entity {
	var result []domain.Entity
	for _, e := range entities {
		if entityFilesUnchanged(e, unchangedPaths) {
			result = append(result, e)
		}
	}
	return result
}

func entityFilesUnchanged(e domain.Entity, unchangedPaths map[string]bool) bool {
	if len(e.Files) > 0 {
		for _, file := range e.Files {
			if !unchangedPaths[file] {
				return false
			}
		}
		return true
	}
	return unchangedPaths[e.Source.File]
}

func packageRescanRequired(previous []domain.Entity, changed []domain.File, deleted []string) bool {
	// Module paths are part of every repository-mode Go entity ID. A module
	// file change can therefore invalidate identities even when no .go file
	// changed.
	for _, file := range changed {
		if filepath.Base(file.RelativePath) == "go.mod" {
			return true
		}
	}
	for _, deletedPath := range deleted {
		if filepath.Base(deletedPath) == "go.mod" {
			return true
		}
	}
	var packageDirs []string
	for _, entity := range previous {
		if entity.Kind != domain.KindPackage {
			continue
		}
		for _, file := range append(append([]string(nil), entity.Files...), entity.Source.File) {
			if filepath.Ext(file) == ".go" {
				packageDirs = append(packageDirs, filepath.ToSlash(filepath.Dir(file)))
			}
		}
	}
	if len(packageDirs) == 0 {
		return false
	}
	for _, file := range changed {
		if filepath.Ext(file.RelativePath) != ".go" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(file.RelativePath))
		for _, packageDir := range packageDirs {
			if dir == packageDir {
				// A new file can be merged safely. Existing package files need a
				// full package rescan because package-level imports and embeds are
				// aggregated and do not retain per-file provenance.
				found := false
				for _, entity := range previous {
					if entity.Kind != domain.KindPackage {
						continue
					}
					for _, oldFile := range entity.Files {
						if oldFile == file.RelativePath {
							found = true
							break
						}
					}
				}
				if found {
					return true
				}
			}
		}
	}
	for _, deletedPath := range deleted {
		if filepath.Ext(deletedPath) != ".go" {
			continue
		}
		dir := filepath.ToSlash(filepath.Dir(deletedPath))
		for _, packageDir := range packageDirs {
			if dir == packageDir {
				return true
			}
		}
	}
	return false
}

func sameRepository(previous, current string) bool {
	if previous == "" || current == "" {
		return false
	}
	previousAbs, err := filepath.Abs(previous)
	if err != nil {
		return false
	}
	currentAbs, err := filepath.Abs(current)
	if err != nil {
		return false
	}
	previousResolved, previousErr := filepath.EvalSymlinks(previousAbs)
	currentResolved, currentErr := filepath.EvalSymlinks(currentAbs)
	if previousErr == nil {
		previousAbs = previousResolved
	}
	if currentErr == nil {
		currentAbs = currentResolved
	}
	return filepath.Clean(previousAbs) == filepath.Clean(currentAbs)
}

func fileFingerprint(f domain.File) string {
	return discovery.Fingerprint(f)
}

func clearTemporalFields(entities []domain.Entity) {
	for i := range entities {
		entities[i].LastAuthor = ""
		entities[i].LastModified = ""
		entities[i].ChangeCount = 0
	}
}

func sortEntities(entities []domain.Entity) {
	for i := range entities {
		sort.Strings(entities[i].Files)
		sort.Strings(entities[i].Imports)
		sort.Strings(entities[i].Literals)
		sort.Strings(entities[i].Properties)
		sort.Strings(entities[i].Embeds)
		sort.Strings(entities[i].Creates)
		sort.Strings(entities[i].Calls)
		sort.Strings(entities[i].Implements)
		sort.Strings(entities[i].EnvVars)
		sort.Slice(entities[i].ImportSites, func(a, b int) bool { return siteLess(entities[i].ImportSites[a], entities[i].ImportSites[b]) })
		sortWatchFacts(&entities[i])
		sort.Slice(entities[i].CreateSites, func(a, b int) bool { return siteLess(entities[i].CreateSites[a], entities[i].CreateSites[b]) })
		sort.Slice(entities[i].CallSites, func(a, b int) bool { return siteLess(entities[i].CallSites[a], entities[i].CallSites[b]) })
		sort.Slice(entities[i].ImplementationSites, func(a, b int) bool {
			return siteLess(entities[i].ImplementationSites[a], entities[i].ImplementationSites[b])
		})
		sort.Slice(entities[i].EmbedSites, func(a, b int) bool { return siteLess(entities[i].EmbedSites[a], entities[i].EmbedSites[b]) })
		sort.Slice(entities[i].ResourceOperations, func(a, b int) bool {
			left, right := entities[i].ResourceOperations[a], entities[i].ResourceOperations[b]
			if left.Source.File != right.Source.File {
				return left.Source.File < right.Source.File
			}
			if left.Source.Line != right.Source.Line {
				return left.Source.Line < right.Source.Line
			}
			if left.Source.Column != right.Source.Column {
				return left.Source.Column < right.Source.Column
			}
			return left.Method < right.Method
		})
		sort.Slice(entities[i].ReferenceSites, func(a, b int) bool {
			left, right := entities[i].ReferenceSites[a], entities[i].ReferenceSites[b]
			if left.Source.File != right.Source.File {
				return left.Source.File < right.Source.File
			}
			if left.Source.Line != right.Source.Line {
				return left.Source.Line < right.Source.Line
			}
			if left.Source.Column != right.Source.Column {
				return left.Source.Column < right.Source.Column
			}
			if left.Target != right.Target {
				return left.Target < right.Target
			}
			if left.Source.EndLine != right.Source.EndLine {
				return left.Source.EndLine < right.Source.EndLine
			}
			return left.Source.Parser < right.Source.Parser
		})
		references := entities[i].ReferenceSites[:0]
		for _, reference := range entities[i].ReferenceSites {
			if len(references) == 0 || references[len(references)-1] != reference {
				references = append(references, reference)
			}
		}
		entities[i].ReferenceSites = references
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })
}

// sortWatchFacts keeps a watch target, its registration method, and its
// source site aligned while making the serialized graph independent of parse
// or incremental merge order.
func sortWatchFacts(entity *domain.Entity) {
	if len(entity.Watches) == 0 || len(entity.WatchMethods) != len(entity.Watches) {
		return
	}
	type watchFact struct {
		name   string
		method string
		site   domain.Site
	}
	facts := make([]watchFact, len(entity.Watches))
	for i, name := range entity.Watches {
		facts[i].name = name
		facts[i].method = entity.WatchMethods[i]
		if i < len(entity.WatchSites) {
			facts[i].site = entity.WatchSites[i]
		}
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].name != facts[j].name {
			return facts[i].name < facts[j].name
		}
		if facts[i].method != facts[j].method {
			return facts[i].method < facts[j].method
		}
		return siteLess(facts[i].site, facts[j].site)
	})
	for i, fact := range facts {
		entity.Watches[i] = fact.name
		entity.WatchMethods[i] = fact.method
		if i < len(entity.WatchSites) {
			entity.WatchSites[i] = fact.site
		}
	}
}

func siteLess(a, b domain.Site) bool {
	if a.Source.File != b.Source.File {
		return a.Source.File < b.Source.File
	}
	if a.Source.Line != b.Source.Line {
		return a.Source.Line < b.Source.Line
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Source.Parser < b.Source.Parser
}

func normalizePackageFiles(entities []domain.Entity, files []domain.File) {
	available := make(map[string]bool, len(files))
	for _, file := range files {
		if filepath.Ext(file.RelativePath) == ".go" {
			available[file.RelativePath] = true
		}
	}
	for i := range entities {
		if entities[i].Kind != domain.KindPackage {
			continue
		}
		var current []string
		seen := make(map[string]bool)
		add := func(file string) {
			if available[file] && !seen[file] {
				seen[file] = true
				current = append(current, file)
			}
		}
		// Parsed package entities already carry every file encountered for that
		// package name, including files added during an incremental scan. The
		// availability check below removes deleted files without assuming that a
		// package name maps to one directory.
		for _, file := range entities[i].Files {
			add(file)
		}
		if filepath.Ext(entities[i].Source.File) == ".go" {
			add(entities[i].Source.File)
		} else {
			// Preserve compatibility with graphs whose package source was a
			// directory rather than a concrete Go file.
			prefix := strings.TrimSuffix(entities[i].Source.File, "/") + "/"
			for file := range available {
				if strings.HasPrefix(file, prefix) {
					add(file)
				}
			}
		}
		entities[i].Files = current
	}
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

func appendUniqueSites(existing, additions []domain.Site) []domain.Site {
	seen := make(map[string]bool, len(existing))
	for _, site := range existing {
		seen[site.Name+"\x00"+site.Source.File+"\x00"+fmt.Sprint(site.Source.Line)] = true
	}
	for _, site := range additions {
		key := site.Name + "\x00" + site.Source.File + "\x00" + fmt.Sprint(site.Source.Line)
		if !seen[key] {
			seen[key] = true
			existing = append(existing, site)
		}
	}
	return existing
}

func appendUniqueWatchFacts(existingNames, existingMethods []string, existingSites []domain.Site, names, methods []string, sites []domain.Site) ([]string, []string, []domain.Site) {
	// Keep legacy entities untouched when their pre-method schema cannot
	// preserve target/method alignment. Current scanner output always supplies
	// one method per target and one site per registration.
	if len(existingMethods) != len(existingNames) || len(methods) != len(names) {
		return appendUnique(existingNames, names), appendUnique(existingMethods, methods), appendUniqueSites(existingSites, sites)
	}
	type watchFact struct {
		name   string
		method string
		site   domain.Site
	}
	facts := make([]watchFact, 0, len(existingNames)+len(names))
	seen := make(map[string]bool)
	add := func(name, method string, site domain.Site) {
		key := name + "\x00" + method + "\x00" + site.Source.File + "\x00" + fmt.Sprint(site.Source.Line)
		if seen[key] {
			return
		}
		seen[key] = true
		facts = append(facts, watchFact{name: name, method: method, site: site})
	}
	for i, name := range existingNames {
		var site domain.Site
		if i < len(existingSites) {
			site = existingSites[i]
		}
		add(name, existingMethods[i], site)
	}
	for i, name := range names {
		var site domain.Site
		if i < len(sites) {
			site = sites[i]
		}
		add(name, methods[i], site)
	}
	sort.Slice(facts, func(i, j int) bool {
		if facts[i].name != facts[j].name {
			return facts[i].name < facts[j].name
		}
		if facts[i].method != facts[j].method {
			return facts[i].method < facts[j].method
		}
		return siteLess(facts[i].site, facts[j].site)
	})
	resultNames := make([]string, len(facts))
	resultMethods := make([]string, len(facts))
	resultSites := make([]domain.Site, len(facts))
	for i, fact := range facts {
		resultNames[i] = fact.name
		resultMethods[i] = fact.method
		resultSites[i] = fact.site
	}
	return resultNames, resultMethods, resultSites
}
