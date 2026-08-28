package graph

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// Generic function names that would create thousands of false-positive edges.
var skipCallNames = map[string]bool{
	"Error": true, "String": true, "Get": true, "Set": true,
	"Errorf": true, "Sprintf": true, "Printf": true, "Fprintf": true,
	"New": true, "Close": true, "Read": true, "Write": true,
	"Wrap": true, "Wrapf": true, "Log": true, "Info": true, "Debug": true,
	"Warn": true, "Fatal": true, "Panic": true, "Format": true,
	"Unmarshal": true, "Marshal": true, "Decode": true, "Encode": true,
	"Len": true, "Less": true, "Swap": true, "Sort": true,
	"append": true, "make": true, "len": true, "cap": true, "delete": true,
	"copy": true, "close": true, "panic": true, "recover": true,
	"print": true, "println": true,
}

type RelationshipBuilder struct {
	RootDir string
}

func NewRelationshipBuilder(rootDir string) *RelationshipBuilder {
	return &RelationshipBuilder{RootDir: rootDir}
}

func (b *RelationshipBuilder) Build(entities []domain.Entity) []domain.Relationship {
	orderedEntities := append([]domain.Entity(nil), entities...)
	sort.Slice(orderedEntities, func(i, j int) bool { return orderedEntities[i].ID < orderedEntities[j].ID })
	entities = orderedEntities

	byName := make(map[string][]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindCRD || e.Kind == domain.KindResource {
			byName[e.Name] = append(byName[e.Name], e)
		}
	}

	var relationships []domain.Relationship

	seen := make(map[string]bool)

	// Package imports are direct AST observations. Resolve them only when the
	// imported path names exactly one package entity in this graph; external or
	// unresolved imports remain parser observations on the package entity.
	packagesByPath := make(map[string][]domain.Entity)
	for _, e := range entities {
		if e.Kind != domain.KindPackage {
			continue
		}
		packagePath := e.Package
		if packagePath == "" {
			packagePath = strings.TrimPrefix(e.ID, "package:")
		}
		if packagePath != "" {
			packagesByPath[packagePath] = append(packagesByPath[packagePath], e)
		}
	}
	for _, e := range entities {
		if e.Kind != domain.KindPackage {
			continue
		}
		for importIndex, importPath := range e.Imports {
			candidates := packagesByPath[importPath]
			if len(candidates) != 1 || candidates[0].ID == e.ID {
				continue
			}
			target := candidates[0]
			relID := domain.NewRelationshipID(e.ID, domain.RelImports, target.ID)
			if seen[relID] {
				continue
			}
			seen[relID] = true
			evidence := b.evidenceForSite(e, e.ImportSites, importIndex, importPath, "package import declaration names the target package")
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelImports,
				Confidence: domain.ConfidenceProven,
				Evidence:   evidence,
			})
		}
	}

	// Controller → CRD/resource relationships (reconciles, watches, creates)
	for _, e := range entities {
		if e.Kind != domain.KindController {
			continue
		}

		for watchIndex, watchName := range e.Watches {
			if candidates := watchTargets(entities, byName, watchName); len(candidates) == 1 {
				target := candidates[0]
				method := "Watches"
				if watchIndex < len(e.WatchMethods) && e.WatchMethods[watchIndex] != "" {
					method = e.WatchMethods[watchIndex]
				} else if len(e.WatchMethods) == 0 && watchIndex == 0 {
					// Preserve compatibility with pre-watchMethods graphs where
					// the first watch was documented as SetupWithManager.For.
					method = "For"
				}
				relType := domain.RelWatches
				confidence := domain.ConfidenceInferred
				reason := "controller watches this resource via SetupWithManager"
				if method == "For" {
					relType = domain.RelReconciles
					confidence = domain.ConfidenceProven
					reason = "controller's SetupWithManager.For(...) identifies the reconciliation target"
				} else if method == "Owns" {
					relType = domain.RelOwns
					reason = "controller owns this resource via SetupWithManager.Owns(...)"
				}
				relID := domain.NewRelationshipID(e.ID, relType, target.ID)
				if seen[relID] {
					continue
				}
				seen[relID] = true
				evidence := b.evidenceForSite(e, e.WatchSites, watchIndex, watchName, reason)
				relationships = append(relationships, domain.Relationship{
					ID:         relID,
					From:       e.ID,
					To:         target.ID,
					Type:       relType,
					Confidence: confidence,
					Evidence:   evidence,
				})
			}
		}

		// A create call proves that code constructs or upserts a named type, but
		// it does not prove which manifest instance is affected. Emit the edge
		// only when exactly one scanned resource has the same Kubernetes kind.
		for _, createName := range e.Creates {
			candidates := resourcesByKind(entities, createName)
			if len(candidates) != 1 {
				continue
			}
			target := candidates[0]
			relID := domain.NewRelationshipID(e.ID, domain.RelCreates, target.ID)
			if seen[relID] {
				continue
			}
			seen[relID] = true
			evidence := b.evidenceForNamedSite(e, e.CreateSites, createName, "explicit create/upsert call; target matched by unique manifest kind")
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelCreates,
				Confidence: domain.ConfidenceInferred,
				Evidence:   evidence,
			})
		}
	}

	// Build function indexes for call matching
	funcByName := make(map[string]domain.Entity)       // bare name (last writer wins, used for unique names)
	funcByQualName := make(map[string]domain.Entity)   // pkg.Name or pkg.Type.Name
	funcNameCount := make(map[string]int)              // count bare name occurrences
	funcsByPkgName := make(map[string][]domain.Entity) // pkg + bare name → all matches
	for _, e := range entities {
		if e.Kind == domain.KindFunction {
			funcByName[e.Name] = e
			funcNameCount[e.Name]++
			// Build qualified key from ID: "function:pkg.Name" → "pkg.Name"
			qual := strings.TrimPrefix(e.ID, "function:")
			funcByQualName[qual] = e
			// Index by package+name for same-package disambiguation
			pkgKey := e.Package + "." + e.Name
			funcsByPkgName[pkgKey] = append(funcsByPkgName[pkgKey], e)
		}
	}

	// Controller → function calls (existing behavior, kept for backward compat)
	for _, e := range entities {
		if e.Kind != domain.KindController || len(e.Calls) == 0 {
			continue
		}
		for _, callName := range e.Calls {
			target, ok := resolveCall(callName, e.Package, funcByName, funcByQualName, funcNameCount, funcsByPkgName)
			if !ok {
				continue
			}
			relID := domain.NewRelationshipID(e.ID, domain.RelCalls, target.ID)
			if seen[relID] {
				continue
			}
			seen[relID] = true
			evidence := b.evidenceForNamedSite(e, e.CallSites, callName, "function called in Reconcile() body")
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelCalls,
				Confidence: domain.ConfidenceInferred,
				Evidence:   evidence,
			})
		}
	}

	// Function → function calls (NEW: Phase 3a)
	for _, e := range entities {
		if e.Kind != domain.KindFunction || len(e.Calls) == 0 {
			continue
		}
		for _, callName := range e.Calls {
			target, ok := resolveCall(callName, e.Package, funcByName, funcByQualName, funcNameCount, funcsByPkgName)
			if !ok || target.ID == e.ID {
				continue
			}
			relID := domain.NewRelationshipID(e.ID, domain.RelCalls, target.ID)
			if seen[relID] {
				continue
			}
			seen[relID] = true
			evidence := b.evidenceForNamedSite(e, e.CallSites, callName, "function call detected in body")
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelCalls,
				Confidence: domain.ConfidenceInferred,
				Evidence:   evidence,
			})
		}
	}

	// Interface assertions remain on the source entity as parser observations.
	// Do not manufacture an interface target: the current entity model does not
	// represent interface declarations, so a name-matched function would be a
	// false relationship. Emit an implements edge only once an interface entity
	// is modeled by a supported parser.

	// Embed relationships: package with //go:embed → resources matched by the
	// actual embed pattern, rather than every resource below the package.
	for _, e := range entities {
		if len(e.EmbedSites) == 0 {
			continue
		}
		for _, site := range e.EmbedSites {
			for _, res := range entities {
				if res.Kind != domain.KindResource || !embedPatternMatches(site, res.Source.File) {
					continue
				}
				relID := domain.NewRelationshipID(e.ID, domain.RelEmbeds, res.ID)
				if seen[relID] {
					continue
				}
				seen[relID] = true
				relationships = append(relationships, domain.Relationship{
					ID:         relID,
					From:       e.ID,
					To:         res.ID,
					Type:       domain.RelEmbeds,
					Confidence: domain.ConfidenceProven,
					Evidence: domain.Evidence{
						Parser:  site.Source.Parser,
						File:    site.Source.File,
						Line:    site.Source.Line,
						Snippet: ReadSnippet(filepath.Join(b.RootDir, site.Source.File), site.Source.Line),
						Reason:  fmt.Sprintf("//go:embed pattern %q matches %s", site.Name, res.Source.File),
					},
				})
			}
		}
	}

	// Test → function relationships (existing)
	funcsByPkgNameForTests := make(map[string][]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindFunction {
			key := e.Package + "." + e.Name
			funcsByPkgNameForTests[key] = append(funcsByPkgNameForTests[key], e)
		}
	}

	for _, e := range entities {
		if e.Kind != domain.KindTest {
			continue
		}

		subject := strings.TrimPrefix(e.Name, "Test")
		key := e.Package + "." + subject
		candidates := funcsByPkgNameForTests[key]
		if len(candidates) != 1 {
			continue
		}
		target := candidates[0]

		snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
		relationships = append(relationships, domain.Relationship{
			ID:         domain.NewRelationshipID(target.ID, domain.RelTestedBy, e.ID),
			From:       target.ID,
			To:         e.ID,
			Type:       domain.RelTestedBy,
			Confidence: domain.ConfidenceInferred,
			Evidence: domain.Evidence{
				Parser:  "test",
				File:    e.Source.File,
				Line:    e.Source.Line,
				Snippet: snippet,
				Reason:  "test function name matches function by convention",
			},
		})
	}
	sort.Slice(relationships, func(i, j int) bool { return relationships[i].ID < relationships[j].ID })
	return relationships
}

func (b *RelationshipBuilder) evidenceForSite(entity domain.Entity, sites []domain.Site, index int, name, reason string) domain.Evidence {
	if index >= 0 && index < len(sites) && (name == "" || sites[index].Name == name) {
		site := sites[index]
		return domain.Evidence{
			Parser:  site.Source.Parser,
			File:    site.Source.File,
			Line:    site.Source.Line,
			Snippet: ReadSnippet(filepath.Join(b.RootDir, site.Source.File), site.Source.Line),
			Reason:  reason,
		}
	}
	return b.evidenceForNamedSite(entity, sites, name, reason)
}

func (b *RelationshipBuilder) evidenceForNamedSite(entity domain.Entity, sites []domain.Site, name, reason string) domain.Evidence {
	for _, site := range sites {
		if name == "" || site.Name == name {
			return domain.Evidence{
				Parser:  site.Source.Parser,
				File:    site.Source.File,
				Line:    site.Source.Line,
				Snippet: ReadSnippet(filepath.Join(b.RootDir, site.Source.File), site.Source.Line),
				Reason:  reason,
			}
		}
	}
	return domain.Evidence{
		Parser:  entity.Source.Parser,
		File:    entity.Source.File,
		Line:    entity.Source.Line,
		Snippet: ReadSnippet(filepath.Join(b.RootDir, entity.Source.File), entity.Source.Line),
		Reason:  reason + " (entity-level fallback)",
	}
}

func resourcesByKind(entities []domain.Entity, kind string) []domain.Entity {
	var matches []domain.Entity
	for _, e := range entities {
		if e.Kind != domain.KindResource || !strings.EqualFold(resourceKind(e), kind) {
			continue
		}
		matches = append(matches, e)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches
}

func watchTargets(entities []domain.Entity, byName map[string][]domain.Entity, watchName string) []domain.Entity {
	seen := make(map[string]bool)
	var matches []domain.Entity
	for _, entity := range byName[watchName] {
		if !seen[entity.ID] {
			seen[entity.ID] = true
			matches = append(matches, entity)
		}
	}
	if len(matches) > 0 {
		return matches
	}
	for _, entity := range entities {
		if entity.Kind != domain.KindResource || !strings.EqualFold(resourceKind(entity), watchName) || seen[entity.ID] {
			continue
		}
		seen[entity.ID] = true
		matches = append(matches, entity)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return matches
}

func resourceKind(entity domain.Entity) string {
	for _, property := range entity.Properties {
		if strings.HasPrefix(property, "kind=") {
			return strings.TrimPrefix(property, "kind=")
		}
	}
	return ""
}

func embedPatternMatches(site domain.Site, resourceFile string) bool {
	pattern := filepath.ToSlash(site.Name)
	pattern = strings.TrimPrefix(pattern, "all:")
	baseDir := filepath.ToSlash(filepath.Dir(site.Source.File))
	resourceFile = filepath.ToSlash(resourceFile)
	relative := resourceFile
	if baseDir != "." && baseDir != "" {
		var err error
		relative, err = filepath.Rel(filepath.FromSlash(baseDir), filepath.FromSlash(resourceFile))
		if err != nil {
			return false
		}
		relative = filepath.ToSlash(relative)
	}
	if strings.HasPrefix(relative, "../") || relative == ".." {
		return false
	}
	if !strings.ContainsAny(pattern, "*?[") && (relative == pattern || strings.HasPrefix(relative, strings.TrimSuffix(pattern, "/")+"/")) {
		return true
	}
	matched, err := path.Match(pattern, relative)
	if err == nil && matched {
		return true
	}
	if !strings.Contains(pattern, "/") && filepath.Dir(relative) == "." {
		matched, err = path.Match(pattern, filepath.Base(relative))
		return err == nil && matched
	}
	return false
}

func resolveCall(callName, callerPkg string, byName map[string]domain.Entity, byQualName map[string]domain.Entity, nameCount map[string]int, byPkgName map[string][]domain.Entity) (domain.Entity, bool) {
	bareName := callName
	if idx := strings.LastIndex(callName, "."); idx >= 0 {
		bareName = callName[idx+1:]
	}
	if skipCallNames[bareName] {
		return domain.Entity{}, false
	}

	// Try qualified match first (e.g., "pkg.FuncName" or "Type.Method")
	if strings.Contains(callName, ".") {
		if target, ok := byQualName[callName]; ok {
			return target, true
		}
	}

	// An unqualified call can only be resolved safely inside the caller's
	// package. A unique name elsewhere in the repository is not evidence that
	// the caller invokes that function.
	if callerPkg != "" {
		candidates := byPkgName[callerPkg+"."+bareName]
		if len(candidates) == 1 {
			return candidates[0], true
		}
	}

	// Same-package disambiguation: when bare name is ambiguous,
	// prefer the match in the caller's package
	if callerPkg != "" && nameCount[bareName] > 1 {
		candidates := byPkgName[callerPkg+"."+bareName]
		if len(candidates) == 1 {
			return candidates[0], true
		}
	}

	return domain.Entity{}, false
}
