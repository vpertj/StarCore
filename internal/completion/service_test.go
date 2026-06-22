package completion

import (
	"testing"
)

func TestLastLine(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\nworld", "world"},
		{"single line", "single line"},
		{"", ""},
		{"line1\nline2\nline3", "line3"},
	}
	for _, tt := range tests {
		result := lastLine(tt.input)
		if result != tt.expected {
			t.Errorf("lastLine(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCompletionCache_PutGet(t *testing.T) {
	cache := &completionCache{
		entries: make(map[string]*cacheEntry),
	}

	sug := &Suggestion{Text: "fmt.Println(", Type: TypeLine, Rank: 1}
	cache.put("test_key", sug)

	result := cache.get("test_key")
	if result == nil {
		t.Error("expected cache hit")
	}
	if result.Text != sug.Text {
		t.Errorf("expected %q, got %q", sug.Text, result.Text)
	}
}

func TestCompletionCache_Miss(t *testing.T) {
	cache := &completionCache{
		entries: make(map[string]*cacheEntry),
	}

	result := cache.get("nonexistent")
	if result != nil {
		t.Error("expected cache miss")
	}
}

func TestFIMRequest(t *testing.T) {
	req := FIMRequest{
		BeforeCursor: "func main() {",
		AfterCursor:  "}",
		FileName:     "main.go",
		Language:     "go",
	}
	if req.Language != "go" {
		t.Errorf("expected language 'go', got %q", req.Language)
	}
}

func TestSuggestion_Types(t *testing.T) {
	sug := Suggestion{Text: "test", Type: TypeFIM, Rank: 1}
	if sug.Type != TypeFIM {
		t.Errorf("expected FIM type, got %q", sug.Type)
	}
}
