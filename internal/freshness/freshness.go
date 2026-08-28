// Package freshness verifies that a graph still describes a repository
// checkout before a consumer uses it for implementation or review guidance.
package freshness

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vsolanki12/codeatlas/internal/discovery"
	"github.com/vsolanki12/codeatlas/internal/domain"
)

type Result struct {
	Available         bool     `json:"available"`
	GraphRepository   string   `json:"graphRepository,omitempty"`
	Repository        string   `json:"repository,omitempty"`
	GraphCommit       string   `json:"graphCommit,omitempty"`
	RepoHead          string   `json:"repoHead,omitempty"`
	EntityIdentity    string   `json:"entityIdentity,omitempty"`
	ScanComplete      bool     `json:"scanComplete"`
	RepositoryMatch   bool     `json:"repositoryMatch"`
	Verifiable        bool     `json:"verifiable"`
	Stale             bool     `json:"stale"`
	Dirty             bool     `json:"dirty"`
	StateVerifiable   bool     `json:"stateVerifiable"`
	FingerprintSource string   `json:"fingerprintSource,omitempty"`
	ChangedFiles      []string `json:"changedFiles,omitempty"`
	NewFiles          []string `json:"newFiles,omitempty"`
	DeletedFiles      []string `json:"deletedFiles,omitempty"`
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
		Available:       true,
		GraphRepository: graph.Repository,
		Repository:      repoPath,
		GraphCommit:     graph.Commit,
		EntityIdentity:  graph.EntityIdentity,
		ScanComplete:    graph.ScanComplete,
		RepositoryMatch: samePath(graph.Repository, repoPath),
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
