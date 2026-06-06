package provider

import (
	"context"
	"encoding/json"
)

type Message struct {
	Role       string                     `json:"role"`
	Content    string                     `json:"content"`
	ToolCalls  []ToolCall                 `json:"tool_calls,omitempty"`
	ToolCallID string                     `json:"tool_call_id,omitempty"`
	Name       string                     `json:"name,omitempty"`
	Extra      map[string]json.RawMessage `json:"-"`
}

// MarshalJSON merges Extra fields into the serialized output, so any
// provider-specific fields (e.g. reasoning_content) are preserved.
func (m Message) MarshalJSON() ([]byte, error) {
	type alias Message
	raw, err := json.Marshal(alias(m))
	if err != nil {
		return nil, err
	}
	if len(m.Extra) == 0 {
		return raw, nil
	}
	var base map[string]json.RawMessage
	if err := json.Unmarshal(raw, &base); err != nil {
		return raw, nil
	}
	for k, v := range m.Extra {
		base[k] = v
	}
	return json.Marshal(base)
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ChatRequest struct {
	ProviderID        string           `json:"providerId"`
	Model             string           `json:"model"`
	Messages          []Message        `json:"messages"`
	Temperature       float64          `json:"temperature"`
	MaxTokens         int              `json:"maxTokens"`
	Stream            bool             `json:"stream"`
	AgentID           string           `json:"agentId,omitempty"`
	ContextFiles      []string         `json:"contextFiles,omitempty"`
	ContextCode       string           `json:"contextCode,omitempty"`
	ProjectPath       string           `json:"projectPath,omitempty"`
	ActiveFile        string           `json:"activeFile,omitempty"`
	ActiveFileContent string           `json:"activeFileContent,omitempty"`
	SelectedCode      string           `json:"selectedCode,omitempty"`
	Tools             []ToolDefinition `json:"tools,omitempty"`
	Mode              string           `json:"mode,omitempty"`
	ConversationID    string           `json:"conversationId,omitempty"`
}

type ChatResponse struct {
	Content  string `json:"content"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type CompletionRequest struct {
	File        string  `json:"file"`
	Content     string  `json:"content"`
	CursorPos   int     `json:"cursorPos"`
	Language    string  `json:"language"`
	Model       string  `json:"model,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type CompletionResponse struct {
	Text string `json:"text"`
}

type StreamEvent struct {
	Type       string                     `json:"type"`
	Content    string                     `json:"content"`
	Name       string                     `json:"name,omitempty"`
	Args       string                     `json:"args,omitempty"`
	Result     string                     `json:"result,omitempty"`
	ToolCalls  []ToolCall                 `json:"tool_calls,omitempty"`
	ToolCallID string                     `json:"tool_call_id,omitempty"`
	Extra      map[string]json.RawMessage `json:"-"`
}

type Model struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	ProviderID       string `json:"providerId"`
	MaxTokens        int    `json:"maxTokens"`
	ContextWindow    int    `json:"contextWindow"`
	SupportsVision   bool   `json:"supportsVision"`
	SupportsTool     bool   `json:"supportsTool"`
	SupportsThinking bool   `json:"supportsThinking"`
}

type ProviderConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	APIKey      string `json:"apiKey"`
	Endpoint    string `json:"endpoint"`
	IsDefault   bool   `json:"isDefault"`
	Enabled     bool   `json:"enabled"`
	TimeoutSecs int    `json:"timeoutSecs,omitempty"` // 0 = use default
}

type ProviderInfo struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Endpoint  string  `json:"endpoint"`
	Enabled   bool    `json:"enabled"`
	IsDefault bool    `json:"isDefault"`
	Models    []Model `json:"models"`
}

type Provider interface {
	ID() string
	Name() string
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
	Completion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
	ListModels(ctx context.Context) ([]Model, error)
	Validate(ctx context.Context) error
	SetConfig(config ProviderConfig)
	GetConfig() ProviderConfig
}

const (
	DefaultFileMode = 0644
	DefaultDirMode  = 0755
)
