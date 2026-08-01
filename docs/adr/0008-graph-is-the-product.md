# ADR-0008: The Atlas Graph Is the Product

**Status:** Accepted
**Date:** 2026-07-14

## Decision

The graph is not an internal data structure — it's the product. The scanner produces it. Every consumer (CLI, MCP server, AI assistants, future API) reads the same JSON file.

## Why

Improving the graph improves every consumer automatically. A new entity kind or relationship type requires no consumer changes — they render what they find. This also means the graph must be a stable, well-documented format, not an implementation detail.

## Future Implication

`atlas scan`, `atlas query`, `atlas diff`, `atlas serve` — all revolve around the Atlas Graph.
