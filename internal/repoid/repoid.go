// Package repoid derives stable repository identity without depending on a
// checkout's local filesystem path.
package repoid

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Identity(root, configuredID string) string {
	if configuredID = strings.TrimSpace(configuredID); configuredID != "" {
		return "configured:" + strings.ToLower(configuredID)
	}
	if remote := OriginRemote(root); remote != "" {
		return "git:" + remote
	}
	if module := GoModule(root); module != "" {
		return "go:" + strings.ToLower(module)
	}
	return ""
}

func GoModule(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], "\"'")
		}
	}
	return ""
}

func Matches(graphID, graphPath, repoPath, configuredID string) bool {
	if graphID != "" {
		return Identity(repoPath, configuredID) == graphID
	}
	if graphPath != "" && repoPath != "" && samePath(graphPath, repoPath) {
		return true
	}
	return false
}

// OriginRemote returns the normalized host/path identity of the Git origin,
// without credentials or transport-specific syntax.
func OriginRemote(root string) string {
	output, err := exec.Command("git", "-C", root, "config", "--get", "remote.origin.url").Output()
	if err != nil {
		return ""
	}
	return normalizeRemote(strings.TrimSpace(string(output)))
}

func MatchesGitHubRepository(graphID, repositoryRemote, goModule, graphRepository, ownerRepository string) bool {
	expected := "github.com/" + strings.ToLower(strings.Trim(strings.TrimSpace(ownerRepository), "/"))
	if repositoryRemote != "" {
		return normalizeRemoteIdentity(repositoryRemote) == expected
	}
	switch {
	case strings.HasPrefix(graphID, "git:"):
		return strings.TrimPrefix(graphID, "git:") == expected
	case strings.HasPrefix(graphID, "go:"):
		return moduleMatches(strings.TrimPrefix(graphID, "go:"), expected)
	case strings.HasPrefix(graphID, "configured:"):
		if strings.EqualFold(strings.TrimPrefix(graphID, "configured:"), expected) {
			return true
		}
		if moduleMatches(goModule, expected) {
			return true
		}
		return normalizeRemote(graphRepository) == expected
	default:
		if moduleMatches(goModule, expected) {
			return true
		}
		return normalizeRemote(graphRepository) == expected
	}
}

func moduleMatches(module, expected string) bool {
	module = strings.ToLower(strings.TrimSpace(module))
	return module == expected || strings.HasPrefix(module, expected+"/")
}

func samePath(left, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(leftAbs); err == nil {
		leftAbs = resolved
	}
	if resolved, err := filepath.EvalSymlinks(rightAbs); err == nil {
		rightAbs = resolved
	}
	return filepath.Clean(leftAbs) == filepath.Clean(rightAbs)
}

func normalizeRemoteIdentity(value string) string {
	value = strings.TrimSpace(value)
	if strings.Contains(value, "://") || strings.Contains(value, ":") {
		if normalized := normalizeRemote(value); normalized != "" {
			return normalized
		}
	}
	value = strings.TrimSuffix(strings.Trim(value, "/"), ".git")
	return strings.ToLower(value)
}

func normalizeRemote(remote string) string {
	if remote == "" {
		return ""
	}
	var host, remotePath string
	if strings.Contains(remote, "://") {
		parsed, err := url.Parse(remote)
		if err != nil || parsed.Hostname() == "" {
			return ""
		}
		host, remotePath = parsed.Hostname(), parsed.Path
	} else {
		value := strings.TrimPrefix(remote, "git@")
		value = strings.TrimPrefix(value, "ssh://")
		colon := strings.IndexByte(value, ':')
		if colon <= 0 {
			return ""
		}
		host, remotePath = value[:colon], value[colon+1:]
		if at := strings.LastIndexByte(host, '@'); at >= 0 {
			host = host[at+1:]
		}
	}
	host = strings.ToLower(strings.TrimSpace(host))
	remotePath = strings.Trim(strings.TrimSpace(remotePath), "/")
	remotePath = strings.TrimSuffix(remotePath, ".git")
	remotePath = strings.ToLower(remotePath)
	if host == "" || remotePath == "" || strings.HasPrefix(remotePath, "../") {
		return ""
	}
	return host + "/" + remotePath
}
