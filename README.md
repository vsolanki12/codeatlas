# CodeAtlas

**A deterministic reasoning engine for software architecture.** CodeAtlas parses source code, Kubernetes manifests, docs, and tests to build a structured graph of every entity and relationship — then serves it to AI assistants through 11 [MCP](https://modelcontextprotocol.io/) tools.

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

Restart Claude Code. All 11 tools are now available. Works with any MCP-compatible client (VS Code, Cursor, Continue.dev).

---

## What You Can Ask

| Question | Tool | What it returns |
|----------|------|-----------------|
| "How does X work?" | `atlas_explain` | Reconciliation chain: what it reconciles, creates, calls, and what tests cover it |
| "What breaks if I change X?" | `atlas_impact` | Every upstream controller, test, resource, file, and owner affected |
| "Tell me everything about X" | `atlas_investigate` | Full entity details, all relationships, callers, tests, siblings — one call |
| "Where is X defined?" | `atlas_search` | Relevance-ranked matches across names, packages, imports, literals |
| "What changed the most?" | `atlas_temporal` | Most-changed, stalest, or recently-modified entities by git history |
| "Quick summary of X" | `atlas_view` | Pre-computed engineering view: manages, managed by, tests, files, owners |
| "How does X work?" (one call) | `atlas_ask` | View + explain/impact/investigate — one call, complete answer |

---

## All MCP Tools

11 tools served via `atlas serve`:

| Tool | Purpose |
|------|---------|
| `atlas_ask` | One-call query planner: entity + intent → view + deep analysis. Use this first |
| `atlas_view` | Pre-computed engineering view for a controller or CRD. Zero graph traversal |
| `atlas_investigate` | Everything about one entity in 1 call: details, relationships, callers, tests, siblings |
| `atlas_explain` | Architectural narrative: reconciles → creates → calls → tested_by tree |
| `atlas_impact` | Blast radius: upstream callers, controllers, tests, resources, owners |
| `atlas_search` | Find entities by text or kind. Relevance-ranked across all fields |
| `atlas_entity` | Full entity detail by ID, or batch fetch multiple IDs |
| `atlas_where` | Find entities by file path |
| `atlas_context` | BFS subgraph around an entity |
| `atlas_temporal` | Git history: most-changed, stalest, or recently-modified entities |
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

**Schema:** 1.4.0 · **MCP Tools:** 11 · **Parsers:** Go AST, YAML, Markdown, Test · **Latest:** Phase 13 (Question Index)

See [roadmap.md](docs/roadmap.md) for full history and future plans.
