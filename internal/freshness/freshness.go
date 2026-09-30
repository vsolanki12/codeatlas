// Package freshness verifies that a graph still describes a repository
// checkout before a consumer uses it for implementation or review guidance.
package freshness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/discovery"
	"github.com/vsolanki12/codeatlas/internal/domain"
)

type Result struct {
	Available              bool                         `json:"available"`
	SchemaVersion          string                       `json:"schemaVersion,omitempty"`
	SchemaCurrent          bool                         `json:"schemaCurrent"`
	GraphRepository        string                       `json:"graphRepository,omitempty"`
	Repository             string                       `json:"repository,omitempty"`
	GraphCommit            string                       `json:"graphCommit,omitempty"`
	RepoHead               string                       `json:"repoHead,omitempty"`
	EntityIdentity         string                       `json:"entityIdentity,omitempty"`
	ExtractorVersion       string                       `json:"extractorVersion,omitempty"`
	ExtractorBuild         string                       `json:"extractorBuild,omitempty"`
	ExpectedExtractorBuild string                       `json:"expectedExtractorBuild,omitempty"`
	ExtractorBuildMatch    bool                         `json:"extractorBuildMatch"`
	ExtractorCurrent       bool                         `json:"extractorCurrent"`
	ExtractionSignature    string                       `json:"extractionSignature,omitempty"`
	BuildContext           *domain.BuildContext         `json:"buildContext,omitempty"`
	TypeAnalysis           *domain.TypeAnalysisCoverage `json:"typeAnalysis,omitempty"`
	ScanComplete           bool                         `json:"scanComplete"`
	ScanWarnings           []string                     `json:"scanWarnings,omitempty"`
	ScanCoverage           *domain.ScanCoverage         `json:"scanCoverage,omitempty"`
	RepositoryMatch        bool                         `json:"repositoryMatch"`
	Verifiable             bool                         `json:"verifiable"`
	Stale                  bool                         `json:"stale"`
	Dirty                  bool                         `json:"dirty"`
	StateVerifiable        bool                         `json:"stateVerifiable"`
	FingerprintSource      string                       `json:"fingerprintSource,omitempty"`
	ChangedFiles           []string                     `json:"changedFiles,omitempty"`
	NewFiles               []string                     `json:"newFiles,omitempty"`
	DeletedFiles           []string                     `json:"deletedFiles,omitempty"`
}

// VerifyOptions controls the repository-integrity contract required by a
// consumer. Ignored files are reported by default but do not invalidate a
// graph unless FailOnIgnored is explicitly requested: an unsupported asset
// should only block work that depends on that asset's evidence.
type VerifyOptions struct {
	RequireCurrentSchema    bool
	RequireCurrentExtractor bool
	ExpectedExtractorBuild  string
	RequireComplete         bool
	FailOnIgnored           bool
}

// Verification is the CI-friendly form of a freshness check. It keeps the
// complete structured freshness result beside a deterministic list of issues
// so scripts and AI clients do not need to parse human-readable text.
type Verification struct {
	Freshness Result   `json:"freshness"`
	Valid     bool     `json:"valid"`
	Issues    []string `json:"issues,omitempty"`
}

// Verify checks the graph against the requested repository and applies the
// explicit consumer contract. The order of issues is stable for reproducible
// CI output and prompt context.
func Verify(repoPath string, graph domain.Graph, graphPath string, opts VerifyOptions) Verification {
	result := Verification{Freshness: CheckWithGraphPath(repoPath, graph, graphPath)}
	if opts.ExpectedExtractorBuild != "" {
		result.Freshness.ExpectedExtractorBuild = opts.ExpectedExtractorBuild
		result.Freshness.ExtractorBuildMatch = result.Freshness.ExtractorBuild == opts.ExpectedExtractorBuild
		result.Freshness.ExtractorCurrent = result.Freshness.ExtractorCurrent && result.Freshness.ExtractorBuildMatch
	}
	result.Issues = verificationIssues(result.Freshness, opts)
	result.Valid = len(result.Issues) == 0
	return result
}

func verificationIssues(result Result, opts VerifyOptions) []string {
	var issues []string
	if !result.Available {
		return []string{"graph metadata is unavailable"}
	}
	if opts.RequireCurrentSchema {
		switch {
		case result.SchemaVersion == "":
			issues = append(issues, "graph schema version is missing")
		case !result.SchemaCurrent:
			issues = append(issues, fmt.Sprintf("graph schema %s is not current (%s required)", result.SchemaVersion, domain.CurrentSchemaVersion))
		}
	}
	if result.EntityIdentity != domain.CurrentEntityIdentity {
		if result.EntityIdentity == "" {
			issues = append(issues, "graph entity identity is missing")
		} else {
			issues = append(issues, fmt.Sprintf("graph entity identity %q is not current (%s required)", result.EntityIdentity, domain.CurrentEntityIdentity))
		}
	}
	if opts.RequireCurrentExtractor && (!result.ExtractorCurrent || result.ExtractorBuild == "" || result.ExtractionSignature == "" || result.BuildContext == nil) {
		issues = append(issues, "graph extractor provenance is missing or incompatible; perform a full scan with the current extractor")
	}
	if opts.ExpectedExtractorBuild != "" && result.ExtractorBuild != opts.ExpectedExtractorBuild {
		issues = append(issues, "graph extractor build differs from the expected executable; perform a full scan with the current build")
	}
	if !result.RepositoryMatch {
		issues = append(issues, "graph repository does not match the requested checkout")
	}
	if !result.Verifiable {
		issues = append(issues, "graph commit could not be verified against the repository")
	}
	if result.Stale {
		issues = append(issues, "graph commit is stale compared with repository HEAD")
	}
	if !result.StateVerifiable {
		issues = append(issues, "graph file state could not be verified")
	}
	if result.Dirty {
		issues = append(issues, "repository files differ from the graph")
	}
	if opts.RequireComplete && !result.ScanComplete {
		if result.ScanCoverage != nil && result.ScanCoverage.Failed > 0 {
			issues = append(issues, fmt.Sprintf("graph scan is incomplete: %d file(s) failed to parse", result.ScanCoverage.Failed))
		} else {
			issues = append(issues, "graph scan is incomplete")
		}
	}
	if opts.FailOnIgnored && result.ScanCoverage != nil && result.ScanCoverage.Ignored > 0 {
		issues = append(issues, fmt.Sprintf("graph has %d ignored file(s) without a registered parser", result.ScanCoverage.Ignored))
	}
	return issues
}

// Check compares graph provenance and discovered file state with repoPath.
// It is intentionally read-only and returns a structured result even when a
// repository cannot be verified, so consumers can explain the limitation.
func Check(repoPath string, graph domain.Graph) Result {
	return CheckWithGraphPath(repoPath, graph, "")
}

// CheckWithGraphPath is Check plus the path of the graph file itself. A graph
// stored inside the scanned repository must be excluded from the repository
// state comparison; otherwise writing the graph changes the very state it is
// meant to describe and every freshness check reports a false positive.
func CheckWithGraphPath(repoPath string, graph domain.Graph, graphPath string) Result {
	if repoPath == "" {
		repoPath = graph.Repository
	}
	result := Result{
		Available:           true,
		SchemaVersion:       graph.SchemaVersion,
		SchemaCurrent:       graph.SchemaVersion == domain.CurrentSchemaVersion,
		GraphRepository:     graph.Repository,
		Repository:          repoPath,
		GraphCommit:         graph.Commit,
		EntityIdentity:      graph.EntityIdentity,
		ExtractorVersion:    graph.ExtractorVersion,
		ExtractorBuild:      graph.ExtractorBuild,
		ExtractorCurrent:    graph.ExtractorVersion == domain.CurrentExtractorVersion && graph.ExtractorBuild != "",
		ExtractionSignature: graph.ExtractionSignature,
		BuildContext:        copyBuildContext(graph.BuildContext),
		TypeAnalysis:        summarizedTypeAnalysis(graph.TypeAnalysis),
		ScanComplete:        graph.ScanComplete,
		ScanWarnings:        append([]string(nil), graph.ScanWarnings...),
		ScanCoverage:        copyScanCoverage(graph.ScanCoverage),
		RepositoryMatch:     samePath(graph.Repository, repoPath),
	}
	if repoPath == "" {
		return result
	}
	if info, err := os.Stat(repoPath); err != nil || !info.IsDir() {
		return result
	}

	result.RepoHead = gitHead(repoPath)
	if result.GraphCommit != "" && result.RepoHead != "" {
		result.Verifiable = true
		result.Stale = result.GraphCommit != result.RepoHead
	}

	disc, err := discovery.New(domain.Repository{RootPath: repoPath})
	if err != nil {
		return result
	}
	files, err := disc.Scan()
	if err != nil {
		return result
	}
	current := make(map[string]string, len(files))
	for _, file := range files {
		current[file.RelativePath] = discovery.Fingerprint(file)
	}

	expected := graph.FileFingerprints
	if len(expected) > 0 {
		result.FingerprintSource = "content-or-file-metadata"
		result.StateVerifiable = true
	} else if len(graph.FileTimestamps) > 0 {
		result.FingerprintSource = "timestamp"
		result.StateVerifiable = true
		for _, file := range files {
			current[file.RelativePath] = discovery.FingerprintTimestamp(file)
		}
		expected = graph.FileTimestamps
	}

	if !result.StateVerifiable {
		return result
	}
	if relativeGraphPath, ok := relativeFile(repoPath, graphPath); ok {
		delete(current, relativeGraphPath)
	}
	for path, value := range expected {
		currentValue, ok := current[path]
		if !ok {
			result.DeletedFiles = append(result.DeletedFiles, path)
			continue
		}
		if currentValue != value {
			result.ChangedFiles = append(result.ChangedFiles, path)
		}
		delete(current, path)
	}
	for path := range current {
		result.NewFiles = append(result.NewFiles, path)
	}
	sort.Strings(result.ChangedFiles)
	sort.Strings(result.NewFiles)
	sort.Strings(result.DeletedFiles)
	result.Dirty = len(result.ChangedFiles) > 0 || len(result.NewFiles) > 0 || len(result.DeletedFiles) > 0
	return result
}

func copyBuildContext(context *domain.BuildContext) *domain.BuildContext {
	if context == nil {
		return nil
	}
	copy := *context
	copy.BuildTags = append([]string(nil), context.BuildTags...)
	return &copy
}

func summarizedTypeAnalysis(coverage *domain.TypeAnalysisCoverage) *domain.TypeAnalysisCoverage {
	if coverage == nil {
		return nil
	}
	copy := *coverage
	const maxDiagnostics = 4
	count := len(coverage.Diagnostics)
	if count > maxDiagnostics {
		count = maxDiagnostics
		copy.DiagnosticsTruncated = true
	}
	copy.Diagnostics = append([]domain.TypeAnalysisDiagnostic(nil), coverage.Diagnostics[:count]...)
	return &copy
}

func copyScanCoverage(coverage *domain.ScanCoverage) *domain.ScanCoverage {
	if coverage == nil {
		return nil
	}
	copy := *coverage
	return &copy
}

func relativeFile(repoPath, filePath string) (string, bool) {
	if repoPath == "" || filePath == "" {
		return "", false
	}
	repoRoot := comparablePath(repoPath)
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return "", false
	}
	if resolved, err := filepath.EvalSymlinks(fileAbs); err == nil {
		fileAbs = resolved
	}
	relative, err := filepath.Rel(repoRoot, filepath.Clean(fileAbs))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return comparablePath(left) == comparablePath(right)
}

func comparablePath(value string) string {
	abs, err := filepath.Abs(value)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return filepath.Clean(abs)
}

func gitHead(repoPath string) string {
	output, err := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
