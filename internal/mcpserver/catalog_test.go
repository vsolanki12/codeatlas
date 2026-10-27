package mcpserver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func writeCatalogGraph(t *testing.T, dir, module string) string {
	t.Helper()
	path := filepath.Join(dir, module+".json")
	if err := storage.WriteGraph(path, domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  domain.CurrentSchemaVersion,
		EntityIdentity: domain.CurrentEntityIdentity,
		Repository:     "/repo/" + module,
		RepositoryID:   "go:example.com/" + module,
		GoModule:       "example.com/" + module,
		Entities: []domain.Entity{{
			ID:     fmt.Sprintf("function:example.com/%s.Run", module),
			Name:   "Run",
			Kind:   domain.KindFunction,
			Source: domain.Source{Parser: "go", File: "run.go", Line: 1},
		}},
	}); err != nil {
		t.Fatalf("write graph: %v", err)
	}
	return path
}

// Startup must cost a header read per repository, not a full decode. A server
// configured with several large repositories otherwise holds every graph
// resident before any tool is called.
func TestLoadGraphCatalogDoesNotDecodeGraphsUntilSelected(t *testing.T) {
	dir := t.TempDir()
	first := writeCatalogGraph(t, dir, "alpha")
	second := writeCatalogGraph(t, dir, "beta")

	catalog, err := loadGraphCatalog([]string{first, second})
	if err != nil {
		t.Fatalf("loadGraphCatalog: %v", err)
	}
	for _, target := range catalog.targets {
		if target.Index != nil {
			t.Fatalf("target %s was decoded at startup", target.Key)
		}
		if target.GoModule == "" || target.RepositoryID == "" {
			t.Fatalf("target %s lost its identity: %+v", target.Key, target)
		}
	}

	target, err := catalog.resolve("example.com/alpha")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.Index == nil {
		t.Fatal("resolve must return a decoded target")
	}
	if catalog.targets[1].Index != nil {
		t.Fatal("resolving one repository must not decode the others")
	}
}

// atlas_stats with no selector is the one handler that reaches into
// catalog.targets instead of going through resolve, so it has to load each
// target itself.
func TestStatsAcrossRepositoriesLoadsEveryGraph(t *testing.T) {
	dir := t.TempDir()
	catalog, err := loadGraphCatalog([]string{
		writeCatalogGraph(t, dir, "alpha"),
		writeCatalogGraph(t, dir, "beta"),
	})
	if err != nil {
		t.Fatalf("loadGraphCatalog: %v", err)
	}

	server := mcp.NewServer(&mcp.Implementation{Name: "codeatlas", Version: "test"}, nil)
	registerCatalogTools(server, catalog)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx := context.Background()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "atlas_stats"})
	if err != nil {
		t.Fatalf("atlas_stats across repositories: %v", err)
	}
	if result.IsError {
		t.Fatalf("atlas_stats returned an error result: %+v", result.Content)
	}
}
