package review

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/freshness"
	"github.com/vsolanki12/codeatlas/internal/query"
)

type EntityReview struct {
	Entity                 *domain.Entity         `json:"entity"`
	Approximate            bool                   `json:"approximate"`
	ChangedFiles           []string               `json:"changedFiles,omitempty"`
	Hunks                  []Hunk                 `json:"hunks,omitempty"`
	Relationships          []*domain.Relationship `json:"relationships,omitempty"`
	RelationshipsTruncated bool                   `json:"relationshipsTruncated,omitempty"`
	Callers                []*domain.Entity       `json:"callers,omitempty"`
	Callees                []string               `json:"callees,omitempty"`
	Controllers            []*domain.Entity       `json:"controllers,omitempty"`
	Tests                  []*domain.Entity       `json:"tests,omitempty"`
	Resources              []*domain.Entity       `json:"resources,omitempty"`
	View                   *domain.View           `json:"view,omitempty"`
}

type TestLink struct {
	Test          *domain.Entity         `json:"test"`
	Targets       []*domain.Entity       `json:"targets,omitempty"`
	Relationships []*domain.Relationship `json:"relationships,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
	Confidence    domain.Confidence      `json:"confidence,omitempty"`
	IsAdded       bool                   `json:"added"`
}

type ReviewResult struct {
	Base              string              `json:"base"`
	Head              string              `json:"head"`
	Graph             query.GraphMetadata `json:"graph"`
	GraphFreshness    string              `json:"graphFreshness"`
	ChangedFiles      []FileDiff          `json:"changedFiles"`
	Functions         []EntityReview      `json:"functions,omitempty"`
	Tests             []TestLink          `json:"tests,omitempty"`
	UnmappedFiles     []string            `json:"unmappedFiles,omitempty"`
	Limitations       []string            `json:"limitations"`
	Heuristics        []string            `json:"heuristics"`
	LLMInterpretation []string            `json:"llmInterpretation"`
	// DiffExcerpt preserves bounded unified-diff evidence for consumers that
	// invoke an LLM. ChangedFiles contains locations and counts, but not the
	// changed source text needed to support an exact-line finding.
	DiffExcerpt          string `json:"diffExcerpt,omitempty"`
	DiffExcerptTruncated bool   `json:"diffExcerptTruncated,omitempty"`
}

func Run(base, head, repo, graphPath string) (*ReviewResult, error) {
	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("load graph: %w", err)
	}
	if err := verifyGraphAtRef(idx, repo, head, graphPath); err != nil {
		return nil, err
	}

	diffOutput, err := gitDiff(repo, base, head)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	diffs := ParseDiff(diffOutput)
	result := Analyze(diffs, idx, base, head)
	attachDiffExcerpt(result, diffOutput)
	markGraphVerified(result)
	return result, nil
}

func RunFromDiff(diffSource, graphPath, base, head string) (*ReviewResult, error) {
	return runFromDiff(diffSource, graphPath, base, head, "")
}

// RunFromDiffInRepo is the verified variant used by the CLI when a repository
// is available. The plain RunFromDiff API remains useful for piped diffs, but
// explicitly reports that graph freshness could not be checked.
func RunFromDiffInRepo(diffSource, graphPath, base, head, repo string) (*ReviewResult, error) {
	return runFromDiff(diffSource, graphPath, base, head, repo)
}

func runFromDiff(diffSource, graphPath, base, head, repo string) (*ReviewResult, error) {
	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("load graph: %w", err)
	}

	var diffOutput string
	if diffSource == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		diffOutput = string(data)
	} else {
		data, err := os.ReadFile(diffSource)
		if err != nil {
			return nil, fmt.Errorf("read diff file: %w", err)
		}
		diffOutput = string(data)
	}

	diffs := ParseDiff(diffOutput)
	result := Analyze(diffs, idx, base, head)
	attachDiffExcerpt(result, diffOutput)
	if repo != "" {
		if err := verifyGraphAtRef(idx, repo, head, graphPath); err != nil {
			return nil, err
		}
		markGraphVerified(result)
	}
	return result, nil
}

const maxDiffExcerptBytes = 32 * 1024

// attachDiffExcerpt keeps the actual changed text available to downstream
// reviewers while bounding the packet sent to a local model. The structured
// entity and hunk data remains available when a large diff is abbreviated.
func attachDiffExcerpt(result *ReviewResult, diffOutput string) {
	result.DiffExcerpt, result.DiffExcerptTruncated = boundedDiffExcerpt(diffOutput)
	result.Limitations = append(result.Limitations, "Supplied diff text was parsed as provided; graph freshness does not prove that it matches the named base and head refs.")
	if result.DiffExcerptTruncated {
		result.Limitations = append(result.Limitations, "Diff content was bounded; omitted changed lines are not available for exact-line review.")
	}
}

func boundedDiffExcerpt(diffOutput string) (string, bool) {
	if len(diffOutput) <= maxDiffExcerptBytes {
		return diffOutput, false
	}

	const marker = "\n\n[DIFF EXCERPT TRUNCATED: omitted changed lines are not available for exact-line review.]\n\n"
	contentLimit := maxDiffExcerptBytes - len(marker)
	head := int(float64(contentLimit) * 0.60)
	tail := contentLimit - head
	if lineEnd := strings.LastIndex(diffOutput[:head], "\n"); lineEnd >= 0 {
		head = lineEnd + 1
	}
	tailStart := len(diffOutput) - tail
	if lineStart := strings.Index(diffOutput[tailStart:], "\n"); lineStart >= 0 {
		tailStart += lineStart + 1
	}
	return diffOutput[:head] + marker + diffOutput[tailStart:], true
}

func Analyze(diffs []FileDiff, idx *query.Index, base, head string) *ReviewResult {
	changed, unmapped := MapToEntities(diffs, idx)

	var functions []EntityReview
	var testEntities []ChangedEntity
	entityIndexes := make(map[string]int)
	truncatedRelationships := false

	for _, ce := range changed {
		if ce.Entity.Kind == domain.KindTest {
			if _, seen := entityIndexes[ce.Entity.ID]; seen {
				continue
			}
			entityIndexes[ce.Entity.ID] = -1
			testEntities = append(testEntities, ce)
			continue
		}
		if index, seen := entityIndexes[ce.Entity.ID]; seen {
			if index >= 0 {
				functions[index].Approximate = functions[index].Approximate || ce.Approximate
				functions[index].ChangedFiles = appendUniqueReviewFile(functions[index].ChangedFiles, ce.Path)
				functions[index].Hunks = append(functions[index].Hunks, ce.Hunks...)
				if len(functions[index].Hunks) > 20 {
					functions[index].Hunks = functions[index].Hunks[:20]
					functions[index].RelationshipsTruncated = true
					truncatedRelationships = true
				}
			}
			continue
		}

		relationships := idx.GetRelationships(ce.Entity.ID, "both", "")
		er := EntityReview{
			Entity:        ce.Entity,
			Approximate:   ce.Approximate,
			ChangedFiles:  appendUniqueReviewFile(nil, ce.Path),
			Hunks:         append([]Hunk(nil), ce.Hunks...),
			Callees:       ce.Entity.Calls,
			Relationships: relationships,
		}
		entityIndexes[ce.Entity.ID] = len(functions)
		if len(er.Callees) > 40 {
			er.Callees = er.Callees[:40]
			er.RelationshipsTruncated = true
			truncatedRelationships = true
		}
		if len(er.Relationships) > 40 {
			er.Relationships = er.Relationships[:40]
			er.RelationshipsTruncated = true
			truncatedRelationships = true
		}

		er.Callers = idx.Callers(ce.Entity.ID)
		if len(er.Callers) > 20 {
			er.Callers = er.Callers[:20]
			er.RelationshipsTruncated = true
			truncatedRelationships = true
		}

		rels := idx.GetRelationships(ce.Entity.ID, "from", string(domain.RelTestedBy))
		for _, r := range rels {
			if t := idx.GetEntity(r.To); t != nil {
				er.Tests = append(er.Tests, t)
			}
		}
		if len(er.Tests) > 20 {
			er.Tests = er.Tests[:20]
			er.RelationshipsTruncated = true
			truncatedRelationships = true
		}

		impact := idx.Impact(ce.Entity.ID)
		if impact != nil {
			er.Controllers = impact.Controllers
			er.Resources = impact.Resources
			if len(er.Controllers) > 20 {
				er.Controllers = er.Controllers[:20]
				er.RelationshipsTruncated = true
				truncatedRelationships = true
			}
			if len(er.Resources) > 30 {
				er.Resources = er.Resources[:30]
				er.RelationshipsTruncated = true
				truncatedRelationships = true
			}
			if len(er.Tests) == 0 {
				er.Tests = impact.Tests
			}
			if len(er.Tests) > 20 {
				er.Tests = er.Tests[:20]
				er.RelationshipsTruncated = true
				truncatedRelationships = true
			}
		}

		er.View = idx.GetView(ce.Entity.ID)

		functions = append(functions, er)
	}
	sort.Slice(functions, func(i, j int) bool { return functions[i].Entity.ID < functions[j].Entity.ID })

	var tests []TestLink
	for _, te := range testEntities {
		tl := TestLink{
			Test:    te.Entity,
			IsAdded: isAddedEntity(te),
		}
		tl.Targets, tl.Relationships, tl.Reason, tl.Confidence = inferTestTargets(te.Entity, functions, idx)
		tests = append(tests, tl)
	}

	meta := idx.GraphMetadata()
	limitations := []string{
		"Atlas cannot prove branch-level coverage.",
		"A tested_by relationship is structural evidence only; execution of the changed behavior remains unproven.",
	}
	if truncatedRelationships {
		limitations = append(limitations, "Relationship, caller, controller, or resource lists were capped; omitted entries are not evidence of absence.")
	}
	for _, f := range functions {
		if f.Approximate {
			limitations = append(limitations, "Some changed lines were mapped approximately because the graph lacked an entity end line.")
			break
		}
	}
	if !meta.ScanComplete {
		limitations = append(limitations, "Graph scan is incomplete; parser warnings mean some repository facts may be absent.")
	}
	if meta.Commit == "" {
		limitations = append(limitations, "Graph commit metadata is unavailable; freshness could not be verified.")
	}

	return &ReviewResult{
		Base:              base,
		Head:              head,
		Graph:             meta,
		GraphFreshness:    "unverified",
		ChangedFiles:      diffs,
		Functions:         functions,
		Tests:             tests,
		UnmappedFiles:     unmapped,
		Limitations:       limitations,
		Heuristics:        []string{"When no tested_by edge exists, test-to-function mapping uses test naming and same-file conventions."},
		LLMInterpretation: []string{},
	}
}

func appendUniqueReviewFile(files []string, file string) []string {
	if file == "" {
		return files
	}
	for _, existing := range files {
		if existing == file {
			return files
		}
	}
	return append(files, file)
}

func verifyGraphAtRef(idx *query.Index, repo, ref, graphPath string) error {
	meta := idx.GraphMetadata()
	if meta.Commit == "" {
		return fmt.Errorf("review refused: graph has no commit metadata; rescan from a git repository")
	}
	if meta.EntityIdentity != domain.CurrentEntityIdentity {
		return fmt.Errorf("review refused: graph uses legacy or unsupported entity identity %q; rescan before reviewing", meta.EntityIdentity)
	}
	if meta.Repository != "" {
		graphRoot := existingRepositoryPath(meta.Repository)
		repoRoot := existingRepositoryPath(repo)
		if graphRoot != "" && repoRoot != "" && graphRoot != repoRoot {
			return fmt.Errorf("review refused: graph repository %s differs from review repository %s", meta.Repository, repo)
		}
	}
	if ref == "" {
		ref = "HEAD"
	}
	cmd := exec.Command("git", "-C", repo, "rev-parse", ref)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("resolve review head %q: %w", ref, err)
	}
	actual := strings.TrimSpace(string(out))
	if actual != meta.Commit {
		return fmt.Errorf("review refused: graph is stale (graph=%s, %s=%s); rescan the requested revision", meta.Commit, ref, actual)
	}
	state := freshness.CheckWithGraphPath(repo, idx.Graph(), graphPath)
	if !state.RepositoryMatch {
		return fmt.Errorf("review refused: graph repository or checkout could not be verified")
	}
	if !state.StateVerifiable {
		return fmt.Errorf("review refused: graph has no file fingerprints; rescan before reviewing")
	}
	if state.Dirty {
		return fmt.Errorf("review refused: repository files differ from the graph; rescan before reviewing")
	}
	return nil
}

func existingRepositoryPath(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}

func markGraphVerified(result *ReviewResult) {
	result.GraphFreshness = "verified"
	if !result.Graph.ScanComplete {
		result.GraphFreshness = "verified-incomplete"
	}
	const freshnessLimitation = "Graph commit metadata is unavailable; freshness could not be verified."
	filtered := result.Limitations[:0]
	for _, limitation := range result.Limitations {
		if limitation != freshnessLimitation {
			filtered = append(filtered, limitation)
		}
	}
	result.Limitations = filtered
}

func isAddedEntity(ce ChangedEntity) bool {
	for _, h := range ce.Hunks {
		if h.OldCount > 0 {
			return false
		}
	}
	return true
}

func inferTestTargets(test *domain.Entity, functions []EntityReview, idx *query.Index) ([]*domain.Entity, []*domain.Relationship, string, domain.Confidence) {
	var targets []*domain.Entity
	var relationships []*domain.Relationship
	for _, r := range idx.GetRelationships(test.ID, "to", string(domain.RelTestedBy)) {
		if target := idx.GetEntity(r.From); target != nil {
			targets = append(targets, target)
			relationships = append(relationships, r)
		}
	}
	if len(targets) > 0 {
		confidence := domain.ConfidenceInferred
		for _, relationship := range relationships {
			if relationship.Confidence == domain.ConfidenceProven {
				confidence = domain.ConfidenceProven
				break
			}
		}
		return targets, relationships, "CodeAtlas tested_by relationship (stored edge)", confidence
	}

	testName := test.Name

	if strings.HasPrefix(testName, "Test") {
		baseName := strings.ToLower(testName[4:])
		var matches []*domain.Entity
		if baseName != "" {
			for _, f := range functions {
				if f.Entity.Kind != domain.KindFunction {
					continue
				}
				funcName := strings.ToLower(f.Entity.Name)
				if baseName == funcName {
					matches = append(matches, f.Entity)
				}
			}
		}
		if len(matches) == 1 {
			return matches, nil, "naming convention", domain.ConfidenceInferred
		}
	}

	testFile := test.Source.File
	testBase := strings.TrimSuffix(testFile, "_test.go")
	var sameFile []*domain.Entity
	for _, f := range functions {
		if f.Entity.Kind != domain.KindFunction {
			continue
		}
		funcBase := strings.TrimSuffix(f.Entity.Source.File, ".go")
		if testBase == funcBase {
			sameFile = append(sameFile, f.Entity)
		}
	}
	if len(sameFile) == 1 {
		return sameFile, nil, "same file", domain.ConfidenceInferred
	}

	return nil, nil, "", ""
}

func gitDiff(repo, base, head string) (string, error) {
	cmd := exec.Command("git", "diff", fmt.Sprintf("%s...%s", base, head))
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("%s: %s", err, exitErr.Stderr)
		}
		return "", err
	}
	return string(out), nil
}
