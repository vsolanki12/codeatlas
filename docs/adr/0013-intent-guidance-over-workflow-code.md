# ADR-0013: Intent Guidance Over Workflow Code

**Status:** Accepted
**Date:** 2026-08-01

## Decision

Teach AI consumers which tool to use via enriched tool descriptions ("Best used when", "Usually followed by") rather than building workflow orchestration code, scenario routers, or playbook engines.

## Why

The gap after Phase 7 wasn't capability — Atlas had 15 tools covering every query pattern. The gap was intent recognition: Claude didn't always pick the right entry tool on first try.

Two approaches were considered:

1. **Workflow code** — scenario router that maps user questions to tool sequences. Requires maintaining workflow definitions, handling edge cases, updating when tools change.
2. **Intent guidance** — enrich existing tool descriptions so Claude learns the patterns from descriptions alone. Zero new code, immediate effect on MCP restart.

Option 2 wins because:
- No code to maintain. Descriptions are the cheapest, most effective way to improve tool selection.
- "Usually followed by" creates implicit workflows WITHOUT code. Claude learns the chains from descriptions.
- Works for any MCP consumer, not just Claude Code.
- If it doesn't work (< 80% first-tool accuracy), the fix is better descriptions, not more code.

## Consequences

- Six engineering intents (Understand, Investigate, Review, Navigate, Test, Implement) map to preferred tools.
- Tool descriptions include "Best used when" and "Usually followed by" hints.
- No scenario router, no playbook engine, no workflow objects.
- Phase 8 implementation cost: zero new code, ~30 minutes of description editing.
