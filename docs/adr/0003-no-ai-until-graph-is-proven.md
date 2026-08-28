# ADR-0003: No AI Reasoning in the Atlas Core

**Status:** Accepted (clarified)
**Date:** 2026-07-14

## Decision

CodeAtlas core contains no LLM, RAG, embedding, or other probabilistic
repository-analysis implementation. The scanner, graph builder, query engine,
and review evidence remain deterministic and authoritative.

AI consumers may be built downstream once the graph exposes entity identity,
relationship evidence, completeness, and freshness. A consumer such as
CodeAtlas Assistant may explain or reason over that evidence, but it must not
become a second repository-analysis engine or turn unsupported output into
graph facts.

## Why

AI without bounded, verifiable context hallucinates. A proven graph lets AI
consumers spend their budget on engineering reasoning instead of reconstructing
repository architecture. Keeping the boundary explicit preserves replaceable
consumers and keeps the graph as the source of truth.
