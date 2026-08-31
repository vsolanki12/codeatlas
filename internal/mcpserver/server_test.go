package mcpserver

import (
	"context"
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func TestRegisterToolsExposesAllTools(t *testing.T) {
	graphPath := t.TempDir() + "/graph.json"
	if err := storage.WriteGraph(graphPath, domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  "1.4.0",
		EntityIdentity: domain.CurrentEntityIdentity,
		Entities: []domain.Entity{{
			ID:   "function:example/pkg.Reconcile",
			Name: "Reconcile",
			Kind: domain.KindFunction,
			Source: domain.Source{
				Parser: "go",
				File:   "pkg/controller.go",
				Line:   10,
			},
		}},
	}); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "codeatlas", Version: "test"}, nil)
	registerTools(server, idx, graphPath)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	reviewResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "atlas_review",
		Arguments: map[string]any{
			"diff":      "diff --git a/pkg/controller.go b/pkg/controller.go\n@@ -10,1 +10,2 @@\n context\n+new behavior\n",
			"omit_diff": true,
		},
	})
	if err != nil {
		t.Fatalf("call atlas_review: %v", err)
	}
	if reviewResult.IsError || len(reviewResult.Content) == 0 || reviewResult.StructuredContent == nil {
		t.Fatalf("atlas_review result = %+v, want deterministic text and structured content", reviewResult)
	}

	freshnessResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "atlas_freshness",
		Arguments: map[string]any{"repo": t.TempDir()},
	})
	if err != nil {
		t.Fatalf("call atlas_freshness: %v", err)
	}
	if freshnessResult.IsError || len(freshnessResult.Content) == 0 || freshnessResult.StructuredContent == nil {
		t.Fatalf("atlas_freshness result = %+v, want deterministic text and structured content", freshnessResult)
	}

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	got := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		got = append(got, tool.Name)
	}
	sort.Strings(got)
	want := []string{
		"atlas_ask",
		"atlas_context",
		"atlas_entity",
		"atlas_explain",
		"atlas_freshness",
		"atlas_impact",
		"atlas_investigate",
		"atlas_review",
		"atlas_search",
		"atlas_stats",
		"atlas_temporal",
		"atlas_view",
		"atlas_where",
	}
	if len(got) != len(want) {
		t.Fatalf("tool count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tools = %v, want %v", got, want)
		}
	}
}
