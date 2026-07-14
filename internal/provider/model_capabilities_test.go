package provider

import (
	"testing"
)

func TestGetModelCapabilities(t *testing.T) {
	tests := []struct {
		model          string
		supportsTool   bool
		supportsVision bool
		contextWindow  int
	}{
		// OpenAI
		{"gpt-4o", true, true, 200000},
		{"gpt-4o-mini", true, true, 128000},
		{"gpt-4-turbo", true, true, 128000},
		{"gpt-4", true, false, 128000},
		{"gpt-3.5-turbo", true, false, 16385},

		// OpenAI o-series
		{"o1", false, true, 200000},
		{"o1-mini", false, false, 128000},
		{"o3", true, true, 200000},
		{"o3-mini", true, false, 200000},

		// Anthropic
		{"claude-opus-4", true, true, 200000},
		{"claude-sonnet-4", true, true, 200000},
		{"claude-3-5-sonnet", true, true, 200000},
		{"claude-3-5-haiku", true, true, 200000},

		// DeepSeek
		{"deepseek-v3", true, false, 128000},
		{"deepseek-r1", false, false, 128000},
		{"deepseek-chat", true, false, 128000},

		// Unknown model
		{"unknown-model-xyz", false, false, 128000},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			caps := GetModelCapabilities(tt.model)
			if caps.SupportsTool != tt.supportsTool {
				t.Errorf("SupportsTool: got %v, want %v", caps.SupportsTool, tt.supportsTool)
			}
			if caps.SupportsVision != tt.supportsVision {
				t.Errorf("SupportsVision: got %v, want %v", caps.SupportsVision, tt.supportsVision)
			}
			if caps.ContextWindow != tt.contextWindow {
				t.Errorf("ContextWindow: got %d, want %d", caps.ContextWindow, tt.contextWindow)
			}
		})
	}
}

func TestSupportsFunctionCalling(t *testing.T) {
	if !SupportsFunctionCalling("gpt-4o") {
		t.Error("gpt-4o should support function calling")
	}
	if SupportsFunctionCalling("o1") {
		t.Error("o1 should NOT support function calling")
	}
	if SupportsFunctionCalling("deepseek-r1") {
		t.Error("deepseek-r1 should NOT support function calling")
	}
	if !SupportsFunctionCalling("deepseek-v3") {
		t.Error("deepseek-v3 should support function calling")
	}
}

func TestEstimateContextWindow(t *testing.T) {
	if got := EstimateContextWindow("gpt-4o"); got != 200000 {
		t.Errorf("gpt-4o context window: got %d, want 200000", got)
	}
	if got := EstimateContextWindow("unknown-model"); got != 128000 {
		t.Errorf("unknown model context window: got %d, want 128000", got)
	}
}
