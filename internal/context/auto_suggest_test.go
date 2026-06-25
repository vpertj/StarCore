package context

import (
	"os"
	"path/filepath"
	"testing"
)

func TestContextSuggester_SameDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{"main.go", "utils.go", "handler.go", "main_test.go"}
	for _, f := range files {
		os.WriteFile(filepath.Join(tmpDir, f), []byte("package main\n"), 0644)
	}

	suggester := NewContextSuggester(tmpDir)
	activeFile := filepath.Join(tmpDir, "main.go")

	suggestions := suggester.Suggest(activeFile, "", 5, nil)

	if len(suggestions) < 2 {
		t.Errorf("expected at least 2 suggestions, got %d", len(suggestions))
	}

	for _, s := range suggestions {
		if filepath.Base(s.FilePath) == "main_test.go" {
			t.Error("should not suggest test file for non-test active file")
		}
	}
}

func TestContextSuggester_NoDuplicates(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte("package main\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte("package main\n"), 0644)

	suggester := NewContextSuggester(tmpDir)
	activeFile := filepath.Join(tmpDir, "a.go")
	existing := []string{filepath.Join(tmpDir, "b.go")}

	suggestions := suggester.Suggest(activeFile, "", 5, existing)

	for _, s := range suggestions {
		if s.FilePath == filepath.Join(tmpDir, "b.go") {
			t.Error("should not suggest already existing file")
		}
	}
}

func TestContextSuggester_EmptyActiveFile(t *testing.T) {
	suggester := NewContextSuggester("/some/path")
	suggestions := suggester.Suggest("", "", 5, nil)
	if len(suggestions) != 0 {
		t.Errorf("expected 0 suggestions for empty active file, got %d", len(suggestions))
	}
}
