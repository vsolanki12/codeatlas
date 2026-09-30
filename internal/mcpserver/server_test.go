package mcpserver

import (
	"context"
	"encoding/json"
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
		}, {
			ID:     "function:example/pkg.WidgetOne",
			Name:   "WidgetOne",
			Kind:   domain.KindFunction,
			Source: domain.Source{Parser: "go", File: "pkg/widget_one.go", Line: 1},
		}, {
			ID:     "function:example/pkg.WidgetTwo",
			Name:   "WidgetTwo",
			Kind:   domain.KindFunction,
			Source: domain.Source{Parser: "go", File: "pkg/widget_two.go", Line: 1},
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

	searchResult, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "atlas_search",
		Arguments: map[string]any{"query": "Widget", "limit": 1},
	})
	if err != nil {
		t.Fatalf("call atlas_search: %v", err)
	}
	var page struct {
		Total      int  `json:"total"`
		Offset     int  `json:"offset"`
		Limit      int  `json:"limit"`
		NextOffset int  `json:"nextOffset"`
		Truncated  bool `json:"truncated"`
	}
	encoded, err := json.Marshal(searchResult.StructuredContent)
	if err != nil || json.Unmarshal(encoded, &page) != nil {
		t.Fatalf("decode atlas_search structured result: %s (%v)", encoded, err)
	}
	if page.Total != 2 || page.Offset != 0 || page.Limit != 1 || page.NextOffset != 1 || !page.Truncated {
		t.Fatalf("search page = %+v, want total=2 offset=0 limit=1 nextOffset=1 truncated", page)
	}
	filtered, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "atlas_search", Arguments: map[string]any{"query": "Widget example/pkg", "kind": "function"},
	})
	if err != nil || filtered.IsError {
		t.Fatalf("kind-filtered full-text search: %v %+v", err, filtered)
	}
	encoded, _ = json.Marshal(filtered.StructuredContent)
	if err := json.Unmarshal(encoded, &page); err != nil || page.Total != 2 {
		t.Fatalf("kind filtering changed multi-term search semantics: %s", encoded)
	}
	for _, tc := range []struct{ entity, status string }{
		{"function:example/pkg.Reconcile", "ok"},
		{"function:example/pkg.DoesNotExist", "no_match"},
	} {
		result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
			Name: "atlas_ask", Arguments: map[string]any{"entity": tc.entity, "evidence": true, "budget_bytes": 4096},
		})
		if err != nil || result.IsError {
			t.Fatalf("evidence retrieval: %v %+v", err, result)
		}
		encoded, _ := json.Marshal(result.StructuredContent)
		var packet query.EvidencePacket
		if err := json.Unmarshal(encoded, &packet); err != nil {
			t.Fatal(err)
		}
		if packet.Version != "1.0" || packet.Status != tc.status || len(encoded) > 4096 {
			t.Fatalf("evidence contract = %s", encoded)
		}
		if tc.status == "ok" && len(packet.Sources) == 0 {
			t.Fatal("resolved entity lost its source span")
		}
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
