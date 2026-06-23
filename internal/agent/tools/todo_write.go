package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"StarCore/internal/agent"
)

// LoopStateRef is set by the Service after tool creation so that
// todo_write can access the shared agent loop state.
var LoopStateRef *LoopState

type TodoWriteTool struct{}

func NewTodoWriteTool() *TodoWriteTool { return &TodoWriteTool{} }

func (t *TodoWriteTool) ID() string             { return "todo_write" }
func (t *TodoWriteTool) Name() string           { return "Update Task List" }
func (t *TodoWriteTool) RequiresApproval() bool { return false }

func (t *TodoWriteTool) Description() string {
	return "更新任务列表。传入完整列表（替换之前的）。每个任务：content（内容）、status（pending/in_progress/completed）、activeForm（进行中时的描述）。"
}

func (t *TodoWriteTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"todos": {Type: "array", Description: "Complete task list. Each: {content, status, activeForm}. One in_progress max."},
		},
		Required: []string{"todos"},
	}
}

func (t *TodoWriteTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	if LoopStateRef == nil {
		return "todo list stored (no persistent state)", nil
	}

	raw, ok := args["todos"]
	if !ok {
		return "", fmt.Errorf("todos parameter is required")
	}

	// The args come from JSON unmarshalling, so todos may be []interface{}
	items, err := parseTodoItems(raw)
	if err != nil {
		return "", fmt.Errorf("invalid todos format: %w", err)
	}

	// Trim whitespace in todo content
	for i := range items {
		items[i].Content = strings.TrimSpace(items[i].Content)
		items[i].ActiveForm = strings.TrimSpace(items[i].ActiveForm)
	}

	LoopStateRef.SetTodos(items)

	if len(items) == 0 {
		return "Task list cleared.", nil
	}

	// Count statuses for the response
	var pending, inProgress, completed int
	for _, it := range items {
		switch it.Status {
		case "in_progress":
			inProgress++
		case "completed":
			completed++
		default:
			pending++
		}
	}
	return fmt.Sprintf("Task list updated: %d pending, %d in progress, %d completed.", pending, inProgress, completed), nil
}

func parseTodoItems(raw any) ([]TodoItem, error) {
	// Try direct JSON marshal/unmarshal (handles map[string]any from tool args)
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var items []TodoItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	return items, nil
}
