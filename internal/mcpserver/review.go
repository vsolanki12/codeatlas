package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/vsolanki12/codeatlas/internal/query"
	"github.com/vsolanki12/codeatlas/internal/review"
)

const maxMCPReviewDiffBytes = 4 * 1024 * 1024

type reviewInput struct {
	PR        string `json:"pr,omitempty" jsonschema:"GitHub pull request in owner/repository/number form"`
	Diff      string `json:"diff,omitempty" jsonschema:"raw unified diff text; use this instead of pr for a supplied diff"`
	Base      string `json:"base,omitempty" jsonschema:"base git ref for verified local review; requires repo"`
	BaseGraph string `json:"base_graph,omitempty" jsonschema:"graph scanned at the review merge base; local ref review only"`
	Head      string `json:"head,omitempty" jsonschema:"head git ref for verified local review; defaults to HEAD"`
	Repo      string `json:"repo,omitempty" jsonschema:"repository checkout path for verified local review"`
	Detail    bool   `json:"detail,omitempty" jsonschema:"true for verbose human-readable output; default is bounded"`
	OmitDiff  bool   `json:"omit_diff,omitempty" jsonschema:"true to omit the bounded diff excerpt and reduce tokens"`
}

func registerReview(s *mcp.Server, idx *query.Index, graphPath string, extractorBuild ...string) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "atlas_review",
		Description: "Run deterministic CodeAtlas PR review preparation. Choose exactly one mode: " +
			"pr=owner/repository/number (fetches GitHub metadata and diff), diff=raw unified diff " +
			"(optionally with repo for verification), or base plus repo for a local git-ref review. " +
			"Returns changed files/entities, graph relationships with evidence, blast radius, structural " +
			"test links, pattern observations, freshness, and explicit limitations. It never invokes an LLM " +
			"and never claims branch or runtime coverage. Default output is bounded; use detail=true only " +
			"when verbose text is needed.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, input reviewInput) (*mcp.CallToolResult, any, error) {
		if err := input.validate(); err != nil {
			return nil, nil, err
		}

		var (
			result *review.ReviewResult
			err    error
		)
		switch {
		case strings.TrimSpace(input.PR) != "":
			result, err = review.RunFromPR(strings.TrimSpace(input.PR), graphPath)
		case input.Diff != "":
			head := strings.TrimSpace(input.Head)
			if head == "" {
				head = "HEAD"
			}
			if repo := strings.TrimSpace(input.Repo); repo != "" {
				result, err = review.RunFromDiffTextInRepo(input.Diff, graphPath, strings.TrimSpace(input.Base), head, repo)
			} else {
				result, err = review.RunFromDiffText(input.Diff, graphPath, strings.TrimSpace(input.Base), head)
			}
		default:
			repo := strings.TrimSpace(input.Repo)
			if repo == "" {
				repo = strings.TrimSpace(idx.GraphMetadata().Repository)
			}
			if repo == "" {
				return nil, nil, fmt.Errorf("local ref review requires repo or a graph with repository metadata")
			}
			head := strings.TrimSpace(input.Head)
			if head == "" {
				head = "HEAD"
			}
			if input.BaseGraph != "" {
				result, err = review.RunWithBaseGraph(strings.TrimSpace(input.Base), head, repo, graphPath, input.BaseGraph, extractorBuild...)
			} else {
				result, err = review.Run(strings.TrimSpace(input.Base), head, repo, graphPath)
			}
		}
		if err != nil {
			return nil, nil, err
		}

		text := review.FormatReviewCompact(result, !input.OmitDiff)
		if input.Detail {
			verbose := result
			if input.OmitDiff {
				copyResult := *result
				copyResult.DiffExcerpt = ""
				copyResult.DiffExcerptTruncated = false
				verbose = &copyResult
			}
			text = review.FormatReview(verbose)
		}
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: text}},
			StructuredContent: review.CompactReview(result, !input.OmitDiff),
		}, nil, nil
	})
}

func (input reviewInput) validate() error {
	pr := strings.TrimSpace(input.PR)
	diff := input.Diff != ""
	base := strings.TrimSpace(input.Base)
	if input.BaseGraph != "" && (pr != "" || diff) {
		return fmt.Errorf("base_graph requires a verified local ref review")
	}
	if pr != "" {
		if diff || base != "" || strings.TrimSpace(input.Head) != "" || strings.TrimSpace(input.Repo) != "" {
			return fmt.Errorf("pr mode cannot be combined with diff, base, head, or repo")
		}
		if _, _, err := review.ParsePRRef(pr); err != nil {
			return err
		}
		return nil
	}
	if diff {
		if len(input.Diff) > maxMCPReviewDiffBytes {
			return fmt.Errorf("diff exceeds MCP limit of %d bytes; use pr mode or the atlas review CLI", maxMCPReviewDiffBytes)
		}
		return nil
	}
	if base == "" {
		return fmt.Errorf("review requires exactly one of pr, diff, or base")
	}
	if strings.TrimSpace(input.Repo) == "" {
		return fmt.Errorf("base review requires repo")
	}
	return nil
}
