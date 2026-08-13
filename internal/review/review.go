package review

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/domain"
	"github.com/vsolanki12/codeatlas/internal/query"
)

type EntityReview struct {
	Entity      *domain.Entity
	Approximate bool
	Callers     []*domain.Entity
	Callees     []string
	Controllers []*domain.Entity
	Tests       []*domain.Entity
	Resources   []*domain.Entity
	View        *domain.View
}

type TestLink struct {
	Test    *domain.Entity
	Targets []*domain.Entity
	Reason  string
	IsAdded bool
}

type ReviewResult struct {
	Base          string
	Head          string
	ChangedFiles  []FileDiff
	Functions     []EntityReview
	Tests         []TestLink
	UnmappedFiles []string
	Limitations   []string
}

func Run(base, head, repo, graphPath string) (*ReviewResult, error) {
	idx, err := query.LoadGraph(graphPath)
	if err != nil {
		return nil, fmt.Errorf("load graph: %w", err)
	}

	diffOutput, err := gitDiff(repo, base, head)
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	diffs := ParseDiff(diffOutput)
	return Analyze(diffs, idx, base, head), nil
}

func RunFromDiff(diffSource, graphPath, base, head string) (*ReviewResult, error) {
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
	return Analyze(diffs, idx, base, head), nil
}

func Analyze(diffs []FileDiff, idx *query.Index, base, head string) *ReviewResult {
	changed, unmapped := MapToEntities(diffs, idx)

	var functions []EntityReview
	var testEntities []ChangedEntity
	seen := make(map[string]bool)

	for _, ce := range changed {
		if seen[ce.Entity.ID] {
			continue
		}
		seen[ce.Entity.ID] = true

		if ce.Entity.Kind == domain.KindTest {
			testEntities = append(testEntities, ce)
			continue
		}

		er := EntityReview{
			Entity:      ce.Entity,
			Approximate: ce.Approximate,
			Callees:     ce.Entity.Calls,
		}

		er.Callers = idx.Callers(ce.Entity.ID)

		rels := idx.GetRelationships(ce.Entity.ID, "from", string(domain.RelTestedBy))
		for _, r := range rels {
			if t := idx.GetEntity(r.To); t != nil {
				er.Tests = append(er.Tests, t)
			}
		}

		impact := idx.Impact(ce.Entity.ID)
		if impact != nil {
			er.Controllers = impact.Controllers
			er.Resources = impact.Resources
			if len(er.Tests) == 0 {
				er.Tests = impact.Tests
			}
		}

		er.View = idx.GetView(ce.Entity.ID)

		functions = append(functions, er)
	}

	var tests []TestLink
	for _, te := range testEntities {
		tl := TestLink{
			Test:    te.Entity,
			IsAdded: isAddedEntity(te),
		}
		tl.Targets, tl.Reason = inferTestTargets(te.Entity, functions)
		tests = append(tests, tl)
	}

	limitations := []string{
		"Entity mapping is approximate (start line only).",
		"Test-to-function link is inferred from naming, not call graph.",
		"Atlas cannot prove branch-level coverage.",
	}

	return &ReviewResult{
		Base:          base,
		Head:          head,
		ChangedFiles:  diffs,
		Functions:     functions,
		Tests:         tests,
		UnmappedFiles: unmapped,
		Limitations:   limitations,
	}
}

func isAddedEntity(ce ChangedEntity) bool {
	for _, h := range ce.Hunks {
		if h.OldCount > 0 {
			return false
		}
	}
	return true
}

func inferTestTargets(test *domain.Entity, functions []EntityReview) ([]*domain.Entity, string) {
	testName := test.Name

	if strings.HasPrefix(testName, "Test") {
		baseName := strings.ToLower(testName[4:])
		for _, f := range functions {
			funcName := strings.ToLower(f.Entity.Name)
			if strings.Contains(baseName, funcName) || strings.Contains(funcName, baseName) {
				return []*domain.Entity{f.Entity}, "naming convention"
			}
		}
	}

	testFile := test.Source.File
	testBase := strings.TrimSuffix(testFile, "_test.go")
	var sameFile []*domain.Entity
	for _, f := range functions {
		funcBase := strings.TrimSuffix(f.Entity.Source.File, ".go")
		if testBase == funcBase {
			sameFile = append(sameFile, f.Entity)
		}
	}
	if len(sameFile) > 0 {
		return sameFile, "same file"
	}

	return nil, ""
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
