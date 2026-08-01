# Phase 6b: Token Optimization

**Status:** Implemented 2026-07-31

## Context

Phase 6 shipped: content indexing working. OCPBUGS-99878 investigation benchmark showed Atlas competitive with grep (22 vs 28 tool calls). But analysis revealed token waste:

1. Single-term search returns matches sorted alphabetically — irrelevant results rank same as perfect matches
2. `atlas_entity` always returns full detail + relationships even when caller just wants file:line
3. Investigating a single file requires N entity lookups (one per function in file)
4. Multi-word searches require post-filtering — `atlas_search "etcd peer"` matches "etcd" OR "peer"

Phase 6b fixes the most impactful token waste patterns.

## Changes

### 1. AND Search Semantics

`Search()` splits query on whitespace. ALL terms must match (can hit different fields). `matchesAllTerms()` iterates terms, calls `matchesTerm()` for each. Single term backward compatible.

Example: `"etcd peer"` matches only entities where "etcd" appears in one field AND "peer" in another.

### 2. Brief Mode for `atlas_entity`

New `brief` parameter (default false). When true, returns single-line `FormatEntity` output (ID | file:line | description). Skips relationships. ~90% token reduction for "just need the location" lookups.

### 3. Batch Entity Fetch — `atlas_entities`

New MCP tool. Takes `ids []string`, returns brief format for all. Replaces N sequential `atlas_entity` calls with 1. Used after `atlas_search` or `atlas_relationships` to resolve IDs in bulk.

### 4. Detail Mode for `atlas_where`

New `detail` parameter on `atlas_where`. When true, returns `FormatEntityDetailList` — full entity details for every match. Replaces pattern: `atlas_where` → get IDs → N × `atlas_entity`. Single call for "show me everything in this file."

## Files Changed

| File | Change |
|------|--------|
| `internal/query/query.go` | `matchesTerm()`, `matchesAllTerms()` helpers for AND search |
| `internal/query/query_test.go` | 4 new AND search tests |
| `internal/query/format.go` | `FormatEntityDetailList()` |
| `internal/mcpserver/server.go` | `brief` on entityInput, `detail` on whereInput, new `entitiesInput` + `registerEntities` |

## Results

- 10 MCP tools (was 8)
- AND search eliminates false positives from multi-term queries
- Brief mode: ~90% token reduction per entity lookup
- Batch fetch: N calls → 1 call
- Detail mode: file investigation goes from ~20 calls to 1
- Projected: ~8-10 tool calls per bug investigation (was 22)
