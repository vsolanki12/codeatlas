package review

import (
	"strings"
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/storage"
)

func TestParsePRRef(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		repository string
		number     int
		wantErr    bool
	}{
		{name: "valid", input: "openshift/hypershift/8968", repository: "openshift/hypershift", number: 8968},
		{name: "trimmed", input: " /acme/project/7/ ", repository: "acme/project", number: 7},
		{name: "missing component", input: "acme/project", wantErr: true},
		{name: "invalid number", input: "acme/project/nope", wantErr: true},
		{name: "zero number", input: "acme/project/0", wantErr: true},
		{name: "unsafe repository", input: "acme/project?x/7", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository, number, err := ParsePRRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ParsePRRef returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePRRef returned error: %v", err)
			}
			if repository != tt.repository || number != tt.number {
				t.Fatalf("ParsePRRef = %s/%d, want %s/%d", repository, number, tt.repository, tt.number)
			}
		})
	}
}

func TestRunFromPRWithRunnerKeepsMetadataAndReportsFreshness(t *testing.T) {
	graphPath := t.TempDir() + "/graph.json"
	if err := storage.WriteGraph(graphPath, domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  "1.4.0",
		EntityIdentity: domain.CurrentEntityIdentity,
		Repository:     "https://github.com/acme/project",
		Commit:         "head-sha",
		Branch:         "main",
		ScanComplete:   true,
	}); err != nil {
		t.Fatalf("write graph: %v", err)
	}

	metadataJSON := `{
  "number": 7,
  "html_url": "https://github.com/acme/project/pull/7",
  "title": "Improve reconciliation",
  "body": "Please review the controller change.",
  "user": {"login": "developer"},
  "base": {"ref": "main", "sha": "base-sha"},
  "head": {"ref": "feature/reconcile", "sha": "head-sha"},
  "changed_files": 1,
  "additions": 2,
  "deletions": 1,
  "labels": [{"name": "enhancement"}, {"name": "review"}]
}`
	diff := `diff --git a/pkg/controller.go b/pkg/controller.go
index abc..def 100644
--- a/pkg/controller.go
+++ b/pkg/controller.go
@@ -10,3 +10,4 @@ func Reconcile() {
 context
+new behavior
 context
`
	filesJSON := `[{"filename":"pkg/controller.go","status":"modified","additions":2,"deletions":1,"changes":3}]`

	var calls []string
	run := func(args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "api repos/acme/project/pulls/7":
			return []byte(metadataJSON), nil
		case "api --header Accept: application/vnd.github.v3.diff repos/acme/project/pulls/7":
			return []byte(diff), nil
		case "api repos/acme/project/pulls/7/files?per_page=100":
			return []byte(filesJSON), nil
		default:
			t.Fatalf("unexpected gh arguments: %v", args)
			return nil, nil
		}
	}

	result, err := runFromPRWithRunner("acme/project/7", graphPath, run)
	if err != nil {
		t.Fatalf("runFromPRWithRunner returned error: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("gh calls = %d, want 3", len(calls))
	}
	if result.PR == nil {
		t.Fatal("PR metadata is nil")
	}
	if result.PR.Title != "Improve reconciliation" || result.PR.Author != "developer" || result.PR.HeadSHA != "head-sha" {
		t.Fatalf("unexpected PR metadata: %+v", result.PR)
	}
	if strings.Join(result.PR.Labels, ",") != "enhancement,review" {
		t.Fatalf("labels = %v, want sorted labels", result.PR.Labels)
	}
	if len(result.PR.Files) != 1 || result.PR.Files[0].Path != "pkg/controller.go" {
		t.Fatalf("PR files = %+v", result.PR.Files)
	}
	if result.GraphFreshness != "head-matched" {
		t.Fatalf("graph freshness = %q, want head-matched", result.GraphFreshness)
	}
	if !strings.Contains(strings.Join(result.Limitations, "\n"), "no checkout was supplied") {
		t.Fatalf("freshness limitation missing: %v", result.Limitations)
	}
	if result.DiffExcerpt == "" {
		t.Fatal("diff excerpt is empty")
	}
}

func TestBoundPRBody(t *testing.T) {
	body, truncated := boundPRBody(strings.Repeat("x", maxPRBodyBytes+100))
	if !truncated {
		t.Fatal("expected body truncation")
	}
	if len(body) > maxPRBodyBytes || !strings.Contains(body, "PR DESCRIPTION TRUNCATED") {
		t.Fatalf("bounded body length/marker invalid: %d", len(body))
	}
}
