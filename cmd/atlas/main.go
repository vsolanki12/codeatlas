package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vsolanki12/codeatlas/internal/mcpserver"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/scanner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: atlas <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: scan, query, context, where, stats, serve")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
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

	result, err := scanner.Scan(*repo, *output, scanner.ScanOptions{Temporal: *temporal})
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Scan complete: %d entities, %d relationships (%s)\n",
		result.EntityCount, result.RelCount, result.Duration.Round(time.Millisecond))

	if len(result.Warnings) > 0 {
		fmt.Printf("Warnings (%d):\n", len(result.Warnings))
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}

	fmt.Printf("Output: %s\n", *output)
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
		os.Exit(1)
	}

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	results := idx.Lookup(kind, name, 20)
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
