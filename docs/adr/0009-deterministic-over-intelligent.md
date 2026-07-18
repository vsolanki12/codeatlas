# ADR-0009: Deterministic Over Intelligent

**Status:** Accepted
**Date:** 2026-07-14

## Decision

Atlas always prefers deterministic extraction over heuristic inference. If a parser can determine a fact, don't use AI. If the AST can find a relationship, don't guess. If YAML declares ownership, don't infer it.

## Why

Deterministic extraction is reproducible — same commit, same graph, every time. Heuristics drift. AI hallucinates. Parsers don't. This decision protects Atlas from the most common failure mode of developer tools: adding intelligence where precision was needed.

## The Test

Before building any new feature, ask: "Can a parser determine this?" If yes, build a parser. If no, document what's missing and revisit when the data model supports it. Only reach for inference when deterministic extraction is provably impossible — and even then, mark the result as `confidence: inferred` so users can distinguish it from proven facts.

## Examples

- HostedClusterReconciler reconciles HostedCluster → Go AST finds the `Reconcile()` method. Don't infer.
- HostedCluster owns NodePool → YAML or kubebuilder markers declare ownership. Don't guess.
- syncEtcd is called before syncKAS → Call order in `Reconcile()` body is parseable. Don't ask an LLM.
- "This controller is complex" → Not deterministic. Not in Atlas.
