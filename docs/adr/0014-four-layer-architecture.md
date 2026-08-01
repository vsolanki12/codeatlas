# ADR-0014: Four-Layer Architecture Model

**Status:** Accepted
**Date:** 2026-08-01

## Decision

Atlas is organized into four layers: Knowledge, Retrieval, Guidance, Experience. Each layer has a distinct rate of change and responsibility.

## The Layers

| Layer | What | Rate of Change |
|-------|------|---------------|
| **Knowledge** | Scanner → Graph | Every scan |
| **Retrieval** | 15 MCP tools (primitives + compounds) | Quarterly |
| **Guidance** | Tool descriptions with intent hints | Rarely after Phase 8 |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Consumer-driven |

## Why

Atlas started as a scanner that produced JSON. Over phases 3-8, it grew a query engine, MCP server, compound queries, and intent-based guidance. Without a model, these additions felt ad-hoc. The four-layer model explains why each piece exists and what changes it.

The key insight: Atlas itself doesn't decide anything. The consumer does. Layer 4 (Experience) is explicitly outside Atlas's control — it's where engineers interact with the graph through whatever tool they prefer. This makes the architecture future-proof: adding a VS Code extension or Cursor integration doesn't change layers 1-3.

## Consequences

- When tempted to add a new tool, first check if enriching an existing tool's description (Layer 3) solves the problem.
- New tools only when the graph can't answer the question at all (Layer 2).
- Scanner changes (Layer 1) are the most expensive — they affect the graph schema and every downstream consumer.
- Layer 4 grows organically. Atlas doesn't need to build every consumer.
