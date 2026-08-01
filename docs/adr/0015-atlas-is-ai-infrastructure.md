# ADR-0015: CodeAtlas Is an AI Infrastructure Platform

**Status:** Accepted
**Date:** 2026-08-01

## Decision

CodeAtlas provides deterministic architectural knowledge that AI assistants consume. CodeAtlas does not perform AI reasoning. AI consumers are replaceable. Better AI models require no CodeAtlas changes.

## Why

As AI-powered development tools proliferate (Claude Code, VS Code Copilot, Cursor, Continue.dev), each needs to understand codebase architecture. Without CodeAtlas, every AI assistant reads thousands of source files — slow, expensive, often incomplete. With CodeAtlas, they query a pre-built graph that already has the answer.

CodeAtlas is infrastructure for AI, not an AI itself. This distinction matters because:

1. **AI models change fast.** A new model next year should work with CodeAtlas immediately — it just calls MCP tools.
2. **The graph is deterministic.** Same commit, same graph. AI adds interpretation; CodeAtlas provides facts.
3. **No vendor lock-in.** CodeAtlas works with any MCP-compatible client. Switching from Claude to Cursor requires zero CodeAtlas changes.
4. **Correctness is verifiable.** Every relationship in the graph carries evidence. An AI consumer can cite the source file and line, not just a model's confidence score.

## Consequences

- CodeAtlas never includes an LLM, embedding model, or RAG pipeline in its core.
- AI reasoning happens in the consumer (Layer 4), not in CodeAtlas (Layers 1-3).
- New AI consumers are supported by adding MCP compatibility, not CodeAtlas code.
- CodeAtlas's value increases as AI tools improve — better models ask better questions of the same graph.
- The graph is the stable foundation; AI consumers are the moving parts.
