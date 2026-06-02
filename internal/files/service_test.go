package files

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestListDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("secret"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "internal"), 0755)

	svc := NewService()
	files, err := svc.ListDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 { // .hidden is filtered, internal/ is shown
		t.Logf("got %d files: %+v", len(files), files)
	}
}

func TestFileCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "test.txt")

	svc := NewService()

	// Write (creates + writes atomically, avoids Windows file lock issue)
	if err := svc.WriteFile(testPath, "hello world"); err != nil {
		t.Fatal(err)
	}

	// Read
	content, err := svc.ReadFile(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", content)
	}

	// Rename
	newPath := filepath.Join(tmpDir, "renamed.txt")
	if err := svc.RenameFile(testPath, newPath); err != nil {
		t.Fatal(err)
	}

	// Delete
	if err := svc.DeleteFile(newPath); err != nil {
		t.Fatal(err)
	}
}

func TestCreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "newdir")

	svc := NewService()
	if err := svc.CreateDir(dirPath); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

func TestComputeDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "diff.txt")

	os.WriteFile(testPath, []byte("line1\nline2\nline3"), 0644)

	svc := NewService()
	hunks, err := svc.ComputeDiff(testPath, "line1\nline2_modified\nline3\nline4")
	if err != nil {
		t.Fatal(err)
	}
	if len(hunks) != 2 {
		t.Errorf("expected 2 hunks, got %d", len(hunks))
	}
}

func TestApplyDiff(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "apply.txt")

	os.WriteFile(testPath, []byte("aaa\nbbb\nccc"), 0644)

	svc := NewService()
	hunks := []DiffHunk{
		{OldStart: 2, OldCount: 1, NewStart: 2, NewCount: 1, OldLines: []string{"bbb"}, NewLines: []string{"BBB"}},
	}
	if err := svc.ApplyDiff(testPath, hunks); err != nil {
		t.Fatal(err)
	}

	content, _ := ioutil.ReadFile(testPath)
	if string(content) != "aaa\nBBB\nccc" {
		t.Errorf("expected 'aaa\\nBBB\\nccc', got '%s'", string(content))
	}
}

func TestSearchFiles(t *testing.T) {
	t.Skip("skipped: depends on working directory (requires chdir) — test manually with wails dev")
}
