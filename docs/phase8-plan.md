# Phase 8: Intent-Based Tool Guidance

**Status:** Sketch 2026-08-01

## Architecture Overview

```
HyperShift Repository
          │
          ▼
     Atlas Scanner
          │
          ▼
      Atlas Graph
          │
          ▼
     Query Engine
          │
          ▼
      MCP Server
          │
          ▼
  Claude / VS Code / Cursor / Any MCP Client
          │
          ▼
      Engineering Tasks
```

## Context

Atlas has 15 MCP tools across four layers:

| Layer | What | Purpose |
|-------|------|---------|
| **Knowledge** | Scanner → Graph | Extract architecture from code |
| **Retrieval** | 15 MCP tools | Answer questions about the graph |
| **Guidance** | Tool descriptions | Teach consumers which engineering intent a tool serves and what the natural next query is |
| **Experience** | Claude Code, VS Code, Cursor, any MCP client | Where engineers interact with Atlas |

Atlas itself doesn't decide anything. The consumer does. Today that's Claude. Tomorrow it could be VS Code, Cursor, Windsurf, Continue.dev, or any MCP-compatible client. The four-layer model makes this future-proof.

Layers 1 and 2 are built. Layer 3 is Phase 8. Layer 4 is the consumer — already exists (Claude Code + CLI), grows organically.

The gap isn't capability — it's intent recognition. When a user asks "why is reconciliation failing?", Claude shouldn't think "which tool do I call?" It should think "the engineer wants to **investigate** — `atlas_investigate` is the right entry point."

## Philosophy

> Engineers don't think in tools — they think in tasks. Atlas should recognize the task first, then choose the smallest set of graph queries needed to answer it.

## Intent Flow

```
Engineer Question
        │
        ▼
Recognize Intent
        │
        ▼
Choose Entry Tool
        │
        ▼
Retrieve Graph Evidence
        │
        ▼
Answer
```

## Intent Model

Six engineering intents, each mapping to a preferred tool:

### Intent 1: Understand

**What the engineer is doing:** Learning a subsystem, onboarding, understanding architecture.

**Trigger phrases:** "explain", "how does X work", "what creates", "walk me through", "I'm new to"

**Preferred tool:** `atlas_explain`

**Usually followed by:** `atlas_investigate` (to deep-dive a specific entity found in the tree)

### Intent 2: Investigate

**What the engineer is doing:** Debugging, root-cause analysis, understanding why something is broken.

**Trigger phrases:** "why is", "what's happening with", "investigate", "debug", "broken", "failing"

**Preferred tool:** `atlas_investigate`

**Usually followed by:** `atlas_explain` (to trace the reconciliation chain), `atlas_impact` (to see blast radius)

### Intent 3: Review

**What the engineer is doing:** PR review, change impact assessment, risk analysis.

**Trigger phrases:** "review", "blast radius", "what does this affect", "will this break", "who owns"

**Preferred tool:** `atlas_impact`

**Usually followed by:** `atlas_callers` (to verify caller test coverage), `atlas_commits` (to check change history)

### Intent 4: Navigate

**What the engineer is doing:** Finding where to make a change, locating code, exploring structure.

**Trigger phrases:** "where is", "find", "which file", "where should I add", "locate"

**Preferred tools:** `atlas_search`, `atlas_where`

**Usually followed by:** `atlas_investigate` (to understand each candidate)

### Intent 5: Test

**What the engineer is doing:** Checking test coverage, finding gaps, verifying safety.

**Trigger phrases:** "is this tested", "coverage", "untested", "missing tests", "test gap"

**Preferred tools:** `atlas_impact` (Tests section), `atlas_hotspots`

**Usually followed by:** `atlas_investigate` (to check tested_by edges on specific entities)

### Intent 6: Implement

**What the engineer is doing:** Feature development, finding where to add code, locating extension points.

**Trigger phrases:** "where should I add", "which controller owns", "best extension point", "where does this validation go", "implement"

**Preferred tools:** `atlas_search` → `atlas_investigate` → `atlas_where` → `atlas_explain`

**Usually followed by:** Reading the actual source file at the identified location.

**Note:** This is ~30-40% of an engineer's day. The flow is: find candidates (search), understand each candidate (investigate), check co-located code (where), verify architectural context (explain).

## Why Intents, Not Scenarios

| Scenarios | Intents |
|-----------|---------|
| Prescriptive: "follow this path" | Declarative: "this is what you want" |
| HyperShift-specific | Engineering-universal |
| Break when questions don't fit | Compose naturally |
| Claude follows a script | Claude recognizes a task |

## Implementation: Enriched Tool Descriptions

Each tool description gets three additions:

1. **Best used when:** — maps intent to tool
2. **Examples:** — concrete questions this tool answers
3. **Usually followed by:** — teaches natural tool chains

### Proposed Descriptions

```
atlas_investigate
  "Get everything about a HyperShift entity in one call: full details,
  all relationships grouped by type, callers, tests, and same-file siblings.
  Replaces 4-5 primitive tool calls.

  Best used when: Investigating a bug, debugging, understanding why something
  is broken. The default starting point for any "why is X..." question.

  Examples: Why is reconciliation failing? What calls this function?
  What tests cover this?

  Usually followed by: atlas_explain (to trace architecture),
  atlas_impact (to assess blast radius)"
```

```
atlas_explain
  "Follow the reconciliation chain from a HyperShift entity: reconciles,
  creates, calls, tested_by. Returns a tree showing the architectural narrative.

  Best used when: Learning a subsystem, onboarding, understanding how
  components connect. The default starting point for any "how does X work..."
  question.

  Examples: Explain HostedCluster. How does CPO work? What creates etcd?

  Usually followed by: atlas_investigate (to deep-dive a specific entity
  found in the tree)"
```

```
atlas_impact
  "Blast radius analysis: walk the call chain upstream to find all controllers,
  tests, resources, files, and owners affected by changing this entity.

  Best used when: Reviewing a PR, assessing change risk, preparing for code
  review. The default starting point for any "will this break..." question.

  Examples: Review this PR. What does this change affect? Who owns this code?

  Usually followed by: atlas_callers (to verify caller test coverage),
  atlas_commits (to check change history)"
```

```
atlas_search
  "Search all HyperShift entities by text. Matches across name, description,
  package, ID, imports, literals, and properties.

  Best used when: Finding where something is implemented, locating code by
  concept or name. The default starting point for any "where is X..." question.

  Usually followed by: atlas_investigate (to understand each candidate)"
```

```
atlas_where
  "Find HyperShift entities by file path. Returns entities defined in files
  matching the path substring.

  Best used when: Exploring a specific file or directory, understanding what
  lives in a location.

  Usually followed by: atlas_investigate (to deep-dive a specific entity)"
```

```
atlas_hotspots
  "Find most-changed or stalest entities by git history.

  Best used when: Identifying test gaps (high-change + no tests = risk),
  finding code that needs attention, prioritizing review effort.

  Usually followed by: atlas_impact (to check test coverage for each hotspot)"
```

Primitive tools (`atlas_entity`, `atlas_entities`, `atlas_relationships`, `atlas_context`, `atlas_callers`, `atlas_commits`, `atlas_lookup`, `atlas_stats`) keep their current descriptions. They're building blocks — Claude reaches for them when compound tools don't fit.

## What This Teaches Claude

Traditional MCP:
```
User → Choose Tool → Run Tool
```

Atlas with intent guidance:
```
User → Recognize engineering task → Select best entry tool → Follow "usually followed by" chain → Done
```

The key insight: "Usually followed by" creates implicit workflows WITHOUT code. Claude learns the chains from descriptions alone.

## Implementation Cost

- Zero new code
- ~30 minutes of description editing in `server.go`
- Immediate effect on next MCP server restart

## Success Criteria

| Goal | Target |
|------|--------|
| Correct first tool | ≥80% |
| Average Atlas calls | ≤3 |
| Source file reads | 0 for architecture questions |
| New code | 0 |

Test with real engineering questions after applying. If first-tool accuracy below 80%, refine descriptions — don't add code.
