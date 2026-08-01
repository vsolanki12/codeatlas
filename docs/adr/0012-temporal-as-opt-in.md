# ADR-0012: Temporal Enrichment as Opt-In

**Status:** Accepted
**Date:** 2026-07-31

## Decision

Git history enrichment (LastAuthor, LastModified, ChangeCount) is opt-in via the `-temporal` flag. Without it, scans are fast and produce a valid graph. With it, scans take longer but entities gain change history.

## Why

Running `git log` and `git rev-list` per unique file adds significant scan time — from ~20s to ~3m15s on the HyperShift repo (11,056 entities). Most development workflows (quick iteration, testing parsers) don't need temporal data. Only hotspot analysis, ownership queries, and blast radius assessment benefit from it.

Making it opt-in keeps the default fast while allowing full enrichment when needed.

## Consequences

- `atlas scan -repo ~/hypershift -output graph.json` = fast scan, no temporal data.
- `atlas scan -repo ~/hypershift -output graph.json -temporal` = full scan with git history.
- `atlas_hotspots` and `atlas_commits` MCP tools return "no temporal data" if the graph was scanned without `-temporal`.
- `atlas_impact` gracefully degrades — shows "(no temporal data)" in Recent Changes and Owners sections.
- File-level cache inside `temporal.Enrich()` avoids redundant git calls when multiple entities share a file.
