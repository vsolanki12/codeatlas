package graph

import (
	"os/exec"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// GitInfo holds the commit hash and branch name from the scanned repository.
type GetInfo struct {
	Commit string
	Branch string
}

// GetGitInfo runs git commands in the given directory to extract the
// current commit hash and branch name. Returns empty strings if git
// is unavailable or the directory isn't a repo — metadata is optional,
// not a reason to fail the scan.
func GetGitInfo(repoDir string) GetInfo {
	var info GetInfo

	commitOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "HEAD").Output()
	if err == nil {
		info.Commit = strings.TrimSpace(string(commitOut))
	}
	branchOut, err := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		info.Branch = strings.TrimSpace(string(branchOut))
	}
	return info
}

// BuildGraph assembles a complete Graph with metadata, entities, and
// relationships. Called by the scanner after all parsing and relationship
// building is done.

func BuildGraph(repoPath string, entities []domain.Entity, relationships []domain.Relationship, scanDuration time.Duration) domain.Graph {
	git := GetGitInfo(repoPath)

	return domain.Graph{
		Schema:        "codeatlas",
		SchemaVersion: "1.4.0",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Repository:    repoPath,
		Commit:        git.Commit,
		Branch:        git.Branch,
		ScanDuration:  scanDuration.String(),
		Entities:      entities,
		Relationship:  relationships,
	}
}