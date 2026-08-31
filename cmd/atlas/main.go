package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/freshness"
	"github.com/vsolanki12/codeatlas/internal/mcpserver"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/review"
	"github.com/vsolanki12/codeatlas/internal/scanner"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: atlas <command> [flags]")
		fmt.Fprintln(os.Stderr, "commands: scan, search, explain, impact, investigate, ask, view,")
		fmt.Fprintln(os.Stderr, "          context, where, stats, freshness, serve, query, review")
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
	case "freshness":
		runFreshness(os.Args[2:])
	case "review":
		runReview(os.Args[2:])
	case "serve":
		runServe(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func reorderArgs(args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			flags = append(flags, args[i])
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") && strings.Contains(args[i], "=") == false {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			positional = append(positional, args[i])
		}
	}
	return append(flags, positional...)
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	repo := fs.String("repo", ".", "path to the repository root")
	output := fs.String("output", "atlas.json", "output file path")
	temporal := fs.Bool("temporal", false, "enrich entities with git history")
	fs.Parse(reorderArgs(args))

	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}

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
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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
	var truncated bool
	if q != "" {
		if *kind != "" {
			results, truncated = idx.LookupWithStatus(*kind, q, 20)
		} else {
			results, truncated = idx.SearchWithStatus(q, 20)
		}
	} else {
		results, truncated = idx.LookupWithStatus(*kind, "", 20)
	}

	if *jsonOutput {
		if *compact {
			result := idx.CompactEntityListResult(results, false, 0)
			result.Truncated = result.Truncated || truncated
			printJSON(result)
			return
		}
		result := idx.EntityListResult(results, false, 0)
		result.Truncated = result.Truncated || truncated
		printJSON(result)
		return
	}

	fmt.Print(query.FormatEntityList(results))
	if truncated {
		fmt.Println("[TRUNCATED: result limit reached; omitted entities are not evidence of absence.]")
	}
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	depth := fs.Int("depth", 2, "traversal depth (max 3)")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactExplain(result))
			return
		}
		printJSON(result)
		return
	}
	fmt.Print(query.FormatExplanation(result))
}

func runImpact(args []string) {
	fs := flag.NewFlagSet("impact", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactImpact(result))
			return
		}
		printJSON(result)
		return
	}
	fmt.Print(query.FormatImpact(result))
}

func runInvestigate(args []string) {
	fs := flag.NewFlagSet("investigate", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactInvestigate(result))
			return
		}
		printJSON(result)
		return
	}
	fmt.Print(query.FormatInvestigation(result))
}

func runAsk(args []string) {
	fs := flag.NewFlagSet("ask", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	intent := fs.String("intent", "", "understand, impact, or debug (default: view only)")
	detail := fs.Bool("detail", false, "full verbose output (no compact formatting)")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

	entity := fs.Arg(0)
	if entity == "" {
		fmt.Fprintln(os.Stderr, "usage: atlas ask <entity> [-intent understand|impact|debug] [-detail] [-graph path]")
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
	result.Detail = *detail
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactAsk(result))
			return
		}
		printJSON(result)
		return
	}
	fmt.Print(query.FormatAsk(result))
}

func runView(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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

	v := idx.ResolveView(entity)
	if v == nil {
		fmt.Fprintf(os.Stderr, "no view found for: %s\n", entity)
		os.Exit(1)
	}
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactViewResult(v))
			return
		}
		printJSON(v)
		return
	}
	fmt.Print(query.FormatView(v))
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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

	results, resultTruncated := idx.LookupWithStatus(kind, name, 20)
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
	if *jsonOutput {
		if *compact {
			result := idx.CompactEntityListResult(results, true, 40)
			result.Truncated = result.Truncated || resultTruncated
			printJSON(result)
			return
		}
		result := idx.EntityListResult(results, true, 40)
		result.Truncated = result.Truncated || resultTruncated
		printJSON(result)
		return
	}
	fmt.Print(query.FormatEntityList(results))
	entityResult := idx.EntityListResult(results, true, 40)
	if len(entityResult.Relationships) > 0 {
		fmt.Print(query.FormatRelationshipList(entityResult.Relationships))
	}
	if entityResult.Truncated {
		fmt.Println("[TRUNCATED: relationship context capped; omitted relationships are not evidence of absence.]")
	}
	if resultTruncated {
		fmt.Println("[TRUNCATED: result limit reached; omitted entities are not evidence of absence.]")
	}
}

func runContext(args []string) {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	depth := fs.Int("depth", 1, "BFS traversal depth")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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
	if *jsonOutput {
		if *compact {
			printJSON(query.CompactSubgraph(sg))
			return
		}
		printJSON(sg)
		return
	}
	fmt.Print(query.FormatSubgraph(sg))
}

func runWhere(args []string) {
	fs := flag.NewFlagSet("where", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	compact := fs.Bool("compact", false, "emit bounded JSON with entity summaries")
	fs.Parse(reorderArgs(args))

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

	results, resultTruncated := idx.WhereWithStatus(symbol, 30)
	if *jsonOutput {
		if *compact {
			result := idx.CompactEntityListResult(results, false, 0)
			result.Truncated = result.Truncated || resultTruncated
			printJSON(result)
			return
		}
		result := idx.EntityListResult(results, false, 0)
		result.Truncated = result.Truncated || resultTruncated
		printJSON(result)
		return
	}
	fmt.Print(query.FormatEntityList(results))
	if resultTruncated {
		fmt.Println("[TRUNCATED: result limit reached; omitted entities are not evidence of absence.]")
	}
}

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(reorderArgs(args))

	idx, err := query.LoadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}

	stats := idx.Stats()
	if *jsonOutput {
		printJSON(stats)
		return
	}
	fmt.Print(query.FormatStats(stats))
}

func runFreshness(args []string) {
	fs := flag.NewFlagSet("freshness", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	repo := fs.String("repo", "", "path to the repository checkout")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(reorderArgs(args))

	g, err := storage.ReadGraph(*graphPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load graph: %v\n", err)
		os.Exit(1)
	}
	result := freshness.CheckWithGraphPath(*repo, g, *graphPath)
	if *jsonOutput {
		printJSON(result)
		return
	}
	fmt.Fprintf(os.Stdout, "repository: %s\n", result.Repository)
	fmt.Fprintf(os.Stdout, "graph repository: %s\n", result.GraphRepository)
	fmt.Fprintf(os.Stdout, "graph commit: %s\n", result.GraphCommit)
	fmt.Fprintf(os.Stdout, "repo HEAD: %s\n", result.RepoHead)
	fmt.Fprintf(os.Stdout, "entity identity: %s\n", result.EntityIdentity)
	fmt.Fprintf(os.Stdout, "scan: %s\n", freshnessStatus(result.ScanComplete))
	fmt.Fprintf(os.Stdout, "repository match: %t\n", result.RepositoryMatch)
	fmt.Fprintf(os.Stdout, "state verification: %t\n", result.StateVerifiable)
	fmt.Fprintf(os.Stdout, "dirty: %t\n", result.Dirty)
	for _, path := range result.ChangedFiles {
		fmt.Fprintf(os.Stdout, "changed file: %s\n", path)
	}
	for _, path := range result.NewFiles {
		fmt.Fprintf(os.Stdout, "new file: %s\n", path)
	}
	for _, path := range result.DeletedFiles {
		fmt.Fprintf(os.Stdout, "deleted file: %s\n", path)
	}
}

func freshnessStatus(complete bool) string {
	if complete {
		return "complete"
	}
	return "incomplete"
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	fs.Parse(reorderArgs(args))

	if err := mcpserver.Run(context.Background(), *graphPath); err != nil {
		fmt.Fprintf(os.Stderr, "serve failed: %v\n", err)
		os.Exit(1)
	}
}

func runReview(args []string) {
	fs := flag.NewFlagSet("review", flag.ExitOnError)
	graphPath := fs.String("graph", "atlas.json", "path to graph JSON")
	base := fs.String("base", "", "base git ref (e.g., upstream/main)")
	head := fs.String("head", "HEAD", "head git ref")
	repo := fs.String("repo", "", "path to the git repository (required for verified diff review)")
	diffSource := fs.String("diff", "", "read diff from file or stdin (-)")
	pr := fs.String("pr", "", "fetch a GitHub pull request (owner/repository/number)")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	fs.Parse(reorderArgs(args))

	var result *review.ReviewResult
	var err error

	if *pr != "" {
		if *diffSource != "" || *base != "" || *repo != "" || *head != "HEAD" {
			fmt.Fprintln(os.Stderr, "usage error: --pr cannot be combined with --base, --head, --repo, or --diff")
			os.Exit(1)
		}
		result, err = review.RunFromPR(*pr, *graphPath)
	} else if *diffSource != "" {
		if *repo != "" {
			result, err = review.RunFromDiffInRepo(*diffSource, *graphPath, *base, *head, *repo)
		} else {
			result, err = review.RunFromDiff(*diffSource, *graphPath, *base, *head)
		}
	} else {
		if *base == "" {
			fmt.Fprintln(os.Stderr, "usage: atlas review --base <ref> [--head <ref>] [--graph path] [--repo path]")
			fmt.Fprintln(os.Stderr, "       atlas review --diff <file|-> [--graph path]")
			fmt.Fprintln(os.Stderr, "       atlas review --pr owner/repository/number --graph path")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "  --base: base git ref to compare against")
			fmt.Fprintln(os.Stderr, "  --diff: read diff from file or stdin (- for pipe)")
			fmt.Fprintln(os.Stderr, "  --pr: fetch GitHub metadata and diff (owner/repository/number)")
			os.Exit(1)
		}
		if *repo == "" {
			*repo = "."
		}
		result, err = review.Run(*base, *head, *repo, *graphPath)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "review failed: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		printJSON(result)
		return
	}
	fmt.Print(review.FormatReview(result))
}

func printJSON(value any) {
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintf(os.Stderr, "JSON output failed: %v\n", err)
		os.Exit(1)
	}
}

func resolveEntity(idx *query.Index, nameOrID string) *domain.Entity {
	entity, _ := idx.Resolve(nameOrID)
	return entity
}
