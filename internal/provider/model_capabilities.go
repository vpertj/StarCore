package provider

import "strings"

// ModelCapabilities describes what a model can do.
type ModelCapabilities struct {
	SupportsTool     bool // Supports function/tool calling
	SupportsVision   bool // Supports image input
	SupportsThinking bool // Supports reasoning/thinking tokens
	ContextWindow    int  // Max context window in tokens
	MaxOutput        int  // Max output tokens
}

// KnownModels maps model name patterns to their capabilities.
// Use lowercase keys. Match by prefix (longest match wins).
var KnownModels = map[string]ModelCapabilities{
	// OpenAI GPT-4o family
	"gpt-4o": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 16384,
	},
	"gpt-4o-mini": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 16384,
	},
	"gpt-4-turbo": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 4096,
	},
	"gpt-4": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"gpt-3.5-turbo": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 16385, MaxOutput: 4096,
	},

	// OpenAI o-series (reasoning models)
	"o1": {
		SupportsTool: false, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 200000, MaxOutput: 100000,
	},
	"o1-mini": {
		SupportsTool: false, SupportsVision: false, SupportsThinking: true,
		ContextWindow: 128000, MaxOutput: 65536,
	},
	"o3": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 200000, MaxOutput: 100000,
	},
	"o3-mini": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: true,
		ContextWindow: 200000, MaxOutput: 100000,
	},

	// Anthropic Claude 4
	"claude-opus-4": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 200000, MaxOutput: 32000,
	},
	"claude-sonnet-4": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 200000, MaxOutput: 64000,
	},

	// Anthropic Claude 3.5
	"claude-3-5-sonnet": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 8192,
	},
	"claude-3-5-haiku": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 8192,
	},

	// Anthropic Claude 3
	"claude-3-opus": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 4096,
	},
	"claude-3-sonnet": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 4096,
	},
	"claude-3-haiku": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 200000, MaxOutput: 4096,
	},

	// DeepSeek
	"deepseek-v3": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"deepseek-v2.5": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"deepseek-r1": {
		SupportsTool: false, SupportsVision: false, SupportsThinking: true,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"deepseek-chat": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"deepseek-coder": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},

	// Qwen
	"qwen-max": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"qwen-plus": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"qwen-turbo": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},
	"qwen-vl": {
		SupportsTool: false, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 8192,
	},

	// Google Gemini
	"gemini-2.5-pro": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 1000000, MaxOutput: 65536,
	},
	"gemini-2.5-flash": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: true,
		ContextWindow: 1000000, MaxOutput: 65536,
	},
	"gemini-2.0-flash": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 1000000, MaxOutput: 8192,
	},
	"gemini-1.5-pro": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 2000000, MaxOutput: 8192,
	},
	"gemini-1.5-flash": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 1000000, MaxOutput: 8192,
	},

	// Meta Llama
	"llama-3.1": {
		SupportsTool: true, SupportsVision: false, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 4096,
	},
	"llama-3.2": {
		SupportsTool: true, SupportsVision: true, SupportsThinking: false,
		ContextWindow: 128000, MaxOutput: 4096,
	},
}

// GetModelCapabilities returns the capabilities for a model.
// Uses longest prefix match against KnownModels.
func GetModelCapabilities(model string) ModelCapabilities {
	modelLower := strings.ToLower(model)

	// Try exact match first
	if caps, ok := KnownModels[modelLower]; ok {
		return caps
	}

	// Try prefix match (longest wins)
	bestMatch := ""
	var bestCaps ModelCapabilities
	for pattern, caps := range KnownModels {
		if strings.HasPrefix(modelLower, pattern) && len(pattern) > len(bestMatch) {
			bestMatch = pattern
			bestCaps = caps
		}
	}

	if bestMatch != "" {
		return bestCaps
	}

	// Unknown model: assume basic capabilities
	return ModelCapabilities{
		SupportsTool:     false, // Don't assume tool support
		SupportsVision:   false,
		SupportsThinking: false,
		ContextWindow:    128000,
		MaxOutput:        4096,
	}
}

// SupportsFunctionCalling checks if a model supports function/tool calling.
func SupportsFunctionCalling(model string) bool {
	return GetModelCapabilities(model).SupportsTool
}
