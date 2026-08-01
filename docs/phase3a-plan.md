# Phase 3a: Deeper Code Analysis — Full Call Graph + Interface Detection

## Context

Phase 2 shipped: 7 MCP tools, 102 tests, query engine working. But the graph has only **936 relationships** — almost all from controllers only. 8,384 function entities have zero call edges between them. "Who calls this helper?" is unanswerable from the graph.

Phase 3a enriches the graph without adding new dependencies. Three changes, all AST-only:

1. **Full call graph** — extract calls from ALL function bodies, not just `Reconcile()`
2. **Interface implementation** — detect `var _ Interface = &Struct{}` patterns
3. **Env var tracking** — detect `os.Getenv("X")` calls, store on entities

Expected impact: relationships grow from ~936 to ~5,000+. "Who calls X?" becomes answerable via `atlas_relationships`.

## Architecture Decisions

- **Stay AST-only** — no `go/types`, no `golang.org/x/tools`. Per-file parsing, same as Phase 1.
- **Calls on function entities** — `Entity.Calls` field already exists but is only populated for controllers. Populate it for all functions.
- **`implements` relationships** — use the existing `RelImplements` type (declared in domain but unused).
- **Env vars as entity metadata** — no new entity kind. New `Entity.EnvVars []string` field. Searchable via `atlas_search`.
- **Relationship builder handles all entities with Calls** — not just controllers.
- **Qualified call names** — store `Receiver.Method` when possible, bare name for same-package calls. Reduces false-positive matches.
- **Generic name skip-list** — skip `Error`, `String`, `Get`, `Set` etc. to prevent graph explosion.

## Files to Modify

| File | Change |
|---|---|
| `internal/domain/entity.go` | Add `EnvVars`, `Implements` fields |
| `internal/parser/goparser.go` | Extract calls from all functions, detect `var _` patterns, detect `os.Getenv` |
| `internal/graph/builder.go` | Build `calls` edges for functions, build `implements` edges |
| `internal/query/format.go` | Show EnvVars/Implements in FormatEntityFull |
| Tests | New test cases for all three features |
