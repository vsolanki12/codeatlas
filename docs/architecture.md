# Architecture

CodeAtlas has three architectures. Most projects describe one. CodeAtlas needs three because the product, the code, and the execution pipeline solve different problems and evolve at different rates.

Detailed rationale for each decision lives in the [ADR directory](adr/).

---

## Architecture 1: Product Architecture

What CodeAtlas is. How it fits together as a system.

```
Source Repository
        │
        ▼
   Atlas Scanner          Go binary (cmd/atlas)
        │
        ▼
   Atlas Graph            JSON file (atlas-graph.json)
        │
   ┌────┼────┬────────┐
   ▼    ▼    ▼        ▼
  CLI    MCP Server    API           All consumers read the same JSON
              │        (Future)
      ┌───────┼──────┐
      ▼       ▼      ▼
   Claude   VS Code  Any MCP        Layer 4: Experience
   Code     Cursor   Client  
```

Four-layer model:

| Layer | What | Purpose |
|-------|------|---------|
| **Knowledge** | Scanner → Graph | Extract architecture from code |
| **Retrieval** | 11 MCP tools (primitives + compounds + views) | Answer questions about the graph |
| **Guidance** | Tool descriptions | Teach consumers which engineering intent a tool serves |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Where engineers interact with CodeAtlas |

CodeAtlas itself doesn't decide anything. The consumer does. Adding a new consumer never changes the scanner or the graph format.

**Status:** Implemented. Scanner, Graph, CLI, and MCP Server all operational.

---

## Architecture 2: Code Architecture

How the Go code is organized. Which package owns which responsibility.

```
internal/domain               The vocabulary of CodeAtlas
    │                         Entity, Relationship, Evidence, Graph, Source
    │
    ▲ (every package imports domain)
    │
cmd/atlas                     CLI entry point (scan, search, explain, impact, investigate,
                                ask, view, context, where, stats, serve, query)
    │
    ├──► internal/scanner      Orchestrator — coordinates the full scan pipeline
    │       │
    │       ├──► internal/discovery    Walks the repository, returns []domain.File
    │       │
    │       ├──► internal/parser       Parses files into []domain.Entity
    │       │       ├── goparser.go    Go AST (controllers, functions, packages, imports, literals, embeds)
    │       │       ├── yaml.go        YAML parser (CRDs, Deployments, Services, property flattening)
    │       │       ├── markdown.go    Markdown parser (docs, design proposals)
    │       │       └── test.go        Test parser (test functions, coverage)
    │       │
    │       ├──► internal/graph        Builds []domain.Relationship between entities
    │       │
    │       ├──► internal/origin       Import path classifier (stdlib vs known repos)
    │       │
    │       ├──► internal/temporal     Git history enrichment (LastAuthor, LastModified, ChangeCount)
    │       │
    │       ├──► internal/views         Compiles pre-computed knowledge views from entities+rels
    │       │
    │       └──► internal/storage      Writes and reads domain.Graph as JSON
    │
    ├──► internal/query        Query engine — Index, search, traversal, compound queries
    │
    └──► internal/mcpserver    MCP server — 11 tools served via stdio transport
```

| Package | Responsibility | Depends On |
|---|---|---|
| `internal/domain` | Defines the vocabulary: Entity, Relationship, Evidence, Graph, Source | Nothing |
| `cmd/atlas` | CLI: scan, search, explain, impact, investigate, ask, view, context, where, stats, serve, query | `domain`, `scanner`, `query`, `mcpserver` |
| `internal/scanner` | Orchestrates the full scan pipeline with merge-aware dedup | `domain`, `discovery`, `parser`, `graph`, `storage`, `origin`, `temporal` |
| `internal/discovery` | Walks the repository, returns files with metadata | `domain` |
| `internal/parser` | Parses individual files into entities; extracts imports, literals, embeds, properties | `domain` |
| `internal/graph` | Connects entities with typed, evidenced relationships | `domain` |
| `internal/storage` | Serializes/deserializes the Atlas Graph JSON | `domain` |
| `internal/origin` | Classifies import paths (stdlib, known repos, external) | Nothing |
| `internal/temporal` | Enriches entities with git history (LastAuthor, LastModified, ChangeCount) | `domain` |
| `internal/views` | Compiles pre-computed knowledge views and question index from entities + relationships | `domain` |
| `internal/query` | Query engine: Index, Search (relevance-scored), Lookup, Where, Neighbors, Temporal, Callers, Investigate, Explain, Impact | `domain`, `storage` |
| `internal/mcpserver` | MCP server: 11 tools via go-sdk stdio transport | `query` |

Key constraints:
- **Every package imports `domain`.** It is the shared vocabulary. `docs/data-model.md` is this vocabulary in English. `internal/domain` is the same vocabulary in Go. They match exactly.
- **Leaf packages don't depend on each other.** `discovery`, `parser`, `graph`, `storage`, `origin`, and `temporal` are independent. Only `scanner` composes them.
- **`domain` depends on nothing.** Zero imports from other CodeAtlas packages. If `domain` ever imports another CodeAtlas package, the architecture is broken.
- **`query` depends only on `domain` and `storage`.** It loads the graph and builds an in-memory index. No dependency on scanner or parsers.
- **`mcpserver` depends only on `query`.** It's a thin MCP wrapper over the query engine.

**Status:** Implemented. Run `atlas_stats` for current counts.

### Architectural Risks

**Risk 1: `internal/parser` — largest single package.**

`goparser.go` handles controllers, functions, packages, imports, literals, embeds, and selector constants. Still one file but growing. The Go parser is approaching the complexity threshold.

**Trigger to split:** When `goparser.go` exceeds ~500 lines or needs a third extraction pass.

**Risk 2: `internal/query` — absorbing compound logic.**

Started as simple index lookups. Now contains Investigate, Explain, Impact — each with graph traversal logic. Still manageable at ~500 lines but watch for it.

**Trigger to split:** When adding a new compound query requires understanding all existing ones.

---

## Architecture 3: Execution Pipeline

What happens when someone runs `atlas scan`. The sequence of operations, in order.

```
atlas scan -repo /path/to/hypershift -output atlas-graph.json -temporal
        │
        ▼
┌─ 1. Configuration ───────────────────────────┐
│  Parse CLI flags (-repo, -output, -temporal)  │
│  Resolve repository path                      │
│  Load scan options                            │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 2. Repository Validation ────────────────────┐
│  Verify path exists                           │
│  Verify it's a Go project (go.mod)            │
│  Read git commit hash and branch              │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 3. File Discovery ──────────────────────────┐
│  Walk the directory tree                      │
│  Return every file with metadata              │
│  Skip: vendor/, .git/, node_modules/,         │
│         testdata/                             │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 4. File Classification ─────────────────────┐
│  Route discovered files to correct parser:    │
│    .go (non-test)  → Go parser                │
│    _test.go        → Test parser              │
│    .yaml/.yml      → YAML parser              │
│    .md             → Markdown parser           │
│  Unrecognized extensions are skipped          │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 5. Parser Execution ────────────────────────┐
│  Go parser      → controllers, functions,     │
│                    packages, imports, literals,│
│                    embeds, selector constants  │
│  YAML parser    → CRDs, Deployments,          │
│                    Services, property flatten  │
│  Markdown parser → documents, headings        │
│  Test parser    → test functions, test types   │
│                                               │
│  Output: flat list of entities                │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 6. Dedup (Merge-Aware) ────────────────────┐
│  Duplicate package entities merge Files and   │
│  Imports arrays (was first-seen-wins)         │
│  Merges: Literals, Properties, Embeds fields  │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 7. Temporal Enrichment (opt-in) ────────────┐
│  If -temporal flag set:                       │
│    git log per unique file → distribute to    │
│    entities: LastAuthor, LastModified,         │
│    ChangeCount. File-level cache avoids       │
│    redundant git calls.                       │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 8. Relationship Building ───────────────────┐
│  Connect entities with typed edges:           │
│                                               │
│  reconciles, creates, calls, tested_by,       │
│  documented_in, imports, owns, watches,       │
│  embeds                                       │
│                                               │
│  Every edge carries evidence (file, line,     │
│  snippet, reason) and confidence              │
│  (proven or inferred)                         │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 9. Graph Validation ────────────────────────┐
│  Check: no entity without a source            │
│  Check: no relationship without evidence      │
│  Check: all entity IDs are unique             │
│  Check: all relationship targets exist        │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 9b. Knowledge View Compilation ─────────────┐
│  For each controller and CRD:                 │
│    Compile ownership, resources, tests,       │
│    files, temporal data into a View.          │
│  Generate deterministic Q&A pairs from views. │
│  Stored in graph as views + questions fields. │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 10. Graph Writing ─────────────────────────┐
│  Serialize to atlas-graph.json                │
│  Include: schema version (1.4.0), commit,     │
│           branch, scan duration, stats        │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 11. Summary ────────────────────────────────┐
│  Atlas scan complete.                         │
│  (entity and relationship counts vary by scan) │
│  Schema 1.4.0                                 │
└───────────────────────────────────────────────┘
```

Each step maps to code:

| Pipeline Step | Package | Entry Function |
|---|---|---|
| 1. Configuration | `cmd/atlas` | `runScan()` |
| 2. Repository Validation | `internal/scanner` | `ValidateRepository()` |
| 3. File Discovery | `internal/discovery` | `Scan()` |
| 4. File Classification | `internal/scanner` | Route files by extension to parsers |
| 5. Parser Execution | `internal/parser` | `Parse()` per parser |
| 6. Dedup | `internal/scanner` | Merge-aware entity deduplication |
| 7. Temporal Enrichment | `internal/temporal` | `Enrich()` |
| 8. Relationship Building | `internal/graph` | `BuildRelationships()` |
| 9. Graph Validation | `internal/graph` | `Validate()` |
| 9b. View Compilation | `internal/views` | `Compile()`, `CompileQuestions()` |
| 10. Graph Writing | `internal/storage` | `Write()` |
| 11. Summary | `internal/scanner` | `PrintSummary()` |

**Status:** Implemented. Run `atlas_stats` for current scan numbers.

---

## Phases

CodeAtlas develops in **phases** — each builds on the previous and unlocks the next.

| Phase | Capability | Status |
|-------|-----------|--------|
| 1–2 | Core scanner + CLI | Implemented |
| 3a | Deep call graph (7,837 call edges), implements, env vars | Implemented |
| 4 | Cross-repo intelligence (import classification, merge-aware dedup) | Implemented |
| 5 | Temporal layer (git history: LastAuthor, LastModified, ChangeCount) | Implemented |
| 6 | Content indexing (literals, YAML properties, go:embed) | Implemented |
| 6b | Token optimization (AND search, brief mode, batch fetch, detail mode) | Implemented |
| 6c | Search quality + call reduction (relevance scoring, callers, commits) | Implemented |
| 7 | Compound queries (atlas_investigate, atlas_explain) | Implemented |
| 7b | Blast radius analysis (atlas_impact) | Implemented |
| 8 | Intent-based tool guidance (enriched MCP descriptions) | Attempted, reverted |
| 9 | Incremental scanning (skip unchanged files) | Implemented |
| 10 | Tool consolidation (14 → 9 tools) | Implemented |
| 11 | Knowledge Views (pre-computed engineering summaries) | Implemented |
| 12 | Query Planner (atlas_ask — one-call orchestration) | Implemented |
| 13 | Question Index (deterministic Q&A pairs) | Implemented |

Current state: 11 MCP tools, schema 1.4.0. Run `atlas_stats` for entity/relationship counts and `go test ./...` for test count.

---

## Architecture Decision Records

| ADR | Decision |
|---|---|
| [0001](adr/0001-scanner-and-viewer-separation.md) | Scanner and viewers are separate |
| [0002](adr/0002-json-before-neo4j.md) | JSON before Neo4j |
| [0003](adr/0003-no-ai-until-graph-is-proven.md) | No AI until the graph is proven |
| [0004](adr/0004-evidence-on-every-relationship.md) | Evidence on every relationship |
| [0005](adr/0005-typed-relationships.md) | Typed relationships over generic edges |
| [0006](adr/0006-unified-entity-model.md) | Unified Entity model |
| [0007](adr/0007-store-forward-compute-inverse.md) | Store forward edges, compute inverses |
| [0008](adr/0008-graph-is-the-product.md) | The Atlas Graph is the product |
| [0009](adr/0009-deterministic-over-intelligent.md) | Deterministic over intelligent |
| [0010](adr/0010-mcp-as-primary-interface.md) | MCP as primary consumer interface |
| [0011](adr/0011-compound-queries-over-primitives.md) | Compound queries over primitive sequences |
| [0012](adr/0012-temporal-as-opt-in.md) | Temporal enrichment as opt-in |
| [0013](adr/0013-intent-guidance-over-workflow-code.md) | Intent guidance over workflow code |
| [0014](adr/0014-four-layer-architecture.md) | Four-layer architecture model |
| [0015](adr/0015-atlas-is-ai-infrastructure.md) | CodeAtlas is an AI infrastructure platform |
