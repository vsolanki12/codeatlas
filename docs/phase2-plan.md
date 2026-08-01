# Phase 2: Query Layer + MCP Server

**Status:** Implemented 2026-07-31

## What This Is

Phase 2 adds a query engine, CLI subcommands, and MCP server to Atlas so Claude Code can query the HyperShift graph directly instead of grepping the repo. Reduces token usage by ~80-90% for codebase navigation.

## Architecture Decisions

- **`atlas serve` subcommand** (not separate binary) — one build, one dispatch, matches existing pattern
- **Load graph per CLI invocation** (~100ms, fine for CLI); **load once at startup** for MCP server
- **No `--format json` flag** — text output optimized for LLM token efficiency
- **Shared query package** — CLI and MCP server both call `internal/query`
- **MCP SDK**: `github.com/modelcontextprotocol/go-sdk v1.6.0` — official Go SDK, stdio transport

## Package Structure

```
internal/query/index.go       — Index type, LoadGraph, in-memory lookup maps
internal/query/query.go       — Query methods (Lookup, Search, Neighbors, etc.)
internal/query/format.go      — Token-efficient text formatters
internal/query/index_test.go
internal/query/query_test.go
internal/query/format_test.go
internal/mcpserver/server.go  — MCP tool registration + handlers
cmd/atlas/main.go             — query/context/where/stats/serve subcommands
```

## Index Design

Pre-computed lookup maps built at load time:

| Map | Key | Value | Purpose |
|-----|-----|-------|---------|
| `byID` | entity ID | `*Entity` | Exact ID lookup |
| `byKind` | `EntityKind` | `[]*Entity` | Filter by kind |
| `byName` | lowercase name | `[]*Entity` | Name search |
| `byPackage` | package name | `[]*Entity` | Package grouping |
| `fromEntity` | entity ID | `[]*Relationship` | Outgoing edges |
| `toEntity` | entity ID | `[]*Relationship` | Incoming edges (ADR-0007: compute inverse) |
| `byRelType` | `RelationshipType` | `[]*Relationship` | Filter by type |

## Query Methods

| Method | Signature | Purpose |
|--------|-----------|---------|
| `GetEntity` | `(id string) *Entity` | Exact ID lookup |
| `Lookup` | `(kind, name string, max int) []*Entity` | Kind (exact) + name (substring, case-insensitive) |
| `GetRelationships` | `(entityID, direction, relType string) []*Relationship` | from/to/both, optional type filter |
| `Neighbors` | `(entityID string, depth int) *Subgraph` | BFS subgraph (depth clamped to 3, max 50 entities) |
| `Search` | `(query string, max int) []*Entity` | Substring across name, description, package, ID |
| `Where` | `(path string, max int) []*Entity` | File path substring match |
| `Stats` | `() *GraphStats` | Counts by kind/type |

All results sorted by ID for determinism (ADR-0009).

## CLI Subcommands

```bash
atlas query <kind> [name] [--graph path]     # Lookup entities + relationships
atlas context <entity-id> [--depth N] [--graph path]  # Subgraph around entity
atlas where <path> [--graph path]             # Find entities by file path
atlas stats [--graph path]                    # Entity/relationship counts
atlas serve [--graph path]                    # Start MCP server (stdio)
```

**Note:** `--graph` flag must come before positional args (Go flag package behavior).

## MCP Server Tools

| Tool | Input | Maps to |
|------|-------|---------|
| `atlas_lookup` | kind?, name? | `idx.Lookup()` |
| `atlas_entity` | id (required) | `idx.GetEntity()` + `GetRelationships()` |
| `atlas_relationships` | entity_id, direction?, type? | `idx.GetRelationships()` |
| `atlas_context` | entity_id, depth? | `idx.Neighbors()` |
| `atlas_search` | query (required) | `idx.Search()` |
| `atlas_where` | path (required) | `idx.Where()` |
| `atlas_stats` | (none) | `idx.Stats()` |

## Claude Code Registration

Add to `~/.claude/settings.json`:
```json
{
  "mcpServers": {
    "codeatlas": {
      "command": "/Users/vsolanki/codeatlas/atlas",
      "args": ["serve", "--graph", "/Users/vsolanki/codeatlas/atlas-graph.json"],
      "description": "CodeAtlas — query entities, relationships, and subgraphs from a codebase"
    }
  }
}
```

## Token Impact

| Operation | Without Atlas | With Atlas MCP |
|-----------|--------------|----------------|
| Find controller | ~50K (grep + reads) | ~500 (atlas_lookup) |
| Trace relationships | ~30K (more reads) | ~1K (atlas_relationships) |
| Find tests | ~20K (grep) | ~500 (atlas_search) |
| Read relevant code | ~20K (full files) | ~5K (targeted lines via atlas_where) |
| **Total per investigation** | **~120K** | **~7K** |

## Future Phases

- **Phase 3**: Deeper code analysis — full call graph, interface implementations, struct field tracking
- **Phase 4**: Cross-repo intelligence — api, library-go, cluster-api, enhancements
- **Phase 5**: Temporal layer — git blame per entity, change frequency, PR history
- **Phase 6**: Semantic/AI layer — natural language queries, embeddings
