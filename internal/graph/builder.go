package graph

import (
	"path/filepath"
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
	byName := make(map[string]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindCRD || e.Kind == domain.KindResource {
			byName[e.Name] = e
		}
	}

	var relationships []domain.Relationship

	seen := make(map[string]bool)

	// Controller → CRD/resource relationships (reconciles, creates)
	for _, e := range entities {
		if e.Kind != domain.KindController || len(e.Watches) == 0 {
			continue
		}

		if target, ok := byName[e.Watches[0]]; ok {
			relID := domain.NewRelationshipID(e.ID, domain.RelReconciles, target.ID)
			if !seen[relID] {
				seen[relID] = true
				snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
				relationships = append(relationships, domain.Relationship{
					ID:         relID,
					From:       e.ID,
					To:         target.ID,
					Type:       domain.RelReconciles,
					Confidence: domain.ConfidenceProven,
					Evidence: domain.Evidence{
						Parser:  "graph",
						File:    e.Source.File,
						Line:    e.Source.Line,
						Snippet: snippet,
						Reason:  "controller has Reconcile() method",
					},
				})
			}
		}

		for _, watchName := range e.Watches[1:] {
			if target, ok := byName[watchName]; ok {
				relID := domain.NewRelationshipID(e.ID, domain.RelWatches, target.ID)
				if seen[relID] {
					continue
				}
				seen[relID] = true
				snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
				relationships = append(relationships, domain.Relationship{
					ID:         relID,
					From:       e.ID,
					To:         target.ID,
					Type:       domain.RelWatches,
					Confidence: domain.ConfidenceInferred,
					Evidence: domain.Evidence{
						Parser:  "graph",
						File:    e.Source.File,
						Line:    e.Source.Line,
						Snippet: snippet,
						Reason:  "controller watches this resource via SetupWithManager",
					},
				})
			}
		}
	}

	// Build function indexes for call matching
	funcByName := make(map[string]domain.Entity)            // bare name (last writer wins, used for unique names)
	funcByQualName := make(map[string]domain.Entity)         // pkg.Name or pkg.Type.Name
	funcNameCount := make(map[string]int)                    // count bare name occurrences
	funcsByPkgName := make(map[string][]domain.Entity)       // pkg + bare name → all matches
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
			snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelCalls,
				Confidence: domain.ConfidenceInferred,
				Evidence: domain.Evidence{
					Parser:  "graph",
					File:    e.Source.File,
					Line:    e.Source.Line,
					Snippet: snippet,
					Reason:  "function called in Reconcile() body",
				},
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
			snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
			relationships = append(relationships, domain.Relationship{
				ID:         relID,
				From:       e.ID,
				To:         target.ID,
				Type:       domain.RelCalls,
				Confidence: domain.ConfidenceInferred,
				Evidence: domain.Evidence{
					Parser:  "graph",
					File:    e.Source.File,
					Line:    e.Source.Line,
					Snippet: snippet,
					Reason:  "function call detected in body",
				},
			})
		}
	}

	// Implements relationships (NEW: Phase 3a)
	entityByKindName := make(map[string]domain.Entity) // "kind:pkg.Name" for controller/function lookup
	for _, e := range entities {
		if e.Kind == domain.KindController || e.Kind == domain.KindFunction {
			entityByKindName[e.ID] = e
		}
	}

	for _, e := range entities {
		if len(e.Implements) == 0 {
			continue
		}
		for _, ifaceName := range e.Implements {
			var targetID string
			for _, candidate := range entities {
				if candidate.Kind == domain.KindFunction && candidate.Name == ifaceName {
					targetID = candidate.ID
					break
				}
			}
			if targetID == "" {
				continue
			}
			snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
			relationships = append(relationships, domain.Relationship{
				ID:         domain.NewRelationshipID(e.ID, domain.RelImplements, targetID),
				From:       e.ID,
				To:         targetID,
				Type:       domain.RelImplements,
				Confidence: domain.ConfidenceInferred,
				Evidence: domain.Evidence{
					Parser:  "graph",
					File:    e.Source.File,
					Line:    e.Source.Line,
					Snippet: snippet,
					Reason:  "var _ assertion detected, target resolved by name match",
				},
			})
		}
	}

	// Embed relationships: package with //go:embed → resource entities under same dir tree
	for _, e := range entities {
		if len(e.Embeds) == 0 {
			continue
		}
		embedDir := filepath.Dir(e.Source.File)
		for _, res := range entities {
			if res.Kind != domain.KindResource {
				continue
			}
			resDir := filepath.Dir(res.Source.File)
			if !strings.HasPrefix(resDir, embedDir) {
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
				Confidence: domain.ConfidenceInferred,
				Evidence: domain.Evidence{
					Parser: "graph",
					File:   e.Source.File,
					Line:   1,
					Reason: "//go:embed detected, resource matched by directory proximity",
				},
			})
		}
	}

	// Test → function relationships (existing)
	funcByPkgName := make(map[string]domain.Entity)
	for _, e := range entities {
		if e.Kind == domain.KindFunction {
			key := e.Package + "." + e.Name
			funcByPkgName[key] = e
		}
	}

	for _, e := range entities {
		if e.Kind != domain.KindTest {
			continue
		}

		subject := strings.TrimPrefix(e.Name, "Test")
		key := e.Package + "." + subject
		target, ok := funcByPkgName[key]
		if !ok {
			continue
		}

		snippet := ReadSnippet(filepath.Join(b.RootDir, e.Source.File), e.Source.Line)
		relationships = append(relationships, domain.Relationship{
			ID:         domain.NewRelationshipID(target.ID, domain.RelTestedBy, e.ID),
			From:       target.ID,
			To:         e.ID,
			Type:       domain.RelTestedBy,
			Confidence: domain.ConfidenceInferred,
			Evidence: domain.Evidence{
				Parser:  "graph",
				File:    e.Source.File,
				Line:    e.Source.Line,
				Snippet: snippet,
				Reason:  "test function name matches function by convention",
			},
		})
	}
	return relationships
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

	// Try unique bare name
	if target, ok := byName[bareName]; ok && nameCount[bareName] == 1 {
		return target, true
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
