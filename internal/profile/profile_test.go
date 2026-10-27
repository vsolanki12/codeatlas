package profile

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, FileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadWithoutAProfileUsesDefaults(t *testing.T) {
	loaded, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Version != 1 || loaded.RepositoryID != "" || len(loaded.Exclude) != 0 {
		t.Fatalf("default profile = %+v", loaded)
	}
}

func TestLoadNormalizesAndOrdersExclusions(t *testing.T) {
	root := t.TempDir()
	write(t, root, "version: 1\nexclude:\n  - ./vendor\n  - test/data/\n  - vendor\n  - docs\n")

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{"docs", "test/data", "vendor"}
	if len(loaded.Exclude) != len(want) {
		t.Fatalf("exclude = %v, want %v", loaded.Exclude, want)
	}
	for i := range want {
		if loaded.Exclude[i] != want[i] {
			t.Fatalf("exclude = %v, want %v", loaded.Exclude, want)
		}
	}
}

// An exclusion that can escape the repository would silently change scope.
func TestLoadRejectsExclusionsOutsideTheRepository(t *testing.T) {
	for _, value := range []string{"/etc", "..", "../sibling", "", "."} {
		root := t.TempDir()
		write(t, root, "version: 1\nexclude:\n  - \""+value+"\"\n")
		if _, err := Load(root); err == nil {
			t.Errorf("exclude %q should be rejected", value)
		}
	}
}

func TestLoadRejectsUnsupportedVersionAndUnknownKeys(t *testing.T) {
	root := t.TempDir()
	write(t, root, "version: 2\n")
	if _, err := Load(root); err == nil {
		t.Error("an unsupported profile version should be rejected")
	}

	root = t.TempDir()
	write(t, root, "version: 1\nexcludes:\n  - vendor\n")
	if _, err := Load(root); err == nil {
		t.Error("an unknown profile key should be rejected rather than silently ignored")
	}
}

func TestLoadRejectsMultiLineRepositoryID(t *testing.T) {
	root := t.TempDir()
	write(t, root, "version: 1\nrepository_id: \"one\\ntwo\"\n")
	if _, err := Load(root); err == nil {
		t.Error("a multi-line repository_id should be rejected")
	}
}

// The digest gates incremental reuse: equal scope must reuse, changed scope
// must force a rescan.
func TestDigestIsStableForEqualScopeAndChangesWithScope(t *testing.T) {
	base := Profile{Version: 1, Exclude: []string{"docs", "vendor"}}
	reordered := Profile{Version: 1, Exclude: []string{"vendor", "docs"}}
	if base.Digest() != reordered.Digest() {
		t.Error("exclusion order must not change the digest")
	}

	for _, changed := range []Profile{
		{Version: 1, Exclude: []string{"docs"}},
		{Version: 1, Exclude: []string{"docs", "vendor", "test"}},
		{Version: 1, RepositoryID: "custom", Exclude: []string{"docs", "vendor"}},
	} {
		if changed.Digest() == base.Digest() {
			t.Errorf("profile %+v must not share a digest with %+v", changed, base)
		}
	}
}
