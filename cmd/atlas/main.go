package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/vsolanki12/hypershift-atlas/internal/scanner"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: atlas scan [flags]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	repo := fs.String("repo", ".", "path to the repository root")
	output := fs.String("output", "atlas.json", "output file path")
	fs.Parse(args)

	result, err := scanner.Scan(*repo, *output)
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
