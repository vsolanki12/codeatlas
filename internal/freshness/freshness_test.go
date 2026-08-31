package freshness

import (
	"testing"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

func TestVerifyReportsCurrentGraphIssuesDeterministically(t *testing.T) {
	graph := domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  domain.CurrentSchemaVersion,
		EntityIdentity: domain.CurrentEntityIdentity,
		Repository:     "/repo",
		Commit:         "abc",
		ScanComplete:   true,
	}
	result := Verification{Freshness: Result{
		Available:       true,
		SchemaVersion:   domain.CurrentSchemaVersion,
		SchemaCurrent:   true,
		EntityIdentity:  domain.CurrentEntityIdentity,
		RepositoryMatch: true,
		Verifiable:      true,
		StateVerifiable: true,
		ScanComplete:    true,
	}}
	result.Issues = verificationIssues(result.Freshness, VerifyOptions{
		RequireCurrentSchema: true,
		RequireComplete:      true,
	})
	if len(result.Issues) != 0 {
		t.Fatalf("current graph issues = %v, want none", result.Issues)
	}

	_ = graph // keep the fixture's graph contract visible in this test.
}

func TestVerificationIssuesRejectLegacyAndIncompleteGraph(t *testing.T) {
	result := Result{
		Available:       true,
		SchemaVersion:   "1.4.0",
		SchemaCurrent:   false,
		EntityIdentity:  "legacy",
		RepositoryMatch: true,
		Verifiable:      true,
		StateVerifiable: true,
		ScanComplete:    false,
		ScanCoverage:    &domain.ScanCoverage{Discovered: 2, Failed: 1},
	}
	issues := verificationIssues(result, VerifyOptions{
		RequireCurrentSchema: true,
		RequireComplete:      true,
	})
	if len(issues) != 3 {
		t.Fatalf("issues = %v, want schema, identity, and scan issues", issues)
	}
	if issues[0] != "graph schema 1.4.0 is not current (1.5.0 required)" ||
		issues[1] != "graph entity identity \"legacy\" is not current (repository-path-v1 required)" ||
		issues[2] != "graph scan is incomplete: 1 file(s) failed to parse" {
		t.Fatalf("issues = %v, want stable issue order", issues)
	}
}

func TestVerificationAllowsIgnoredFilesUnlessStrict(t *testing.T) {
	result := Result{
		Available:       true,
		SchemaVersion:   domain.CurrentSchemaVersion,
		SchemaCurrent:   true,
		EntityIdentity:  domain.CurrentEntityIdentity,
		RepositoryMatch: true,
		Verifiable:      true,
		StateVerifiable: true,
		ScanComplete:    true,
		ScanCoverage:    &domain.ScanCoverage{Discovered: 2, Parsed: 1, Ignored: 1},
	}
	if issues := verificationIssues(result, VerifyOptions{RequireCurrentSchema: true, RequireComplete: true}); len(issues) != 0 {
		t.Fatalf("ignored-only graph issues = %v, want none", issues)
	}
	issues := verificationIssues(result, VerifyOptions{RequireCurrentSchema: true, RequireComplete: true, FailOnIgnored: true})
	if len(issues) != 1 || issues[0] != "graph has 1 ignored file(s) without a registered parser" {
		t.Fatalf("strict issues = %v, want ignored-file issue", issues)
	}
}
