# Data Model

This document is the contract between the scanner and everything that consumes the Atlas Graph. If an entity or field isn't defined here, it doesn't exist in CodeAtlas.

The graph is an evidence-bearing snapshot, not a claim that every repository
fact was discovered. `scanComplete: false` and `scanWarnings` mean the graph is
usable for the facts it contains but cannot support completeness claims.
`scanCoverage` and `scanFiles` make the supported-parser boundary explicit:
parsed files have parser-backed facts, ignored files have no registered parser,
and failed files must be treated as missing evidence. Coverage does not claim
that runtime behavior or every language/tooling file is modeled.
Consumers must preserve that distinction. Relationships that cannot be
resolved from a supported parser signal are omitted; relationships resolved by
convention or name matching are retained only as `confidence: inferred`.

Schema 1.6.0 also records `extractorVersion`, `extractorBuild`, `extractionSignature`,
`buildContext`, and `typeAnalysis`. The extraction signature identifies the
extractor contract, executable SHA256, and effective Go build settings; incompatible signatures
cannot reuse incremental parser facts. `buildContext` contains `goos`,
`goarch`, `buildTags`, `goVersion`, and `cgoEnabled`. `typeAnalysis` records
package counts (`packages`, `checkedPackages`, `failedPackages`), file counts
(`files`, `excludedFiles`), `resolvedCalls`, `unresolvedCalls`,
`resolvedReferences`, and package/variant diagnostics. This quality report is
independent of parse-level `scanComplete`.
It also records `dependencyMode`, `externalImports`, `externalImportFailures`,
`diagnosticCount`, and `diagnosticsTruncated`. External package imports currently
use host compiler export data; package diagnostics make that limitation visible.

A `field` entity has a deterministic `field:<package>.<struct>.<field>` ID,
declaration source span, and extracted documentation. The forward
`references` relationship links a function or test to a statically resolved
field use with source evidence. Unresolved selectors do not produce guessed
field edges. `referenceSites` retains every resolved occurrence as
`{target, source}`, where `target` is the exact field ID and `source` includes
the occurrence line and optional column. The stored forward edge retains one
proof; repeated occurrences are not lost during entity-to-field deduplication.

Each `ResourceOperation` contains `operation`, optional `method`, optional
`objectType`, `confidence`, and `source`. An empty object type reports failed
resolution. These observations remain available when the relationship builder
cannot select a unique scanned manifest entity. They do not assert a runtime
object identity or successful resource creation.
`observedMethod` and `observedObjectType` retain immutable AST observations so
incremental scans can reset and rerun type enrichment without carrying stale
resolved values. `Source.column` optionally distinguishes operations on one line.

---

## Core Concept: Everything is an Entity

CodeAtlas does not have separate types for Component, Controller, CRD, etc. It has one type: **Entity**.

An Entity's `kind` field determines what it represents. A controller is an Entity with `kind: controller`. A CRD is an Entity with `kind: crd`. This avoids duplication — HostedClusterReconciler is one object, not a Component and a Controller that must stay in sync.

Different kinds carry different optional fields. The scanner populates only the fields relevant to each kind. The viewer ignores fields that are absent.

---

## Entity

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique, deterministic identifier (see ID rules below) |
| `name` | string | yes | Display name extracted from the source |
| `kind` | enum | yes | Entity category (see Kind table) |
| `description` | string | no | GoDoc, Markdown-derived text, or manifest description |
| `package` | string | no | Repository-relative package identity or Go module import path |
| `files` | string[] | no | Additional files merged into the same entity |
| `watches` | string[] | no | Resource type names observed in `SetupWithManager` calls |
| `watchMethods` | string[] | no | Method aligned with each `watches` entry: `For`, `Owns`, or `Watches` |
| `watchSites` | Site[] | no | Source sites aligned with each `watches` entry |
| `creates` | string[] | no | Type names observed in explicit create/upsert calls |
| `createSites` | Site[] | no | Source sites for `creates` observations |
| `calls` | string[] | no | Call expressions observed in a function, test, or controller reconcile body |
| `callSites` | Site[] | no | Source sites for `calls` observations |
| `implements` | string[] | no | Interface names observed in compile-time assertions |
| `implementationSites` | Site[] | no | Source sites for `implements` observations |
| `env_vars` | string[] | no | Literal environment-variable names passed to `os.Getenv` |
| `imports` | string[] | no | Import paths observed in a Go package |
| `importSites` | Site[] | no | Source sites for `imports` observations |
| `literals` | string[] | no | Bounded literal values observed in Go function bodies |
| `properties` | string[] | no | Flattened, bounded YAML properties such as `kind=Deployment` |
| `embeds` | string[] | no | `//go:embed` patterns observed in a Go package |
| `embedSites` | Site[] | no | Source sites for `embeds` observations |
| `lastAuthor` | string | no | Optional git enrichment |
| `lastModified` | string | no | Optional git enrichment |
| `changeCount` | number | no | Optional git enrichment |
| `content` | string | no | Bounded Markdown content excerpt |
| `source` | Source | yes | Primary location where this entity was discovered |
| `generated` | boolean | no | Go source declares a generated-code header |
| `resourceOperations` | ResourceOperation[] | no | Source-backed create/upsert observations, retained when no manifest edge resolves |
| `referenceSites` | ReferenceSite[] | no | Every typed field selector/literal occurrence, with exact target ID and source coordinates |

The scanner populates only fields supported by the parser. Relationship-like
fields are observations, not edges. A site is not a relationship by itself;
the relationship builder must resolve the observation to an entity before it
emits a graph edge.

### Kind Table

| Kind | What It Represents | Current scanner signal |
|---|---|---|
| `controller` | A receiver with a `Reconcile` or `SetupWithManager` method | Go AST method declarations |
| `crd` | A Custom Resource Definition | Kubernetes CRD YAML with `spec.names.kind`, or supported CRD manifest shape |
| `function` | A Go function or method | `go/ast.FuncDecl` |
| `field` | A named Go struct field, including API configuration fields | Source-linked Go AST field declaration |
| `package` | A Go package in a scanned directory | Go AST package clause; files are merged by repository package identity |
| `test` | A top-level `Test*` function in a test file | Go AST function declarations in `_test.go` |
| `document` | A Markdown document | Markdown files discovered by the scanner |
| `resource` | A named Kubernetes object in YAML | YAML with `kind` and `metadata.name` |
| `template` | A Kubernetes manifest template whose runtime identity is unresolved | YAML/template syntax with a static `kind` but templated identity |
| `operator` | Reserved entity kind | Not emitted by the current parsers |

The current scanner does not emit separate component, interface, or operator
entities. An interface assertion is retained as an observation on the entity;
it is not converted into an `implements` edge without a modeled interface
target.

### Important kind details

- Controller `watches`, `watchMethods`, and `watchSites` are aligned by index.
  `For` is the deterministic source for a `reconciles` edge, `Owns` for an
  `owns` edge, and `Watches` for a `watches` edge. A controller found only from
  `SetupWithManager` can have no reconcile-body calls.
- The Go parser emits a separate controller entity for every receiver with a
  `Reconcile` or `SetupWithManager` method, including when multiple controllers
  share one source file. Facts from a setup method stay attached to that
  receiver.
- Function receiver names are encoded in the function ID; there is no separate
  `receiver`, `file`, `line`, `signature`, or `doc` field. Use `source` and the
  ID to locate the declaration.
- A CRD stores its group in `package` and uses a normalized, lower-case kind in
  its ID. Group/version/scope are not separate Entity fields in the current
  schema.
- A resource stores its namespace in `package` and its Kubernetes kind in the
  `properties` observation `kind=<Kind>`.
- A templated Kubernetes manifest is represented as a `template` entity when
  `metadata.name` cannot be known statically. Its file and line are evidence;
  the scanner does not invent a runtime object name or resource relationship.
- In repository scans, package, function, and test IDs use the Go module import
  path so equal short package names in different directories cannot collide.
  Fixture-only parser constructors may retain legacy short IDs.

### Entity examples

**Repository-mode controller:**

```json
{
  "id": "controller:example.com/project/controllers.WidgetReconciler",
  "name": "WidgetReconciler",
  "kind": "controller",
  "package": "example.com/project/controllers",
  "watches": ["Widget", "ConfigMap"],
  "watchMethods": ["For", "Owns"],
  "watchSites": [
    {"name": "Widget", "source": {"parser": "go-ast", "file": "controllers/widget_setup.go", "line": 24}},
    {"name": "ConfigMap", "source": {"parser": "go-ast", "file": "controllers/widget_setup.go", "line": 25}}
  ],
  "source": {
    "parser": "go",
    "file": "controllers/widget_controller.go",
    "line": 31,
    "endLine": 47
  }
}
```

**Function:**

```json
{
  "id": "function:example.com/project/controllers.WidgetReconciler.sync",
  "name": "sync",
  "kind": "function",
  "package": "example.com/project/controllers",
  "calls": ["helper"],
  "callSites": [
    {"name": "helper", "source": {"parser": "go-ast", "file": "controllers/widget_controller.go", "line": 42}}
  ],
  "source": {
    "parser": "go",
    "file": "controllers/widget_controller.go",
    "line": 38,
    "endLine": 45
  }
}
```

**CRD manifest:**

```json
{
  "id": "crd:example.io.widget",
  "name": "Widget",
  "kind": "crd",
  "package": "example.io",
  "source": {"parser": "yaml", "file": "config/crd.yaml", "line": 1}
}
```

---

## Relationships

Relationships are the edges of the Atlas Graph. They are first-class citizens — not just pointers between entities, but objects that carry evidence explaining *why* the relationship exists.

### Relationship Schema

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | `from--type--to` (deterministic) |
| `from` | string | yes | Source entity ID |
| `to` | string | yes | Target entity ID |
| `type` | enum | yes | One of the relationship types below |
| `confidence` | enum | yes | `proven` or `inferred` |
| `evidence` | Evidence | yes | What proves this relationship exists |

### Evidence

This is what makes CodeAtlas trustworthy. Every relationship carries proof — not just "where" it was found, but "what" was found.

| Field | Type | Required | Description |
|---|---|---|---|
| `parser` | enum | yes | Source of the evidence: `go`, `go-ast`, `test`, `yaml`, `markdown`, or `git` |
| `file` | string | yes | File path relative to repo root |
| `line` | number | yes | Positive source line number |
| `snippet` | string | no | The actual code or text that proves the relationship (1-2 lines max) |
| `reason` | string | yes | Human-readable explanation of why this relationship exists |

### Site

Sites attach exact source locations to repeated entity facts such as a call,
watch, or explicit create operation.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Extracted operation or target name |
| `source` | Source | yes | File and line where the operation was observed |

When a user clicks a relationship and asks "why does CodeAtlas think HostedCluster creates HostedControlPlane?" — the `evidence` answers it.

### Confidence Levels

| Level | Meaning | Example |
|---|---|---|
| `proven` | Directly observed with an exact supported semantic match | A `SetupWithManager.For(...)` registration, an exact package import, or an embed pattern matching a resource file |
| `inferred` | The source operation is observed, but target resolution relies on a convention or name match | A test call resolved within its package (invocation does not prove coverage), a test name matching a function, an AST call name matched within the caller package, or a create type matched to a unique manifest kind |

Two levels only. No percentages. An unresolved or ambiguous observation is
kept on the entity (when supported) and is not emitted as an edge. A
relationship target resolved by convention or name is explicitly `inferred`.

The scanner may upgrade a call edge to `proven` when the standard Go type
checker resolves its static target to an existing function entity. These edges
use `parser: go-types` and exact source evidence. Type checking does not prove
runtime dispatch, execution, or behavior; unresolved, external, ambiguous, or
dynamic targets are not guessed, while the original AST observation remains
`inferred` when it can be represented safely.

### Relationship Types

| Type | From → To | Meaning |
|---|---|---|
| `reconciles` | controller → crd/resource | This controller manages this resource |
| `creates` | controller/function → resource | An explicit create/upsert observation resolved to one manifest kind |
| `contains` | controller → function | Exact `Reconcile` or `SetupWithManager` method on the controller receiver |
| `owns` | controller → crd/resource | `SetupWithManager.Owns(...)` registers the target |
| `watches` | controller → crd/resource | Changes to this resource trigger reconciliation |
| `calls` | controller/function → function | A call observation resolved to a function entity |
| `tested_by` | function → test | A test directly invokes the function or its name matches a package-local convention; neither proves assertions or behavior coverage |
| `imports` | package → package | An exact internal package import resolved to a package entity |
| `embeds` | package → resource | A `//go:embed` pattern matches a scanned resource file |

The domain vocabulary also reserves `documented_in`, `depends_on`,
`implements`, `emits`, and `part_of`; the current scanner does not emit those
types. Their presence in the enum does not mean the graph proves such
relationships.

### Inverse Relationships

These are **not stored** in the graph. They are computed at load time:

| Stored | Computed Inverse |
|---|---|
| `calls` | callers (query direction `to`) |
| `imports` | imported-by (query direction `to`) |
| `owns` | owned-by (query direction `to`) |
| `tested_by` | tests (query direction `to`) |

This avoids inconsistency. One direction is the source of truth; the other is derived.

### Relationship Example

```json
{
  "id": "controller:example.com/project/controllers.WidgetReconciler--creates--resource:deployment._cluster/api@config/deployment.yaml",
  "from": "controller:example.com/project/controllers.WidgetReconciler",
  "to": "resource:deployment._cluster/api@config/deployment.yaml",
  "type": "creates",
  "confidence": "inferred",
  "evidence": {
    "parser": "go-ast",
  "file": "controllers/widget_controller.go",
    "line": 213,
  "snippet": "controllerutil.CreateOrUpdate(ctx, r.Client, deployment, mutate)",
    "reason": "explicit create/upsert call; target matched by unique manifest kind"
  }
}
```

---

## Relationship Resolution

The relationship builder (`internal/graph/builder.go`) resolves parser
observations only when the target can be identified without an arbitrary
choice. Resolution is intentionally conservative.

### Controller/resource resolution

- `For`, `Owns`, and `Watches` observations are matched to exactly one CRD by
  name, or to resource entities by their exact Kubernetes `kind` when no CRD
  name match exists. Ambiguous matches are omitted.
- An explicit create/upsert call recorded on a controller or function is
  matched only when exactly one scanned resource has the observed Kubernetes
  kind. Function-level edges preserve the Go function containing the call;
  controller-level edges preserve the controller aggregate used by controller
  views. Both are `inferred` because a type-level call does not identify a
  particular manifest instance. Ambiguous and unmatched kinds remain
  observations on the entity and do not produce an edge.
- A controller is linked with `contains` to its exact `Reconcile` and
  `SetupWithManager` function entities when their receiver-qualified IDs are
  present. The edge is `proven` from the Go method declaration and points to
  that declaration's source line; absent methods are omitted.
- `//go:embed` edges are emitted only when the pattern matches the resource
  path relative to the package's source file.

### Call resolution order

1. **Exact qualified match:** an import-path-qualified call is matched against
   the repository-qualified function ID.
2. **Exact caller-package match:** an unqualified call or simple receiver
   method is resolved only when exactly one function with that name exists in
   the caller's package. A nested selector chain is not reduced to its final
   method name; it requires an exact qualified match or a `go/types` result.
3. **Otherwise omit:** a call that is external, unsupported, or ambiguous is
   retained as an entity-level `calls` observation but does not become an edge.

Test links use a separate conservative rule: an unqualified call resolves only
to one function in the test's package, and a selector call requires an exact
qualified function ID. Otherwise, CodeAtlas may retain the package-local
`TestFoo` → `Foo` naming convention as an inferred link. A link establishes a
direct invocation or naming association only; it does not prove assertions,
behavioral coverage, or branch execution.

Common generic names (`Get`, `Set`, `Error`, `String`, `New`, `Close`, `Read`,
`Write`, `Marshal`, `Unmarshal`, and similar built-ins) are skipped to avoid
false-positive edges.

### Deduplication

All relationship types use a shared `seen` map keyed by relationship ID. This prevents duplicate edges when the same call appears multiple times (e.g., a controller watching the same CRD via different code paths).

### Relationship Types Built

| Type | From | To | How |
|---|---|---|---|
| `reconciles` | controller | CRD/resource | A `For` watch observation resolves to one target |
| `owns` | controller | CRD/resource | An `Owns` watch observation resolves to one target |
| `watches` | controller | CRD/resource | A `Watches` observation resolves to one target |
| `creates` | controller/function | resource | An explicit create/upsert observation resolves to one manifest kind |
| `contains` | controller | function | The exact receiver-qualified `Reconcile` or `SetupWithManager` method exists |
| `imports` | package | package | An import path exactly matches one scanned package |
| `calls` | controller/function | function | A call observation resolves by exact qualification or caller package |
| `tested_by` | function | test | The test directly invokes the function, or `TestFoo` matches `Foo` in the same package |
| `embeds` | package | resource | `//go:embed` pattern matches a resource path |

The `implements` parser observation is intentionally not an emitted edge until
interface declarations have a first-class entity model.

---

## Shared Types

### Source

Every entity carries a `source` proving where it was discovered.

| Field | Type | Required | Description |
|---|---|---|---|
| `parser` | enum | yes | One of: `go`, `go-ast`, `test`, `yaml`, `markdown`, or `git` |
| `file` | string | yes | File path relative to repo root |
| `line` | number | yes | Positive source line number |
| `endLine` | number | no | End line for a source span, when the parser can determine it |

### ID Rules

IDs are deterministic for the same repository layout. No UUIDs, no counters,
and no runtime timestamps in IDs. Repository scans use the current
`repository-path-v1` identity scheme; older fixture constructors may emit
legacy short IDs and are not safe for incremental reuse or implementation
guidance.

| Kind | ID Format | Example |
|---|---|---|
| `operator` | `operator:{name}` | Reserved; not emitted by current scanner |
| `controller` | `controller:{module/package}.{Receiver}` | `controller:example.com/project/controllers.WidgetReconciler` |
| `crd` | `crd:{group}.{lowercase-kind}` | `crd:example.io.widget` |
| `function` | `function:{module/package}.{Receiver.}Name` | `function:example.com/project/controllers.WidgetReconciler.sync` |
| `package` | `package:{module/package}` | `package:example.com/project/controllers` |
| `test` | `test:{module/package}.TestName` | `test:example.com/project/controllers.TestSync` |
| `document` | `document:{repository-relative-path}` | `document:docs/hostedcluster.md` |
| `resource` | `resource:{lowerkind}.{namespace-or-_cluster}/{name}@{repository-relative-file}` | `resource:deployment.openshift-config/api@config/deployment.yaml` |
| `template` | `template:kubernetes.{lowerkind}@{repository-relative-file}#{document-line}` | `template:kubernetes.deployment@config/deployment.yaml#1` |

The `kind:` prefix prevents ID collisions between different entity types that might share names.

---

## Atlas Graph Schema

The top-level output of `atlas scan`. One JSON file containing everything.

```json
{
  "schema": "codeatlas",
  "schemaVersion": "1.6.0",
  "entityIdentity": "repository-path-v1",
  "generatedAt": "2026-07-14T16:00:00Z",
  "repository": "/work/project",
  "commit": "abc1234def5678",
  "branch": "main",
  "scanDuration": "",

  "entities": [],
  "relationships": [],
  "fileTimestamps": {},
  "fileFingerprints": {},
  "scanComplete": true,
  "scanWarnings": [],
  "scanCoverage": {"discovered": 0, "parsed": 0, "reused": 0, "ignored": 0, "failed": 0},
  "scanFiles": [],
  "views": {},
  "questions": {}
}
```

**Key fields:**
- `schema` — always `"codeatlas"` for the current graph contract. Identifies this file as an Atlas Graph.
- `schemaVersion` — semver. Consumers check this to know which fields exist. Bump major on breaking changes, minor on new optional fields.
- `entityIdentity` — identity scheme used by the scanner. Current repository scans use `repository-path-v1`; consumers must reject legacy identities when exact implementation context is required.
- `generatedAt` — commit timestamp when available. It is provenance, not a wall-clock scan timestamp; this keeps the graph deterministic for a fixed commit.
- `commit` — exact git commit that was scanned. Enables diffing two graphs.
- `branch` — git branch. Enables comparing `release-4.19` vs `release-4.20`.
- `entities` — flat array of all entities (all kinds mixed together, distinguished by `kind`).
- `relationships` — flat array of all relationships.
- `fileTimestamps` — per-file RFC3339 modification times used as incremental state when fingerprints are unavailable.
- `fileFingerprints` — content fingerprints for supported files. Used to detect changes that preserve a timestamp.
- `scanComplete` — true when all files with registered parsers were processed without warnings. An ignored file is outside the parser set and remains explicitly listed in `scanCoverage`/`scanFiles`; false means a supported parser failed or another scan warning exists.
- `scanWarnings` — deterministic parser or discovery warnings that explain why completeness is unavailable.
- `scanCoverage` — counts of discovered files by disposition. `parsed` and `reused` are graph-file statuses accepted by the schema; current scans persist parsed facts while incremental reuse is reported by the scan result so identical repository snapshots remain deterministic.
- `scanFiles` — deterministic per-file disposition, parser name, entity count, and reason for ignored/failed files. Unsupported extensions are explicit `ignored` entries rather than silent omissions.
- `views` — pre-computed engineering views for controllers and CRDs, generated during scan. Each keyed by entity ID, containing ownership, resources, tests, files, and temporal data.
- `questions` — deterministic Q&A pairs derived from views (for example, `"reconciles:Widget"` → `"Widget"`). Entity/relationship counts are computed by `atlas stats`; they are not stored as a top-level graph field.
- Temporal fields are opt-in. A scan without `--temporal` leaves
  `lastAuthor`, `lastModified`, and `changeCount` empty, including when
  unchanged entities are reused from a previous graph.

---

## Rules

1. **No entity without a source.** If the scanner can't point to a file and line, it doesn't create the entity.
2. **No relationship without evidence.** Every edge carries `evidence` explaining what was found and why it constitutes this relationship.
3. **No manual entries.** If a fact needs to be added by hand, that's a missing parser, not a data entry task.
4. **IDs are deterministic.** Same repository layout → same IDs. No UUIDs, no timestamps in IDs.
5. **Imports preserve observations.** The package entity records every parsed import path. An `imports` relationship is emitted only for an exact match to one scanned package in the same graph.
6. **Store forward, compute inverse.** `calls` is stored; `called_by` is computed. `imports` is stored; `imported_by` is computed. One direction is truth.
7. **One entity per thing.** HostedClusterReconciler is one entity with `kind: controller`. Not a Component and a Controller.
8. **Template identities stay unresolved.** A templated manifest may be represented as `kind: template`, but the scanner must not turn a runtime placeholder into a concrete resource name.

---

## Open Design Questions

These are documented but not resolved. They don't block V1.

### Workflows

Eventually CodeAtlas should model reconciliation workflows:

```
HostedCluster Reconcile()
  → Create HostedControlPlane
  → Start CPO
  → Deploy ETCD
  → Deploy KAS
  → Update Status
```

A workflow would be an ordered sequence of steps, each pointing to a function entity. This likely becomes a new `kind: workflow` or a new relationship type `step_of` with an `order` field.

**Not in V1.** Needs real call graph data to design well. Revisit in Phase 4.

### Core Schema vs. Extensions

Today, parser observations (`watches`, `creates`, `calls`, imports, literals,
and so on) live directly on the Entity. The current CRD/resource parsers do not
expose separate group/version/scope fields. This flat shape is intentionally
small for V1.

Eventually, if CodeAtlas supports other projects (controller-runtime, Operator SDK, Kubebuilder, plain Kubernetes), the schema should split:

```
Entity (core — universal)
├── id, name, kind, description, package, files, source

ControllerExtension (Kubernetes-specific)
├── watches, watchMethods, watchSites, creates, createSites

CRDExtension (Kubernetes-specific)
├── future manifest/API metadata

OperatorExtension (Kubernetes-specific)
├── entrypoint, controllers
```

The core stays stable across any Go project. Extensions evolve per domain.

**Not in V1.** Today all fields live flat on Entity. But when adding new kind-specific fields, ask: "Is this universal to Go, or specific to Kubernetes?" If specific, it's a future extension field — keep it optional and don't let core logic depend on it.

### Multi-Source Discovery

Today, each entity has a single `source` field — the parser and file that discovered it. But a real entity like HostedCluster might be discovered from multiple sources:

- Go AST finds the struct definition
- YAML finds the CRD manifest
- Markdown finds the design doc that names it

A future `discoveredBy` field would replace the single `source` with an array:

```json
{
  "id": "crd:hypershift.openshift.io/v1beta1.HostedCluster",
  "name": "HostedCluster",
  "kind": "crd",
  "discoveredBy": [
    { "parser": "go-ast", "file": "api/hypershift/v1beta1/hostedcluster_types.go", "line": 42 },
    { "parser": "yaml", "file": "config/crds/hostedclusters.yaml", "line": 1 },
    { "parser": "markdown", "file": "docs/hostedcluster.md", "line": 1 }
  ]
}
```

The viewer could then show corroboration — "this entity was confirmed by 3 independent parsers" — making CodeAtlas's claims visibly stronger.

**Not in V1.** Today `source` is a single object pointing to the primary discovery. But the scanner should already be aware that multiple parsers may find the same entity — it needs a merge strategy (first-wins, highest-confidence-wins, or merge-all). Designing that merge is the prerequisite for `discoveredBy`.

### Generalization

The data model uses no HyperShift-specific concepts. Entity kinds like `controller`, `crd`, `package`, and `function` exist in any Go + Kubernetes project. CodeAtlas can scan any Go repository using this same schema.

Worth preserving — don't add target-specific fields to the core schema.

---

## What This Document Does Not Cover

- How the scanner implements parsing (that's code, not data model)
- How the viewer renders entities (that's UI, not data model)
- How `atlas scan` is structured internally (that's architecture, not data model)

This document defines the **shape of the Atlas Graph**. The scanner produces it. Everything else consumes it.
