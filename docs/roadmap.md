# Roadmap

Atlas develops in **phases**. Each phase builds on the previous and unlocks the next. A phase is done when its success metric passes.

---

## Implemented

### Phase 1–2: Core Scanner + CLI

**Delivered:** Scanner pipeline (discovery → parsing → relationships → graph), CLI.

- 4 parsers: Go AST, YAML, Markdown, Test
- Entity kinds: controller, crd, function, package, test, document, resource
- Typed relationships with evidence
- CLI: `atlas scan`, `atlas query`

**Result:** 11,290 entities, 432 relationships.

---

### Phase 3a: Deep Call Graph + Enrichment

**Delivered:** Function-level call graph, implements detection, env var tracking.

- Call extraction from Go AST function bodies
- 7,837 call edges added
- `implements` relationship detection
- Environment variable tracking (60 entities)

**Result:** 8,246 relationships (up from 936).

---

### Phase 4: Cross-Repo Intelligence

**Delivered:** Import path classification, merge-aware dedup.

- `internal/origin` — import path classifier: stdlib vs known repos vs external
- Import extraction from Go AST
- Merge-aware dedup: duplicate package entities merge Files and Imports arrays

**Result:** 225 packages with import data, 154 with merged multi-file data.

---

### Phase 5: Temporal Layer

**Delivered:** Git history enrichment (opt-in via `-temporal` flag).

- `internal/temporal` — `Enrich()` runs `git log` + `git rev-list` per unique file
- Fields: LastAuthor, LastModified, ChangeCount per entity
- File-level cache avoids redundant git calls
- `atlas_hotspots` MCP tool — most-changed or stalest entities

**Result:** 10,567 entities with temporal data. Top hotspot: `package:hostedcontrolplane` (2,129 commits).

---

### Phase 6: Content Indexing

**Delivered:** String literals, YAML properties, go:embed detection.

- `extractLiterals()` — filtered string literals from function bodies
- `flattenYAML()` — key-value pairs on resource entities
- `extractEmbeds()` — `//go:embed` directives, `RelEmbeds` relationship
- Search extended to match Literals and Properties

**Result:** 3,100 entities with literals, 398 with properties, 16 with embeds, 425 embeds relationships. Schema 1.2.0.

---

### Phase 6b: Token Optimization

**Delivered:** AND search, brief mode, batch fetch, detail mode.

- AND search: space-separated terms, all must match
- `atlas_entity` brief mode (~90% token reduction)
- `atlas_entities` batch fetch (N calls → 1)
- `atlas_where` detail mode (N entity lookups → 1)

**Result:** 10 MCP tools. ~8-10 calls for bug investigations (was 22).

---

### Phase 6c: Search Quality + Call Reduction

**Delivered:** Relevance scoring, reverse call graph, selector extraction, temporal search.

- Relevance-ranked search: Name=100, ID=90, Description=70, Package=60, Import=40, Literal=30, Property=20
- `atlas_callers` — reverse call graph lookup
- Selector/constant extraction for Go AST
- `atlas_commits` — temporal search by name/since/author

**Result:** 12 MCP tools. ~6-7 calls for bug investigations.

---

### Phase 7: Compound Queries

**Delivered:** `atlas_investigate` and `atlas_explain`.

- `Investigate` — entity + all relationships grouped + callers + tests + siblings in 1 call
- `Explain` — DFS tree following reconciles → creates → calls → tested_by chain
- Caps: 100 nodes, depth 3, cycle-safe via visited set

**Result:** 14 MCP tools. 1-2 calls for entity investigation (was 4-5).

---

### Phase 7b: Blast Radius Analysis

**Delivered:** `atlas_impact`.

- BFS upstream call chain (depth 5, cap 50 entities)
- Collects: controllers, tests, resources, files, owners, recent changes
- One-call PR review preparation

**Result:** 15 MCP tools.

---

## In Progress

### Phase 8: Intent-Based Tool Guidance

**Goal:** Teach AI consumers which tool to use via enriched descriptions — zero new code.

- Six engineering intents: Understand, Investigate, Review, Navigate, Test, Implement
- "Best used when" + "Usually followed by" in tool descriptions
- Four-layer model: Knowledge → Retrieval → Guidance → Experience

| Goal | Target |
|------|--------|
| Correct first tool | ≥80% |
| Average Atlas calls | ≤3 |
| Source file reads | 0 for architecture questions |
| New code | 0 |

**Plan:** `docs/phase8-plan.md`

---

## Future

Capabilities not yet scheduled. Captured so they're not lost.

### Version Intelligence

Compare architecture across branches and releases.

- Scan different branches (e.g., `release-4.19` vs `release-4.20`)
- `atlas diff release-4.19.json release-4.20.json`
- Highlight added, removed, and changed entities and relationships

**Depends on:** Stable, deterministic graph (done). `commit` and `branch` fields already in schema.

### Continuous Architecture Intelligence

Deterministic graph analyses — no AI required.

- Architecture diff on every PR (what relationships changed?)
- Dependency drift detection (new imports, removed calls)
- Orphan detection (entities with no incoming relationships)
- Dead code identification (functions with no callers and no tests)
- Missing test coverage (high-change entities without tested_by edges)
- Architecture regression detection (broken relationships between releases)

**Depends on:** Version Intelligence (graph diff).

### Multi-Repository Knowledge

Atlas scans more than HyperShift.

- Any large Go project with controllers, CRDs, and complex call graphs
- Cross-repo relationship tracking (e.g., HyperShift → cluster-api → machine-api)
- Federated graph queries across multiple repositories

**Depends on:** Origin classifier (done). Import path tracking (done).

### AI Consumer Ecosystem

Every AI assistant understands HyperShift by querying Atlas instead of reading thousands of files.

- Claude Code (current)
- VS Code / Cursor / Continue.dev / Copilot Chat (any MCP client)
- Atlas API for non-MCP consumers

**Depends on:** MCP server (done). Intent guidance (Phase 8).

### Live Repository Monitoring

Keep the graph up to date as you code.

- `atlas watch /path/to/hypershift` — filesystem monitoring
- Incremental re-scan (only changed files)
- Auto-refresh consumers

**Depends on:** Discovery metadata (ModifiedTime already captured).
