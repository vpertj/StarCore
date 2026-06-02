package provider

import (
	"fmt"
	"testing"
)

func TestDiagnoseError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantType    string
		wantRetry   bool
	}{
		{"nil error", nil, "none", false},
		{"auth 401", fmt.Errorf("API returned status 401"), "auth", false},
		{"auth 403", fmt.Errorf("API returned status 403: forbidden"), "auth", false},
		{"auth unauthorized", fmt.Errorf("unauthorized access"), "auth", false},
		{"auth api key", fmt.Errorf("invalid api key"), "auth", false},
		{"rate limit 429", fmt.Errorf("API returned status 429: rate limit exceeded"), "rate_limit", true},
		{"rate limit too many", fmt.Errorf("rate limit: too many requests"), "rate_limit", true},
		{"context length", fmt.Errorf("context_length_exceeded: max 4096"), "context", false},
		{"context token limit", fmt.Errorf("token limit exceeded"), "context", false},
		{"service 500", fmt.Errorf("API returned status 500"), "service", true},
		{"service 502", fmt.Errorf("API returned status 502"), "service", true},
		{"service 503", fmt.Errorf("API returned status 503: service unavailable"), "service", true},
		{"service 504", fmt.Errorf("API returned status 504"), "service", true},
		{"service error", fmt.Errorf("internal server error"), "service", true},
		{"network timeout", fmt.Errorf("network timeout"), "network", true},
		{"network connection", fmt.Errorf("connection refused"), "network", true},
		{"network dns", fmt.Errorf("no such host: api.openai.com"), "network", true},
		{"unknown", fmt.Errorf("something weird happened"), "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DiagnoseError(tt.err)
			if d.Type != tt.wantType {
				t.Errorf("DiagnoseError().Type = %v, want %v", d.Type, tt.wantType)
			}
			if d.Retryable != tt.wantRetry {
				t.Errorf("DiagnoseError().Retryable = %v, want %v", d.Retryable, tt.wantRetry)
			}
		})
	}
}

func TestDiagnoseError_Titles(t *testing.T) {
	auth := DiagnoseError(fmt.Errorf("401"))
	if auth.Title == "" {
		t.Error("auth error should have a title")
	}
	if auth.Action == "" {
		t.Error("auth error should have an action")
	}
	if auth.ActionLabel == "" {
		t.Error("auth error should have an actionLabel")
	}
}

func TestClassifyProviderError(t *testing.T) {
	result := ClassifyProviderError(fmt.Errorf("API returned status 429"))
	if result == "" {
		t.Error("ClassifyProviderError should return non-empty string")
	}
	if !contains(result, "💡") {
		t.Error("ClassifyProviderError should include action suggestion with 💡")
	}

	nilResult := ClassifyProviderError(nil)
	if nilResult != "" {
		t.Errorf("ClassifyProviderError(nil) = %q, want empty string", nilResult)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("truncate should not modify strings shorter than max")
	}
	long := "a very long string that exceeds the maximum length"
	truncated := truncate(long, 10)
	if len(truncated) != 13 { // 10 chars + "..."
		t.Errorf("truncate length = %d, want 13", len(truncated))
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
