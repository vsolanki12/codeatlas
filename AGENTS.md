# AGENTS.md

This file provides guidance to AI coding agents when working with code in this repository. `CLAUDE.md` is a symlink to this file so that Claude Code auto-loads it; the `AGENTS.md` name is canonical.

CodeAtlas is a deterministic architecture reasoning engine for Go repositories. It parses Go AST, YAML, Markdown, and tests into a typed, evidence-backed graph, then serves that graph to AI assistants via 11 MCP tools and a CLI.

This file is intentionally minimal — detailed guidance lives in the referenced files below and should be updated there, not here.

For product, code, and pipeline architecture, see [docs/architecture.md](docs/architecture.md).

## Key References

| Topic | Where to look |
|-------|---------------|
| **Product overview and quick start** | [README.md](README.md) |
| **Architecture (product, code, pipeline)** | [docs/architecture.md](docs/architecture.md) |
| **Data model (entity/relationship schema)** | [docs/data-model.md](docs/data-model.md) — the contract between the scanner and all consumers |
| **Vision and principles** | [docs/vision.md](docs/vision.md) |
| **Roadmap (phases 1–14 done, 15–18 planned)** | [docs/roadmap.md](docs/roadmap.md) |
| **Architecture Decision Records (15 ADRs)** | [docs/adr/](docs/adr/) |
| **Repo/binary/graph freshness checks** | [.claude/skills/check-repos/SKILL.md](.claude/skills/check-repos/SKILL.md) |

## Development Workflow

```bash
go build -o atlas ./cmd/atlas                          # Build
go test ./...                                          # Run all tests
atlas scan -repo /path/to/repo -output atlas-graph.json  # Scan a repo
atlas serve -graph atlas-graph.json                     # Start MCP server
```

There is no `Makefile`, CI, or Docker configuration — the project builds with standard Go tooling.

## Code Architecture

The codebase follows a strict dependency hierarchy. Violating these rules breaks the architecture:

- **`internal/domain` depends on nothing.** It defines the shared vocabulary (`Entity`, `Relationship`, `Evidence`, `Graph`, `Source`). Every other package imports it. If `domain` ever imports another CodeAtlas package, the architecture is broken.
- **Leaf packages are independent.** `discovery`, `parser`, `graph`, `storage`, `origin`, and `temporal` do not depend on each other. Only `scanner` composes them.
- **`query` depends only on `domain` and `storage`.** It loads the graph and builds an in-memory index. No dependency on scanner or parsers.
- **`mcpserver` depends only on `query`.** It is a thin MCP wrapper over the query engine.

| Package | Responsibility |
|---------|----------------|
| `internal/domain` | Core types: `Entity`, `Relationship`, `Evidence`, `Graph`, `Source` |
| `cmd/atlas` | CLI entry point: scan, search, explain, impact, investigate, ask, view, context, where, stats, serve, query, review |
| `internal/scanner` | Orchestrates the full scan pipeline with merge-aware dedup |
| `internal/discovery` | Walks the repository, returns files with metadata |
| `internal/parser` | Go AST, YAML, Markdown, Test parsers — produces entities |
| `internal/graph` | Builds typed, evidenced relationships between entities |
| `internal/origin` | Import path classifier (stdlib vs known repos vs external) |
| `internal/temporal` | Optional git history enrichment (LastAuthor, LastModified, ChangeCount) |
| `internal/views` | Pre-computed controller/CRD knowledge views and question index |
| `internal/storage` | JSON graph read/write |
| `internal/query` | In-memory index, search, explain, impact, investigate, ask |
| `internal/review` | PR diff parsing, entity-to-hunk mapping, blast radius, test coverage |
| `internal/mcpserver` | 11 MCP tools served via stdio transport |

## Graph Schema Invariants

The Atlas Graph JSON (schema 1.4.0) is the product. All consumers read the same file. Key rules:

1. **No entity without a source.** If the scanner can't point to a file and line, the entity is not created.
2. **No relationship without evidence.** Every edge carries `evidence` (parser, file, line, snippet, reason).
3. **No manual entries.** If a fact must be added by hand, that's a missing parser — not a data entry task.
4. **IDs are deterministic.** Same commit produces the same graph. No UUIDs, no timestamps in IDs.
5. **Store forward, compute inverse.** `calls` is stored; `called_by` is computed at load time. Same for `imports`/`imported_by`, `contains`/`contained_in`, etc.

See [docs/data-model.md](docs/data-model.md) for the full schema specification.

## Core Design Principles

- **Extract, never invent.** If a parser can't determine a fact, CodeAtlas doesn't guess.
- **Deterministic before intelligent.** Same commit, same graph, every time.
- **Graph is the product.** Everything else — CLI, MCP, future API — consumes it.
- **Consumers are replaceable.** Adding or removing a consumer never changes the graph format.
- **Evidence on every relationship.** Users can always ask "why does CodeAtlas think X?" and get proof.

## Companion Project

[codeatlas-assistant](https://github.com/vsolanki12/codeatlas-assistant) is a separate CLI that sits on top of CodeAtlas + a local Ollama LLM. It provides natural-language question answering, JIRA analysis, code generation, and Claude prompt distillation — all grounded in the Atlas graph. Changes to the graph schema, MCP tools, or CLI output format in this repo may affect the assistant.

## Testing Conventions

- Tests are colocated with packages (`*_test.go` in the same directory)
- Standard Go `testing` package only — no external test frameworks
- Table-driven tests where appropriate
- Parser and graph fixtures live under `internal/parser/testdata/` and `internal/graph/testdata/`
- Run all tests with `go test ./...`
