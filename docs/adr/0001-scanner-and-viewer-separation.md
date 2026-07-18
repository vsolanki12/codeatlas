# ADR-0001: Scanner and Viewers Are Separate

**Status:** Accepted
**Date:** 2026-07-14

## Decision

The scanner is a Go binary. Viewers (HTML page, React app, CLI, etc.) consume JSON. They never parse Go.

## Why

The scanner needs Go's AST packages. Viewers need JavaScript or terminal output. Coupling them would make both harder to develop and test. A Go engineer can improve the scanner without touching frontend code.

## Implication

The Atlas Graph JSON is the contract between them. Its schema must be stable and well-defined (see [data-model.md](../data-model.md)).
