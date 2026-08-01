# Phase 6: Content Indexing

**Status:** Implemented 2026-07-31

## Context

Phase 5 shipped: temporal data on 10,567 entities. But Atlas can't see inside function bodies or YAML manifests. `atlas_search "etcd-discovery"` returns nothing because string literals like `"etcd-discovery.%s.svc"` aren't indexed. Similarly, searching for a specific Kubernetes resource by its metadata.name or label selector fails because YAML key-value pairs aren't searchable.

Phase 6 indexes function body content and YAML properties to make the graph searchable at the content level.

## Architecture Decisions

- **Literals on function entities** — `Entity.Literals []string` field. Extracted from `*ast.BasicLit` string tokens in function bodies.
- **Literal filtering** — length >= 4, must contain `./-_:` (structural characters). Filters noise like `"ok"`, `"err"`. Cap at 50 per function.
- **YAML properties on resource entities** — `Entity.Properties []string` field. Flattened key=value pairs from YAML manifests.
- **YAML flattening** — recursive walk, max depth 5. Skips noisy keys (status, metadata.managedFields, metadata.annotations). Format: `spec.selector.app=etcd`.
- **Embeds on package entities** — `Entity.Embeds []string` field. Detected from `//go:embed` directives in Go source.
- **RelEmbeds relationship** — links packages with `//go:embed` to resource entities under same directory. New relationship type.
- **Search extended** — `Search()` matches against `Literals` and `Properties`.
- **Dedup merges all new fields** — scanner merge-aware dedup extended to handle Literals, Properties, Embeds.
- **Schema version bumped to 1.2.0** — new fields are backward-incompatible for consumers expecting old schema.

## Files Changed

| File | Change |
|------|--------|
| `internal/domain/entity.go` | Add `Literals`, `Properties`, `Embeds` fields |
| `internal/domain/relationship.go` | Add `RelEmbeds` constant |
| `internal/parser/goparser.go` | `extractLiterals(body)` — string literal extraction |
| `internal/parser/goparser_test.go` | `TestExtractLiterals` with test fixture |
| `internal/parser/testdata/literals.go` | Test fixture for literal extraction |
| `internal/parser/yamlparser.go` | `flattenYAML()` — recursive YAML key-value flattening |
| `internal/graph/builder.go` | Build `embeds` relationships |
| `internal/scanner/scanner.go` | Dedup merges Literals, Properties, Embeds |
| `internal/query/query.go` | `Search()` matches Literals and Properties |
| `internal/query/format.go` | Show Literals/Properties/Embeds in FormatEntityFull |

## Key Functions

```go
// goparser.go
func extractLiterals(body *ast.BlockStmt) []string
// Walks AST, captures *ast.BasicLit strings matching filter

// goparser.go (added Phase 6, extended Phase 6c)
func extractEmbeds(astFile *ast.File) []string
// Scans comments for //go:embed directives

// yamlparser.go
func flattenYAML(data map[string]interface{}, prefix string, depth int) []string
// Recursive flattening: {"spec": {"selector": {"app": "etcd"}}} → "spec.selector.app=etcd"
```

## Results

- 3,100 entities with literals (10,920 literal entries total)
- 398 entities with properties (5,916 property entries total)
- 16 packages with embeds
- 425 embeds relationships
- Schema version: 1.2.0
- `atlas_search "etcd-discovery"` now finds functions using that string literal
- `atlas_search "spec.selector.app=etcd"` finds YAML resources by label
