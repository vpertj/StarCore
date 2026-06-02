package agent

import "context"

type ToolParameters struct {
	Type       string                   `json:"type"`
	Properties map[string]ToolParamProp `json:"properties"`
	Required   []string                 `json:"required"`
}

type ToolParamProp struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ToolCall struct {
	ID   string         `json:"id"`
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

type ToolResult struct {
	CallID string `json:"callId"`
	Name   string `json:"name"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

type ToolDef struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Parameters       ToolParameters `json:"parameters"`
	RequiresApproval bool           `json:"requiresApproval"`
}

type Tool interface {
	ID() string
	Name() string
	Description() string
	Parameters() ToolParameters
	Execute(ctx context.Context, args map[string]any) (string, error)
	RequiresApproval() bool // false = read-only safe tool, true = needs user approval
}
