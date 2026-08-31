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

### Phase 11: Knowledge Views

**Delivered:** View compiler — deterministic engineering summaries generated during scan for every controller and CRD.

- `internal/views/compiler.go` — compiles views from entities + relationships
- `domain.View` struct: reconciles, creates, watches, calls, tests, files, owners, temporal data
- Views stored in graph as `views` field (first-class JSON artifacts)
- `atlas_view` MCP tool — zero graph traversal, pure lookup
- 139 views generated for HyperShift (all controllers + CRDs)

**Result:** One-call entity summary. Claude retrieves `NodePool View` instead of traversing graph nodes. Schema 1.4.0.

---

### Phase 12: Query Planner

**Delivered:** `atlas_ask` — one MCP call replaces multi-tool chains.

- `Ask(entity, intent)` method: search → view → compound query in one call
- Intents: `understand` (view + explain), `impact` (view + impact), `debug` (view + investigate)
- Default (no intent): returns view only (cheapest path)
- Tool description guides Claude to use `atlas_ask` first

**Result:** A consumer asks one question, gets a bounded evidence packet. 11 graph-query MCP tools total at this stage.

---

### Phase 13: Question Index

**Delivered:** Deterministic Q&A pairs generated during scan, stored in graph.

- `CompileQuestions()` — derives answers from pre-computed views
- Question templates: reconciles, reconciled-by, creates, created-by, tests, owns, files, watches
- Stored in graph as `questions` field (key = `"verb:subject"`, value = answer)
- 60 Q&A pairs generated for HyperShift
- `LookupQuestion(verb, subject)` for instant retrieval

**Result:** Common engineering questions answered by pure lookup — no graph traversal, no AI reasoning.

---

### Phase 14: PR Review

**Delivered:** `atlas review` — deterministic PR review using the Atlas graph.

- `internal/review` package: bounded diff evidence, diff parser, entity mapper, enrichment, formatter
- Git unified diff parsing (handles new files, deleted files, renames, multiple hunks, binary files)
- Three-dot diff (`base...head`) for PR review semantics
- Entity-to-hunk mapping uses parser source spans (`endLine`) when available; merged implementation files and legacy spans are labeled approximate
- Test-to-function links use stored `tested_by` edges first; naming and same-file fallback remain explicitly inferred
- Callee noise filtering: stdlib packages + generic K8s/Go methods
- Evidence classification: FOUND, NOT_FOUND, INSUFFICIENT_EVIDENCE (never "Missing test")
- Facts vs recommendations separation — output shows what Atlas knows, not what it guesses
- `--diff` flag: read diff from file or stdin (no git fetch needed)
- Human-readable and JSON output: bounded changed diff, changed entities/hunks, callers, blast radius, test links, freshness, and limitations

**Result:** Deterministic PR review from Atlas graph. Reviewer sees changed entities and diff hunks, who calls them, what controllers/resources they affect, and which tests are structurally linked — with evidence and inference labels preserved.

---

### Architecture Audit Fixes

**Delivered:** Corrected semantic falsehoods and strengthened the graph evidence contract.

- **Watches vs creates**: `For`, `Owns`, and `Watches` are retained as aligned observations and emit distinct relationship types. Watching is not creating.
- **Implements**: Interface assertions remain observations; no `implements` edge is emitted without a first-class interface entity.
- **Embeds**: Resources are matched against the actual `//go:embed` pattern relative to its declaration file before an `embeds` edge is emitted.
- **Validation contract**: Entities require source and kind-matching IDs; relationships require deterministic IDs, valid endpoints, evidence, and valid confidence.
- **Question index**: `Ask()` now calls `LookupQuestion()` — the index was generated but never served.
- **PR review tests**: Graph-linked tests rendered per function (were calculated but omitted from output).
- **PR review coordinates**: Mapper uses parser source spans and `NewStart`/`NewCount` (head-side) to match graph entity locations; approximate mappings are labeled.
- **Deleted file reporting**: Explicitly reported as "no head-side mapping" instead of silently skipped.
- **Discovery**: `testdata/` added to skip list (code now matches documented behavior).

**Graph metadata in stats:** `atlas stats` now outputs `commit`, `branch`, and `generated` fields from graph metadata, enabling downstream freshness checks.

**Current contract:**
- Embed matching uses the actual `//go:embed` pattern relative to the declaration file and is marked `proven` only when a scanned resource path matches.
- Relationship IDs must equal the deterministic `from--type--to` form; duplicate, orphan, unsupported, and evidence-less edges are rejected before persistence.
- Repository scans use `repository-path-v1` entity identity. Incremental reuse and implementation-oriented consumers require a current, complete graph with verifiable file state.
- The Go parser keeps controller observations separate by receiver, including when multiple controllers share one source file.
- Temporal enrichment remains opt-in; a non-temporal incremental scan clears history reused from an earlier temporal graph.
- Compound MCP text is bounded by default; `detail=true` is required for verbose investigation, explanation, or impact text while structured output remains bounded and evidence-bearing.

**Result:** Validation catches graph identity, endpoint, deterministic-ID, evidence, and completeness violations at scan and storage boundaries. Review output separates deterministic facts, inferred links, and unsupported coverage claims.

---

## Vision: Deterministic Query and Evidence Engine

CodeAtlas is not an AI reasoning engine. It is a **deterministic query and
evidence engine for software architecture**.

A graph stores facts. Deterministic views and compound queries derive bounded,
reusable engineering context from those facts — without inventing anything.
Claude, CodeAtlas Assistant, or another AI consumer may reason over that
context, but remains a replaceable downstream consumer and presentation layer.

### Evolution

**Stage 1** (done): Graph → MCP → Claude. ~70-80% token reduction vs grep+read.

**Stage 2** (done): Move orchestration into Atlas. One call replaces search → investigate → explain → impact chain. Atlas traverses internally, returns one structured object. Consumers receive bounded evidence, not an unbounded raw graph dump.

**Stage 3** (done): Pre-computed views at scan time. Entity and lifecycle views are generated during `atlas scan`; query consumers use bounded view and graph evidence rather than traversing source during inference.

**Stage 4** (future): Expand deterministic question → answer storage for stable repository patterns. AI consumers may rewrite or reason over those answers, but cannot add unsupported repository facts. Target: further reduction in architecture-related reasoning tokens.

---

## Next: Reducing Consumer Work Without Moving Probabilistic Reasoning into Atlas

Phases 1–17 optimize retrieval and deterministic review preparation. Remaining
work should reduce repeated context and add evidence-backed analysis; it must
not move probabilistic repository reasoning into the CodeAtlas core.

| Phase | Goal | Status |
|-------|------|--------|
| 11. Knowledge Views | Generate lifecycle, ownership, dependencies, tests, execution flow during scan | Done |
| 12. Query Planner | Atlas internally orchestrates graph traversals and returns one result | Done |
| 13. Knowledge Cache + Question Index | Cache precomputed answers to common engineering questions | Done |
| 14. PR Review | Deterministic PR review: diff → entities → blast radius → test coverage | Done |
| 15. PR Metadata | Fetch PR title, description, labels from GitHub API (`--pr` flag) | Done |
| 16. Pattern Analysis | Compare PR against repo conventions: naming, error handling, logging | Done |
| 17. Test Analysis | Evaluate test sufficiency: is changed behavior actually covered? | Done |
| 18. LLM Integration | Optional downstream AI layer for natural language summary and recommendations | Implemented in `codeatlas-assistant` |
| 19. MCP Consumer Parity | Expose deterministic freshness and PR review through bounded MCP tools | Done |

Target: ~92–95% total reduction (from current ~70–80%).

---

### Phase 15: PR Metadata (Delivered)

Fetch PR title, description, and labels from GitHub API via `--pr` flag.

- `atlas review --pr openshift/hypershift/8968 --graph atlas.json`
- Uses `gh api` to fetch PR metadata (title, body, labels, files)
- Adds "What This PR Does" section to review output using PR description
- Diff fetched via GitHub API (standard unified diff format)
- No git clone or fetch required
- Bounds and labels PR body/file metadata, and reports graph-to-PR-head freshness
  without claiming checkout verification

**Principle:** Still deterministic. PR description is user-provided context, not AI-generated.

---

### Phase 16: Pattern Analysis (Delivered)

Compare PR changes against existing repo patterns extracted from the graph.

- Naming conventions: does the new function match package naming patterns?
- Error handling: does the code follow the repo's error wrapping style?
- Logging patterns: are log calls consistent with the file/package?
- Controller patterns: does a new reconciler follow existing reconciler structure?
- Compares only against deterministic same-package or same-package-controller
  peer observations, with source evidence and bounded output
- Reports `MATCHES_OBSERVED`, `DIFFERS_OBSERVED`, or
  `INSUFFICIENT_EVIDENCE`; a difference is never presented as a defect

**Principle:** Patterns derived from the graph, not from rules. "This repo does X, this PR does Y" — facts, not opinions.

---

### Phase 17: Test Analysis (Delivered)

Evaluate test sufficiency for changed code.

- Does a changed function have tests? (graph `tested_by` edges)
- Are new functions tested? (new entities without test links)
- Do tests cover the changed behavior or just the function signature?
- Separates stored test links from naming or same-file inferred links
- Reports `INSUFFICIENT_EVIDENCE` whenever changed behavior or runtime coverage
  cannot be proven; absence of an edge is not called a missing test
- Keeps branch/runtime execution claims outside the deterministic review

**Principle:** Conservative. "INSUFFICIENT_EVIDENCE" when Atlas can't prove coverage. Never "Missing test" — Atlas doesn't know what you chose not to test.

---

### Phase 18: LLM Integration (Implemented downstream)

Optional AI layer — last in the pipeline, never first.

- Takes deterministic Atlas review output as input
- Generates natural language summary: "This PR adds X to solve Y"
- Optionally suggests review focus areas based on blast radius
- Uses `codeatlas-assistant` as the integration layer
- Atlas output is the ground truth; LLM is the presentation layer
- Assistant implementation prompts use exact graph-selected source files and
  functions when a repository checkout is supplied; ambiguous or unverifiable
  implementation context is refused or reported.

**Principle:** LLM adds clarity, not knowledge. All facts come from Atlas. If Atlas can't prove it, LLM can't claim it.

---

### Phase 19: MCP Consumer Parity (Delivered)

The MCP server now exposes the deterministic capabilities that were previously
CLI-only:

- `atlas_freshness` verifies graph provenance and checkout state without writing
  files.
- `atlas_review` supports GitHub PRs, supplied unified diffs, and verified local
  git refs through the existing `internal/review` service.
- MCP review responses use bounded entity summaries, preserve relationship
  evidence, and mark truncation explicitly to reduce downstream context.
- Scanning remains CLI-only; MCP consumers cannot replace the graph they query.

---

## Other Future Work

### Architecture Intelligence

Deterministic graph analyses — no AI required.

- `atlas diff old.json new.json` — compare graphs across branches/releases
- Dead code identification — functions with no callers and no tests
- Missing test coverage — high-change entities without tested_by edges
- Orphan detection — entities with no incoming relationships
- Incremental knowledge generation — if only NodePool.go changed, only regenerate NodePool views

### Multi-Repository Knowledge

- Cross-repo relationship tracking (e.g., HyperShift → cluster-api → machine-api)
- Federated graph queries across multiple repositories

### Stateful Sessions (Optional)

Atlas becomes stateful — maintains investigation context across a conversation. `"What creates it?"` no longer requires another search. Separate track from the deterministic pipeline.
