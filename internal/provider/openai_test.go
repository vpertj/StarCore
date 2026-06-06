package provider

import "testing"

func TestResolveChatEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", "https://api.openai.com/v1/chat/completions"},
		{"full openai", "https://api.openai.com/v1/chat/completions", "https://api.openai.com/v1/chat/completions"},
		{"openai base only", "https://api.openai.com", "https://api.openai.com/v1/chat/completions"},
		{"openai with v1", "https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"deepseek base only", "https://api.deepseek.com", "https://api.deepseek.com/v1/chat/completions"},
		{"deepseek full", "https://api.deepseek.com/v1/chat/completions", "https://api.deepseek.com/v1/chat/completions"},
		{"deepseek with trailing slash", "https://api.deepseek.com/", "https://api.deepseek.com/v1/chat/completions"},
		{"volcengine full", "https://ark.cn-beijing.volces.com/api/v3/chat/completions", "https://ark.cn-beijing.volces.com/api/v3/chat/completions"},
		{"volcengine base", "https://ark.cn-beijing.volces.com/api/v3", "https://ark.cn-beijing.volces.com/api/v3/chat/completions"},
		{"siliconflow base", "https://api.siliconflow.cn", "https://api.siliconflow.cn/v1/chat/completions"},
		{"moonshot base", "https://api.moonshot.cn", "https://api.moonshot.cn/v1/chat/completions"},
		{"xai base", "https://api.x.ai", "https://api.x.ai/v1/chat/completions"},
		{"custom with port", "http://localhost:8080", "http://localhost:8080/v1/chat/completions"},
		{"anthropic base", "https://api.anthropic.com", "https://api.anthropic.com/v1/messages"},
		{"anthropic full", "https://api.anthropic.com/v1/messages", "https://api.anthropic.com/v1/messages"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveChatEndpoint(tt.input)
			if got != tt.expected {
				t.Errorf("resolveChatEndpoint(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
