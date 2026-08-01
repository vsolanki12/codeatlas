# Vision

## Why Atlas Exists

Large codebases are hostile to newcomers. A project like HyperShift has 11,000+ entities across hundreds of packages — controllers, CRDs, functions, tests, resources — connected by relationships that only exist in the heads of people who've been there for years.

A new engineer asks "how does HostedCluster reconciliation work?" and gets pointed to a directory with 40 files. They read for hours. They build a partial mental model. They miss connections. They ask a teammate, who explains from memory. That teammate leaves. The knowledge leaves with them.

Atlas exists because architecture should be extracted from code, not reconstructed from memory. Every extracted relationship is backed by evidence from the source code, making the graph explainable rather than inferred.

## Who Atlas Is For

**Engineers joining a large Go project.** Instead of weeks of reading source files and asking teammates, they get a structured map of every entity and relationship on day one.

**Engineers reviewing PRs.** Instead of guessing what a change might break, they get blast radius analysis — every controller, test, resource, and file affected — in one query.

**Engineers debugging production issues.** Instead of grepping through thousands of files, they trace call chains, find test coverage gaps, and identify owners in seconds.

**AI development assistants.** Instead of reading source files to answer architecture questions, they query a graph that already has the answer. Faster, cheaper, more accurate.

Atlas is not for managers, PMs, or executives. It's an engineering tool that speaks in entities, relationships, and evidence.

## What Problems Atlas Solves

**Knowledge lives in people's heads.** Atlas extracts it into a graph that survives turnover, reorgs, and vacations. The architecture is always available, always current, always queryable.

**Reading code doesn't scale.** A human can hold maybe 5-7 files in working memory. Atlas holds 11,000 entities and 8,000+ relationships in a queryable index. It answers "what calls this function" in milliseconds, not minutes.

**AI tools waste tokens on the wrong things.** Without Atlas, an AI assistant reads source files to understand architecture — slow, expensive, often incomplete. With Atlas, it queries a pre-built graph. Same answer, fraction of the cost.

**PR reviews miss blast radius.** Reviewers check the changed files but rarely trace upstream callers, affected controllers, or missing test coverage. Atlas does this automatically.

## Core Principles

- **Extract, never invent.** If a parser can't determine a fact, Atlas doesn't guess.
- **Evidence for every relationship.** Every edge carries a file, line, snippet, and reason.
- **Deterministic before intelligent.** Same commit, same graph, every time.
- **Graph is the product.** Everything else — viewers, MCP, CLI — consumes it.
- **Consumers are replaceable.** Adding or removing a consumer never changes the graph.

## Where Atlas Will Be in 2 Years

**The graph runs in CI.** Every PR triggers a scan. The diff between the old graph and the new graph shows exactly what changed architecturally — not just which lines moved, but which relationships were created, broken, or modified.

**Multiple repositories.** HyperShift was the first repository Atlas targeted, but the architecture is intentionally repository-agnostic. Over time, Atlas will support any large Go project with controllers, CRDs, and complex call graphs using the same pipeline.

**AI consumer ecosystem.** Claude Code is the first AI consumer. VS Code, Cursor, Continue.dev, Copilot Chat — any MCP-compatible client can query Atlas without reading source files. The four-layer architecture (Knowledge, Retrieval, Guidance, Experience) means adding a new consumer never changes the graph or the query engine.

**Continuous architecture intelligence.** The graph enables deterministic analyses that don't require AI: architecture diff across branches, dependency drift detection, orphan detection, dead code identification, missing test coverage, architecture regression. These are graph operations, not LLM tasks.

**The standard way to onboard.** "Read the Atlas graph" replaces "ask someone who's been here a while." New engineers get productive in days, not weeks. The graph becomes living architecture documentation, generated directly from source code instead of maintained by hand.

Atlas is not an AI framework. Atlas provides deterministic architectural knowledge that AI assistants can consume. As AI models evolve, Atlas remains stable because the graph — not the model — is the source of truth.

Atlas won't become smarter. It will become more complete. More entities extracted, more relationships proven, more consumers served — all grounded in the same principle: extract what exists, never invent what doesn't.
