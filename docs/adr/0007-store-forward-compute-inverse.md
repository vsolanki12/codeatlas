# ADR-0007: Store Forward Edges, Compute Inverses

**Status:** Accepted
**Date:** 2026-07-14

## Decision

`calls` is stored; `called_by` is computed at load time. Same for `imports`/`imported_by`, `owns`/`owned_by`, `tested_by`/`tests`.

## Why

Storing both directions creates an inconsistency risk. If `A calls B` is stored but `B called_by A` is forgotten, the graph contradicts itself. One direction is the source of truth; the inverse is derived.
