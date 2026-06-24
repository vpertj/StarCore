package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestToolExecutor_FileFingerprintCache(t *testing.T) {
	executor := NewToolExecutor()

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(testFile)

	entry := &cacheEntry{
		result:      &ToolResult{Result: "cached"},
		createdAt:   time.Now().Add(-1 * time.Minute),
		accessAt:    time.Now(),
		key:         "read_file:" + testFile,
		fileModTime: info.ModTime(),
		isFileCache: true,
	}

	executor.mu.Lock()
	executor.cache[entry.key] = entry
	executor.mu.Unlock()

	if !executor.isCacheValid(entry) {
		t.Error("cache should be valid when file unchanged")
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(testFile, []byte("modified"), 0644); err != nil {
		t.Fatal(err)
	}

	if executor.isCacheValid(entry) {
		t.Error("cache should be invalid after file modification")
	}
}

func TestExtractFilePathFromCacheKey(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"read_file:/path/to/file", "/path/to/file"},
		{"glob_files:pattern:/path/to/dir", "/path/to/dir"},
		{"unknown:key", ""},
		{"invalid", ""},
	}
	for _, tt := range tests {
		got := extractFilePathFromCacheKey(tt.key)
		if got != tt.expected {
			t.Errorf("extractFilePathFromCacheKey(%q) = %q, want %q", tt.key, got, tt.expected)
		}
	}
}

func TestToolExecutor_NonFileCacheTTL(t *testing.T) {
	executor := NewToolExecutor()

	entry := &cacheEntry{
		result:    &ToolResult{Result: "cached"},
		createdAt: time.Now(),
		accessAt:  time.Now(),
		key:       "search_files:query:path",
	}

	if !executor.isCacheValid(entry) {
		t.Error("recent non-file cache should be valid")
	}

	entry.createdAt = time.Now().Add(-1 * time.Minute)
	if executor.isCacheValid(entry) {
		t.Error("old non-file cache should be invalid")
	}
}
