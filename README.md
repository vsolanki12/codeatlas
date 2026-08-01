# CodeAtlas

> Build a structured architecture graph from any Go codebase. Query it with 15 MCP tools. Give your AI assistant architectural knowledge instead of raw source files.

CodeAtlas parses Go source, Kubernetes manifests, documentation, and tests to produce a single JSON graph of every entity and relationship in a codebase. AI assistants query this graph through the [Model Context Protocol](https://modelcontextprotocol.io/) instead of reading thousands of source files — faster, cheaper, and more accurate.

CodeAtlas never invents architecture. It extracts it. Every relationship is backed by evidence from the source code itself.

---

## The Problem

Large codebases are hostile to newcomers and expensive for AI. A project with 11,000+ entities across hundreds of packages has architecture that only exists in the heads of people who've been there for years. Without a graph, an AI assistant reads source files one by one — slow, token-heavy, and incomplete. CodeAtlas solves both: engineers get a queryable architecture map on day one, and AI assistants get structured answers instead of raw code.

---

## How It Works

```
Source Repository
        │
        ▼
   Atlas Scanner          Parses Go AST, YAML, Markdown, Tests
        │
        ▼
   Atlas Graph            Single JSON file — the product
        │
   ┌────┼────┬────────┐
   ▼    ▼    ▼        ▼
  CLI  MCP Server  API (future)
            │
    ┌───────┼──────┐
    ▼       ▼      ▼
 Claude   VS Code  Any MCP
 Code     Cursor   Client
```

Three rules:
1. **The scanner is the only thing that parses code.** Everything else reads the graph.
2. **The graph is the product.** Every consumer reads the same JSON.
3. **Consumers are replaceable.** Adding a consumer never changes the graph.

---

## Quick Start

```bash
# Build
go build -o atlas ./cmd/atlas

# Scan a Go repository
atlas scan -repo /path/to/your/project -output atlas-graph.json

# Scan with git history (slower, enables hotspot and ownership queries)
atlas scan -repo /path/to/your/project -output atlas-graph.json -temporal

# Query from the command line
atlas query -graph atlas-graph.json controller HostedCluster

# Start the MCP server
atlas serve --graph atlas-graph.json
```

---

## Connect to Your AI Assistant

Add CodeAtlas to your MCP client config. For Claude Code (`~/.mcp.json`):

```json
{
  "mcpServers": {
    "codeatlas": {
      "command": "/path/to/atlas",
      "args": ["serve", "--graph", "/path/to/atlas-graph.json"]
    }
  }
}
```

Restart your AI assistant. CodeAtlas tools are now available.

---

## Example Queries

Once connected, your AI assistant can answer architecture questions directly:

**"How does HostedCluster reconciliation work?"**
→ `atlas_explain` traces the reconciliation chain: what it reconciles, what it creates, what it calls, what tests cover it.

**"What breaks if I change this function?"**
→ `atlas_impact` walks upstream callers to find every controller, test, resource, and file affected.

**"Why is this controller failing?"**
→ `atlas_investigate` returns full entity details, all relationships, callers, tests, and sibling entities in one call.

**"Which code changes most often but has no tests?"**
→ `atlas_hotspots` finds high-churn entities; `atlas_impact` checks their test coverage.

---

## MCP Tools

15 tools served via `atlas serve`:

| Tool | Purpose |
|------|---------|
| `atlas_investigate` | Everything about one entity in 1 call (replaces 4-5 primitives) |
| `atlas_explain` | Architectural narrative: reconciles → creates → calls → tested_by tree |
| `atlas_impact` | Blast radius: upstream callers, controllers, tests, resources, owners |
| `atlas_search` | Relevance-ranked text search across all entity fields |
| `atlas_where` | Find entities by file path |
| `atlas_lookup` | Find entities by kind and/or name |
| `atlas_entity` | Full entity detail by exact ID |
| `atlas_entities` | Batch fetch multiple entities |
| `atlas_relationships` | Relationships for an entity |
| `atlas_context` | BFS subgraph around an entity |
| `atlas_callers` | Reverse call graph |
| `atlas_hotspots` | Most-changed or stalest entities (requires `-temporal` scan) |
| `atlas_commits` | Search by name, date, or author in git history |
| `atlas_stats` | Graph statistics |

---

## Documentation

| Document | Purpose |
|----------|---------|
| [Vision](docs/vision.md) | Why CodeAtlas exists, who it's for, where it's going |
| [Overview](docs/overview.md) | CodeAtlas in 5 minutes |
| [Architecture](docs/architecture.md) | Product, code, and pipeline architecture |
| [Data Model](docs/data-model.md) | Entity and relationship schema specification |
| [Roadmap](docs/roadmap.md) | Phase-by-phase development history and future plans |
| [ADRs](docs/adr/) | 15 Architecture Decision Records |

---

## Project Status

**Schema:** 1.2.0 · **MCP Tools:** 15 · **In Progress:** Phase 8 (Intent-Based Tool Guidance)

See [roadmap.md](docs/roadmap.md) for full development history and future plans.
