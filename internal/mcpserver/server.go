package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/query"
)

func Run(ctx context.Context, graphPath string) error {
	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		return fmt.Errorf("load graph: %w", err)
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "codeatlas",
		Version: "0.2.0",
	}, nil)

	registerTools(server, idx)

	return server.Run(ctx, &mcp.StdioTransport{})
}

func registerTools(s *mcp.Server, idx *query.Index) {
	registerLookup(s, idx)
	registerEntity(s, idx)
	registerEntities(s, idx)
	registerRelationships(s, idx)
	registerContext(s, idx)
	registerSearch(s, idx)
	registerWhere(s, idx)
	registerStats(s, idx)
	registerHotspots(s, idx)
	registerCallers(s, idx)
	registerCommits(s, idx)
	registerInvestigate(s, idx)
	registerExplain(s, idx)
	registerImpact(s, idx)
}

type lookupInput struct {
	Kind string `json:"kind,omitempty" jsonschema:"entity kind: controller, crd, function, package, test, document, resource"`
	Name string `json:"name,omitempty" jsonschema:"name substring to match (case-insensitive)"`
}

func registerLookup(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_lookup",
		Description: "Find entities in the codebase by kind and/or name. Returns matching entities with file locations.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input lookupInput) (*mcp.CallToolResult, any, error) {
		results := idx.Lookup(input.Kind, input.Name, 20)
		text := query.FormatEntityList(results)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type entityInput struct {
	ID    string `json:"id" jsonschema:"exact entity ID, e.g. controller:hostedclusters.HostedClusterReconciler"`
	Brief bool   `json:"brief,omitempty" jsonschema:"if true, return only ID, file, line, description (saves tokens)"`
}

func registerEntity(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_entity",
		Description: "Get full details for a CodeAtlas entity by exact ID. Shows name, kind, package, file, description, watches, and calls. Use brief=true for compact output (ID, file, line only).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input entityInput) (*mcp.CallToolResult, any, error) {
		e := idx.GetEntity(input.ID)
		if e == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.ID)}},
			}, nil, nil
		}
		if input.Brief {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: query.FormatEntity(e) + "\n"}},
			}, nil, nil
		}
		text := query.FormatEntityFull(e)
		rels := idx.GetRelationships(input.ID, "both", "")
		if len(rels) > 0 {
			text += "Relationships:\n" + query.FormatRelationshipList(rels)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type entitiesInput struct {
	IDs []string `json:"ids" jsonschema:"list of entity IDs to fetch"`
}

func registerEntities(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_entities",
		Description: "Get brief details for multiple CodeAtlas entities by ID list. Returns ID, file, line for each. Use for batch lookups instead of multiple atlas_entity calls.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input entitiesInput) (*mcp.CallToolResult, any, error) {
		var lines []string
		for _, id := range input.IDs {
			e := idx.GetEntity(id)
			if e != nil {
				lines = append(lines, query.FormatEntity(e))
			}
		}
		text := query.FormatEntityList(nil)
		if len(lines) > 0 {
			text = joinLines(lines)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type relationshipsInput struct {
	EntityID  string `json:"entity_id" jsonschema:"entity ID to get relationships for"`
	Direction string `json:"direction,omitempty" jsonschema:"filter: from (outgoing), to (incoming), or both (default)"`
	Type      string `json:"type,omitempty" jsonschema:"filter by relationship type: reconciles, calls, tested_by, creates"`
}

func registerRelationships(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_relationships",
		Description: "Get relationships for a CodeAtlas entity. Shows connections like reconciles, calls, tested_by, creates.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input relationshipsInput) (*mcp.CallToolResult, any, error) {
		rels := idx.GetRelationships(input.EntityID, input.Direction, input.Type)
		text := query.FormatRelationshipList(rels)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
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
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type searchInput struct {
	Query string `json:"query" jsonschema:"search text, matches name, description, package, ID, imports, literals, and properties. Space-separated terms are AND-ed (all must match)."`
}

func registerSearch(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_search",
		Description: "Search all entities by text. Matches across name, description, package, ID, imports, literals, and properties. Space-separated terms are AND-ed (all must match).\n\nBest used when: Finding where something is implemented, locating code by concept or name. The default starting point for any \"where is X...\" question.\n\nUsually followed by: atlas_investigate (to understand each candidate).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input searchInput) (*mcp.CallToolResult, any, error) {
		results := idx.Search(input.Query, 20)
		text := query.FormatEntityList(results)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type whereInput struct {
	Path   string `json:"path" jsonschema:"file path substring to search for"`
	Detail bool   `json:"detail,omitempty" jsonschema:"if true, return full entity details (name, calls, watches, description) instead of brief ID|file:line. Use for deep-diving a single file."`
}

func registerWhere(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_where",
		Description: "Find entities by file path. Returns entities defined in files matching the path substring. Use detail=true to get full entity info (replaces multiple atlas_entity calls for single-file investigation).\n\nBest used when: Exploring a specific file or directory, understanding what lives in a location.\n\nUsually followed by: atlas_investigate (to deep-dive a specific entity).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input whereInput) (*mcp.CallToolResult, any, error) {
		results := idx.Where(input.Path, 30)
		var text string
		if input.Detail {
			text = query.FormatEntityDetailList(results)
		} else {
			text = query.FormatEntityList(results)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type hotspotsInput struct {
	Kind  string `json:"kind,omitempty" jsonschema:"entity kind filter: controller, function, package, etc."`
	Stale bool   `json:"stale,omitempty" jsonschema:"if true, sort by oldest modification instead of most changes"`
}

func registerHotspots(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_hotspots",
		Description: "Find most-changed or stalest entities by git history. Requires --temporal scan.\n\nBest used when: Identifying test gaps (high-change + no tests = risk), finding code that needs attention, prioritizing review effort.\n\nUsually followed by: atlas_impact (to check test coverage for each hotspot).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input hotspotsInput) (*mcp.CallToolResult, any, error) {
		results := idx.Hotspots(input.Kind, input.Stale, 20)
		if len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No temporal data. Re-scan with --temporal flag."}},
			}, nil, nil
		}
		var lines []string
		for _, e := range results {
			lines = append(lines, fmt.Sprintf("%s | %s:%d | changes=%d last=%s by=%s",
				e.ID, e.Source.File, e.Source.Line, e.ChangeCount, e.LastModified, e.LastAuthor))
		}
		text := fmt.Sprintf("%d hotspots:\n%s\n", len(results), joinLines(lines))
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
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
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type callersInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to find callers of"`
}

func registerCallers(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_callers",
		Description: "Find all functions/methods that call the function at a position. Returns callers with file locations.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input callersInput) (*mcp.CallToolResult, any, error) {
		results := idx.Callers(input.EntityID)
		text := query.FormatEntityList(results)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type commitsInput struct {
	Name   string `json:"name,omitempty" jsonschema:"function/entity name substring to filter"`
	Since  string `json:"since,omitempty" jsonschema:"ISO date cutoff, e.g. 2026-05-01"`
	Author string `json:"author,omitempty" jsonschema:"author email/name substring"`
}

func registerCommits(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_commits",
		Description: "Search entities by git history: who changed what and when. Requires --temporal scan.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input commitsInput) (*mcp.CallToolResult, any, error) {
		results := idx.Commits(input.Name, input.Since, input.Author, 20)
		if len(results) == 0 {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "No matching entities with temporal data.\n"}},
			}, nil, nil
		}
		var lines []string
		for _, e := range results {
			lines = append(lines, fmt.Sprintf("%s | %s:%d | changes=%d last=%s by=%s",
				e.ID, e.Source.File, e.Source.Line, e.ChangeCount, e.LastModified, e.LastAuthor))
		}
		text := joinLines(lines)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type investigateInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to investigate"`
}

func registerInvestigate(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_investigate",
		Description: "Get everything about an entity in one call: full details, all relationships grouped by type, callers, tests, and same-file siblings. Replaces 4-5 primitive tool calls.\n\nBest used when: Investigating a bug, debugging, understanding why something is broken. The default starting point for any \"why is X...\" question.\n\nExamples: Why is reconciliation failing? What calls this function? What tests cover this?\n\nUsually followed by: atlas_explain (to trace architecture), atlas_impact (to assess blast radius).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input investigateInput) (*mcp.CallToolResult, any, error) {
		r := idx.Investigate(input.EntityID)
		if r == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatInvestigation(r)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type explainInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to explain"`
	Depth    int    `json:"depth,omitempty" jsonschema:"traversal depth (default 2, max 3)"`
}

func registerExplain(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_explain",
		Description: "Follow the reconciliation chain from an entity: reconciles, creates, calls, tested_by. Returns a tree showing the architectural narrative. Replaces reading source files to understand flow.\n\nBest used when: Learning a subsystem, onboarding, understanding how components connect. The default starting point for any \"how does X work...\" question.\n\nExamples: Explain HostedCluster. How does CPO work? What creates etcd?\n\nUsually followed by: atlas_investigate (to deep-dive a specific entity found in the tree).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input explainInput) (*mcp.CallToolResult, any, error) {
		r := idx.Explain(input.EntityID, input.Depth)
		if r.Root == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatExplanation(r)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}

type impactInput struct {
	EntityID string `json:"entity_id" jsonschema:"entity ID to analyze blast radius for"`
}

func registerImpact(s *mcp.Server, idx *query.Index) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_impact",
		Description: "Blast radius analysis: walk the call chain upstream to find all controllers, tests, resources, files, and owners affected by changing this entity. Use for PR review preparation.\n\nBest used when: Reviewing a PR, assessing change risk, preparing for code review. The default starting point for any \"will this break...\" question.\n\nExamples: Review this PR. What does this change affect? Who owns this code?\n\nUsually followed by: atlas_callers (to verify caller test coverage), atlas_commits (to check change history).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input impactInput) (*mcp.CallToolResult, any, error) {
		r := idx.Impact(input.EntityID)
		if r == nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Entity not found: %s", input.EntityID)}},
			}, nil, nil
		}
		text := query.FormatImpact(r)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: text}},
		}, nil, nil
	})
}
