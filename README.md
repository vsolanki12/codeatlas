# CodeAtlas

**Turn any Go codebase into a queryable architecture graph.** CodeAtlas parses source code, Kubernetes manifests, docs, and tests to build a structured graph of every entity and relationship — then serves it to AI assistants through 15 [MCP](https://modelcontextprotocol.io/) tools.

Instead of reading thousands of source files, your AI assistant queries a pre-built graph. Same answers, fraction of the cost, backed by evidence from the code itself.

---

## Why CodeAtlas

Large codebases are hostile territory. A project with 11,000+ entities across hundreds of packages has architecture that lives in the heads of people who've been there for years. When they leave, the knowledge leaves with them.

AI assistants try to help, but without architectural context they read source files one by one — slow, expensive, and incomplete. They miss the connections between controllers, CRDs, functions, and tests that make the architecture make sense.

CodeAtlas extracts those connections once and serves them to any AI assistant that speaks MCP. Engineers get a queryable architecture map on day one. AI assistants get structured facts instead of raw code.

---

## Before and After

**Without CodeAtlas** — "What handles HostedCluster reconciliation?"

```
$ grep -rn "HostedCluster" --include="*.go" | wc -l
847

$ grep -rn "Reconcile.*HostedCluster" --include="*.go"
# 23 matches across 14 files. Which one is the entry point?
# What does it create? What tests cover it? You're reading files for the next hour.
```

**With CodeAtlas** — same question, one tool call:

```
> atlas_explain HostedClusterReconciler

controller:hostedclusters.HostedClusterReconciler
├── reconciles: HostedCluster (CRD)
├── creates: ControlPlaneOperator, KubeAPIServer, Etcd, Ignition, ...
│   ├── calls: reconcileEtcd(), reconcileKAS(), reconcileKonnectivity(), ...
│   └── tested_by: TestReconcileHostedCluster, TestHostedClusterCreation
└── 847 entities explored, 12 controllers found in chain
```

**Without CodeAtlas** — "What breaks if I change `reconcileEtcd`?"

```
$ grep -rn "reconcileEtcd" --include="*.go"
# 6 matches. But what CALLS those callers? What controllers are upstream?
# What tests cover the chain? grep can't answer that.
```

**With CodeAtlas** — blast radius in one call:

```
> atlas_impact reconcileEtcd

Affected controllers: HostedClusterReconciler, NodePoolReconciler
Affected tests:       TestReconcileEtcd, TestHostedClusterCreation (+ 3 more)
Affected files:       14 files across 6 packages
Owners:               @openshift/hypershift-etcd (last 30 days: 2 authors, 47 commits)
```

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

# Scan any Go repository
atlas scan -repo /path/to/your/project -output atlas-graph.json

# Include git history (enables hotspot and ownership queries)
atlas scan -repo /path/to/your/project -output atlas-graph.json -temporal

# Start the MCP server
atlas serve --graph atlas-graph.json
```

### Connect to Claude Code

Add to `~/.mcp.json`:

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

Restart Claude Code. All 15 tools are now available. Works with any MCP-compatible client (VS Code, Cursor, Continue.dev).

---

## What You Can Ask

| Question | Tool | What it returns |
|----------|------|-----------------|
| "How does X work?" | `atlas_explain` | Reconciliation chain: what it reconciles, creates, calls, and what tests cover it |
| "What breaks if I change X?" | `atlas_impact` | Every upstream controller, test, resource, file, and owner affected |
| "Tell me everything about X" | `atlas_investigate` | Full entity details, all relationships, callers, tests, siblings — one call |
| "Where is X defined?" | `atlas_search` | Relevance-ranked matches across names, packages, imports, literals |
| "What changed the most?" | `atlas_hotspots` | Most-changed or stalest entities by git history |
| "Who changed X recently?" | `atlas_commits` | Authors, dates, and change counts from git history |

---

## All MCP Tools

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

## Why Not Just Use grep?

| | grep | CodeAtlas |
|---|---|---|
| "What calls this function?" | Text matches, no call chain | Full upstream/downstream call graph |
| "What tests cover this?" | Filename guessing | `tested_by` edges with evidence |
| "What does this controller manage?" | Read every file it touches | `reconciles`, `creates`, `calls` in one query |
| "What breaks if I change this?" | No answer | Blast radius: controllers, tests, files, owners |
| "Who owns this code?" | `git blame` one file at a time | Aggregated ownership across the call chain |
| Token cost for AI | Reads 100s of source files | One graph query |

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

## Status

**Schema:** 1.2.0 · **MCP Tools:** 15 · **Parsers:** Go AST, YAML, Markdown, Test · **In Progress:** Phase 8 (Intent-Based Tool Guidance)

See [roadmap.md](docs/roadmap.md) for full history and future plans.
