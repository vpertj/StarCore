package rag

import (
	"testing"
)

func TestChunkText(t *testing.T) {
	svc := NewService(nil)

	text := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"
	chunks := svc.chunkText(text, 512, 64)

	if len(chunks) == 0 {
		t.Error("expected at least 1 chunk")
	}

	combined := ""
	for _, chunk := range chunks {
		combined += chunk
	}
	if len(combined) < len(text)/2 {
		t.Error("combined chunks should cover most of the text")
	}
}

func TestChunkText_Empty(t *testing.T) {
	svc := NewService(nil)
	chunks := svc.chunkText("", 512, 64)
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

func TestExtToLanguage(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".go", "go"},
		{".ts", "typescript"},
		{".js", "javascript"},
		{".py", "python"},
		{".rs", "rust"},
		{".java", "java"},
		{".md", "markdown"},
		{".xyz", "unknown"},
	}
	for _, tt := range tests {
		result := extToLanguage(tt.ext)
		if result != tt.expected {
			t.Errorf("extToLanguage(%q) = %q, want %q", tt.ext, result, tt.expected)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	score := cosineSimilarity(a, b)
	if score < 0.99 {
		t.Errorf("identical vectors should have similarity ~1.0, got %f", score)
	}

	c := []float32{0, 1, 0}
	score = cosineSimilarity(a, c)
	if score > 0.01 {
		t.Errorf("orthogonal vectors should have similarity ~0.0, got %f", score)
	}

	score = cosineSimilarity(nil, b)
	if score != 0 {
		t.Errorf("nil vector should return 0, got %f", score)
	}
}

func TestService_IsIndexed(t *testing.T) {
	svc := NewService(nil)
	if svc.IsIndexed("/nonexistent") {
		t.Error("expected project to not be indexed")
	}
}

func TestDocument(t *testing.T) {
	doc := &Document{
		ID:      "main.go:0",
		Content: "package main",
		Metadata: map[string]string{
			"path":     "main.go",
			"language": "go",
		},
	}
	if doc.Metadata["language"] != "go" {
		t.Error("expected go language")
	}
}
