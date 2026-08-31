package mcpserver

import (
	"strings"
	"testing"
)

func TestReviewInputValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   reviewInput
		wantErr string
	}{
		{name: "PR", input: reviewInput{PR: "openshift/hypershift/8968"}},
		{name: "diff", input: reviewInput{Diff: "diff --git a/a.go b/a.go\n"}},
		{name: "verified diff", input: reviewInput{Diff: "diff", Repo: "/repo", Head: "HEAD"}},
		{name: "local refs", input: reviewInput{Base: "origin/main", Repo: "/repo"}},
		{name: "invalid PR", input: reviewInput{PR: "not-a-pr"}, wantErr: "invalid PR reference"},
		{name: "PR with diff", input: reviewInput{PR: "acme/project/1", Diff: "diff"}, wantErr: "cannot be combined"},
		{name: "empty", input: reviewInput{}, wantErr: "requires exactly one"},
		{name: "local refs without repo", input: reviewInput{Base: "origin/main"}, wantErr: "requires repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validate returned error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validate error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestReviewInputRejectsOversizedDiff(t *testing.T) {
	input := reviewInput{Diff: strings.Repeat("x", maxMCPReviewDiffBytes+1)}
	if err := input.validate(); err == nil || !strings.Contains(err.Error(), "exceeds MCP limit") {
		t.Fatalf("validate error = %v, want oversized diff error", err)
	}
}
