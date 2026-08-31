package graph

import (
	"os/exec"
	"strings"
	"time"

	"github.com/vsolanki12/codeatlas/internal/domain"
)

// GitInfo holds the commit hash and branch name from the scanned repository.
type GetInfo struct {
	Commit      string
	Branch      string
	CommittedAt string
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
	dateOut, err := exec.Command("git", "-C", repoDir, "show", "-s", "--format=%cI", "HEAD").Output()
	if err == nil {
		info.CommittedAt = strings.TrimSpace(string(dateOut))
	}
	return info
}

// BuildGraph assembles a complete Graph with metadata, entities, and
// relationships. Called by the scanner after all parsing and relationship
// building is done.

func BuildGraph(repoPath string, entities []domain.Entity, relationships []domain.Relationship, scanDuration time.Duration) domain.Graph {
	git := GetGitInfo(repoPath)

	return domain.Graph{
		Schema:         "codeatlas",
		SchemaVersion:  domain.CurrentSchemaVersion,
		EntityIdentity: domain.CurrentEntityIdentity,
		// A scan timestamp and wall-clock duration make identical source
		// snapshots produce different graph bytes. The commit timestamp is a
		// stable provenance value; runtime duration remains available on the
		// scanner Result and CLI output.
		GeneratedAt:  git.CommittedAt,
		Repository:   repoPath,
		Commit:       git.Commit,
		Branch:       git.Branch,
		ScanDuration: "",
		ScanComplete: true,
		Entities:     entities,
		Relationship: relationships,
	}
}
