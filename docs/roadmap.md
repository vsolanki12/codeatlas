# Roadmap

Atlas develops in **capabilities**, not sprints. Each capability builds on the previous and unlocks the next. A capability is done when its success metric passes.

---

## Capability 1 — Repository Discovery

**Unlocks:** Atlas can walk a repository, return file metadata, and route files to the correct parser.

| Week | Focus | Deliverable |
|------|-------|-------------|
| 1 | `internal/discovery` package | Walk directory tree, return files with metadata (RelativePath, Size, ModifiedTime), skip vendor/.git |
| 1 | File classification (in scanner) | Route discovered files to correct parser by extension (.go, .yaml, .md, _test.go) |

**Success metric:** Discovery returns all files with metadata. Classification routes each file to the correct parser. Unrecognized files are skipped.

**Depends on:** Nothing.

---

## Capability 2 — Go Parsing

**Unlocks:** Atlas can extract entities from Go source code.

| Week | Focus | Deliverable |
|------|-------|-------------|
| 1 | Learn Go AST basics | Parse a single Go file, extract function names and signatures |
| 2 | Controller discovery | Find structs with `Reconcile()`, extract watched resources |
| 3 | Package parsing | Extract imports, package docs, file listings |

**Success metric:** Parsers produce entities matching the data model schema for controllers, functions, and packages.

**Depends on:** Capability 1 (discovery provides the file list).

---

## Capability 3 — Relationship Extraction

**Unlocks:** Atlas can connect entities with typed, evidenced edges.

| Week | Focus | Deliverable |
|------|-------|-------------|
| 3 | YAML parser | Extract CRDs, Deployments, Services |
| 3 | Markdown parser | Extract documents, headings, component references |
| 4 | Relationship builder | Connect entities: reconciles, creates, calls, tested_by, etc. |

**Success metric:** Every relationship has a type, confidence level, and evidence (file, line, snippet, reason).

**Depends on:** Capabilities 1, 2.

---

## Capability 4 — Atlas Graph

**Unlocks:** `atlas scan` produces a complete, valid `atlas-graph.json`.

| Week | Focus | Deliverable |
|------|-------|-------------|
| 4 | Graph validation | Verify: unique IDs, no orphans, all targets exist |
| 4 | Graph writing | Serialize to JSON with schema version, commit, stats |
| 4 | CLI output | Print summary (entity counts, relationship counts, duration) |

**Success metric:** Run `atlas scan /path/to/hypershift` and get a deterministic, valid Atlas Graph. Run it again on the same commit — get the same graph.

**Depends on:** Capabilities 1, 2, 3.

---

## Capability 5 — Visualization

**Unlocks:** A user can explore the Atlas Graph visually.

### V1: Simple Viewer (Weeks 5–6)

| Week | Focus | Deliverable |
|------|-------|-------------|
| 5 | Load JSON, list entities | Static HTML page showing all discovered entities |
| 6 | Entity detail view | Click an entity, see package/functions/tests/docs/relationships |

No React. No bundler. One HTML file that loads the JSON.

### V2: Interactive Web UI (Weeks 7–10)

| Week | Focus | Deliverable |
|------|-------|-------------|
| 7 | React setup, homepage | Entity cards with filtering |
| 8 | Entity detail pages | Full detail view with tabbed sections |
| 9 | Graph visualization | Interactive dependency graph |
| 10 | Search + polish | Search bar, deep links, responsive layout |

### V3: Call Graphs and Workflows (Weeks 11–14)

Function-level call chains and controller reconciliation workflows.

**Success metric:** Click "HostedCluster" and immediately understand how it works — what it creates, what manages it, what tests validate it — without opening the repository.

**Depends on:** Capability 4.

---

## Capability 6 — AI

**Unlocks:** Atlas can answer natural language questions grounded in the graph.

No timeline. This happens when capabilities 1–5 are mature and proven.

- Ollama with full Atlas Graph as context
- Answers backed by evidence from the graph, not hallucinations
- "Why is syncEtcd called before syncKAS?" → answered from the actual call graph

**Depends on:** Capabilities 4, 5.

---

## Capability 7 — Version Intelligence (Future)

**Unlocks:** Atlas can compare architecture across branches, releases, and time.

- Scan different HyperShift branches (e.g., `release-4.19` vs `release-4.20`)
- Store branch and commit metadata in each Atlas Graph (already in the schema: `commit`, `branch`)
- Compare two Atlas Graphs: `atlas diff release-4.19.json release-4.20.json`
- Highlight added, removed, and changed entities and relationships
- Visualize architectural evolution across releases

**Success metric:** Run `atlas diff` on two release branches and immediately see what changed — new controllers, removed CRDs, changed relationships — without reading a single commit message.

**Depends on:** Capability 4 (stable, deterministic graph). The `commit` and `branch` fields already exist in the schema — this capability consumes them.

---

## Capability 8 — Live Repository Monitoring (Future)

**Unlocks:** Atlas keeps the graph up to date as you code.

- `atlas watch /path/to/hypershift` — monitor the repository for filesystem changes
- Detect modified, added, and deleted files
- Re-run the scanner automatically (ideally incremental — only re-parse changed files)
- Refresh the viewer in the browser without manual reload

**Success metric:** Edit a controller file, save it, and see the Atlas Graph and viewer update within seconds — without running `atlas scan` manually.

**Depends on:** Capability 4 (working scanner). Benefits from Discovery Risk 2 (file metadata with `ModifiedTime` for incremental scanning).

---

## Future Capabilities

Not numbered. Not scheduled. Captured so they're not lost.

- **Atlas API** — Serve the graph over HTTP for external consumers
- **CLI explorer** — `atlas query "what does CPO create?"`
- **Impact analysis** — Highlight what breaks when a file changes
- **PR integration** — Diff the graph on every PR, comment with impact
- **VS Code extension** — Show Atlas context inline while reading code
- **Staleness detection** — Flag when docs and code are out of sync
- **Export** — Generate SVG/PNG of any graph view for presentations
