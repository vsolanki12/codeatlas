# Data Model

This document is the contract between the scanner and everything that consumes the Atlas Graph. If an entity or field isn't defined here, it doesn't exist in Atlas.

---

## Core Concept: Everything is an Entity

Atlas does not have separate types for Component, Controller, CRD, etc. It has one type: **Entity**.

An Entity's `kind` field determines what it represents. A controller is an Entity with `kind: controller`. A CRD is an Entity with `kind: crd`. This avoids duplication — HostedClusterReconciler is one object, not a Component and a Controller that must stay in sync.

Different kinds carry different optional fields. The scanner populates only the fields relevant to each kind. The viewer ignores fields that are absent.

---

## Entity

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Unique, deterministic identifier (see ID rules below) |
| `name` | string | yes | Human-readable name (e.g., "HostedClusterReconciler") |
| `kind` | enum | yes | What this entity represents (see Kind table) |
| `description` | string | no | Extracted from GoDoc or documentation. Never written by Atlas. |
| `package` | string | no | Go package path (e.g., `pkg/controllers/hostedcluster`) |
| `files` | string[] | no | Files that implement this entity |
| `source` | Source | yes | Where this entity was discovered |

### Kind-Specific Fields

Additional fields based on `kind`. All optional — the scanner includes them when it can extract them.

#### kind: `controller`

| Field | Type | Description |
|---|---|---|
| `reconciles` | string | CRD or resource kind this controller manages |
| `reconcileFile` | string | File containing the `Reconcile()` method |
| `reconcileLine` | number | Line number of `Reconcile()` |
| `setupFile` | string | File containing `SetupWithManager()` |
| `watches` | string[] | Resources this controller watches |

#### kind: `crd`

| Field | Type | Description |
|---|---|---|
| `group` | string | API group (e.g., `hypershift.openshift.io`) |
| `version` | string | API version (e.g., `v1beta1`) |
| `scope` | enum | `Namespaced` or `Cluster` |
| `specFile` | string | Go file defining the spec struct |
| `conditions` | string[] | Status conditions this CRD emits |

#### kind: `function`

| Field | Type | Description |
|---|---|---|
| `receiver` | string | Struct receiver, if a method |
| `file` | string | File where this function is defined |
| `line` | number | Line number of the declaration |
| `signature` | string | Full function signature |
| `calls` | string[] | IDs of functions this function calls |
| `doc` | string | GoDoc comment |

Note: `calledBy` is **not stored**. It is computed at load time by inverting `calls`. Same principle as `importedBy` — store the forward edge, compute the reverse.

#### kind: `package`

| Field | Type | Description |
|---|---|---|
| `path` | string | Directory path relative to repo root |
| `goFiles` | string[] | `.go` files (excluding tests) |
| `testFiles` | string[] | `_test.go` files |
| `imports` | string[] | Internal packages this package imports |

Note: `importedBy` is **not stored**. Computed at load time.

#### kind: `test`

| Field | Type | Description |
|---|---|---|
| `testType` | enum | `unit`, `integration`, or `e2e` |
| `file` | string | Test file path |
| `line` | number | Line number of the test function |

#### kind: `document`

| Field | Type | Description |
|---|---|---|
| `path` | string | File path relative to repo root |
| `docType` | enum | `readme`, `design`, `enhancement`, `guide`, `api-reference` |
| `headings` | string[] | All headings extracted from the document |

#### kind: `operator`

| Field | Type | Description |
|---|---|---|
| `entrypoint` | string | Path to `main.go` |
| `controllers` | string[] | Controller entity IDs registered by this operator |

#### kind: `resource`

| Field | Type | Description |
|---|---|---|
| `resourceKind` | string | Kubernetes kind (e.g., `Deployment`, `Service`, `ConfigMap`) |
| `namespace` | string | Target namespace, if specified |

### Kind Table

| Kind | What It Represents | How Scanner Finds It |
|---|---|---|
| `operator` | A binary that registers controllers | `main.go` that calls `SetupWithManager` |
| `controller` | A struct with a `Reconcile()` method | `reconcile.Reconciler` implementation |
| `crd` | A Custom Resource Definition | `+kubebuilder` markers or CRD YAML |
| `function` | A Go function or method | `go/ast.FuncDecl` |
| `package` | A Go package | `go/parser.ParseDir` |
| `test` | A test function (`Test*`) | Functions in `_test.go` files |
| `document` | A markdown file | `.md` files in `docs/`, `enhancements/`, root |
| `resource` | A Kubernetes resource in YAML | Deployment/Service/ConfigMap YAML files |

### Entity Examples

**Controller:**

```json
{
  "id": "controller:hosted-cluster-reconciler",
  "name": "HostedClusterReconciler",
  "kind": "controller",
  "description": "Reconciles HostedCluster resources and manages the lifecycle of hosted control planes",
  "package": "pkg/controllers/hostedcluster",
  "files": [
    "pkg/controllers/hostedcluster/hostedcluster_controller.go",
    "pkg/controllers/hostedcluster/status.go",
    "pkg/controllers/hostedcluster/kas.go"
  ],
  "reconciles": "HostedCluster",
  "reconcileFile": "pkg/controllers/hostedcluster/hostedcluster_controller.go",
  "reconcileLine": 142,
  "watches": ["HostedCluster", "HostedControlPlane", "Secret"],
  "source": {
    "parser": "go-ast",
    "file": "pkg/controllers/hostedcluster/hostedcluster_controller.go",
    "line": 58
  }
}
```

**Function:**

```json
{
  "id": "function:hostedcluster.HostedClusterReconciler.syncEtcd",
  "name": "syncEtcd",
  "kind": "function",
  "package": "pkg/controllers/hostedcluster",
  "receiver": "HostedClusterReconciler",
  "file": "pkg/controllers/hostedcluster/etcd.go",
  "line": 24,
  "signature": "func (r *HostedClusterReconciler) syncEtcd(ctx context.Context, hcp *hyperv1.HostedControlPlane) error",
  "calls": [
    "function:manifests.EtcdStatefulSet",
    "function:hostedcluster.reconcileEtcdStatefulSet",
    "function:controllerutil.CreateOrUpdate"
  ],
  "doc": "syncEtcd ensures the etcd StatefulSet exists and is up to date",
  "source": {
    "parser": "go-ast",
    "file": "pkg/controllers/hostedcluster/etcd.go",
    "line": 24
  }
}
```

**CRD:**

```json
{
  "id": "crd:hypershift.openshift.io/v1beta1.HostedCluster",
  "name": "HostedCluster",
  "kind": "crd",
  "group": "hypershift.openshift.io",
  "version": "v1beta1",
  "scope": "Namespaced",
  "specFile": "api/hypershift/v1beta1/hostedcluster_types.go",
  "conditions": ["Available", "Progressing", "Degraded", "ValidConfiguration"],
  "source": {
    "parser": "go-ast",
    "file": "api/hypershift/v1beta1/hostedcluster_types.go",
    "line": 42
  }
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

This is what makes Atlas trustworthy. Every relationship carries proof — not just "where" it was found, but "what" was found.

| Field | Type | Required | Description |
|---|---|---|---|
| `parser` | enum | yes | Which parser discovered this: `go-ast`, `yaml`, `markdown`, `git` |
| `file` | string | yes | File path relative to repo root |
| `line` | number | yes | Line number (0 if not applicable) |
| `snippet` | string | no | The actual code or text that proves the relationship (1-2 lines max) |
| `reason` | string | no | Human-readable explanation of why this relationship exists |

When a user clicks a relationship and asks "why does Atlas think HostedCluster creates HostedControlPlane?" — the `evidence` answers it.

### Confidence Levels

| Level | Meaning | Example |
|---|---|---|
| `proven` | Directly observed in code, YAML, or AST | A `CreateOrUpdate()` call creating a HostedControlPlane |
| `inferred` | Derived from naming conventions or proximity | Test file name matches controller file name |

Two levels only. No percentages. If it's a guess, it doesn't go in the graph.

### Relationship Types

| Type | From → To | Meaning |
|---|---|---|
| `reconciles` | controller → crd/resource | This controller manages this resource |
| `creates` | controller/operator → resource/crd | This entity creates this Kubernetes resource |
| `owns` | crd → crd | Parent-child ownership |
| `watches` | controller → crd/resource | Changes to this resource trigger reconciliation |
| `calls` | function → function | This function calls that function |
| `tested_by` | any → test | This entity is tested by this test |
| `documented_in` | any → document | This entity is described in this document |
| `depends_on` | any → any | This entity requires that entity to function |
| `imports` | package → package | This package imports that package |
| `implements` | controller → any | This struct implements that interface |
| `emits` | crd/controller → any | This entity sets this condition or fires this event |
| `contains` | package/operator → function/controller | This entity contains that entity |
| `part_of` | controller → operator | This controller belongs to this operator |

### Inverse Relationships

These are **not stored** in the graph. They are computed at load time:

| Stored | Computed Inverse |
|---|---|
| `calls` | `called_by` |
| `imports` | `imported_by` |
| `contains` | `contained_in` |
| `owns` | `owned_by` |
| `tested_by` | `tests` |
| `documented_in` | `documents` |

This avoids inconsistency. One direction is the source of truth; the other is derived.

### Relationship Example

```json
{
  "id": "controller:hosted-cluster-reconciler--creates--crd:hypershift.openshift.io/v1beta1.HostedControlPlane",
  "from": "controller:hosted-cluster-reconciler",
  "to": "crd:hypershift.openshift.io/v1beta1.HostedControlPlane",
  "type": "creates",
  "confidence": "proven",
  "evidence": {
    "parser": "go-ast",
    "file": "pkg/controllers/hostedcluster/hostedcluster_controller.go",
    "line": 213,
    "snippet": "controllerutil.CreateOrUpdate(ctx, r.Client, hcp, func() error {",
    "reason": "HostedClusterReconciler calls CreateOrUpdate to create a HostedControlPlane resource"
  }
}
```

---

## Shared Types

### Source

Every entity carries a `source` proving where it was discovered.

| Field | Type | Required | Description |
|---|---|---|---|
| `parser` | enum | yes | One of: `go-ast`, `yaml`, `markdown`, `git` |
| `file` | string | yes | File path relative to repo root |
| `line` | number | yes | Line number (0 if not applicable) |

### ID Rules

IDs are deterministic. Same commit, same IDs. No UUIDs, no counters.

| Kind | ID Format | Example |
|---|---|---|
| `operator` | `operator:{name}` | `operator:control-plane-operator` |
| `controller` | `controller:{kebab-name}` | `controller:hosted-cluster-reconciler` |
| `crd` | `crd:{group/version.Kind}` | `crd:hypershift.openshift.io/v1beta1.HostedCluster` |
| `function` | `function:{pkg.Receiver.Name}` | `function:hostedcluster.HostedClusterReconciler.syncEtcd` |
| `package` | `package:{path}` | `package:pkg/controllers/hostedcluster` |
| `test` | `test:{pkg.TestName}` | `test:hostedcluster.TestReconcileHostedCluster` |
| `document` | `document:{path}` | `document:docs/hostedcluster.md` |
| `resource` | `resource:{kind}/{name}` | `resource:Deployment/kube-apiserver` |

The `kind:` prefix prevents ID collisions between different entity types that might share names.

---

## Atlas Graph Schema

The top-level output of `atlas scan`. One JSON file containing everything.

```json
{
  "schema": "atlas-graph",
  "schemaVersion": "1.0.0",
  "generatedAt": "2026-07-14T16:00:00Z",
  "repository": "https://github.com/openshift/hypershift",
  "commit": "abc1234def5678",
  "branch": "main",
  "scanDuration": "12.4s",

  "entities": [],
  "relationships": [],

  "stats": {
    "entities": {
      "operator": 0,
      "controller": 0,
      "crd": 0,
      "function": 0,
      "package": 0,
      "test": 0,
      "document": 0,
      "resource": 0,
      "total": 0
    },
    "relationships": {
      "total": 0,
      "proven": 0,
      "inferred": 0
    }
  }
}
```

**Key fields:**
- `schema` — always `"atlas-graph"`. Identifies this file as an Atlas Graph.
- `schemaVersion` — semver. Consumers check this to know which fields exist. Bump major on breaking changes, minor on new optional fields.
- `commit` — exact git commit that was scanned. Enables diffing two graphs.
- `branch` — git branch. Enables comparing `release-4.19` vs `release-4.20`.
- `entities` — flat array of all entities (all kinds mixed together, distinguished by `kind`).
- `relationships` — flat array of all relationships.

---

## Rules

1. **No entity without a source.** If the scanner can't point to a file and line, it doesn't create the entity.
2. **No relationship without evidence.** Every edge carries `evidence` explaining what was found and why it constitutes this relationship.
3. **No manual entries.** If a fact needs to be added by hand, that's a missing parser, not a data entry task.
4. **IDs are deterministic.** Same commit → same graph. No UUIDs, no timestamps in IDs.
5. **Only internal imports.** Package imports only track HyperShift-internal packages.
6. **Store forward, compute inverse.** `calls` is stored; `called_by` is computed. `imports` is stored; `imported_by` is computed. One direction is truth.
7. **One entity per thing.** HostedClusterReconciler is one entity with `kind: controller`. Not a Component and a Controller.

---

## Open Design Questions

These are documented but not resolved. They don't block V1.

### Workflows

Eventually Atlas should model reconciliation workflows:

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

Today, kind-specific fields (`reconciles`, `watches`, `conditions`, `group`, `version`) live directly on the Entity. This works for V1 because HyperShift is the only target.

Eventually, if Atlas supports other projects (controller-runtime, Operator SDK, Kubebuilder, plain Kubernetes), the schema should split:

```
Entity (core — universal)
├── id, name, kind, description, package, files, source

ControllerExtension (Kubernetes-specific)
├── reconciles, watches, setupFile

CRDExtension (Kubernetes-specific)
├── group, version, scope, conditions

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
    { "parser": "markdown", "file": "docs/hostedcluster.md", "line": 0 }
  ]
}
```

The viewer could then show corroboration — "this entity was confirmed by 3 independent parsers" — making Atlas's claims visibly stronger.

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
