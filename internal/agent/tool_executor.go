package agent

import (
	"context"
	"fmt"
	"sync"
)

type ToolExecutor struct {
	tools           map[string]Tool
	autoApprove     map[string]bool
	pendingApproval map[string]ToolCall
	mu              sync.RWMutex
}

func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		tools:           make(map[string]Tool),
		autoApprove:     make(map[string]bool),
		pendingApproval: make(map[string]ToolCall),
	}
}

func (e *ToolExecutor) Register(tool Tool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tools[tool.ID()] = tool
	if !tool.RequiresApproval() {
		e.autoApprove[tool.ID()] = true
	}
}

func (e *ToolExecutor) Get(toolID string) (Tool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	t, ok := e.tools[toolID]
	return t, ok
}

func (e *ToolExecutor) List() []Tool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]Tool, 0, len(e.tools))
	for _, t := range e.tools {
		result = append(result, t)
	}
	return result
}

func (e *ToolExecutor) ListToolDefs() []ToolDef {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]ToolDef, 0, len(e.tools))
	for _, t := range e.tools {
		result = append(result, ToolDef{
			ID:               t.ID(),
			Name:             t.Name(),
			Description:      t.Description(),
			Parameters:       t.Parameters(),
			RequiresApproval: t.RequiresApproval(),
		})
	}
	return result
}

func (e *ToolExecutor) SetAutoApprove(toolID string, approve bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.autoApprove[toolID] = approve
}

func (e *ToolExecutor) IsAutoApproved(toolID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.autoApprove[toolID]
}

func (e *ToolExecutor) Unregister(toolID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.tools, toolID)
	delete(e.autoApprove, toolID)
}

func (e *ToolExecutor) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	e.mu.RLock()
	tool, ok := e.tools[call.Name]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool not found: %s", call.Name)
	}

	result, err := tool.Execute(ctx, call.Args)
	if err != nil {
		return &ToolResult{
			CallID: call.ID,
			Name:   call.Name,
			Error:  err.Error(),
		}, nil
	}

	return &ToolResult{
		CallID: call.ID,
		Name:   call.Name,
		Result: result,
	}, nil
}
