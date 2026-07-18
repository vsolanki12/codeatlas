# Architecture

Atlas has three architectures. Most projects describe one. Atlas needs three because the product, the code, and the execution pipeline solve different problems and evolve at different rates.

Detailed rationale for each decision lives in the [ADR directory](adr/).

---

## Architecture 1: Product Architecture

What Atlas is. How it fits together as a system.

```
HyperShift Repository
        │
        ▼
   Atlas Scanner          Go binary (cmd/atlas)
        │
        ▼
   Atlas Graph            JSON file (atlas-graph.json)
        │
   ┌────┼────┬────┬────┐
   ▼    ▼    ▼    ▼    ▼
Viewer  Web  CLI  API  AI    All consumers read the same JSON
(V1)   (V2)           (Future)
```

This architecture is stable. It answers: "What are the pieces and how do they relate?" The scanner produces the graph. Everything else consumes it. Adding a new consumer never changes the scanner or the graph format.

**Status:** Complete.

---

## Architecture 2: Code Architecture

How the Go code is organized. Which package owns which responsibility.

```
internal/domain               The vocabulary of Atlas
    │                         Repository, File, Entity, Relationship, Evidence, Graph
    │
    ▲ (every package imports domain)
    │
cmd/atlas                     CLI entry point
    │
    ▼
internal/scanner              Orchestrator — coordinates the full pipeline
    │
    ├──► internal/discovery    Walks the repository, returns []domain.File
    │         │
    │         ▼ classify by extension, route to correct parser
    │
    ├──► internal/parser       Parses files into []domain.Entity
    │       ├── go.go          Go AST parser (controllers, functions, packages)
    │       ├── yaml.go        YAML parser (CRDs, Deployments, Services)
    │       ├── markdown.go    Markdown parser (docs, design proposals)
    │       └── test.go        Test parser (test functions, coverage)
    │
    ├──► internal/graph        Builds []domain.Relationship between entities
    │
    └──► internal/storage      Writes and reads domain.Graph as JSON
```

| Package | Responsibility | Depends On |
|---|---|---|
| `internal/domain` | Defines the vocabulary: Repository, File, Entity, Relationship, Evidence, Graph | Nothing |
| `cmd/atlas` | CLI parsing, flags, subcommands | `internal/scanner` |
| `internal/scanner` | Orchestrates the full scan pipeline | `domain`, `discovery`, `parser`, `graph`, `storage` |
| `internal/discovery` | Walks the repository, returns files with metadata | `domain` |
| `internal/parser` | Parses individual files into entities | `domain` |
| `internal/graph` | Connects entities with typed, evidenced relationships | `domain` |
| `internal/storage` | Serializes/deserializes the Atlas Graph JSON | `domain` |

Key constraints:
- **Every package imports `domain`.** It is the shared vocabulary. `docs/data-model.md` is this vocabulary in English. `internal/domain` is the same vocabulary in Go. They match exactly.
- **`discovery`, `parser`, `graph`, and `storage` don't depend on each other.** Only `scanner` composes them. This makes each package testable in isolation.
- **`domain` depends on nothing.** It has zero imports from other Atlas packages. If `domain` ever imports `scanner` or `parser`, the architecture is broken.

**Status:** Designed. Code starts next.

### Known Architectural Risks

Three packages will outgrow their initial structure. Don't split them prematurely — but know when to split.

**Risk 1: `internal/parser` won't stay flat.**

Today: one file per parser (`go.go`, `yaml.go`, `markdown.go`, `test.go`). This works for Capability 1–2. By Capability 3, the Go parser alone will need separate files for controllers, functions, packages, and imports. When that happens, split into sub-packages:

```
internal/parser/
    parser.go           Common interface + types
    registry.go         Parser registration
    go/
        parser.go       Go AST entry point
        controller.go   Controller discovery
        function.go     Function extraction
        package.go      Package extraction
    yaml/
        parser.go
    markdown/
        parser.go
    test/
        parser.go
```

**Trigger to split:** When any single parser file exceeds ~300 lines or handles more than two entity kinds.

**Risk 2: `internal/discovery` may need more metadata.**

Today: discovery returns file paths, sizes, and modification times. Classification is the parser's job — discovery walks and reports, nothing more.

```go
type File struct {
    RelativePath string
    Size         int64
    ModifiedTime time.Time
}
```

**Trigger to extend:** When you implement incremental scanning (skip unchanged files) or parallel parsing (batch by size), additional metadata may be needed.

**Risk 3: `internal/graph` will grow the fastest.**

This package builds relationships, validates the graph, merges entities from multiple parsers, and manages IDs. It will eventually need sub-packages:

```
internal/graph/
    builder/        Relationship construction
    validator/      Graph integrity checks
    merger/         Entity deduplication across parsers
    ids/            Deterministic ID generation
    entity/         Entity types and kind-specific logic
    relationship/   Relationship types and evidence
```

**Trigger to split:** When the package has more than 3–4 files or when `builder` and `validator` logic start tangling.

---

## Architecture 3: Execution Pipeline

What happens when someone runs `atlas scan`. The sequence of operations, in order.

```
atlas scan /path/to/hypershift
        │
        ▼
┌─ 1. Configuration ───────────────────────────┐
│  Parse CLI flags                              │
│  Resolve repository path                      │
│  Load scan options (include/exclude patterns) │
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
│    (RelativePath, Size, ModifiedTime)         │
│  Skip: vendor/, .git/, node_modules/          │
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
│  For each classified file group, run the      │
│  appropriate parser:                          │
│                                               │
│  Go parser      → controllers, functions,     │
│                    packages, imports           │
│  YAML parser    → CRDs, Deployments,          │
│                    Services, ConfigMaps        │
│  Markdown parser → documents, headings,       │
│                    component references        │
│  Test parser    → test functions, test types,  │
│                    tested components           │
│                                               │
│  Output: flat list of entities                │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 6. Relationship Building ───────────────────┐
│  Connect entities with typed edges:           │
│                                               │
│  reconciles  — controller → CRD              │
│  creates     — controller → resource          │
│  calls       — function → function            │
│  tested_by   — entity → test                  │
│  documented_in — entity → document            │
│  imports     — package → package              │
│  owns        — CRD → CRD                     │
│  watches     — controller → CRD              │
│                                               │
│  Every edge carries evidence (file, line,     │
│  snippet, reason)                             │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 7. Graph Validation ────────────────────────┐
│  Check: no entity without a source            │
│  Check: no relationship without evidence      │
│  Check: all entity IDs are unique             │
│  Check: all relationship targets exist        │
│  Check: no orphaned entities (optional warn)  │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 8. Graph Writing ──────────────────────────┐
│  Serialize to atlas-graph.json                │
│  Include: schema version, commit, branch,     │
│           scan duration, stats                │
└───────────────┬───────────────────────────────┘
                ▼
┌─ 9. Summary ─────────────────────────────────┐
│  Print to terminal:                           │
│                                               │
│  Atlas scan complete.                         │
│  Repository: openshift/hypershift @ abc123    │
│  Duration:   12.4s                            │
│                                               │
│  Entities:   847                              │
│    controllers: 23                            │
│    functions:   612                            │
│    packages:    45                             │
│    crds:        8                              │
│    tests:       134                            │
│    documents:   25                             │
│                                               │
│  Relationships: 2,341                         │
│    proven:   1,987                             │
│    inferred: 354                               │
│                                               │
│  Output: atlas-graph.json (2.1 MB)            │
└───────────────────────────────────────────────┘
```

Each step maps to code:

| Pipeline Step | Package | Entry Function |
|---|---|---|
| 1. Configuration | `cmd/atlas` | `main()` / cobra command |
| 2. Repository Validation | `internal/scanner` | `ValidateRepository()` |
| 3. File Discovery | `internal/discovery` | `Scan()` |
| 4. File Classification | `internal/scanner` | Route files by extension to parsers |
| 5. Parser Execution | `internal/parser` | `Parse()` per parser |
| 6. Relationship Building | `internal/graph` | `BuildRelationships()` |
| 7. Graph Validation | `internal/graph` | `Validate()` |
| 8. Graph Writing | `internal/storage` | `Write()` |
| 9. Summary | `internal/scanner` | `PrintSummary()` |

**Status:** Designed. Implementation follows the capability roadmap.

---

## Capabilities

Atlas does not develop in sprints. It develops in **capabilities** — each one builds on the previous and unlocks the next.

| # | Capability | What It Unlocks | Depends On |
|---|---|---|---|
| 1 | **Repository Discovery** | Atlas can walk a repo, return file metadata, and route files to parsers | Nothing |
| 2 | **Go Parsing** | Atlas can extract controllers, functions, packages from Go source | Capability 1 |
| 3 | **Relationship Extraction** | Atlas can connect entities with typed, evidenced edges | Capabilities 1, 2 |
| 4 | **Atlas Graph** | Atlas can produce a complete, valid `atlas-graph.json` | Capabilities 1, 2, 3 |
| 5 | **Visualization** | Atlas can render the graph for human exploration | Capability 4 |
| 6 | **AI** | Atlas can answer natural language questions grounded in the graph | Capabilities 4, 5 |

Each capability is independently testable:

- **Capability 1 done** = discovery returns all files with metadata, classification routes them to the correct parser
- **Capability 2 done** = parsers extract entities that match the data model schema
- **Capability 3 done** = relationships have evidence and pass graph validation
- **Capability 4 done** = `atlas-graph.json` is complete, valid, and deterministic
- **Capability 5 done** = a user can click HostedCluster and understand it
- **Capability 6 done** = AI answers match what the graph already proves

The order matters. You can't extract relationships (3) without parsing (2). You can't build the graph (4) without relationships (3). You can't add AI (6) without a graph to ground it.

This is how platforms evolve — one capability at a time.

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
