package context

import (
	"path/filepath"
	"testing"
)

func TestDeduplicateContextFiles_PathNormalization(t *testing.T) {
	absPath, _ := filepath.Abs("test.go")
	files := []string{
		"test.go",
		"./test.go",
		absPath,
		"other.go",
	}

	result := deduplicateContextFiles(files)

	if len(result) != 2 {
		t.Errorf("expected 2 files after dedup, got %d: %v", len(result), result)
	}

	found := false
	for _, f := range result {
		if filepath.Base(f) == "other.go" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected other.go in result, got %v", result)
	}
}
