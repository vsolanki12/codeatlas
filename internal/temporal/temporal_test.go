package temporal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vsolanki12/hypershift-atlas/internal/domain"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	return dir
}

func TestGitFileHistory(t *testing.T) {
	dir := initTestRepo(t)

	f := filepath.Join(dir, "main.go")
	os.WriteFile(f, []byte("package main"), 0644)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("add", "main.go")
	run("commit", "-m", "first")

	h, err := gitFileHistory(dir, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if h.LastAuthor != "test@example.com" {
		t.Errorf("LastAuthor = %q, want test@example.com", h.LastAuthor)
	}
	if h.ChangeCount != 1 {
		t.Errorf("ChangeCount = %d, want 1", h.ChangeCount)
	}

	os.WriteFile(f, []byte("package main\nfunc main(){}"), 0644)
	run("add", "main.go")
	run("commit", "-m", "second")

	h, err = gitFileHistory(dir, "main.go")
	if err != nil {
		t.Fatal(err)
	}
	if h.ChangeCount != 2 {
		t.Errorf("ChangeCount = %d, want 2", h.ChangeCount)
	}
}

func TestGitFileHistory_Untracked(t *testing.T) {
	dir := initTestRepo(t)

	h, err := gitFileHistory(dir, "nonexistent.go")
	if err != nil {
		t.Fatal(err)
	}
	if h != nil {
		t.Errorf("expected nil for untracked file, got %+v", h)
	}
}

func TestEnrich(t *testing.T) {
	dir := initTestRepo(t)

	f := filepath.Join(dir, "pkg.go")
	os.WriteFile(f, []byte("package pkg"), 0644)

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=author@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=author@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}

	run("add", "pkg.go")
	run("commit", "-m", "init")

	entities := []domain.Entity{
		{ID: "a", Source: domain.Source{File: "pkg.go"}},
		{ID: "b", Source: domain.Source{File: "pkg.go"}},
		{ID: "c", Source: domain.Source{File: "missing.go"}},
		{ID: "d", Source: domain.Source{}},
	}

	if err := Enrich(dir, entities); err != nil {
		t.Fatal(err)
	}

	if entities[0].LastAuthor != "author@test.com" {
		t.Errorf("entity[0] LastAuthor = %q", entities[0].LastAuthor)
	}
	if entities[0].ChangeCount != 1 {
		t.Errorf("entity[0] ChangeCount = %d", entities[0].ChangeCount)
	}
	if entities[1].LastAuthor != "author@test.com" {
		t.Errorf("entity[1] should share cached result")
	}
	if entities[2].LastAuthor != "" {
		t.Errorf("entity[2] missing file should have empty LastAuthor")
	}
	if entities[3].LastAuthor != "" {
		t.Errorf("entity[3] no source file should have empty LastAuthor")
	}
}
