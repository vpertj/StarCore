package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile_Basic(t *testing.T) {
	dir, _ := os.MkdirTemp("", "readfile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "test.txt")
	os.WriteFile(fp, []byte("line1\nline2\nline3\n"), 0644)

	tool := NewReadFileTool()
	result, err := tool.Execute(context.Background(), map[string]any{"path": fp})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestReadFile_WithLineRange(t *testing.T) {
	dir, _ := os.MkdirTemp("", "readfile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "test.txt")
	os.WriteFile(fp, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

	tool := NewReadFileTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":       fp,
		"start_line": 2,
		"end_line":   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "line2\nline3" {
		t.Errorf("unexpected result: %q", result)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	tool := NewReadFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{"path": "/nonexistent/file.txt"})
	if err == nil {
		t.Error("should error for nonexistent file")
	}
}

func TestWriteFile_Create(t *testing.T) {
	dir, _ := os.MkdirTemp("", "writefile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "new.txt")

	tool := NewWriteFileTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":    fp,
		"content": "hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	data, _ := os.ReadFile(fp)
	if string(data) != "hello world" {
		t.Errorf("file content = %q, want %q", string(data), "hello world")
	}
}

func TestWriteFile_Overwrite(t *testing.T) {
	dir, _ := os.MkdirTemp("", "writefile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "existing.txt")
	os.WriteFile(fp, []byte("old"), 0644)

	tool := NewWriteFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":    fp,
		"content": "new",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(fp)
	if string(data) != "new" {
		t.Errorf("file content = %q, want %q", string(data), "new")
	}
}

func TestWriteFile_MissingPath(t *testing.T) {
	tool := NewWriteFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{"content": "data"})
	if err == nil {
		t.Error("should error for missing path")
	}
}

func TestEditFile_Replace(t *testing.T) {
	dir, _ := os.MkdirTemp("", "editfile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "edit.txt")
	os.WriteFile(fp, []byte("hello world\nfoo bar\n"), 0644)

	tool := NewEditFileTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"path":       fp,
		"old_string": "hello world",
		"new_string": "goodbye world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
	data, _ := os.ReadFile(fp)
	if string(data) != "goodbye world\nfoo bar\n" {
		t.Errorf("file content = %q", string(data))
	}
}

func TestEditFile_NotFound(t *testing.T) {
	dir, _ := os.MkdirTemp("", "editfile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "edit.txt")
	os.WriteFile(fp, []byte("hello\n"), 0644)

	tool := NewEditFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":       fp,
		"old_string": "not present",
		"new_string": "replacement",
	})
	if err == nil {
		t.Error("should error when old_string not found")
	}
}

func TestEditFile_DuplicateMatch(t *testing.T) {
	dir, _ := os.MkdirTemp("", "editfile")
	defer os.RemoveAll(dir)
	fp := filepath.Join(dir, "dup.txt")
	os.WriteFile(fp, []byte("foo\nbar\nfoo\n"), 0644)

	tool := NewEditFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":       fp,
		"old_string": "foo",
		"new_string": "baz",
	})
	if err == nil {
		t.Error("should error when old_string matches multiple times")
	}
}

func TestEditFile_MissingPath(t *testing.T) {
	tool := NewEditFileTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"path":       "",
		"old_string": "a",
		"new_string": "b",
	})
	if err == nil {
		t.Error("should error for missing path")
	}
}

func TestExecuteCommand_Simple(t *testing.T) {
	tool := NewExecuteCommandTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hello",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("result = %q, want %q", result, "hello")
	}
}

func TestExecuteCommand_Timeout(t *testing.T) {
	tool := NewExecuteCommandTool()
	_, err := tool.Execute(context.Background(), map[string]any{
		"command":     "timeout 5 ping -n 5 127.0.0.1 >nul 2>&1 || ping -c 5 127.0.0.1",
		"timeout_sec": 1,
	})
	if err == nil {
		t.Error("should timeout")
	}
}

func TestExecuteCommand_MissingCommand(t *testing.T) {
	tool := NewExecuteCommandTool()
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("should error for missing command")
	}
}

func TestGlobFiles_Basic(t *testing.T) {
	dir, _ := os.MkdirTemp("", "glob")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("pkg"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("txt"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	tool := NewGlobTool()
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGlobFiles_DoubleStar(t *testing.T) {
	dir, _ := os.MkdirTemp("", "glob2")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("pkg main"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "pkg", "util.go"), []byte("pkg util"), 0644)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	tool := NewGlobTool()

	// Test **/*.go should find both .go files in nested dirs
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("expected main.go in result, got: %s", result)
	}
	if !strings.Contains(result, "util.go") {
		t.Errorf("expected util.go in result, got: %s", result)
	}

	// Test src/**/*.go should only find files under src/
	result2, err := tool.Execute(context.Background(), map[string]any{"pattern": "src/**/*.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result2, "main.go") {
		t.Errorf("expected main.go in result, got: %s", result2)
	}
	if !strings.Contains(result2, "util.go") {
		t.Errorf("expected util.go in result, got: %s", result2)
	}

	// Test *.md should only find README.md (no **)
	result3, err := tool.Execute(context.Background(), map[string]any{"pattern": "*.md"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result3, "README.md") {
		t.Errorf("expected README.md in result, got: %s", result3)
	}
	if strings.Contains(result3, "main.go") {
		t.Errorf("should not contain main.go, got: %s", result3)
	}
}

func TestGlobFiles_NoMatch(t *testing.T) {
	dir, _ := os.MkdirTemp("", "glob")
	defer os.RemoveAll(dir)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	tool := NewGlobTool()
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "*.xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("should return 'no match' message")
	}
}

func TestGlobFiles_MissingPattern(t *testing.T) {
	tool := NewGlobTool()
	_, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Error("should error for missing pattern")
	}
}
