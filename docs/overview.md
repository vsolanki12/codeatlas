# CodeAtlas in 5 Minutes

Understand any codebase in minutes instead of hours by generating architecture directly from source code.

Atlas is a code intelligence platform that works with any Go repository. It parses Go source, YAML, docs, and tests to produce a structured graph of every entity and relationship in the codebase.

Atlas never invents architecture. Every entity and relationship must be traceable back to source code.

## End-to-End Pipeline

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
   Query Engine        Loads Atlas Graph and answers graph queries
        │
        ▼
    MCP Server         15 tools via Model Context Protocol
        │
        ▼
    Consumers          Claude Code, VS Code, Cursor, any MCP client
```

## Four Layers

| Layer | What | Changes |
|-------|------|---------|
| **Knowledge** | Scanner → Graph | Every scan |
| **Retrieval** | 15 MCP tools | Quarterly |
| **Guidance** | Tool descriptions with intent hints | Rarely |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Consumer-driven |

## Key Numbers

| Metric | Value |
|--------|-------|
| MCP tools | 15 |
| Go packages | 11 |
| Schema version | 1.2.0 |

Run `atlas serve` then `atlas_stats` for current entity/relationship counts from your latest scan.

## Stability Levels

| Level | Components | What It Means |
|-------|-----------|---------------|
| **Stable** | Entity, Relationship, Graph schema, domain package | Safe to build on. Breaking changes require migration. |
| **Stable** | Scanner pipeline, parsers, storage | Core infrastructure. Changes are additive. |
| **Growing** | Query engine, MCP tools, compound queries | Actively adding capabilities. API may expand. |
| **Experimental** | Intent guidance (Phase 8) | Design validated, implementation pending. May evolve. |

## Non-Goals

Atlas will never become:

- **A CI/CD platform.** Atlas reads code, it doesn't build or deploy it.
- **A Kubernetes dashboard.** Atlas understands the code that manages clusters, not the clusters themselves.
- **A GitHub replacement.** Atlas consumes repositories, it doesn't host them.
- **An IDE.** Atlas provides intelligence to IDEs, it isn't one.
- **A documentation generator.** Atlas extracts architecture from code. It doesn't write docs.
- **An AI that invents architecture.** Atlas extracts what exists. If a parser can't determine a fact, Atlas doesn't guess. (See [ADR-0009](adr/0009-deterministic-over-intelligent.md).)

## Quick Start

```bash
# Scan
atlas scan -repo ~/hypershift -output atlas-graph.json -temporal

# Query
atlas query -graph atlas-graph.json controller HostedCluster

# Serve (for Claude Code / MCP consumers)
atlas serve --graph atlas-graph.json
```

## Where to Go Next

| If you want to... | Read |
|-------------------|------|
| Understand the architecture | [architecture.md](architecture.md) |
| Learn the graph schema | [data-model.md](data-model.md) |
| Build a new parser | [architecture.md](architecture.md) (Code Architecture section) |
| Understand project evolution | [roadmap.md](roadmap.md) |
| Know why decisions were made | [adr/](adr/) |
