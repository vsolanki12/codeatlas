# CodeAtlas

**A deterministic engineering knowledge layer for large Go repositories.** CodeAtlas parses source code, Kubernetes manifests, docs, and tests to build a structured graph of entities and evidenced relationships — then serves bounded graph context to CLI users, AI assistants, and review tooling through 13 [MCP](https://modelcontextprotocol.io/) tools.

Instead of reading thousands of source files, your AI assistant queries a pre-built graph. Repeated questions reuse the same deterministic facts, reducing context size and cost while keeping evidence tied to the source repository.

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
> atlas explain HostedClusterReconciler

controller:hostedcluster.HostedClusterReconciler | hostedcluster_controller.go:357
  HostedClusterReconciler reconciles a HostedCluster object
reconciles:
  crd:hypershift.openshift.io.hostedcluster | HostedCluster is the primary representation of a HyperShi...
calls:
  function:hostedcluster.HostedClusterReconciler.reconcileLegacy | reconcile_legacy.go:52
  calls:
    function:azureutil.IsAroHCPByHC | azureutil.go:279
    function:configrefs.ConfigMapRefs | refs.go:33
    function:controlplaneoperator.HostedControlPlane | manifests.go:102
    function:controlplaneoperator.PullSecret | manifests.go:111
    function:hostedcluster.HostedClusterReconciler.defaultClusterIDsIfNeeded | hostedcluster_controller.go:5061
    ...
  function:nodepool.SetStatusCondition | conditions.go:46
  function:nodepool.FindStatusCondition | conditions.go:91
17 nodes explored
```

**Without CodeAtlas** — "What breaks if I change `SetStatusCondition`?"

```
$ grep -rn "SetStatusCondition" --include="*.go"
# Matches in dozens of files. But what CALLS those callers? What controllers are upstream?
# What tests cover the chain? grep can't answer that.
```

**With CodeAtlas** — blast radius in one call:

```
> atlas impact SetStatusCondition

=== Call Chain (49 callers) ===
...

=== Controllers (6) ===
controller:aws.AWSEndpointServiceReconciler
controller:awsprivatelink.AWSEndpointServiceReconciler
controller:azureprivatelinkservice.AzurePrivateLinkServiceReconciler
controller:hostedcluster.HostedClusterReconciler
controller:hostedcontrolplane.HostedControlPlaneReconciler
controller:nodepool.CAPI

=== Resources (5) ===
crd:hypershift.openshift.io.awsendpointservice
crd:hypershift.openshift.io.hostedcluster
crd:hypershift.openshift.io.hostedcontrolplane
...

=== Files Affected (15) ===
control-plane-operator/controllers/awsprivatelink/awsprivatelink_controller.go
control-plane-operator/controllers/hostedcontrolplane/hostedcontrolplane_controller.go
hypershift-operator/controllers/hostedcluster/hostedcluster_controller.go
...
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
2. **The graph is the product.** Every consumer reads the same JSON, including its scan status, warnings, and freshness metadata.
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
atlas serve -graph atlas-graph.json

# CLI queries (same capabilities as MCP tools)
atlas stats -graph atlas-graph.json
atlas search -graph atlas-graph.json reconcileEtcd
atlas explain -graph atlas-graph.json HostedClusterReconciler
atlas impact -graph atlas-graph.json SetStatusCondition
atlas ask -graph atlas-graph.json NodePool -intent understand
atlas view -graph atlas-graph.json HostedClusterReconciler

# Verify that the graph still describes the checkout before implementation work
atlas freshness -graph atlas-graph.json -repo /path/to/your/project

# Enforce current schema, verifiable checkout state, and complete scanning
atlas verify -graph atlas-graph.json -repo /path/to/your/project

# Compact JSON for assistants and other token-sensitive consumers
atlas ask -graph atlas-graph.json HostedClusterReconciler -intent debug --json --compact
```

`atlas stats --json` exposes compact graph metadata (`commit`, `branch`,
`generatedAt`, `entityIdentity`, `scanComplete`, and scan warnings). Query JSON envelopes include
the same status so consumers can reject stale or incomplete evidence before
asking an LLM to reason over it. Use `--json` for machine-facing commands to
avoid repeating human-readable formatting in prompts.

### Review a PR

```bash
# Review using local git refs
atlas review --base upstream/main --head feature-branch --graph atlas-graph.json --repo /path/to/repo

# Review using a diff file (no git fetch needed; this is explicitly unverified)
gh api repos/openshift/hypershift/pulls/8968 -H 'Accept: application/vnd.github.diff' > pr.diff
atlas review --diff pr.diff --graph atlas-graph.json

# Pipe diff from stdin (also unverified unless --repo is supplied)
gh api repos/openshift/hypershift/pulls/8968 -H 'Accept: application/vnd.github.diff' | atlas review --diff - --graph atlas-graph.json

# Verified diff review against the graph's checkout
atlas review --diff pr.diff --graph atlas-graph.json --repo /path/to/repo --head HEAD

# Fetch PR metadata and diff directly from GitHub (requires gh auth)
atlas review --pr openshift/hypershift/8968 --graph atlas-graph.json
```

Output shows: the bounded changed diff, changed entities with callers/callees,
bounded blast radius (controllers and resources), directly evidenced tests,
explicitly labeled heuristic test links, evidence-backed repository-pattern
observations, conservative test analysis, unmapped files, graph freshness, and
evidence limitations. It does not claim branch-level or changed-behavior
coverage. A diff review without `--repo` is useful for mapping but remains
`unverified`; `--pr` reports a head-matched graph only when the graph commit
matches the GitHub PR head and still reports that no checkout was verified.
The PR description is user-provided context and is displayed as untrusted
input, not as a CodeAtlas fact.

### Connect to Claude Code

Add to `~/.mcp.json`:

```json
{
  "mcpServers": {
    "codeatlas": {
      "command": "/path/to/atlas",
      "args": ["serve", "-graph", "/path/to/atlas-graph.json"]
    }
  }
}
```

Restart Claude Code. All 13 tools are now available. Works with any MCP-compatible client (VS Code, Cursor, Continue.dev, and Codex).

Compound tools return bounded human-readable text by default so MCP clients do
not pay for repeated full entity payloads. Use `detail=true` on
`atlas_investigate`, `atlas_explain`, or `atlas_impact` when verbose text is
needed. Structured responses remain bounded and retain relationship evidence.

### Connect to Codex

Codex uses `~/.codex/config.toml` for local MCP servers. Add CodeAtlas with the
Codex CLI, using absolute paths so the server does not depend on the current
working directory:

```bash
codex mcp add codeatlas -- \
  /absolute/path/to/atlas serve \
  --graph /absolute/path/to/atlas-graph.json
codex mcp list
```

Restart the local Codex app or CLI, then use `/mcp` in the Codex TUI to confirm
the server. The server loads one graph at startup; configure one server per
repository graph when working across repositories. Prefer `atlas_ask`,
`atlas_freshness`, and compact responses before requesting verbose details.

The graph-query tools are read-only. `atlas_review` is deterministic, but `pr`
mode may call the authenticated `gh` CLI to retrieve GitHub metadata and a diff.
Scanning remains a CLI operation so an MCP client cannot silently replace the
graph it is using.

MCP review examples:

```json
{"pr":"openshift/hypershift/8968"}
{"diff":"diff --git ...", "base":"upstream/main", "head":"HEAD", "repo":"/path/to/repo"}
{"base":"upstream/main", "head":"HEAD", "repo":"/path/to/repo", "omit_diff":true}
```

The first form uses `gh` and reports whether the graph commit matches the PR
head. The second form is verified only when `repo` is supplied and the graph
matches that checkout. The third form is useful for a low-token structural
review; it intentionally omits changed source text.

---

## What You Can Ask

| Question | Tool | What it returns |
|----------|------|-----------------|
| "How does X work?" | `atlas_explain` | Reconciliation chain: what it reconciles, creates, calls, and what tests are linked by graph evidence |
| "What breaks if I change X?" | `atlas_impact` | Bounded upstream callers, tests, resources, files, owners, and supporting relationship evidence |
| "Tell me everything about X" | `atlas_investigate` | Bounded entity details, relationships, callers, tests, siblings — one call; request `detail=true` for verbose text |
| "Where is X defined?" | `atlas_search` | Relevance-ranked matches across names, packages, imports, literals |
| "What changed the most?" | `atlas_temporal` | Most-changed, stalest, or recently-modified entities by git history |
| "Quick summary of X" | `atlas_view` | Pre-computed engineering view: manages, managed by, tests, files, owners |
| "How does X work?" (one call) | `atlas_ask` | Bounded view + explain/impact/investigate context in one JSON-capable call |
| "Is this graph current?" | `atlas_freshness` | Read-only checkout/provenance verification with bounded file differences |
| "What does this PR change?" | `atlas_review` | Deterministic diff-to-graph review with evidence, blast radius, tests, patterns, and limitations |

---

## All MCP Tools

13 tools served via `atlas serve`:

| Tool | Purpose |
|------|---------|
| `atlas_ask` | One-call query planner: entity + intent → view + deep analysis. Use this first |
| `atlas_view` | Pre-computed engineering view for a controller or CRD. Zero graph traversal |
| `atlas_investigate` | Bounded entity context: relationships, callers, tests, siblings; `detail=true` enables verbose text |
| `atlas_explain` | Bounded architectural narrative: reconciles → creates → calls → tested_by; `detail=true` enables verbose text |
| `atlas_impact` | Bounded blast radius: callers, controllers, tests, resources, owners, and evidence; `detail=true` enables verbose text |
| `atlas_search` | Find entities by text or kind. Relevance-ranked across all fields; returns total/nextOffset for bounded pagination |
| `atlas_entity` | Full entity detail by ID, or batch fetch multiple IDs; relationship evidence is bounded and resumable with `relationship_offset` |
| `atlas_where` | Find entities by file path with bounded, resumable pagination |
| `atlas_context` | BFS subgraph around an entity |
| `atlas_temporal` | Git history: most-changed, stalest, or recently-modified entities with bounded, resumable pagination |
| `atlas_stats` | Graph statistics |
| `atlas_freshness` | Verify graph provenance and stored file state against a checkout |
| `atlas_review` | Deterministic PR/diff review; supports `pr`, raw `diff`, and verified local `base`/`head` modes |

---

## CLI Commands

The CLI mirrors MCP tools — same graph queries and evidence, with client-specific
text/JSON formatting and no server needed.

| Command | What it does |
|---------|-------------|
| `atlas scan` | Parse a repository and generate the graph |
| `atlas search <query>` | Text search across all entities (with optional `--kind` filter) |
| `atlas explain <entity>` | Reconciliation chain: reconciles, creates, calls, tested_by |
| `atlas impact <entity>` | Blast radius: upstream callers, controllers, tests, files, owners, and supporting relationship evidence |
| `atlas investigate <entity>` | Full entity details, relationships, callers, tests, siblings |
| `atlas ask <entity>` | View + deep analysis (with optional `--intent understand\|impact\|debug`) |
| `atlas view <entity>` | Pre-computed engineering view for a controller or CRD |
| `atlas context <entity-id>` | BFS subgraph around an entity |
| `atlas where <path>` | Find entities by file path |
| `atlas stats` | Graph statistics |
| `atlas freshness` | Compare graph commit and stored file state with a checkout |
| `atlas verify` | Fail when graph schema, identity, freshness, file state, or scan completeness cannot be verified |
| `atlas review --base <ref>` | PR review: map diff hunks to graph entities, show blast radius, pattern observations, and structural test links |
| `atlas review --pr <owner/repo/number>` | Fetch GitHub PR metadata and diff, then run the same deterministic graph review |
| `atlas serve` | Start the MCP server |
| `atlas query <kind> [name]` | Legacy: lookup entities by kind (controller, function, crd, etc.); use relationship offset/limit for resumable evidence |

All commands accept `--graph path` (defaults to `atlas.json`). JSON-capable
queries also accept `--compact`, which keeps graph metadata, entity identity,
source locations, bounded fields, and relationship evidence while removing
repeated full entities. Use the default JSON form or `--detail` when inspecting
the complete entity payload. Flags may appear before or after positional
arguments.

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

**Schema:** 1.5.0 · **MCP Tools:** 13 · **CLI Commands:** 15 · **Parsers:** Go AST, template-aware YAML, Markdown, Test · **Current:** deterministic graph, explicit scan coverage, resumable bounded retrieval, type-aware call evidence, freshness verification, and evidence-based PR review; LLM reasoning remains downstream in `codeatlas-assistant`

See [roadmap.md](docs/roadmap.md) for full history and future plans.
