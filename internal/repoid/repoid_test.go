package repoid

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeRemoteAcceptsEveryGitURLForm(t *testing.T) {
	for _, testCase := range []struct {
		remote string
		want   string
	}{
		{"https://github.com/openshift/hypershift.git", "github.com/openshift/hypershift"},
		{"https://github.com/openshift/hypershift", "github.com/openshift/hypershift"},
		{"git@github.com:openshift/hypershift.git", "github.com/openshift/hypershift"},
		{"ssh://git@github.com/openshift/hypershift.git", "github.com/openshift/hypershift"},
		{"https://TOKEN@github.com/OpenShift/HyperShift.git", "github.com/openshift/hypershift"},
		{"https://gitlab.example.com/group/sub/project.git", "gitlab.example.com/group/sub/project"},
		// A remote with no host identity cannot become a portable ID.
		{"/srv/git/repo.git", ""},
		{"", ""},
	} {
		if got := normalizeRemote(testCase.remote); got != testCase.want {
			t.Errorf("normalizeRemote(%q) = %q, want %q", testCase.remote, got, testCase.want)
		}
	}
}

// Two clones of one repository at different paths must produce the same
// identity, which is the whole point of not using the checkout path.
func TestIdentityPrefersConfiguredThenRemoteThenModule(t *testing.T) {
	root := t.TempDir()
	if got := Identity(root, "  My-Repo  "); got != "configured:my-repo" {
		t.Errorf("configured identity = %q", got)
	}
	if got := Identity(root, ""); got != "" {
		t.Errorf("identity without git or go.mod = %q, want empty", got)
	}

	writeFile(t, filepath.Join(root, "go.mod"), "module Example.com/Repo\n\ngo 1.21\n")
	if got := Identity(root, ""); got != "go:example.com/repo" {
		t.Errorf("module identity = %q, want go:example.com/repo", got)
	}
	if got := Identity(root, "custom"); got != "configured:custom" {
		t.Errorf("configured ID must win over the module path, got %q", got)
	}
}

func TestGoModuleReadsTheModuleDirective(t *testing.T) {
	root := t.TempDir()
	if got := GoModule(root); got != "" {
		t.Errorf("missing go.mod = %q, want empty", got)
	}
	writeFile(t, filepath.Join(root, "go.mod"), "// module commented.example/no\n\nmodule example.com/repo\n\nrequire other.example/dep v1.2.3\n")
	if got := GoModule(root); got != "example.com/repo" {
		t.Errorf("GoModule = %q, want example.com/repo", got)
	}
}

func TestMatchesFallsBackToPathOnlyWithoutAnIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/repo\n")

	if !Matches("go:example.com/repo", "", root, "") {
		t.Error("a graph identity equal to the recomputed identity must match")
	}
	if Matches("go:example.com/other", "", root, "") {
		t.Error("a different graph identity must not match")
	}
	// A legacy graph with no identity can only be matched by checkout path.
	if !Matches("", root, root, "") {
		t.Error("identical paths must match when the graph has no identity")
	}
	if Matches("", t.TempDir(), root, "") {
		t.Error("different paths must not match")
	}
}

// Review must refuse a PR whose repository is not the one the graph describes,
// including a fork of the same project.
func TestMatchesGitHubRepository(t *testing.T) {
	for _, testCase := range []struct {
		name            string
		graphID         string
		remote          string
		goModule        string
		graphRepository string
		owner           string
		want            bool
	}{
		{"remote matches", "git:github.com/openshift/hypershift", "github.com/openshift/hypershift", "", "", "openshift/hypershift", true},
		{"fork does not match", "git:github.com/vsolanki12/hypershift", "github.com/vsolanki12/hypershift", "", "", "openshift/hypershift", false},
		{"case insensitive owner", "git:github.com/openshift/hypershift", "github.com/openshift/hypershift", "", "", "OpenShift/HyperShift", true},
		{"module identity", "go:github.com/openshift/hypershift/api", "", "github.com/openshift/hypershift/api", "", "openshift/hypershift", true},
		{"configured id falls back to module", "configured:hypershift", "", "github.com/openshift/hypershift", "", "openshift/hypershift", true},
		{"configured id with unrelated module", "configured:hypershift", "", "example.com/other", "", "openshift/hypershift", false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := MatchesGitHubRepository(testCase.graphID, testCase.remote, testCase.goModule, testCase.graphRepository, testCase.owner)
			if got != testCase.want {
				t.Errorf("MatchesGitHubRepository = %v, want %v", got, testCase.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
