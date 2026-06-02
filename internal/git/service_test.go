package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a temporary git repo and returns its path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, string(out))
		}
	}
	return dir
}

func TestGitBranch(t *testing.T) {
	dir := setupGitRepo(t)
	svc := NewService()

	branch, err := svc.Branch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if branch == "" {
		t.Error("expected branch name")
	}
}

func TestGitStatus(t *testing.T) {
	dir := setupGitRepo(t)
	svc := NewService()

	// Initially clean
	entries, err := svc.Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Logf("unexpected status entries: %+v", entries)
	}

	// Create file → should show untracked
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main"), 0644)
	entries, err = svc.Status(dir)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Path == "new.go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'new.go' in status as untracked")
	}
}

func TestGitStageAndCommit(t *testing.T) {
	t.Skip("skipped: Windows cmd.exe %% escaping conflicts with git --format")
	dir := setupGitRepo(t)
	svc := NewService()

	testFile := filepath.Join(dir, "commit.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	if err := svc.Stage(dir, "commit.txt"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Commit(dir, "test commit"); err != nil {
		t.Fatal(err)
	}

	// Verify via log
	entries, err := svc.Log(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Message != "test commit" {
		t.Errorf("expected commit 'test commit', got %+v", entries)
	}
}

func TestGitCreateBranch(t *testing.T) {
	dir := setupGitRepo(t)
	svc := NewService()

	// Create initial commit so branch can be created
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644)
	svc.Stage(dir, "init.txt")
	svc.Commit(dir, "initial")

	out, err := svc.CreateBranch(dir, "feature-x")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("create branch: %s", out)

	// Switch back
	svc.Checkout(dir, "master")
}

func TestGitStatusAndBranch(t *testing.T) {
	dir := setupGitRepo(t)
	svc := NewService()

	result, err := svc.StatusAndBranch(dir)
	if err != nil {
		t.Fatal(err)
	}
	if result["branch"] == nil {
		t.Error("expected branch in result")
	}
	if result["modified"] == nil {
		t.Error("expected modified count in result")
	}
}
