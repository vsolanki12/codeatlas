# Evidence evaluation — 2026-10-01

This report evaluates the local changes implementing the 2026-09-29 audit.
It measures deterministic retrieval and prompt representation, without a live
LLM answer-quality comparison. The inputs were HyperShift at `c28a4f2`, including
its current checkout file state, schema 1.6.0, evidence version 1.0, and Go 1.26.4
on darwin/arm64. Fixtures use revision-specific source coordinates.

## Correctness before size

All six fixtures pass with the default 16,384-byte evidence budget and
24,000-byte source budget. They cover PascalCase, lowerCamelCase, exact IDs,
ambiguous methods, unknown concepts, and unknown exact IDs. The successful
AutoRepair cases require API GoDoc, the behavior branch, the full health-check
helper at `capi.go:726-827`, positive test setup, and its assertion at
`capi_test.go:1940-1944`.

| Manifest budget (bytes) | Result | Complete prompt (bytes) | Estimated input tokens | Required evidence |
|---|---|---|---|---|
| 4096 | fail | 4317 | 1079 | API only; implementation and tests missing |
| 8192 | fail | 15846 | 3961 | Implementation helper missing |
| 16384 | pass | 23269 | 5817 | Required API, full helper, and positive test evidence retained |
| 32768 | fail | 29677 | 7419 | Full helper excerpt shortened by fixed source budget |

Increasing the manifest budget while keeping the source budget fixed can
crowd out a required body excerpt. Tune both budgets against correctness
fixtures; larger packets do not guarantee more useful retained source.

Estimates use the existing character-based metric, not a model tokenizer.
The default case retains 372 unique source lines across four files and makes
one evidence query per task. Preflight verification calls are excluded from
that task count. Unique lines count retained excerpts, not physical file I/O;
the materializer caches each selected file once.

## Prompt representation comparison

For the same selected evidence, retained source, and question instructions:

- Raw JSON manifest prompt: **34,686 bytes**.
- Deterministic entity legend and proof prompt: **23,269 bytes**.
- Representation reduction: **32.9%**.

Exact entity IDs are listed once. Relationships retain their confidence,
source coordinates, snippet, and reason. Source excerpts refer to that legend
and keep their actual retained spans. Ranking scores and repeated JSON keys
are omitted from the model-facing representation. The original manifest
remains available to tooling.

This comparison does not establish billed-token savings, model answer quality,
or a reduction against a competent source-search baseline. No actual model
usage is reported from the localhost generation mock.

## Checks and limits

Both projects passed race tests, static checks, builds, formatting checks, and
focused adversarial regressions. A native Assistant question used one mocked
generation and validated its output reference; a no-match question exited
nonzero without contacting the model. Local base/head review retained a deleted
entity and relationship, and rejected a mismatched merge-base graph.

Both configured repository graphs pass current schema, file-state, and exact
executable provenance verification. Type enrichment remains partial:
HyperShift reports 47 fully checked package variants out of 683. External
imports use host compiler export data, and unresolved calls remain reported.
Parser completeness does not imply complete typed or runtime coverage.

See [machine-readable measurements](evidence-evaluation.json), the Assistant
fixture at `benchmarks/hypershift-autorepair.json`, and
[the shared workflow](evidence-workflow.md). Restart MCP consumers after replacing
their binary or graph; a running server keeps its loaded snapshot.
