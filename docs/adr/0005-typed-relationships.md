# ADR-0005: Typed Relationships Over Generic Edges

**Status:** Accepted
**Date:** 2026-07-14

## Decision

Edges have explicit types (creates, owns, calls, watches) rather than generic "relates to."

## Why

Typed edges answer specific questions. "What does CPO create?" is a filtered traversal on `creates` edges. Generic edges would require reading every relationship to understand what it means.
