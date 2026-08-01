# ADR-0011: Compound Queries Over Primitive Sequences

**Status:** Accepted
**Date:** 2026-08-01

## Decision

Build compound query tools (`atlas_investigate`, `atlas_explain`, `atlas_impact`) that compose multiple primitive operations into single calls, rather than relying on AI consumers to chain primitives correctly.

## Why

Investigating a single entity with primitives required 4-5 sequential MCP calls: search, entity, relationships, callers, where. Each call carries ~8K system prompt overhead. Five calls = 40K tokens in prompt tax alone, plus the latency of five round-trips.

Compound queries collapse this to 1 call with ~2K structured output. The graph traversal happens server-side in Go — fast, deterministic, no token cost.

More importantly, AI consumers don't always chain primitives correctly. They forget to check callers, skip test coverage, or stop after the first relationship. Compound queries encode expert investigation patterns that always return complete results.

## Consequences

- `Investigate` = entity + relationships grouped by type + callers + tests + siblings. One call replaces 4-5.
- `Explain` = DFS tree following reconciles → creates → calls → tested_by. Replaces reading source files.
- `Impact` = BFS upstream call chain + controllers + tests + resources + files + owners. One-call PR review prep.
- Primitives remain available for edge cases compound queries don't cover.
- New compound queries should be added sparingly — only when a common investigation pattern requires 3+ primitive calls.
