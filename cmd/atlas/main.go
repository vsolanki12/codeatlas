package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/mcpserver"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/scanner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: atlas <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: scan, search, explain, impact, investigate, ask, view,")
		fmt.Fprintln(os.Stderr, "          context, where, stats, serve, query")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	case "search":
		runSearch(os.Args[2:])
	case "explain":
		runExplain(os.Args[2:])
	case "impact":
		runImpact(os.Args[2:])
	case "investigate":
		runInvestigate(os.Args[2:])
	case "ask":
		runAsk(os.Args[2:])
	case "view":
		runView(os.Args[2:])
	case "query":
		runQuery(os.Args[2:])
	case "context":
		runContext(os.Args[2:])
	case "where":
		runWhere(os.Args[2:])
	case "stats":
		runStats(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	repo := fs.String("repo", ".", "path to the repository root")
	output := fs.String("output", "atlas.json", "output file path")
	temporal := fs.Bool("temporal", false, "enrich entities with git history")
	fs.Parse(args)

	opts := scanner.ScanOptions{Temporal: *temporal}
	if absOut, err := filepath.Abs(*output); err == nil {
		if _, err := os.Stat(absOut); err == nil {
			opts.PreviousGraph = absOut
		}
	}

	result, err := scanner.Scan(*repo, *output, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	if result.Incremental {
		fmt.Printf("Incremental scan: %d changed, %d reused, %d deleted (%s)\n",
			result.ChangedFiles, result.ReusedFiles, result.DeletedFiles, result.Duration.Round(time.Millisecond))
	} else {
		fmt.Printf("Full scan (%s)\n", result.Duration.Round(time.Millisecond))
	}
	fmt.Printf("%d entities, %d relationships\n", result.EntityCount, result.RelCount)

	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings (%d):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Printf("Output: %s\n", *output)
}

func runSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	kind := fs.String("kind", "", "filter by entity kind (controller, function, crd, etc.)")
	fs.Parse(args)

	q := fs.Arg(0)
	if q == "" && *kind == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas search <query> [--kind kind] [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	var results []*domain.Entity
	if q != "" {
		results = idx.Search(q, 20)
	} else {
		results = idx.Lookup(*kind, "", 20)
	}

	if *kind != "" && q != "" {
		filtered := results[:0]
		for _, e := range results {
			if e.Kind.String() == *kind {
				filtered = append(filtered, e)
			}
		}
		results = filtered
	}

	fmt.Print(query.FormatEntityList(results))
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	depth := fs.Int("depth", 2, "traversal depth (max 3)")
	fs.Parse(args)

	entityID := fs.Arg(0)
	if entityID == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas explain <entity-id-or-name> [--depth N] [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	entity := resolveEntity(idx, entityID)
	if entity == nil {
		fmt.Fprintf(os.Stderr, "entity not found: %s\n", entityID)
		os.Exit(1)
	}

	result := idx.Explain(entity.ID, *depth)
	if result == nil {
		fmt.Fprintf(os.Stderr, "no explanation for: %s\n", entity.ID)
		os.Exit(1)
	}
	fmt.Print(query.FormatExplanation(result))
}

func runImpact(args []string) {
	fs := flag.NewFlagSet("impact", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	entityID := fs.Arg(0)
	if entityID == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas impact <entity-id-or-name> [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	entity := resolveEntity(idx, entityID)
	if entity == nil {
		fmt.Fprintf(os.Stderr, "entity not found: %s\n", entityID)
		os.Exit(1)
	}

	result := idx.Impact(entity.ID)
	if result == nil {
		fmt.Fprintf(os.Stderr, "no impact data for: %s\n", entity.ID)
		os.Exit(1)
	}
	fmt.Print(query.FormatImpact(result))
}

func runInvestigate(args []string) {
	fs := flag.NewFlagSet("investigate", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	entityID := fs.Arg(0)
	if entityID == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas investigate <entity-id-or-name> [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	entity := resolveEntity(idx, entityID)
	if entity == nil {
		fmt.Fprintf(os.Stderr, "entity not found: %s\n", entityID)
		os.Exit(1)
	}

	result := idx.Investigate(entity.ID)
	if result == nil {
		fmt.Fprintf(os.Stderr, "no data for: %s\n", entity.ID)
		os.Exit(1)
	}
	fmt.Print(query.FormatInvestigation(result))
}

func runAsk(args []string) {
	fs := flag.NewFlagSet("ask", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	intent := fs.String("intent", "", "understand, impact, or debug (default: view only)")
	fs.Parse(args)

	entity := fs.Arg(0)
	if entity == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas ask <entity> [--intent understand|impact|debug] [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	result := idx.Ask(entity, *intent)
	if result == nil {
		fmt.Fprintf(os.Stderr, "entity not found: %s\n", entity)
		os.Exit(1)
	}
	fmt.Print(query.FormatAsk(result))
}

func runView(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	entity := fs.Arg(0)
	if entity == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas view <entity-id-or-name> [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	v := idx.GetView(entity)
	if v == nil {
		v = idx.SearchView(entity)
	}
	if v == nil {
		fmt.Fprintf(os.Stderr, "no view found for: %s\n", entity)
		os.Exit(1)
	}
	fmt.Print(query.FormatView(v))
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	remaining := fs.Args()
	var kind, name string
	if len(remaining) >= 1 {
		kind = remaining[0]
	}
	if len(remaining) >= 2 {
		name = remaining[1]
	}

	if kind == "" && name == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas query <kind> [name] [--graph path]")
		fmt.Fprintln(os.Stderr, "  kind: controller, function, crd, test, package, document, resource")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "tip: use 'atlas search', 'atlas explain', 'atlas impact' for richer queries")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	results := idx.Lookup(kind, name, 20)
	if len(results) == 0 {
		fmt.Fprintf(os.Stderr, "no entities of kind %q", kind)
		if name != "" {
			fmt.Fprintf(os.Stderr, " matching %q", name)
		}
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "valid kinds: controller, function, crd, test, package, document, resource")
		fmt.Fprintln(os.Stderr, "tip: use 'atlas search' for text search across all entity types")
		os.Exit(1)
	}
	fmt.Print(query.FormatEntityList(results))

	for _, e := range results {
		rels := idx.GetRelationships(e.ID, "both", "")
		if len(rels) > 0 {
			fmt.Print(query.FormatRelationshipList(rels))
		}
	}
}

func runContext(args []string) {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	depth := fs.Int("depth", 1, "BFS traversal depth")
	fs.Parse(args)

	entityID := fs.Arg(0)
	if entityID == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas context <entity-id> [--depth N] [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	sg := idx.Neighbors(entityID, *depth)
	fmt.Print(query.FormatSubgraph(sg))
}

func runWhere(args []string) {
	fs := flag.NewFlagSet("where", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	symbol := fs.Arg(0)
	if symbol == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas where <symbol-or-path> [--graph path]")
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	results := idx.Where(symbol, 30)
	fmt.Print(query.FormatEntityList(results))
}

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(query.FormatStats(idx.Stats()))
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(args)

	if err := mcpserver.Run(context.Background(), *graphPath); err != nil {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func resolveEntity(idx *query.Index, nameOrID string) *domain.Entity {
	if e := idx.GetEntity(nameOrID); e != nil {
		return e
	}
	results := idx.Search(nameOrID, 1)
	if len(results) > 0 {
		return results[0]
	}
	return nil
}
