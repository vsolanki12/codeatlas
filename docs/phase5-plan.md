# Phase 5: Temporal Layer

**Status:** Implemented 2026-07-31

## Context

Phase 4 shipped: import classification, 225 packages with dependency data. But Atlas has no sense of time — can't answer "when was this function last changed?", "who changed it?", or "what are the most actively developed files?" These questions matter for bug triage (is the fix recent?) and code review (is this a hotspot?).

Phase 5 adds git history enrichment as an opt-in scan step.

## Architecture Decisions

- **Opt-in via `--temporal` flag** — git log is slow (~3m28s for HyperShift). Default scan stays fast.
- **File-level git queries, entity-level distribution** — run `git log` once per unique source file, distribute results to all entities in that file. Avoids N×M git calls.
- **File-level cache** — `map[string]fileHistory` prevents redundant git calls when multiple entities share a file.
- **Three fields per entity** — `LastModified` (ISO timestamp), `LastAuthor` (email), `ChangeCount` (total commits touching file). Stored directly on `domain.Entity`.
- **No commit messages or diffs** — just metadata. Keeps graph size manageable.
- **Graceful degradation** — repos with no commits return nil, nil (not an error). Entities without git data get zero values.

## Files Changed

| File | Change |
|------|--------|
| `internal/temporal/temporal.go` | New package — `Enrich(repoDir string, entities []domain.Entity)` |
| `internal/temporal/temporal_test.go` | Tests for enrichment |
| `internal/domain/entity.go` | Add `LastModified`, `LastAuthor`, `ChangeCount` fields |
| `internal/scanner/scanner.go` | Call `temporal.Enrich()` when `ScanOptions.Temporal` is true |
| `internal/query/query.go` | `Hotspots(kind, stale, limit)` — sort by change count or oldest |
| `internal/mcpserver/server.go` | `atlas_hotspots` MCP tool |
| `cmd/atlas/main.go` | `--temporal` flag on scan subcommand |

## Key Functions

```go
// temporal.go
func Enrich(repoDir string, entities []domain.Entity) error
// Runs: git log --format="%H %aI %ae" --follow <file>
// Runs: git rev-list --count HEAD -- <file>
// Distributes to entities by Source.File match

// query.go
func (idx *Index) Hotspots(kind string, stale bool, limit int) []*domain.Entity
// stale=false: sort by ChangeCount desc (most-changed first)
// stale=true:  sort by LastModified asc (oldest first)
```

## Results

- 10,567 entities with temporal data (out of 11,056 total)
- Top hotspot: `package:hostedcontrolplane` with 2,129 commits
- Scan time with `--temporal`: 3m28s (vs ~15s without)
- `atlas_hotspots` MCP tool enables "what's most actively developed?" queries
