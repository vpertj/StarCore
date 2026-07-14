package context

import (
	"os"
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

func TestDeduplicateContextFiles_ContentHash(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.go")
	file2 := filepath.Join(tmpDir, "file2.go")
	content := "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"
	os.WriteFile(file1, []byte(content), 0644)
	os.WriteFile(file2, []byte(content), 0644)
	file3 := filepath.Join(tmpDir, "file3.go")
	os.WriteFile(file3, []byte("package other\n"), 0644)

	result := deduplicateContextFiles([]string{file1, file2, file3})
	if len(result) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(result), result)
	}
}

func TestDeduplicateContextFiles_Containment(t *testing.T) {
	tmpDir := t.TempDir()
	parent := filepath.Join(tmpDir, "parent.go")
	child := filepath.Join(tmpDir, "child.go")
	os.WriteFile(parent, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n\nfunc helper() {}\n"), 0644)
	os.WriteFile(child, []byte("func main() {\n\tprintln(\"hello\")\n}\n"), 0644)

	result := deduplicateContextFiles([]string{parent, child})
	if len(result) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(result), result)
	}
}

func TestDeduplicateContextFiles_Empty(t *testing.T) {
	result := deduplicateContextFiles([]string{})
	if len(result) != 0 {
		t.Errorf("expected empty, got %v", result)
	}
	result = deduplicateContextFiles(nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}
