# HyperShift Atlas

**A code intelligence platform that automatically discovers, maps, and explains the HyperShift architecture — directly from source code, not from assumptions.**

HyperShift Atlas parses Go source, Kubernetes manifests, documentation, and tests to build the Atlas Graph — a structured map of every entity, relationship, and workflow in the HyperShift project. It answers questions like *what creates this?*, *which tests cover this?*, *what breaks if I change this?* — deterministically, without AI, backed by evidence from the source code itself.

---

## Problem

HyperShift is a large, fast-moving codebase. Understanding it today requires:

- **Scattered knowledge** — Architecture lives across Go files, CRDs, markdown docs, design proposals, and tribal knowledge. No single place connects them.
- **Hidden relationships** — Changing one controller can silently affect others. These dependencies exist in code but aren't visible until something breaks.
- **Onboarding friction** — New engineers spend weeks building a mental model that could be generated in seconds from the source code itself.
- **Documentation drift** — Written docs fall out of sync with implementation. The code is always the truth, but reading raw code doesn't show the big picture.

Atlas solves this by treating **source code as the single source of truth** and generating architecture from it — never inventing, always extracting.

---

## Core Principle

> **Atlas never invents architecture. It extracts it.**

Every node, edge, and relationship in the graph is backed by a parseable source:

| Source | Confidence | What It Provides |
|---|---|---|
| Go source code | Highest | Controllers, reconcilers, function calls, packages, imports |
| Kubernetes resources | Highest | CRDs, Deployments, Services, ConfigMaps, ownership |
| Go AST (call graphs) | Highest | Function call chains, method receivers, interface implementations |
| Documentation | High | Purpose, design decisions, architecture intent |
| Tests | High | Behavioral verification, coverage, tested scenarios |
| Go comments | Medium | Summaries, context, intent behind specific functions |
| Git history | Medium | Timeline, authorship, evolution of components |

If Atlas can't trace an answer back to a source, it doesn't show it.

---

## Who Is Atlas For

- **New HyperShift engineers** — Build a mental model of the system in minutes, not weeks.
- **Existing engineers exploring unfamiliar areas** — Click into a component you've never touched and understand its relationships before reading a single line of code.
- **Contributors preparing changes** — See what a component depends on and what depends on it before writing a PR.
- **Code reviewers** — Understand the blast radius of a change without manually tracing function calls.
- **Anyone explaining HyperShift** — Generate accurate architecture views for presentations, onboarding docs, or design discussions.

---

## What Atlas Does (Version 1)

Version 1 is deliberately small. It proves the core idea works before adding UI complexity.

A user can:

1. **Run `atlas scan`** — Point it at a HyperShift repository checkout. It produces `atlas-graph.json`.
2. **Open a simple HTML page** — It reads the JSON and renders a list of discovered components.
3. **Click any component** — See its package, functions, documentation, and tests on a single page.
4. **See relationships** — Each component shows what it creates, owns, watches, and depends on — with a link to the source file and line that proves it.

That's it. No React. No graph visualization. No search. No API server.

The three products are:

| Product | What It Is | Technology |
|---|---|---|
| **Scanner** | `atlas scan` — parses the repository | Go binary |
| **Atlas Graph** | `atlas-graph.json` — the extracted architecture | JSON file |
| **Viewer** | Simple HTML page that reads the JSON | Static HTML + vanilla JS |

If V1 works, V2 adds interactive graphs, search, and a proper React frontend. But V1 must ship first.

---

## What Atlas Does Not Do (Version 1)

Drawing boundaries is more important than drawing features.

- **Not a Kubernetes dashboard** — Atlas reads source code, not live clusters.
- **Not an AI chatbot** — No LLM, no Ollama, no RAG. The graph speaks for itself.
- **Not a PR review bot** — No GitHub integration in V1.
- **Not a code editor or IDE** — Atlas helps you understand code, not write it.
- **Not a cluster management tool** — No deployments, no kubectl, no live state.
- **Not a documentation generator** — Atlas links to existing docs; it doesn't write new ones.
- **Not a React app yet** — V1 is a static HTML viewer. React comes in V2.
- **Not a graph database** — V1 is JSON. Neo4j comes only if JSON can't keep up.

These are future possibilities, not V1 features.

---

## Architecture

```
HyperShift Repository
        │
        ▼
┌─────────────────────────────────────┐
│           Atlas Scanner              │  ← Go binary
│                                      │
│  ├── Go Parser      functions, controllers, call graphs
│  ├── YAML Parser    CRDs, Deployments, Services
│  ├── Doc Parser     markdown, design docs
│  ├── Test Parser    test files, tested components
│  └── Git Parser     (future) history, authorship
│                                      │
└──────────────┬──────────────────────┘
               │
               ▼
┌─────────────────────────────────────┐
│      Atlas Graph (JSON)              │  ← The product
│                                      │
│  Entities: controllers, CRDs,        │
│    functions, packages, tests, docs  │
│  Relationships: creates, owns, calls,│
│    tested_by, documented_in,         │
│    watches, depends_on               │
│                                      │
└──┬──────┬──────┬──────┬──────┬─────┘
   │      │      │      │      │
   ▼      ▼      ▼      ▼      ▼      (all future except Viewer)
 Viewer  Web    CLI    API   AI Layer
 (V1)   (V2)         (V3)   (Future)
```

Three rules govern this architecture:

1. **The scanner is the only thing that parses Go.** Everything else reads the graph.
2. **The graph is the product.** Every consumer — viewer, web app, CLI, API, AI — reads the same JSON. Improve the graph, and every consumer improves automatically.
3. **Consumers are interchangeable.** The V1 viewer is a static HTML page. V2 replaces it with React. The graph doesn't change.

---

## Data Model

Everything in Atlas is an **Entity**. An entity's `kind` determines what it represents. There is no separate Component vs. Controller — a controller is just an entity with `kind: controller`. One object per thing, no duplication.

```
Entity
├── id                      (deterministic, prefixed by kind)
├── name                    (e.g., "HostedClusterReconciler")
├── kind                    (operator | controller | crd | function | package | test | document | resource)
├── description             (extracted from GoDoc or docs — never written by Atlas)
├── source                  (parser + file + line that discovered this entity)
└── ...kind-specific fields (reconciles, watches, calls, group, version, etc.)
```

Relationships are first-class objects with **evidence** — not just "A relates to B" but *what code proves it*.

```
Relationship
├── from                    (source entity ID)
├── to                      (target entity ID)
├── type                    (creates | owns | calls | watches | tested_by | ...)
├── confidence              (proven | inferred)
└── evidence
    ├── file + line         (where it was found)
    ├── snippet             (the actual code, 1-2 lines)
    └── reason              (human-readable explanation)
```

Example: clicking the `creates` edge between HostedCluster and HostedControlPlane shows:

```
Evidence:  pkg/controllers/hostedcluster/hostedcluster_controller.go:213
Snippet:   controllerutil.CreateOrUpdate(ctx, r.Client, hcp, func() error {
Reason:    HostedClusterReconciler calls CreateOrUpdate to create a HostedControlPlane resource
```

Inverse relationships (`called_by`, `imported_by`, `owned_by`) are **not stored** — they are computed at load time from the forward edge. One direction is truth.

Full specification: [docs/data-model.md](docs/data-model.md)

---

## Project Structure

```
hypershift-atlas/
│
├── README.md
├── LICENSE
├── .gitignore
│
├── cmd/
│   └── atlas/              CLI entry point (atlas scan, atlas query, atlas diff, atlas serve)
│
├── internal/
│   ├── domain/             The vocabulary of Atlas (Repository, File, Entity, Relationship, Graph)
│   ├── discovery/          Walks the repository, returns []domain.File
│   ├── parser/             Parses files into []domain.Entity
│   ├── graph/              Builds []domain.Relationship between entities
│   ├── scanner/            Orchestrates discovery → parsing → graph → output
│   └── storage/            Writes and reads domain.Graph as JSON
│
├── docs/
│   ├── architecture.md     Architecture overview and package map
│   ├── data-model.md       Data model contract (entity and relationship schemas)
│   ├── roadmap.md          Phase-by-phase development plan
│   └── adr/                Architecture Decision Records
│       ├── 0001-scanner-and-viewer-separation.md
│       ├── ...
│       └── 0009-deterministic-over-intelligent.md
│
├── examples/               Example atlas-graph.json outputs
│
├── testdata/               Test fixtures for parsers
│
├── web/                    Frontend (empty until Phase 2)
│
└── assets/
    ├── logo/
    ├── screenshots/
    └── diagrams/
```

---

## Roadmap

Atlas develops in **capabilities**, not sprints. Each one builds on the previous and unlocks the next.

| # | Capability | What It Unlocks | Timeline |
|---|---|---|---|
| 1 | **Repository Discovery** | Atlas can walk a repo, return file metadata, and route files to parsers | Week 1 |
| 2 | **Go Parsing** | Atlas can extract controllers, functions, packages from Go | Weeks 1–3 |
| 3 | **Relationship Extraction** | Atlas can connect entities with typed, evidenced edges | Weeks 3–4 |
| 4 | **Atlas Graph** | `atlas scan` produces a complete, valid `atlas-graph.json` | Week 4 |
| 5 | **Visualization** | Users can explore the graph (HTML viewer → React → call graphs) | Weeks 5–14 |
| 6 | **AI** | Natural language questions answered from the graph | Future |

Each capability is independently testable. Capability 4 is the milestone — once the graph exists, everything else consumes it.

Full roadmap with week-by-week breakdown: [docs/roadmap.md](docs/roadmap.md)

---

## Additional Capabilities to Consider

These aren't committed — they're ideas worth evaluating as the project matures:

- **Diff mode** — Compare two versions of the graph side by side to see what changed between releases or branches.
- **Shareable URLs** — Deep link to any component, function, or graph view so engineers can share specific views in Slack or PR comments.
- **Freshness indicators** — Show when a component was last modified, how active it is, and whether its docs are current.
- **Complexity heatmap** — Highlight components with the most dependencies, largest files, or deepest call chains — potential risk areas.
- **Onboarding paths** — Curated "start here" paths for new engineers: "Understand HostedCluster in 5 clicks."
- **Multi-repo support** — Eventually parse related repositories (e.g., cluster-api, machine-api) to show cross-project dependencies.
- **Embed mode** — Render Atlas graphs inside Confluence pages or GitHub READMEs as static images.

---

## Success Criteria

In 90 days:

> Click **HostedCluster** and immediately understand how it works — what it creates, what manages it, what tests validate it, and what documentation describes it — without opening a single file in the repository.

---

## Design Principles

These guide every decision — from data model design to what features to build next.

1. **Code is the source of truth.** When documentation and code disagree, code wins. Always.
2. **Every relationship is traceable.** No edge in the graph exists without a file and line number proving it. If you can't point to the source, it doesn't go in the graph.
3. **Generated over manually maintained.** Run `atlas scan` again and the graph updates. No human curation required. If a fact can't be auto-extracted, it's a parser improvement, not a manual entry.
4. **Understand before visualize.** The data model matters more than the UI. A correct graph with an ugly viewer beats a beautiful viewer over a broken graph.
5. **Components are first-class citizens.** Everything in Atlas is anchored to a component. Functions, tests, docs, and CRDs exist in relation to the component they belong to.
6. **AI never replaces extraction.** AI may explain the graph. AI may answer questions about the graph. AI never populates the graph. The graph is deterministic and reproducible.
7. **Confidence is visible.** Every fact shows its source and confidence level. Users can always verify what Atlas claims by following the link to the original code or document.
8. **The graph is the product.** The website, CLI, API, and AI layer are all consumers. If you improve the graph, every consumer improves automatically.
