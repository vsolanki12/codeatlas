# Phase 7b: `atlas_impact` — Blast Radius Analysis

**Status:** Implemented 2026-08-01

## Context

Phase 7 delivered two compound queries:
- `atlas_investigate` — "What IS this entity?" (details, relationships, callers, tests, siblings)
- `atlas_explain` — "What does this entity DO?" (reconciliation chain, architectural narrative)

Missing: "What BREAKS if I change this?" — the reverse dependency walk. A PR reviewer's first question.

`atlas_impact` walks the call chain UPSTREAM from an entity: who calls it, who calls those callers, what controllers sit at the top, what tests cover the chain, what resources are affected, who owns the code. One call replaces the entire "understand the blast radius" investigation.

## Tool: `atlas_impact`

**Input:** `entity_id` (string)
**Output:** Structured blast radius analysis.

### Output Format

```
=== Impact: function:pkg.reconcileEtcd ===
pkg/etcd.go:20 | Handles etcd reconciliation

=== Call Chain (2 callers) ===
controller:pkg.MyController | pkg/controller.go:10 | Reconciles HostedCluster resources
function:pkg.setup | pkg/setup.go:1 | -

=== Controllers (1) ===
controller:pkg.MyController | pkg/controller.go:10 | Reconciles HostedCluster resources

=== Tests (2) ===
test:pkg.TestReconcileEtcd | pkg/etcd_test.go:1 | -
test:pkg.TestMyController | pkg/controller_test.go:1 | -

=== Resources (2) ===
crd:group.HostedCluster | api/types.go:5
resource:Deployment.my-deploy | deploy.yaml:1

=== Files Affected (3) ===
pkg/etcd.go
pkg/controller.go
pkg/setup.go

=== Recent Changes (2) ===
function:pkg.reconcileEtcd | changes=5 last=2026-07-15 by=alice@redhat.com
controller:pkg.MyController | changes=42 last=2026-07-20 by=bob@redhat.com

=== Owners (2) ===
alice@redhat.com
bob@redhat.com
```

### Query Logic — `Impact(entityID string) *ImpactResult`

1. `GetEntity(entityID)` — return nil if not found
2. BFS up incoming `calls` edges:
   - Seed: `[entityID]`
   - For each entity, find `toEntity[id]` where `Type == RelCalls` — the `From` is a caller
   - Track visited set (cycle prevention)
   - Depth cap: 5 levels up
   - Entity cap: 50 total in chain
3. For ALL entities in chain (including root):
   - If `Kind == KindController` → add to Controllers
   - Check outgoing `tested_by` edges → add targets to Tests
   - Check outgoing `reconciles` / `creates` edges → add targets to Resources
   - Collect `Source.File` → Files
   - If `LastAuthor != ""` → add to Owners, add entity to RecentChanges
4. Deduplicate all lists. Sort Controllers/Tests/Resources by ID. Sort Files alphabetically. Sort Owners alphabetically. Sort RecentChanges by LastModified desc.

### Types

```go
type ImpactResult struct {
    Entity        *domain.Entity
    CallChain     []*domain.Entity   // transitive callers (BFS order, root excluded)
    Controllers   []*domain.Entity   // controllers found in call chain
    Tests         []*domain.Entity   // tests covering any entity in chain
    Resources     []*domain.Entity   // resources touched by any entity in chain
    Files         []string           // unique source files in chain
    RecentChanges []*domain.Entity   // entities with temporal data
    Owners        []string           // unique LastAuthors
}
```

## Comparison with Investigate/Explain

| Dimension | Investigate | Explain | Impact |
|-----------|------------|---------|--------|
| Direction | Both (in + out) | Outgoing only | Incoming (upstream) |
| Depth | 1 hop | 2-3 hops down | 5 hops up |
| Question | "What is this?" | "What does this do?" | "What breaks?" |
| Use case | Understand entity | Understand architecture | PR review |
| Traversal | Direct neighbors | DFS reconciles→creates→calls→tested_by | BFS up calls chain |

## Files Changed

| File | Change | Est. Lines |
|------|--------|-----------|
| `internal/query/query.go` | `ImpactResult` type + `Impact()` method | +60 |
| `internal/query/format.go` | `FormatImpact()` | +60 |
| `internal/query/query_test.go` | 5 impact tests | +80 |
| `internal/query/format_test.go` | 1 format test | +40 |
| `internal/mcpserver/server.go` | `registerImpact` + update `registerTools` | +20 |

**Total: ~260 lines across 5 existing files.**

## Caps

| What | Cap |
|------|-----|
| Call chain depth | 5 levels |
| Total chain entities | 50 |
| Tests | 30 |
| Resources | 30 |
| Files | 50 |
| Owners | 20 |

## Verification

1. `go test ./...` — all tests pass (target: ~170 tests)
2. `go build -o atlas ./cmd/atlas`
3. MCP smoke test: `atlas_impact function:pki.ReconcileEtcdPeerSecret`
   - Should show caller chain up to HostedClusterReconciler
   - Should show tests covering each level
   - Should show affected CRDs/resources
   - Should show file list and owners
