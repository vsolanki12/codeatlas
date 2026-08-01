# Roadmap

CodeAtlas is a **deterministic reasoning engine for software architecture**. It derives higher-level engineering knowledge from code — without AI — so that AI assistants become presentation layers, not reasoning layers.

Development progresses in phases. Each builds on the previous.

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
- `atlas_temporal` MCP tool — most-changed or stalest entities

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
- `atlas_entity` batch fetch via `ids` param (N calls → 1)
- `atlas_where` detail mode (N entity lookups → 1)

**Result:** ~8-10 calls for bug investigations (was 22).

---

### Phase 6c: Search Quality + Call Reduction

**Delivered:** Relevance scoring, reverse call graph, selector extraction, temporal search.

- Relevance-ranked search: Name=100, ID=90, Description=70, Package=60, Import=40, Literal=30, Property=20
- Reverse call graph (now in `atlas_investigate`)
- Selector/constant extraction for Go AST
- Temporal search by name/since/author (now `atlas_temporal`)

**Result:** ~6-7 calls for bug investigations.

---

### Phase 7: Compound Queries

**Delivered:** `atlas_investigate` and `atlas_explain`.

- `Investigate` — entity + all relationships grouped + callers + tests + siblings in 1 call
- `Explain` — DFS tree following reconciles → creates → calls → tested_by chain
- Caps: 100 nodes, depth 3, cycle-safe via visited set

**Result:** 1-2 calls for entity investigation (was 4-5).

---

### Phase 7b: Blast Radius Analysis

**Delivered:** `atlas_impact`.

- BFS upstream call chain (depth 5, cap 50 entities)
- Collects: controllers, tests, resources, files, owners, recent changes
- One-call PR review preparation

**Result:** One-call PR review preparation.

---

### Phase 8: Intent-Based Tool Guidance

**Status:** Attempted and reverted.

- Enriched 6 tool descriptions with "Best used when", "Examples", "Usually followed by"
- Result: longer descriptions increased token cost on every API turn (tool schemas are sent as input on every call, not just when used)
- "Usually followed by" hints also caused AI to chain 8-11 follow-up calls instead of stopping
- Net effect: higher cost and more calls — opposite of the goal

**Lesson:** Intent guidance via descriptions is a per-turn tax. The right approach is intent-based compound tools (see future: Intent-Based Tools) that reduce both call count and schema size. Descriptions reverted to original concise form.

---

### Phase 9: Incremental Scanning

**Delivered:** Skip unchanged files during re-scans using stored timestamps.

- `FileTimestamps` field in graph (schema 1.3.0) — stores per-file RFC3339 timestamps
- `changedFiles()` compares filesystem mtime against stored values
- `entitiesFromFiles()` extracts entities from previous graph for unchanged files
- CLI auto-detects previous graph at output path
- Relationships always rebuilt globally (pure function, ~50ms)
- Handles directory-based Source.File (package entities)

**Result:** HyperShift re-scan: 1.5s → 400ms (3 files changed out of 4,896). Schema 1.3.0.

---

### Phase 10: Tool Consolidation

**Delivered:** 14 MCP tools → 9. Reduced per-turn schema cost by ~900 tokens.

- `atlas_lookup` merged into `atlas_search` (added `kind` param)
- `atlas_entities` merged into `atlas_entity` (added `ids` array param)
- `atlas_relationships` removed (subsumed by `atlas_investigate`)
- `atlas_callers` removed (subsumed by `atlas_investigate`)
- `atlas_hotspots` + `atlas_commits` merged into `atlas_temporal`

**Result:** 9 MCP tools. ~1,200 schema tokens per turn (was ~2,100).

---

## Vision: Deterministic Reasoning Engine

CodeAtlas is not a graph queried by AI. It is a **deterministic reasoning engine for software architecture**.

A graph stores facts. A reasoning engine derives higher-level, reusable engineering knowledge from those facts — without inventing anything. Claude (or any AI assistant) becomes the presentation layer, turning deterministic results into natural language.

### Evolution

**Stage 1** (done): Graph → MCP → Claude. ~70-80% token reduction vs grep+read.

**Stage 2**: Move orchestration into Atlas. One call replaces search → investigate → explain → impact chain. Atlas traverses internally, returns one structured object. Claude receives the answer, not raw graph data.

**Stage 3**: Pre-computed views at scan time. Entity views, lifecycle views, subsystem views, impact views — generated during `atlas scan`, not traversed during inference. Query becomes pure lookup. No graph traversal during AI inference.

**Stage 4**: Question → answer storage. During scan, discover patterns (e.g., "NodePool lifecycle") and derive deterministic answers. At query time, look up the pre-computed answer. Claude only rewrites into English. Target: ~95% reduction in architecture-related reasoning tokens.

---

## Future

### Architecture Intelligence

Deterministic graph analyses that surface architectural changes — no AI required.

- `atlas diff old.json new.json` — compare graphs across branches/releases
- Dead code identification — functions with no callers and no tests
- Missing test coverage — high-change entities without tested_by edges
- Orphan detection — entities with no incoming relationships

**Depends on:** Incremental Scanning (done). Aligns with Stage 2.

### Multi-Repository Knowledge

- Cross-repo relationship tracking (e.g., HyperShift → cluster-api → machine-api)
- Federated graph queries across multiple repositories

**Depends on:** Origin classifier (done). Import path tracking (done).
