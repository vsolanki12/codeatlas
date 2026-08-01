# CodeAtlas

> Understand any codebase in minutes instead of hours by generating architecture directly from source code.

Atlas is a code intelligence platform that parses Go source, Kubernetes manifests, documentation, and tests to build a structured graph of every entity and relationship in a codebase. It answers questions like *what creates this?*, *which tests cover this?*, *what breaks if I change this?* — deterministically, backed by evidence from the source code itself.

Atlas never invents architecture. It extracts it.

---

## Project Status

**Current Release:** v0.1.0 (Atlas Core Foundation)

**Completed:**
- Repository discovery and file classification
- Go AST parser (controllers, functions, packages, imports, literals, embeds, call graphs)
- YAML parser (CRDs, Deployments, Services, property flattening)
- Markdown and Test parsers
- Typed relationships with evidence (proven/inferred)
- Merge-aware entity deduplication
- Temporal enrichment (git history: authorship, change counts)
- Content indexing (string literals, YAML properties, go:embed)
- Query engine (relevance-ranked search, compound queries)
- MCP server (15 tools via Model Context Protocol)
- Compound queries (investigate, explain, impact)
- Architecture documentation (ADRs, roadmap, vision, overview)

**In Progress:**
- Intent-based tool guidance (Phase 8 — enriched MCP descriptions)

**Planned:**
- Version intelligence (graph diff across branches/releases)
- Continuous architecture intelligence (PR diff, drift, orphans, dead code)
- Multi-repository knowledge
- AI consumer ecosystem (VS Code, Cursor, Continue.dev, any MCP client)

---

## Quick Start

```bash
# Build
go build -o atlas ./cmd/atlas

# Scan a repository
atlas scan -repo ~/hypershift -output atlas-graph.json

# Scan with git history (slower, enables hotspot/ownership queries)
atlas scan -repo ~/hypershift -output atlas-graph.json -temporal

# Query
atlas query -graph atlas-graph.json controller HostedCluster

# Serve for AI assistants (Claude Code, VS Code, etc.)
atlas serve --graph atlas-graph.json
```

---

## Architecture

```
Source Repository
        │
        ▼
    Discovery          Walk the repo, classify files
        │
        ▼
     Parsers           Go AST, YAML, Markdown, Test
        │
        ▼
   Graph Builder       Typed relationships with evidence
        │
        ▼
    Atlas Graph        JSON file — the product
        │
        ▼
   Query Engine        Loads graph, answers queries
        │
        ▼
    MCP Server         15 tools via Model Context Protocol
        │
        ▼
    Consumers          Claude Code, VS Code, Cursor, any MCP client
```

Four layers:

| Layer | What | Purpose |
|-------|------|---------|
| **Knowledge** | Scanner → Graph | Extract architecture from code |
| **Retrieval** | 15 MCP tools | Answer questions about the graph |
| **Guidance** | Tool descriptions | Teach consumers which tool serves which engineering intent |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Where engineers interact with Atlas |

Three rules:
1. **The scanner is the only thing that parses code.** Everything else reads the graph.
2. **The graph is the product.** Every consumer reads the same JSON.
3. **Consumers are replaceable.** Adding a consumer never changes the graph.

---

## Core Principles

- **Extract, never invent.** If a parser can't determine a fact, Atlas doesn't guess.
- **Evidence for every relationship.** Every edge carries a file, line, snippet, and reason.
- **Deterministic before intelligent.** Same commit, same graph, every time.
- **Graph is the product.** Everything else consumes it.
- **Consumers are replaceable.** Adding or removing a consumer never changes the graph.

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
| `atlas_hotspots` | Most-changed or stalest entities |
| `atlas_commits` | Temporal search by name/since/author |
| `atlas_stats` | Graph statistics |

---

## Project Structure

```
codeatlas/
├── cmd/atlas/              CLI: scan, query, serve, stats, where, context
├── internal/
│   ├── domain/             Entity, Relationship, Evidence, Graph, Source
│   ├── discovery/          File walker
│   ├── parser/             Go AST, YAML, Markdown, Test parsers
│   ├── graph/              Relationship builder + graph validation
│   ├── scanner/            Pipeline orchestrator with merge-aware dedup
│   ├── storage/            JSON read/write
│   ├── origin/             Import path classifier (stdlib vs known repos)
│   ├── temporal/           Git history enrichment
│   ├── query/              Query engine: Index, search, traversal, compound queries
│   └── mcpserver/          MCP server: 15 tools via go-sdk
└── docs/
    ├── vision.md           Why Atlas exists
    ├── overview.md         Atlas in 5 minutes
    ├── architecture.md     Architecture deep-dive
    ├── data-model.md       Graph schema specification
    ├── roadmap.md          Phase-by-phase development history
    └── adr/                15 Architecture Decision Records
```

---

## Non-Goals

Atlas will never become:

- **A CI/CD platform.** Atlas reads code, it doesn't build or deploy it.
- **A Kubernetes dashboard.** Atlas understands the code that manages clusters, not the clusters themselves.
- **An IDE.** Atlas provides intelligence to IDEs, it isn't one.
- **A documentation generator.** Atlas extracts architecture, it doesn't write docs.
- **An AI that invents architecture.** If a parser can't determine it, Atlas doesn't guess.

---

## Documentation

| Document | Purpose |
|----------|---------|
| [Vision](docs/vision.md) | Why Atlas exists, who it's for, where it's going |
| [Overview](docs/overview.md) | Atlas in 5 minutes — quick start for new readers |
| [Architecture](docs/architecture.md) | Product, code, and pipeline architecture |
| [Data Model](docs/data-model.md) | Entity and relationship schema specification |
| [Roadmap](docs/roadmap.md) | Phase-by-phase development history and future plans |
| [ADRs](docs/adr/) | 15 Architecture Decision Records |
