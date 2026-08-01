# ADR-0015: Atlas Is an AI Infrastructure Platform

**Status:** Accepted
**Date:** 2026-08-01

## Decision

Atlas provides deterministic architectural knowledge that AI assistants consume. Atlas does not perform AI reasoning. AI consumers are replaceable. Better AI models require no Atlas changes.

## Why

As AI-powered development tools proliferate (Claude Code, VS Code Copilot, Cursor, Continue.dev), each needs to understand codebase architecture. Without Atlas, every AI assistant reads thousands of source files — slow, expensive, often incomplete. With Atlas, they query a pre-built graph that already has the answer.

Atlas is infrastructure for AI, not an AI itself. This distinction matters because:

1. **AI models change fast.** A new model next year should work with Atlas immediately — it just calls MCP tools.
2. **The graph is deterministic.** Same commit, same graph. AI adds interpretation; Atlas provides facts.
3. **No vendor lock-in.** Atlas works with any MCP-compatible client. Switching from Claude to Cursor requires zero Atlas changes.
4. **Correctness is verifiable.** Every relationship in the graph carries evidence. An AI consumer can cite the source file and line, not just a model's confidence score.

## Consequences

- Atlas never includes an LLM, embedding model, or RAG pipeline in its core.
- AI reasoning happens in the consumer (Layer 4), not in Atlas (Layers 1-3).
- New AI consumers are supported by adding MCP compatibility, not Atlas code.
- Atlas's value increases as AI tools improve — better models ask better questions of the same graph.
- The graph is the stable foundation; AI consumers are the moving parts.
