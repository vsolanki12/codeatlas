# ADR-0002: JSON Before Neo4j

**Status:** Accepted
**Date:** 2026-07-14

## Decision

The Atlas Graph starts as a JSON file, not a graph database.

## Why

JSON is zero-dependency, version-controllable, and debuggable. Neo4j adds operational complexity that isn't justified until the graph is too large or the queries are too complex for in-memory traversal.

## When to Reconsider

When the graph exceeds ~10MB or when query patterns require multi-hop traversals that are slow in JSON.
