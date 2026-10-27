// Package profile loads repository-specific CodeAtlas scope settings.
package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"sigs.k8s.io/yaml"
)

const FileName = ".codeatlas.yaml"

// Profile keeps repository selection deterministic. Exclude entries are
// repository-relative file or directory paths; a directory excludes its
// descendants.
type Profile struct {
	Version      int      `json:"version" yaml:"version"`
	RepositoryID string   `json:"repositoryId,omitempty" yaml:"repository_id,omitempty"`
	Exclude      []string `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

func Load(root string) (Profile, error) {
	profile := Profile{Version: 1}
	data, err := os.ReadFile(filepath.Join(root, FileName))
	if os.IsNotExist(err) {
		return profile, nil
	}
	if err != nil {
		return Profile{}, fmt.Errorf("read %s: %w", FileName, err)
	}
	if err := yaml.UnmarshalStrict(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse %s: %w", FileName, err)
	}
	if profile.Version == 0 {
		profile.Version = 1
	}
	if profile.Version != 1 {
		return Profile{}, fmt.Errorf("unsupported %s version %d (supported: 1)", FileName, profile.Version)
	}
	profile.RepositoryID = strings.TrimSpace(profile.RepositoryID)
	if strings.ContainsAny(profile.RepositoryID, "\r\n\t") {
		return Profile{}, fmt.Errorf("%s repository_id must be a single-line identifier", FileName)
	}

	seen := make(map[string]bool, len(profile.Exclude))
	normalized := make([]string, 0, len(profile.Exclude))
	for _, value := range profile.Exclude {
		value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
		clean := path.Clean(value)
		if value == "" || clean == "." || path.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, "../") {
			return Profile{}, fmt.Errorf("%s exclude path %q must be a repository-relative file or directory", FileName, value)
		}
		if !seen[clean] {
			seen[clean] = true
			normalized = append(normalized, clean)
		}
	}
	sort.Strings(normalized)
	profile.Exclude = normalized
	return profile, nil
}

func (p Profile) Digest() string {
	canonical := struct {
		Version      int      `json:"version"`
		RepositoryID string   `json:"repositoryId,omitempty"`
		Exclude      []string `json:"exclude,omitempty"`
	}{Version: p.Version, RepositoryID: p.RepositoryID, Exclude: append([]string(nil), p.Exclude...)}
	sort.Strings(canonical.Exclude)
	data, _ := json.Marshal(canonical)
	hash := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(hash[:])
}
