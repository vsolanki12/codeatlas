# Phase 6c: Search Quality + Call Reduction

**Status:** Implemented 2026-07-31

## Context

OCPBUGS-99843 dual investigation (Atlas vs grep) revealed 4 remaining gaps:

1. Search results sorted alphabetically, not by relevance — a function named `ReconcileEtcdPeer` ranks same as a resource where "etcd" appears in a property
2. Finding "who calls X" requires 2+ calls: `atlas_relationships(id, to, calls)` → get From IDs → `atlas_entities([ids])`
3. Condition type constants (`PreviousCertificatesRevokedType`) invisible to search — only `*ast.BasicLit` strings captured, not `*ast.SelectorExpr`
4. No temporal search path — "when was this function last changed?" forces bash `git log` fallback despite Atlas having the data

## Changes

### 1. Relevance-Ranked Search

Replaced boolean `matchesAllTerms` with scored `scoreEntity`/`scoreTerm`. Scoring tiers:

| Field | Score |
|-------|-------|
| Name | 100 |
| ID | 90 |
| Description | 70 |
| Package | 60 |
| Import | 40 |
| Literal | 30 |
| Property | 20 |

Multi-term: scores summed across terms. If any term scores 0, entity excluded (AND semantics preserved). Results sorted by total score desc, then ID asc for determinism.

### 2. `atlas_callers` Tool

New MCP tool. `Callers(entityID)` traverses `toEntity` map, filters `RelCalls`, returns caller entities. Replaces 2-call pattern with 1.

```go
func (idx *Index) Callers(entityID string) []*domain.Entity
```

### 3. Selector/Constant Extraction

Extended `extractLiterals` to capture `*ast.SelectorExpr` — the selector name from expressions like `certificatesv1alpha1.PreviousCertificatesRevokedType`. Filter: exported (`ast.IsExported`), length >= 8. Captures meaningful constant names, filters short method names like `Do` or `Sprintf`.

### 4. `atlas_commits` Tool

New MCP tool. `Commits(name, since, author, limit)` searches entities with temporal data. Filters by name substring, date cutoff, author substring. Sorted by LastModified desc. Replaces `git log` fallback.

```go
func (idx *Index) Commits(name, since, author string, limit int) []*domain.Entity
```

## Files Changed

| File | Change |
|------|--------|
| `internal/query/query.go` | `scoreEntity`/`scoreTerm` replacing `matchesAllTerms`, `Callers()`, `Commits()` |
| `internal/query/query_test.go` | Relevance order test, callers tests (3), commits tests (5) |
| `internal/parser/goparser.go` | `*ast.SelectorExpr` capture in `extractLiterals` |
| `internal/parser/goparser_test.go` | `TestExtractLiterals_Selectors` |
| `internal/parser/testdata/selectors.go` | Test fixture with exported selector references |
| `internal/mcpserver/server.go` | `registerCallers`, `registerCommits` |

## Results

- 12 MCP tools (was 10)
- Name-match entities rank before literal/property matches
- `atlas_search "PreviousCertificatesRevoked"` finds functions using that constant
- `atlas_callers` replaces 2-call relationship+entity lookup with 1 call
- `atlas_commits` replaces `git log` bash fallback
- Projected: ~6-7 tool calls per bug investigation (was 8-10)
- 148 tests, all green
