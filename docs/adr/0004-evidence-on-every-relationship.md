# ADR-0004: Evidence on Every Relationship

**Status:** Accepted
**Date:** 2026-07-14

## Decision

Every relationship carries an `evidence` object: parser, file, line, code snippet, and a human-readable reason.

## Why

This is the "extract, never invent" principle enforced at the data level. It allows users to click any edge and see exactly why Atlas believes that relationship exists. It also enables staleness detection — if a source file changes, the facts derived from it can be flagged for re-scanning.
