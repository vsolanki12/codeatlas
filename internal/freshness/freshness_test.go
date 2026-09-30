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
	if issues[0] != "graph schema 1.4.0 is not current ("+domain.CurrentSchemaVersion+" required)" ||
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

func TestFreshnessSummarizesTypeDiagnosticsWithoutMutatingGraph(t *testing.T) {
	coverage := &domain.TypeAnalysisCoverage{Packages: 10, FailedPackages: 10, DiagnosticCount: 10}
	for i := 0; i < 10; i++ {
		coverage.Diagnostics = append(coverage.Diagnostics, domain.TypeAnalysisDiagnostic{Package: "p", Message: "unresolved dependency"})
	}
	summary := summarizedTypeAnalysis(coverage)
	if len(summary.Diagnostics) != 4 || !summary.DiagnosticsTruncated || summary.DiagnosticCount != 10 {
		t.Fatalf("unbounded or incomplete type summary: %+v", summary)
	}
	summary.Diagnostics[0].Message = "changed"
	if len(coverage.Diagnostics) != 10 || coverage.Diagnostics[0].Message != "unresolved dependency" {
		t.Fatal("freshness mutated stored diagnostic evidence")
	}
}

func TestVerificationRejectsDifferentExecutableWithSameExtractorVersion(t *testing.T) {
	result := Result{Available: true, SchemaVersion: domain.CurrentSchemaVersion, SchemaCurrent: true, EntityIdentity: domain.CurrentEntityIdentity, ExtractorVersion: domain.CurrentExtractorVersion, ExtractorCurrent: true, ExtractorBuild: "sha256:old", ExtractionSignature: "signature", BuildContext: &domain.BuildContext{GOOS: "linux", GOARCH: "amd64"}, RepositoryMatch: true, Verifiable: true, StateVerifiable: true, ScanComplete: true}
	issues := verificationIssues(result, VerifyOptions{RequireCurrentSchema: true, RequireCurrentExtractor: true, ExpectedExtractorBuild: "sha256:new"})
	if len(issues) != 1 || issues[0] != "graph extractor build differs from the expected executable; perform a full scan with the current build" {
		t.Fatalf("old same-version binary was not rejected precisely: %v", issues)
	}
	issues = verificationIssues(result, VerifyOptions{RequireCurrentExtractor: true, ExpectedExtractorBuild: "sha256:old"})
	if len(issues) != 0 {
		t.Fatalf("matching build rejected: %v", issues)
	}
}
