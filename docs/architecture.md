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
   ┌────┼──────────┬──────────┐
   ▼    ▼          ▼          ▼
  CLI  MCP      Assistant   PR review     All consumers read the same graph
       Server   (bounded     (diff +
                  graph      graph)
                 context)
        │          │          │
        ▼          ▼          ▼
   MCP clients  Local LLM   Optional LLM  Human reviewer
                reasoning   reasoning
```

Four-layer model:

| Layer | What | Purpose |
|-------|------|---------|
| **Knowledge** | Scanner → Graph | Extract architecture from code |
| **Retrieval** | 11 MCP tools (primitives + compounds + views) | Answer questions about the graph |
| **Guidance** | Tool descriptions | Teach consumers which engineering intent a tool serves |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Where engineers interact with CodeAtlas |

CodeAtlas itself doesn't decide anything. The consumer does. Adding a new consumer never changes the scanner or the graph format.

**Status:** Implemented. Scanner, Graph, CLI, MCP Server, Assistant integration,
and deterministic PR-review preparation are operational. LLM reasoning remains
optional and downstream of graph-derived evidence.

### Operational contract

The scanner is the only component that parses repository files. It records
content fingerprints for supported source, configuration, and document files
and marks a graph `scanComplete: false` when a file cannot be parsed.
Incremental reuse is allowed only for a previously complete graph whose file
state still matches the repository; incomplete graphs trigger a full scan.
Consumers must carry graph metadata and warnings into their own output.

Relationship targets are emitted only when the parser provides a supported
signal and the builder can resolve the target without an ambiguous match.
Name-based or convention-based resolution is retained only when it can be
explained and is marked `confidence: inferred`. Unsupported relationships are
omitted. Query and MCP JSON responses are bounded and expose truncation and
graph-status metadata so an LLM cannot mistake a partial response for a full
repository inventory.

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
                                ask, view, context, where, stats, freshness, serve, query, review)
    │
    ├──► internal/scanner      Orchestrator — coordinates the full scan pipeline
    │       │
    │       ├──► internal/discovery    Walks the repository, returns []domain.File
    │       │
    │       ├──► internal/parser       Parses files into []domain.Entity
    │       │       ├── goparser.go    Go AST (controllers, functions, packages, imports, literals, embeds)
    │       │       ├── yamlparser.go  YAML parser (CRDs, resources, property flattening)
    │       │       ├── mdparser.go    Markdown parser (documents and bounded excerpts)
    │       │       └── testparser.go  Test parser (test functions and calls)
    │       │
    │       ├──► internal/graph        Builds []domain.Relationship between entities
    │       │
    │       ├──► internal/temporal     Git history enrichment (LastAuthor, LastModified, ChangeCount)
    │       │
    │       ├──► internal/views         Compiles pre-computed knowledge views from entities+rels
    │       │
    │       └──► internal/storage      Writes and reads domain.Graph as JSON
    │
    ├──► internal/query        Query engine — Index, search, traversal, compound queries
    │
    ├──► internal/review       PR review — diff parsing, entity mapping, enrichment, formatting
    │
    └──► internal/mcpserver    MCP server — 11 tools served via stdio transport
```

| Package | Responsibility | Depends On |
|---|---|---|
| `internal/domain` | Defines the vocabulary: Entity, Relationship, Evidence, Graph, Source | Nothing |
| `cmd/atlas` | CLI: scan, search, explain, impact, investigate, ask, view, context, where, stats, freshness, serve, query, review | `domain`, `scanner`, `query`, `mcpserver`, `review`, `freshness` |
| `internal/scanner` | Orchestrates the full scan pipeline with merge-aware dedup | `domain`, `discovery`, `parser`, `graph`, `storage`, `temporal`, `views` |
| `internal/discovery` | Walks the repository, returns files with metadata | `domain` |
| `internal/parser` | Parses individual files into entities; extracts imports (including alias normalization), literals, embeds, properties | `domain` |
| `internal/graph` | Connects entities with typed, evidenced relationships | `domain` |
| `internal/storage` | Serializes/deserializes the Atlas Graph JSON | `domain` |
| `internal/temporal` | Enriches entities with git history (LastAuthor, LastModified, ChangeCount) | `domain` |
| `internal/views` | Compiles pre-computed knowledge views and question index from entities + relationships | `domain` |
| `internal/query` | Query engine: Index, Search (relevance-scored), Lookup, Where, Neighbors, Temporal, Callers, Investigate, Explain, Impact | `domain`, `storage` |
| `internal/review` | PR review: bounded diff evidence, diff parsing, entity-to-hunk mapping, graph enrichment, human-readable formatting | `domain`, `query` |
| `internal/mcpserver` | MCP server: 11 tools via go-sdk stdio transport | `query` |

Key constraints:
- **Graph-producing packages use `domain` as the shared vocabulary.** `docs/data-model.md` describes the same contract. Thin consumers such as `mcpserver` depend on `query` and do not need to import `domain` directly.
- **Leaf packages don't depend on each other.** `discovery`, `parser`, `graph`, `storage`, and `temporal` are independent. Only `scanner` composes them.
- **`domain` depends on nothing.** Zero imports from other CodeAtlas packages. If `domain` ever imports another CodeAtlas package, the architecture is broken.
- **`query` depends only on `domain` and `storage`.** It loads the graph and builds an in-memory index. No dependency on scanner or parsers.
- **`mcpserver` depends only on `query`.** It's a thin MCP wrapper over the query engine.

**Status:** Implemented. Run `atlas stats` for current counts.

### Architectural Risks

**Risk 1: `internal/parser` — largest single package.**

`goparser.go` handles controllers, functions, packages, imports, literals, embeds, and selector constants. Still one file but growing. The Go parser is approaching the complexity threshold.

**Trigger to split:** When `goparser.go` exceeds ~500 lines or needs a third extraction pass.

**Risk 2: `internal/query` — absorbing compound logic.**

Started as simple index lookups. Now contains Investigate, Explain, Impact — each with graph traversal logic. Still manageable at ~500 lines but watch for it.

**Trigger to split:** When adding a new compound query requires understanding all existing ones.

---

## Architecture 3: Execution Pipeline

What happens when someone runs `atlas scan`. The scanner is deterministic for a
given repository snapshot; wall-clock duration is reported by the CLI and is
not persisted into graph content.

```
atlas scan -repo /path/to/repository -output atlas.json -temporal
        │
        ▼
1. Resolve the repository and output paths; discovery validates that the
   repository root is an existing directory.
        │
        ▼
2. Walk the repository deterministically, skipping .git, .worktrees, vendor,
   node_modules, and testdata. Record timestamps and content/metadata
   fingerprints. The graph output file is excluded when it is inside the repo.
        │
        ▼
3. If a previous graph is complete, current, repository-matching, and uses the
   current entity identity, reuse unchanged files. A changed go.mod or package
   file that invalidates package aggregation forces the affected/full rescan.
        │
        ▼
4. Classify .go, _test.go, YAML/YML, and Markdown files and run the matching
   deterministic parser. Parse failures become scan warnings and make the
   graph incomplete; unsupported file extensions are not parsed.
        │
        ▼
5. Merge duplicate package/controller observations, deduplicate facts and
   sites, sort entities and fact arrays, then optionally add git history.
        │
        ▼
6. Build only supported relationships whose endpoints resolve. Each emitted
   edge has deterministic ID, type, confidence, and source evidence.
        │
        ▼
7. Assemble metadata, status, file state, relationships, deterministic views,
   and deterministic question answers. Validate the final graph before JSON
   persistence.
        │
        ▼
8. Write the graph atomically at the requested path. Query, MCP, Assistant,
   and review consumers load this graph; they do not invoke repository parsers.
```

| Pipeline Step | Package | Entry Function |
|---|---|---|
| Configuration | `cmd/atlas` | `runScan()` |
| Discovery and repository validation | `internal/discovery` | `New()`, `(*Discovery).Scan()` |
| Incremental eligibility and merge | `internal/scanner` | `Scan()` |
| File parsing | `internal/parser` | `Parser.Parse()` |
| Temporal enrichment | `internal/temporal` | `Enrich()` |
| Relationship building | `internal/graph` | `(*RelationshipBuilder).Build()` |
| Structural validation | `internal/domain` | `Graph.Validate()` |
| View compilation | `internal/views` | `Compile()`, `CompileQuestions()` |
| Graph persistence | `internal/storage` | `WriteGraph()` |

**Status:** Implemented. `atlas freshness` compares graph provenance and stored
file state with a checkout. A current, complete graph is required for
implementation guidance and verified PR review.

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
| 14 | PR Review (deterministic diff-to-graph review) | Implemented |

Current state: 11 MCP tools, 14 CLI commands, schema 1.4.0. Run `atlas stats` for entity/relationship counts and `go test ./...` for test count.

---

## Architecture Decision Records

| ADR | Decision |
|---|---|
| [0001](adr/0001-scanner-and-viewer-separation.md) | Scanner and viewers are separate |
| [0002](adr/0002-json-before-neo4j.md) | JSON before Neo4j |
| [0003](adr/0003-no-ai-until-graph-is-proven.md) | No AI reasoning in the Atlas core |
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
