package agent

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type ToolExecutor struct {
	tools           map[string]Tool
	autoApprove     map[string]bool
	pendingApproval map[string]chan bool
	mu              sync.RWMutex
}

func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{
		tools:           make(map[string]Tool),
		autoApprove:     make(map[string]bool),
		pendingApproval: make(map[string]chan bool),
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

// WaitForApproval blocks until the user approves or rejects a tool call.
// Returns true if approved, false if rejected.
func (e *ToolExecutor) WaitForApproval(ctx context.Context, callID string) bool {
	ch := make(chan bool, 1)
	e.mu.Lock()
	e.pendingApproval[callID] = ch
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.pendingApproval, callID)
		e.mu.Unlock()
	}()

	select {
	case approved := <-ch:
		return approved
	case <-ctx.Done():
		return false
	}
}

// RespondApproval is called from the frontend (via Wails) to approve/reject a tool call.
func (e *ToolExecutor) RespondApproval(callID string, approved bool) bool {
	e.mu.Lock()
	ch, ok := e.pendingApproval[callID]
	e.mu.Unlock()
	if !ok {
		return false
	}
	ch <- approved
	return true
}

func (e *ToolExecutor) Execute(ctx context.Context, call ToolCall) (*ToolResult, error) {
	e.mu.RLock()
	tool, ok := e.tools[call.Name]
	autoApproved := e.autoApprove[call.Name]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("tool not found: %s", call.Name)
	}

	// Check if approval is needed
	if tool.RequiresApproval() && !autoApproved {
		if !e.WaitForApproval(ctx, call.ID) {
			return &ToolResult{
				CallID: call.ID,
				Name:   call.Name,
				Error:  "用户拒绝了此操作",
			}, nil
		}
	}

	result, err := tool.Execute(ctx, call.Args)
	if err != nil {
		return &ToolResult{
			CallID: call.ID,
			Name:   call.Name,
			Error:  err.Error(),
		}, nil
	}

	tr := &ToolResult{
		CallID: call.ID,
		Name:   call.Name,
		Result: result,
	}
	tr.FileMeta = buildFileMetaFromResult(call.Name, call.Args, result)
	return tr, nil
}

// buildFileMetaFromResult constructs FileMeta from the tool name, args, and result.
func buildFileMetaFromResult(name string, args map[string]any, result string) *FileMeta {
	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		lines := strings.Count(result, "\n") + 1
		return &FileMeta{Operation: "read", FilePath: path, StartLine: 1, EndLine: lines, Summary: fmt.Sprintf("L1-%d", lines)}
	case "write_file":
		path, _ := args["path"].(string)
		lines := strings.Count(result, "\n") + 1
		return &FileMeta{Operation: "write", FilePath: path, Summary: fmt.Sprintf("%d lines", lines)}
	case "edit_file":
		path, _ := args["path"].(string)
		// Try to find the line number of the edited text
		oldStr, _ := args["old_string"].(string)
		startLine, endLine := findLineRangeInResult(result)
		fm := &FileMeta{Operation: "edit", FilePath: path}
		if startLine > 0 {
			fm.StartLine = startLine
			fm.EndLine = endLine
			fm.Summary = fmt.Sprintf("L%d-%d", startLine, endLine)
		} else if oldStr != "" {
			oldLines := strings.Count(oldStr, "\n") + 1
			fm.Summary = fmt.Sprintf("%d lines replaced", oldLines)
		}
		return fm
	case "search_files":
		path, _ := args["path"].(string)
		matches := strings.Count(result, "\n") + 1
		if matches > 100 {
			matches = 100
		}
		return &FileMeta{Operation: "search", FilePath: path, Summary: fmt.Sprintf("%d results", matches)}
	case "glob_files":
		path, _ := args["path"].(string)
		count := strings.Count(result, "\n") + 1
		return &FileMeta{Operation: "glob", FilePath: path, Summary: fmt.Sprintf("%d files", count)}
	case "execute_command":
		cmd, _ := args["command"].(string)
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}
		return &FileMeta{Operation: "exec", Summary: cmd}
	default:
		return nil
	}
}

func findLineRangeInResult(result string) (int, int) {
	// Look for line number patterns in edit result like "L39-43" or "line 39"
	// This is a best-effort heuristic
	re := regexp.MustCompile(`L(\d+)(?:-(\d+))?`)
	m := re.FindStringSubmatch(result)
	if len(m) >= 2 {
		start, _ := strconv.Atoi(m[1])
		end := start
		if len(m) >= 3 && m[2] != "" {
			end, _ = strconv.Atoi(m[2])
		}
		return start, end
	}
	return 0, 0
}
