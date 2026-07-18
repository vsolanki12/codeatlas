# ADR-0006: Unified Entity Model

**Status:** Accepted
**Date:** 2026-07-14

## Decision

There is one type: Entity. Different kinds (controller, crd, function, etc.) carry different optional fields. There is no separate Component type that duplicates Controller.

## Why

A HostedClusterReconciler should be one object, not a Component and a Controller that must stay in sync. The `kind` field distinguishes entity types. Kind-specific fields (like `reconciles`, `watches`, `group`, `version`) are optional — the scanner includes them when they apply.
