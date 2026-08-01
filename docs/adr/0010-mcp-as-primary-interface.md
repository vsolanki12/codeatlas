# ADR-0010: MCP as Primary Consumer Interface

**Status:** Accepted
**Date:** 2026-07-31

## Decision

The MCP (Model Context Protocol) server is the primary interface between the Atlas Graph and AI consumers. All graph queries go through the query engine; the MCP server is a thin wrapper that formats results as text.

## Why

The original product architecture listed five consumers: Viewer, Web, CLI, API, AI. In practice, Claude Code via MCP became the dominant consumer within weeks. Rather than building separate REST APIs or GraphQL layers, MCP provides structured tool access with schema validation, streaming, and tool chaining — all for free.

MCP is also consumer-agnostic. The same tools work for Claude Code, VS Code extensions, Cursor, or any MCP-compatible client. One interface serves all AI consumers.

## Consequences

- The query engine (`internal/query`) is the real API. MCP is a presentation layer.
- Adding a new query means: implement in query engine, add one MCP registration in `mcpserver/server.go`.
- Non-AI consumers (CLI, web viewer) bypass MCP and use the query engine or JSON directly.
- No REST API needed until a non-MCP consumer requires one.
