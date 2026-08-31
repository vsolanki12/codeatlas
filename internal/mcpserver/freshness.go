package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/freshness"
	"github.com/vsolanki12/codeatlas/internal/query"
)

const maxMCPFreshnessFiles = 50

type freshnessInput struct {
	Repo string `json:"repo,omitempty" jsonschema:"absolute path to the repository checkout; defaults to the repository recorded in the graph"`
}

type compactFreshnessResult struct {
	Available         bool     `json:"available"`
	GraphRepository   string   `json:"graphRepository,omitempty"`
	Repository        string   `json:"repository,omitempty"`
	GraphCommit       string   `json:"graphCommit,omitempty"`
	RepoHead          string   `json:"repoHead,omitempty"`
	EntityIdentity    string   `json:"entityIdentity,omitempty"`
	ScanComplete      bool     `json:"scanComplete"`
	RepositoryMatch   bool     `json:"repositoryMatch"`
	Verifiable        bool     `json:"verifiable"`
	Stale             bool     `json:"stale"`
	Dirty             bool     `json:"dirty"`
	StateVerifiable   bool     `json:"stateVerifiable"`
	FingerprintSource string   `json:"fingerprintSource,omitempty"`
	ChangedFiles      []string `json:"changedFiles,omitempty"`
	NewFiles          []string `json:"newFiles,omitempty"`
	DeletedFiles      []string `json:"deletedFiles,omitempty"`
	Truncated         bool     `json:"truncated,omitempty"`
}

func registerFreshness(s *mcp.Server, idx *query.Index, graphPath string) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "atlas_freshness",
		Description: "Verify whether the loaded CodeAtlas graph describes a repository checkout. " +
			"Use before implementation guidance or verified review. This is read-only. " +
			"A dirty, stale, incomplete, or unverifiable result is a limitation, not proof that a fact is absent.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input freshnessInput) (*mcp.CallToolResult, any, error) {
		repo := strings.TrimSpace(input.Repo)
		result := freshness.CheckWithGraphPath(repo, idx.Graph(), graphPath)
		compact := compactFreshness(result)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: formatFreshness(compact)}},
			StructuredContent: compact,
		}, nil, nil
	})
}

func compactFreshness(result freshness.Result) compactFreshnessResult {
	compact := compactFreshnessResult{
		Available:         result.Available,
		GraphRepository:   result.GraphRepository,
		Repository:        result.Repository,
		GraphCommit:       result.GraphCommit,
		RepoHead:          result.RepoHead,
		EntityIdentity:    result.EntityIdentity,
		ScanComplete:      result.ScanComplete,
		RepositoryMatch:   result.RepositoryMatch,
		Verifiable:        result.Verifiable,
		Stale:             result.Stale,
		Dirty:             result.Dirty,
		StateVerifiable:   result.StateVerifiable,
		FingerprintSource: result.FingerprintSource,
	}
	compact.ChangedFiles, compact.Truncated = boundedFreshnessFiles(result.ChangedFiles, compact.Truncated)
	compact.NewFiles, compact.Truncated = boundedFreshnessFiles(result.NewFiles, compact.Truncated)
	compact.DeletedFiles, compact.Truncated = boundedFreshnessFiles(result.DeletedFiles, compact.Truncated)
	return compact
}

func boundedFreshnessFiles(files []string, truncated bool) ([]string, bool) {
	if len(files) > maxMCPFreshnessFiles {
		files = files[:maxMCPFreshnessFiles]
		truncated = true
	}
	return append([]string(nil), files...), truncated
}

func formatFreshness(result compactFreshnessResult) string {
	var b strings.Builder
	b.WriteString("CODEATLAS GRAPH FRESHNESS\n")
	b.WriteString("========================\n")
	fmt.Fprintf(&b, "Graph repository: %s\n", result.GraphRepository)
	fmt.Fprintf(&b, "Repository checked: %s\n", result.Repository)
	fmt.Fprintf(&b, "Graph commit: %s\n", result.GraphCommit)
	fmt.Fprintf(&b, "Repository HEAD: %s\n", result.RepoHead)
	fmt.Fprintf(&b, "Entity identity: %s\n", result.EntityIdentity)
	fmt.Fprintf(&b, "Scan complete: %t\n", result.ScanComplete)
	fmt.Fprintf(&b, "Repository match: %t\n", result.RepositoryMatch)
	fmt.Fprintf(&b, "Commit verifiable: %t\n", result.Verifiable)
	fmt.Fprintf(&b, "Stale: %t\n", result.Stale)
	fmt.Fprintf(&b, "Checkout dirty: %t\n", result.Dirty)
	fmt.Fprintf(&b, "File state verifiable: %t\n", result.StateVerifiable)
	if result.FingerprintSource != "" {
		fmt.Fprintf(&b, "Fingerprint source: %s\n", result.FingerprintSource)
	}
	formatFreshnessFiles(&b, "Changed files", result.ChangedFiles)
	formatFreshnessFiles(&b, "New files", result.NewFiles)
	formatFreshnessFiles(&b, "Deleted files", result.DeletedFiles)
	if result.Truncated {
		b.WriteString("[TRUNCATED: file lists capped; omitted files are not evidence of absence.]\n")
	}
	return b.String()
}

func formatFreshnessFiles(b *strings.Builder, label string, files []string) {
	if len(files) == 0 {
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, file := range files {
		fmt.Fprintf(b, "- %s\n", file)
	}
}
