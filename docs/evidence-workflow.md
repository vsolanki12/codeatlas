# Using CodeAtlas and Assistant together

CodeAtlas selects source-backed repository evidence. Assistant materializes
the selected source spans and passes a bounded working set to a model. The
selection is shared with direct MCP consumers; it does not depend on Ollama.

## Prepare a repository

Build both projects, then scan and verify the checkout you will work on:

```sh
go build -o atlas ./cmd/atlas
atlas version
atlas scan --repo /path/to/repository --output /tmp/repository-atlas.json
atlas verify --repo /path/to/repository --graph /tmp/repository-atlas.json --json
```

Build Assistant with `go build -o assistant ./cmd/assistant` in its repository.
Pass the matching Atlas executable explicitly with `--atlas-bin` when several
versions are installed. Each repository needs its own graph. A graph describes
supported Go, YAML, Markdown, and test evidence; ignored files and unresolved
type information remain explicit limitations.

Schema 1.6.0 records the extractor version, executable SHA256, extraction signature, Go build
context, and type-analysis coverage. A scanner upgrade or a changed build
context invalidates incompatible incremental facts. Verification also checks
that the graph was produced by the current Atlas executable. Use `--goos`, `--goarch`,
and comma-separated `--tags` to select a build context. Parser completeness and
type-enrichment coverage are separate: a successfully parsed repository can
still contain unresolved calls.

## Retrieve a scoped working set

```sh
atlas evidence --graph /tmp/repository-atlas.json \
  --question 'How does AutoRepair work for NodePool?' \
  --scope NodePool --intent understand --budget-bytes 16384 --json

assistant --atlas-bin /path/to/atlas --graph /tmp/repository-atlas.json \
  --repo /path/to/repository --evidence-budget-bytes 16384 \
  --source-budget-bytes 24000 'How does AutoRepair work for NodePool?'
```

The version 1.0 manifest contains lightweight entities once, evidenced
relationships once, ranked source selections, a graph fingerprint, and
explicit omissions. Source selections retain entity identity, role, location,
and selection reason. Focused source selections include `focusLine`, retaining
the observed site even when a consumer cannot include the whole span.
Ranking is deterministic routing evidence; it does not
prove behavior or test coverage. Exact entity IDs can be supplied with
`--entity` without shortening their repository-qualified identity.

`budgetBytes` bounds the compact serialized manifest. It is not a tokenizer or
a provider billing measurement, and excludes CLI newlines, MCP transport
envelopes, and source excerpts materialized by consumers. Assistant separately
bounds rendered excerpt blocks (including their headers); prompt instructions,
the evidence summary, and omission diagnostics are counted separately.
Assistant renders a deterministic entity legend and relationship proofs once
for the model, while retaining the original structured manifest for tooling.
The budget flags apply to question, solve, generate, and Claude workflows.
Missing, ambiguous, incompatible, or unusable evidence
must be resolved before invoking a model.

When a manifest has `nextOffset`, repeat the same question, entity, scope,
intent, and budget with `--offset` and the returned `--graph-fingerprint`.
A changed snapshot is rejected. Inspect `omissions` even when a result contains
useful evidence; bounded retrieval is not evidence of absence.

For MCP, use the existing `atlas_ask` with `question`, optional `scope`, and
`budget_bytes`. Set `evidence: true` for an exact `entity` manifest. The text
response is a short summary and structured content holds the evidence packet.
Clients must preserve structured content. Legacy entity reports remain
available when only `entity` is supplied. After replacing a graph or Atlas
binary, restart the MCP consumer: the server loads its graph at startup.

## Evaluate quality and cost

Assistant supports `--benchmark-fixtures tasks.json`, `--benchmark-budgets`,
and `--benchmark-json` without resolving or invoking a model. Use fixtures
with required entity IDs, source roles, and implementation evidence. Include
case variants, exact IDs, ambiguous names, unknown concepts, and small budgets.
A packet that misses required evidence fails even when it is smaller.

The Assistant includes `benchmarks/hypershift-autorepair.json` with API
documentation, the behavior branch, health-check configuration, and positive
test setup/assertion requirements. Run it from the Assistant checkout:

```sh
./assistant --atlas-bin /path/to/atlas --graph /tmp/hypershift-atlas.json \
  --repo /path/to/hypershift \
  --benchmark-fixtures benchmarks/hypershift-autorepair.json \
  --benchmark-budgets 4096,8192,16384,32768 --benchmark-json
```

Failures identify required evidence that was not selected or materialized.
Adjust manifest and source budgets together, then rerun the fixtures. A larger
manifest with a fixed source budget can crowd out a required source body.
Request a continuation when the manifest itself reports `nextOffset`.

Payload bytes and estimated tokens measure representation size. Actual model
input/output tokens, tool calls, duplicate source reads, follow-up exploration,
and answer correctness are needed to establish end-to-end savings. The legacy
full/compact/raw benchmark is a size diagnostic, not proof of equal answers.

See the [2026-10-01 evidence evaluation](evidence-evaluation.md) for a measured
correctness and prompt-size sweep on the audit's AutoRepair example.

## Review historical evidence

For verified local ref reviews, optionally supply a graph scanned at
`git merge-base <base> <head>`:

```sh
atlas review --repo /path/to/repository --base main --head HEAD \
  --graph /tmp/head-atlas.json --base-graph /tmp/base-atlas.json --json
```

The graphs require matching extraction signatures and the base graph commit
must match the review merge base. Review output retains entities and
relationships absent from the head graph for changed paths, with their base
source evidence. Historical checkout file state is not independently verified;
graph identity changes do not prove runtime impact. MCP exposes the same local
review option as `base_graph`.
