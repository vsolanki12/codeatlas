package mcpserver

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/query"
)

func Run(ctx context.Context, graphPath string, extractorBuild ...string) error {
	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "codeatlas",
		Version: "0.3.0",
	}, nil)

	registerTools(server, idx, graphPath, extractorBuild...)

	return server.Run(ctx, &mcp.StdioTransport{})
}

func registerTools(s *mcp.Server, idx *query.Index, graphPath string, extractorBuild ...string) {
	registerSearch(s, idx)
	registerEntity(s, idx)
	registerContext(s, idx)
	registerWhere(s, idx)
	registerStats(s, idx)
	registerFreshness(s, idx, graphPath, extractorBuild...)
	registerTemporal(s, idx)
	registerInvestigate(s, idx)
	registerExplain(s, idx)
	registerImpact(s, idx)
	registerView(s, idx)
	registerAsk(s, idx)
	registerReview(s, idx, graphPath, extractorBuild...)
}

type entityInput struct {
	ID                 string   `json:"id,omitempty" jsonschema:"exact entity ID, e.g. controller:hostedclusters.HostedClusterReconciler"`
	IDs                []string `json:"ids,omitempty" jsonschema:"list of entity IDs to fetch"`
	Brief              bool     `json:"brief,omitempty" jsonschema:"if true, return only ID, file, line, description (saves tokens)"`
	RelationshipOffset int      `json:"relationship_offset,omitempty" jsonschema:"zero-based offset for this entity's relationships"`
	RelationshipLimit  int      `json:"relationship_limit,omitempty" jsonschema:"maximum relationships to return (default 40, maximum 100)"`
}

func registerEntity(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_entity",
		Description: "Get full details for a CodeAtlas entity by exact ID. Shows name, kind, package, file, description, watches, calls, and a bounded evidence-bearing relationship page. Use brief=true for compact output; use relationship_offset to continue high-degree relationship results.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input entityInput) (*mcp.CallToolResult, any, error) {
		if len(input.IDs) > 0 {
			const maxBatch = 50
			ids := input.IDs
			truncated := false
			if len(ids) > maxBatch {
				ids = ids[:maxBatch]
				truncated = true
			}
			var lines []string
			entities := idx.EntitiesByID(ids)
			for _, id := range ids {
				if e := idx.GetEntity(id); e != nil {
					lines = append(lines, query.FormatEntity(e))
				}
			}
			text := query.FormatEntityList(nil)
			if len(lines) > 0 {
				text = joinLines(lines)
			}
			if truncated {
				text += "[TRUNCATED: entity batch capped at 50 IDs; omitted IDs were not queried.]\n"
			}
			result := idx.CompactEntityListResult(entities, false, 0)
			result.Truncated = result.Truncated || truncated
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: text}},
				StructuredContent: result,
			}, nil, nil
		}
		e := idx.GetEntity(input.ID)
		if e == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.ID)}},
			}, nil, nil
		}
		if input.Brief {
			description := e.Description
			if len(description) > 240 {
				description = description[:240]
				for !utf8.ValidString(description) {
					description = description[:len(description)-1]
				}
			}
			entity := map[string]any{"id": e.ID, "name": e.Name, "kind": e.Kind.String(), "source": e.Source, "description": description}
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: query.FormatEntity(e) + "\n"}},
				StructuredContent: map[string]any{"graph": idx.GraphMetadata(), "entity": entity, "truncated": len(e.Description) > 240},
			}, nil, nil
		}
		text := query.FormatEntityFull(e)
		relationshipLimit := input.RelationshipLimit
		if relationshipLimit <= 0 {
			relationshipLimit = 40
		}
		relationshipOffset, relationshipLimit, err := query.NormalizePage(input.RelationshipOffset, relationshipLimit)
		if err != nil {
			return nil, nil, err
		}
		relationships := idx.RelationshipPage(input.ID, "both", "", relationshipOffset, relationshipLimit)
		if len(relationships.Relationships) > 0 {
			text += "Relationships:\n" + query.FormatRelationshipList(relationships.Relationships)
		}
		if relationships.HasMore {
			text += fmt.Sprintf("[TRUNCATED: relationship context capped; request relationship_offset=%d for the next page; omitted relationships are not evidence of absence.]\n", relationships.NextOffset())
		}
		result := idx.CompactEntityRelationshipPageResult(e, relationships)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: result,
		}, nil, nil
	})
}

type contextInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to center the subgraph on"`
	Depth    int    `json:"depth,omitempty" jsonschema:"BFS traversal depth (default 1, max 3)"`
}

func registerContext(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_context",
		Description: "Get a subgraph around a CodeAtlas entity. Shows the entity and all connected entities within the given depth.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input contextInput) (*mcp.CallToolResult, any, error) {
		depth := input.Depth
		if depth <= 0 {
			depth = 1
		}
		sg := idx.Neighbors(input.EntityID, depth)
		text := query.FormatSubgraph(sg)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: query.CompactSubgraph(sg),
		}, nil, nil
	})
}

type searchInput struct {
	Query  string `json:"query,omitempty" jsonschema:"search text, matches name, description, package, ID, imports, literals, and properties. Space-separated terms are AND-ed (all must match)."`
	Kind   string `json:"kind,omitempty" jsonschema:"entity kind: controller, crd, function, package, test, document, resource, template"`
	Offset int    `json:"offset,omitempty" jsonschema:"zero-based result offset for deterministic pagination"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20, maximum 100)"`
}

func registerSearch(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_search",
		Description: "Find entities in the codebase by text and/or kind. Returns bounded entity summaries with file locations and total/nextOffset pagination metadata; use offset to continue a result set and atlas_entity for full details.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, any, error) {
		offset, limit, err := query.NormalizePage(input.Offset, input.Limit)
		if err != nil {
			return nil, nil, err
		}
		page, err := idx.SearchKindPage(input.Query, input.Kind, offset, limit)
		if err != nil {
			return nil, nil, err
		}
		text := query.FormatEntityList(page.Entities) + query.FormatEntityPage(page)
		result := idx.CompactEntityListPageResult(page, false, 0)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: result,
		}, nil, nil
	})
}

type whereInput struct {
	Path   string `json:"path" jsonschema:"file path substring to search for"`
	Detail bool   `json:"detail,omitempty" jsonschema:"if true, return full entity details (name, calls, watches, description) instead of brief ID|file:line. Use for deep-diving a single file."`
	Offset int    `json:"offset,omitempty" jsonschema:"zero-based result offset for deterministic pagination"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20, maximum 100)"`
}

func registerWhere(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_where",
		Description: "Find entities by file path. Returns entities defined in files matching the path substring with total/nextOffset pagination metadata. Use offset to continue and detail=true for full entity info.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input whereInput) (*mcp.CallToolResult, any, error) {
		offset, limit, err := query.NormalizePage(input.Offset, input.Limit)
		if err != nil {
			return nil, nil, err
		}
		page := idx.WherePage(input.Path, offset, limit)
		var text string
		if input.Detail {
			text = query.FormatEntityDetailList(page.Entities)
		} else {
			text = query.FormatEntityList(page.Entities)
		}
		text += query.FormatEntityPage(page)
		// Detail expands only the human-readable text. Keep MCP structured
		// content bounded so a large file/package cannot bypass the token
		// contract.
		result := idx.CompactEntityListPageResult(page, input.Detail, 40)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: result,
		}, nil, nil
	})
}

type temporalInput struct {
	Kind   string `json:"kind,omitempty" jsonschema:"entity kind filter: controller, function, package, etc."`
	Name   string `json:"name,omitempty" jsonschema:"function/entity name substring to filter"`
	Since  string `json:"since,omitempty" jsonschema:"ISO date cutoff, e.g. 2026-05-01"`
	Author string `json:"author,omitempty" jsonschema:"author email/name substring"`
	Stale  bool   `json:"stale,omitempty" jsonschema:"if true, sort by oldest modification instead of most changes"`
	Offset int    `json:"offset,omitempty" jsonschema:"zero-based result offset for deterministic pagination"`
	Limit  int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 20, maximum 100)"`
}

func registerTemporal(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_temporal",
		Description: "Search entities by git history: who changed what and when. Requires --temporal scan and returns total/nextOffset pagination metadata.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input temporalInput) (*mcp.CallToolResult, any, error) {
		offset, limit, err := query.NormalizePage(input.Offset, input.Limit)
		if err != nil {
			return nil, nil, err
		}
		page := idx.TemporalPage(input.Kind, input.Name, input.Since, input.Author, input.Stale, offset, limit)
		if len(page.Entities) == 0 && page.Total == 0 {
			result := idx.CompactEntityListPageResult(page, false, 0)
			message := "No temporal results match these filters.\n"
			if idx.TemporalPage("", "", "", "", false, 0, 1).Total == 0 {
				message = "No temporal data. Re-scan with --temporal flag.\n"
			}
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: message + query.FormatEntityPage(page)}},
				StructuredContent: result,
			}, nil, nil
		}
		var lines []string
		for _, e := range page.Entities {
			lines = append(lines, fmt.Sprintf("%s | %s:%d | changes=%d last=%s by=%s",
				e.ID, e.Source.File, e.Source.Line, e.ChangeCount, e.LastModified, e.LastAuthor))
		}
		text := joinLines(lines) + query.FormatEntityPage(page)
		result := idx.CompactEntityListPageResult(page, false, 0)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: result,
		}, nil, nil
	})
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l + "\n"
	}
	return result
}

type statsInput struct{}

func registerStats(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_stats",
		Description: "Get CodeAtlas graph statistics: entity counts by kind and relationship counts by type.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ statsInput) (*mcp.CallToolResult, any, error) {
		text := query.FormatStats(idx.Stats())
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: idx.Stats(),
		}, nil, nil
	})
}

type investigateInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to investigate"`
	Detail   bool   `json:"detail,omitempty" jsonschema:"true for verbose human-readable output; default is bounded"`
}

func registerInvestigate(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_investigate",
		Description: "Get bounded details about an entity in one call: relationships grouped by type, callers, tests, and same-file siblings. Default text is compact; use detail=true for verbose text, or atlas_entity for full entity fields.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input investigateInput) (*mcp.CallToolResult, any, error) {
		r := idx.Investigate(input.EntityID)
		if r == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatInvestigationCompact(r)
		if input.Detail {
			text = query.FormatInvestigation(r)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: query.CompactInvestigate(r),
		}, nil, nil
	})
}

type explainInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to explain"`
	Depth    int    `json:"depth,omitempty" jsonschema:"traversal depth (default 2, max 3)"`
	Detail   bool   `json:"detail,omitempty" jsonschema:"true for verbose human-readable output; default is bounded"`
}

func registerExplain(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_explain",
		Description: "Follow the reconciliation chain from an entity: controller implementation methods (contains), reconciles, creates, calls, and tested_by. Returns a bounded tree showing the architectural narrative. Default text is compact; use detail=true for verbose text.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input explainInput) (*mcp.CallToolResult, any, error) {
		r := idx.Explain(input.EntityID, input.Depth)
		if r.Root == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatExplanationCompact(r)
		if input.Detail {
			text = query.FormatExplanation(r)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: query.CompactExplain(r),
		}, nil, nil
	})
}

type impactInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to analyze blast radius for"`
	Detail   bool   `json:"detail,omitempty" jsonschema:"true for verbose human-readable output; default is bounded"`
}

func registerImpact(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_impact",
		Description: "Blast radius analysis: walk the call chain upstream to find bounded controllers, tests, resources, files, owners, and supporting relationship evidence. Default text is compact; use detail=true for verbose text.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input impactInput) (*mcp.CallToolResult, any, error) {
		r := idx.Impact(input.EntityID)
		if r == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatImpactCompact(r)
		if input.Detail {
			text = query.FormatImpact(r)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: query.CompactImpact(r),
		}, nil, nil
	})
}

type viewInput struct {
	Entity string `json:"entity" jsonschema:"entity ID or name to get pre-computed knowledge view for"`
}

func registerView(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_view",
		Description: "Get a pre-computed engineering view for a controller or CRD: what it manages, what manages it, tests, files, and ownership. Generated during scanning, zero graph traversal.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input viewInput) (*mcp.CallToolResult, any, error) {
		v := idx.ResolveView(input.Entity)
		if v == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("No view found for: %s (views exist for controllers and CRDs only)", input.Entity)}},
			}, nil, nil
		}
		text := query.FormatView(v)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: map[string]any{"graph": idx.GraphMetadata(), "view": query.CompactViewResult(v)},
		}, nil, nil
	})
}

type askInput struct {
	Entity           string `json:"entity,omitempty" jsonschema:"exact entity name or ID; optional when question is provided"`
	Question         string `json:"question,omitempty" jsonschema:"complete scoped question; selects a deduplicated evidence manifest"`
	Scope            string `json:"scope,omitempty" jsonschema:"package, file path, or entity scope"`
	Evidence         bool   `json:"evidence,omitempty" jsonschema:"return versioned evidence manifest for an exact entity"`
	BudgetBytes      int    `json:"budget_bytes,omitempty" jsonschema:"serialized manifest budget in bytes (default 16384, minimum 2048)"`
	Offset           int    `json:"offset,omitempty" jsonschema:"manifest continuation offset"`
	GraphFingerprint string `json:"graph_fingerprint,omitempty" jsonschema:"snapshot fingerprint from the previous manifest"`
	Intent           string `json:"intent,omitempty" jsonschema:"understand (how it works), impact (what breaks), or debug (everything about it). Default: view only"`
	Detail           bool   `json:"detail,omitempty" jsonschema:"true for full verbose output with complete entity IDs and paths. Default: compact"`
}

func registerAsk(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_ask",
		Description: "Retrieve engineering evidence. Prefer question with optional scope for a versioned, deduplicated manifest of implementation, definitions, relationships, tests, and source spans under budget_bytes. Use graph_fingerprint and offset to continue. Entity alone retains the legacy entity report; evidence=true selects the manifest.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input askInput) (*mcp.CallToolResult, any, error) {
		if input.Question != "" || input.Evidence || input.Scope != "" || input.BudgetBytes != 0 || input.Offset != 0 || input.GraphFingerprint != "" {
			packet, err := idx.Evidence(query.EvidenceRequest{Question: input.Question, Entity: input.Entity, Scope: input.Scope,
				Intent: input.Intent, BudgetBytes: input.BudgetBytes, Offset: input.Offset, GraphFingerprint: input.GraphFingerprint})
			if err != nil {
				return nil, nil, err
			}
			// The text is a small summary of the selected packet. Evidence appears
			// once in structured content, avoiding two complete representations.
			text := fmt.Sprintf("Evidence %s: %d entities, %d relationships, %d source spans; truncated=%t.\n", packet.Status, len(packet.Entities), len(packet.Relationships), len(packet.Sources), packet.Truncated)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, StructuredContent: packet}, nil, nil
		}
		r := idx.Ask(input.Entity, input.Intent)
		if r == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.Entity)}},
			}, nil, nil
		}
		r.Detail = input.Detail
		text := query.FormatAsk(r)
		// MCP's machine-readable contract stays bounded even when detail=true;
		// detail expands the human text only. This prevents a large document or
		// package entity from bypassing the token guard through raw JSON.
		structured := query.CompactAsk(r)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: structured,
		}, nil, nil
	})
}
