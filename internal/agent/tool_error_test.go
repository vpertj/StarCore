package agent

import (
	"errors"
	"testing"
)

func TestClassifyToolError_FileNotFound(t *testing.T) {
	err := errors.New("file not found: /path/to/file")
	result := ClassifyToolError("read_file", err)
	if result.Category != "file_not_found" {
		t.Errorf("expected file_not_found, got %s", result.Category)
	}
	if result.AutoRetry {
		t.Error("file_not_found should not auto-retry")
	}
}

func TestClassifyToolError_Timeout(t *testing.T) {
	err := errors.New("context deadline exceeded")
	result := ClassifyToolError("execute_command", err)
	if result.Category != "timeout" {
		t.Errorf("expected timeout, got %s", result.Category)
	}
	if !result.AutoRetry {
		t.Error("timeout should auto-retry")
	}
}

func TestClassifyToolError_Security(t *testing.T) {
	err := errors.New("path traversal detected")
	result := ClassifyToolError("read_file", err)
	if result.Category != "security" {
		t.Errorf("expected security, got %s", result.Category)
	}
	if result.Severity != ErrorFatal {
		t.Error("security should be fatal")
	}
}

func TestClassifyToolError_Syntax(t *testing.T) {
	err := errors.New("syntax error in file.go")
	result := ClassifyToolError("write_file", err)
	if result.Category != "syntax" {
		t.Errorf("expected syntax, got %s", result.Category)
	}
}

func TestClassifyToolError_Unknown(t *testing.T) {
	err := errors.New("something unexpected happened")
	result := ClassifyToolError("execute_command", err)
	if result.Category != "unknown" {
		t.Errorf("expected unknown, got %s", result.Category)
	}
}

func TestClassifyToolError_Nil(t *testing.T) {
	result := ClassifyToolError("read_file", nil)
	if result.Category != "none" {
		t.Errorf("expected none, got %s", result.Category)
	}
}

func TestFormatClassifiedError(t *testing.T) {
	classified := &ClassifiedError{
		Severity:   ErrorNeedsLLM,
		Category:   "file_not_found",
		Message:    "file not found",
		Suggestion: "check path",
	}
	result := FormatClassifiedError(classified)
	if result == "" {
		t.Error("expected non-empty formatted error")
	}
}

func TestExtractFilePathFromToolArgs(t *testing.T) {
	args := map[string]any{"path": "/test/file.go"}
	if got := ExtractFilePathFromToolArgs(args); got != "/test/file.go" {
		t.Errorf("expected /test/file.go, got %s", got)
	}

	args2 := map[string]any{}
	if got := ExtractFilePathFromToolArgs(args2); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}
