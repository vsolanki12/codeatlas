# Phase 4: Import Awareness and Package Aggregation

**Status:** Implemented 2026-07-31

## Context

Phase 3a shipped: 8,246 relationships, full call graph. But Atlas treats every import as opaque — can't distinguish stdlib from HyperShift's own packages from external deps like client-go or controller-runtime. "What external repos does this package depend on?" is unanswerable.

Phase 4 adds import observations, same-graph package edges, and merge-aware dedup. It does not link package entities across separate repository graphs.

## Architecture Decisions

- **Import classification stays simple** — the standalone utility uses longest-prefix matches against known repo prefixes, with path-segment boundaries and no module resolution. Its labels are not yet attached to graph import observations, and it does not create cross-repository edges.
- **Stdlib detection** — `IsStdLib()`: no dot in first path segment (`fmt`, `context`, `net/http` vs `github.com/...`)
- **Imports stored on package entities** — `Entity.Imports []string` field. Functions don't track imports individually.
- **Merge-aware dedup** — scanner already deduped package entities (first-seen-wins). Extended to merge `Files` and `Imports` arrays from all instances of same package ID.
- **Search extended** — `Search()` matches against `e.Imports` so "find packages that import controller-runtime" works.

## Files Changed

| File | Change |
|------|--------|
| `internal/origin/origin.go` | New package — `Classify()` (prefix match), `IsStdLib()` |
| `internal/origin/origin_test.go` | Tests for classification |
| `internal/parser/goparser.go` | `extractImports(astFile)` — extracts all import paths |
| `internal/scanner/scanner.go` | Merge-aware dedup: merge `Files` and `Imports` arrays |
| `internal/query/query.go` | `Search()` matches against `e.Imports` |

## Key Functions

```go
// origin.go
func Classify(importPath string) string    // "github.com/openshift/hypershift/..." → "hypershift"
func IsStdLib(importPath string) bool       // "fmt" → true, "github.com/..." → false

// goparser.go
func extractImports(astFile *ast.File) []string  // all import paths from file
```

## Results and Limits

- 225 packages with import data
- 154 packages with merged multi-file data (Files + Imports arrays combined from duplicate package entities)
- `atlas_search "controller-runtime"` finds packages importing it
- `atlas serve` can load multiple graphs and route each query by repository ID or Go module path.
- Cross-graph package resolution is still not implemented; loaded graphs remain separate query indexes.
