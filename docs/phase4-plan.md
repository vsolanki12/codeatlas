# Phase 4: Cross-Repo Intelligence

**Status:** Implemented 2026-07-31

## Context

Phase 3a shipped: 8,246 relationships, full call graph. But Atlas treats every import as opaque — can't distinguish stdlib from HyperShift's own packages from external deps like client-go or controller-runtime. "What external repos does this package depend on?" is unanswerable.

Phase 4 adds import path classification and merge-aware dedup so Atlas understands dependency boundaries.

## Architecture Decisions

- **Import classification stays simple** — longest-prefix match against known repo prefixes, no module resolution. Good enough for "which packages talk to client-go?"
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

## Results

- 225 packages with import data
- 154 packages with merged multi-file data (Files + Imports arrays combined from duplicate package entities)
- `atlas_search "controller-runtime"` finds packages importing it
