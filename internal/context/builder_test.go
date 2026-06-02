package context

import (
	"os"
	"path/filepath"
	"testing"

	"StarCore/internal/provider"
)

func TestBuildContextMessage_WithContextFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	content := "package main\n\nfunc main() {}"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	b := &Builder{}
	req := provider.ChatRequest{
		ContextFiles: []string{testFile},
	}

	msg := b.BuildContextMessage(req)
	if msg == "" {
		t.Fatal("expected non-empty context message")
	}
	if !contains(msg, "[Context Files]") {
		t.Error("expected [Context Files] section")
	}
	if !contains(msg, testFile) {
		t.Error("expected file path in context")
	}
	if !contains(msg, content) {
		t.Error("expected file content in context")
	}
}

func TestBuildContextMessage_WithProjectPath(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "internal"), 0755); err != nil {
		t.Fatal(err)
	}

	b := &Builder{}
	req := provider.ChatRequest{
		ProjectPath: tmpDir,
	}

	msg := b.BuildContextMessage(req)
	if msg == "" {
		t.Fatal("expected non-empty context message")
	}
	if !contains(msg, "[Project Structure]") {
		t.Error("expected [Project Structure] section")
	}
}

func TestBuildContextMessage_WithActiveFile(t *testing.T) {
	b := &Builder{}
	req := provider.ChatRequest{
		ActiveFile:        "app.go",
		ActiveFileContent: "package main\n\nfunc main() {}",
	}

	msg := b.BuildContextMessage(req)
	if msg == "" {
		t.Fatal("expected non-empty context message")
	}
	if !contains(msg, "[Currently Open File]") {
		t.Error("expected [Currently Open File] section")
	}
	if !contains(msg, "app.go") {
		t.Error("expected active file name in context")
	}
}

func TestBuildContextMessage_WithSelectedCode(t *testing.T) {
	b := &Builder{}
	req := provider.ChatRequest{
		SelectedCode: "func hello() { fmt.Println(\"hello\") }",
	}

	msg := b.BuildContextMessage(req)
	if msg == "" {
		t.Fatal("expected non-empty context message")
	}
	if !contains(msg, "[Selected Code]") {
		t.Error("expected [Selected Code] section")
	}
}

func TestBuildContextMessage_EmptyRequest(t *testing.T) {
	b := &Builder{}
	req := provider.ChatRequest{}

	msg := b.BuildContextMessage(req)
	if msg != "" {
		t.Errorf("expected empty message for empty request, got: %s", msg)
	}
}

func TestBuildContextMessage_ContextFilesTruncation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.go")

	largeContent := make([]byte, maxContextFileSize+1000)
	for i := range largeContent {
		largeContent[i] = 'a'
	}
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatal(err)
	}

	b := &Builder{}
	req := provider.ChatRequest{
		ContextFiles: []string{testFile},
	}

	msg := b.BuildContextMessage(req)
	if !contains(msg, "[truncated]") {
		t.Error("expected truncated marker for large file")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
