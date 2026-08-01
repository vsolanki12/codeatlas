# Phase 7: Compound Queries — `atlas_investigate` + `atlas_explain`

**Status:** Implemented 2026-08-01

## Context

Atlas has 12 MCP tools, all primitives: lookup, search, entity, relationships, where, callers, etc. Investigating a single entity requires 4-5 sequential calls:

1. `atlas_search` → find entity
2. `atlas_entity` → get details
3. `atlas_relationships` → get connections
4. `atlas_callers` → who calls it
5. `atlas_where` → related files

Each call carries ~8K system prompt overhead. 5 calls = 40K tokens just in prompt tax.

Two compound tools collapse this into 1 call each, using pure graph traversal (no AI):

- **`atlas_investigate`** — everything about one entity in one response (~2KB structured output replaces 5 calls)
- **`atlas_explain`** — follow the reconciliation chain to build an architectural narrative (~2KB replaces reading 100KB of source)

## Tool 1: `atlas_investigate`

**Input:** `entity_id` (string)
**Output:** Structured sections covering everything an engineer needs.

### Output Format

```
=== Entity ===
ID: controller:pkg.MyController
Name: MyController
Kind: controller
Package: pkg
File: pkg/controller.go:10
Description: Reconciles HostedCluster resources
Watches: HostedCluster, Secret
Calls: reconcileEtcd, reconcileNetwork, ...+3 more
LastModified: 2026-07-15
LastAuthor: alice@redhat.com
ChangeCount: 42

=== Relationships (3 outgoing, 1 incoming) ===
reconciles:
  -> crd:group.HostedCluster | api/types.go:5
creates:
  -> resource:Deployment.my-deploy | deploy.yaml:1
calls:
  -> function:pkg.reconcileEtcd | pkg/etcd.go:20 | Handles etcd reconciliation

=== Callers (0) ===
(none)

=== Tests (1) ===
test:pkg.TestReconcileEtcd | pkg/etcd_test.go:1

=== Same File (2 others) ===
function:pkg.reconcileNetwork | pkg/controller.go:50
package:pkg | pkg:1
```

### Query Logic — `Investigate(entityID string) *InvestigateResult`

1. `GetEntity(entityID)` — return nil if not found
2. Build `OutRels` map from `idx.fromEntity[entityID]`, resolving each `r.To` via `idx.byID`, grouped by relationship type. Cap 20 per type.
3. Build `InRels` map from `idx.toEntity[entityID]`, resolving each `r.From` via `idx.byID`, grouped by type. Cap 20 per type.
4. Extract Tests — entities reached via `tested_by` edges (outgoing)
5. `Callers(entityID)` — reuse existing method
6. Siblings — `Where(entity.Source.File, 20)`, filter out self

### Types

```go
type ResolvedRel struct {
    Rel    *domain.Relationship
    Target *domain.Entity
}

type InvestigateResult struct {
    Entity   *domain.Entity
    OutRels  map[domain.RelationshipType][]ResolvedRel
    InRels   map[domain.RelationshipType][]ResolvedRel
    Callers  []*domain.Entity
    Tests    []*domain.Entity
    Siblings []*domain.Entity
}
```

## Tool 2: `atlas_explain`

**Input:** `entity_id` (string), `depth` (int, default 2, max 3)
**Output:** Architectural narrative following edge chains.

### Output Format

```
controller:pkg.MyController | pkg/controller.go:10
  Reconciles HostedCluster resources
  reconciles:
    crd:group.HostedCluster | api/types.go:5
  creates:
    resource:Deployment.my-deploy | deploy.yaml:1
  calls:
    function:pkg.reconcileEtcd | pkg/etcd.go:20 | Handles etcd reconciliation
      tested_by:
        test:pkg.TestReconcileEtcd | pkg/etcd_test.go:1
5 nodes explored (depth 2)
```

### Query Logic — `Explain(entityID string, depth int) *ExplainResult`

1. Cap depth to 3. Resolve root entity.
2. Edge traversal priority: `reconciles` → `creates` → `calls` → `tested_by` (outgoing only)
3. Per-type caps: calls max 10, others max 20
4. Global cap: 100 nodes total
5. Recursive DFS with visited set to avoid cycles
6. Build tree of `ExplainNode` structs

### Types

```go
type ExplainNode struct {
    Entity   *domain.Entity
    EdgeType domain.RelationshipType
    Children []*ExplainNode
}

type ExplainResult struct {
    Root       *ExplainNode
    TotalNodes int
    Capped     bool
}
```

## Files Changed

| File | Change | Est. Lines |
|------|--------|-----------|
| `internal/query/query.go` | Types + `Investigate()` + `Explain()` | +120 |
| `internal/query/format.go` | `FormatInvestigation()` + `FormatExplanation()` | +110 |
| `internal/mcpserver/server.go` | `registerInvestigate` + `registerExplain` + update `registerTools` | +50 |
| `internal/query/query_test.go` | `compoundTestGraph()` + 7 tests | +130 |
| `internal/query/format_test.go` | 3 format tests | +60 |

**Total: ~470 lines across 5 existing files. No new files.**

## Implementation Order

1. Types (`ResolvedRel`, `InvestigateResult`, `ExplainNode`, `ExplainResult`) in query.go
2. `Investigate()` method in query.go
3. `Explain()` method in query.go
4. `FormatInvestigation()` in format.go
5. `FormatExplanation()` in format.go
6. `compoundTestGraph()` + query tests in query_test.go
7. Format tests in format_test.go
8. MCP registration in server.go
9. Build + run all tests

## Key Decisions

- **Separate `compoundTestGraph()`** — avoids modifying `testGraph()` which would break 25+ existing tests
- **Explain follows outgoing edges only** — incoming edges create inverse narratives, callers are covered by Investigate
- **Edge priority hardcoded** — `[reconciles, creates, calls, tested_by]`. Other types (watches, depends_on, etc.) skipped to avoid noise. Extensible later.
- **ResolvedRel carries entity pointer** — formatter doesn't need Index access, matches existing pattern
- **`===` section headers for Investigate, tree indentation for Explain** — token-efficient structured text, consistent with existing formatters

## Caps

| What | Cap | Truncation |
|------|-----|-----------|
| Relationships per type per direction | 20 | `...+N more` |
| Callers | 20 | count message |
| Tests | 20 | count message |
| Siblings | 20 | via Where maxResults |
| Explain calls per node | 10 | `...+N more` |
| Explain other edges per node | 20 | `...+N more` |
| Explain total nodes | 100 | `(capped at 100 nodes)` footer |
| Explain depth | 3 | hard cap |

## Verification

1. `go test ./...` — all tests pass (target: ~165 tests)
2. `go build -o atlas ./cmd/atlas`
3. Re-scan: `./atlas scan ~/hypershift -o ~/atlas-graph.json --temporal`
4. MCP smoke tests (after session restart):
   - `atlas_investigate controller:hostedclusters.HostedClusterReconciler` — returns entity + all relationships grouped + callers + tests + siblings + temporal. One call.
   - `atlas_explain controller:hostedclusters.HostedClusterReconciler` — shows reconciles → HostedCluster → creates → HostedControlPlane chain
   - `atlas_investigate function:pki.ReconcileEtcdPeerSecret` — shows callers, tests, same-file entities
5. Token comparison: same CNTRLPLANE-3527 investigation with investigate/explain should need ~3-4 calls total (was 24 with Atlas primitives, 31 with grep)

## Impact

| Metric | Before (12 primitive tools) | After (14 tools, 2 compound) |
|--------|----------------------------|------------------------------|
| Calls per investigation | 4-5 | 1-2 |
| System prompt overhead | 40K tokens (5 calls) | 8-16K tokens (1-2 calls) |
| Output tokens | ~3K (5 responses) | ~2K (1 structured response) |
| MCP tools total | 12 | 14 |
