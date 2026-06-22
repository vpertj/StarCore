package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"StarCore/internal/agent"
)

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }

func (t *WriteFileTool) ID() string             { return "write_file" }
func (t *WriteFileTool) Name() string           { return "Write File" }
func (t *WriteFileTool) RequiresApproval() bool { return true }

func (t *WriteFileTool) Description() string {
	return "Write content to a file. Creates the file if it does not exist."
}

func (t *WriteFileTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":    {Type: "string", Description: "File path to write"},
			"content": {Type: "string", Description: "Content to write to the file"},
		},
		Required: []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	path, ok := args["path"].(string)
	if !ok {
		return "", fmt.Errorf("path is required")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	if SandboxConfig != nil {
		if err := SandboxConfig.ValidatePath(path); err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	// Read old content before overwriting (for diff)
	oldData, readErr := os.ReadFile(path)
	var oldContent string
	existed := readErr == nil
	if existed {
		oldContent = string(oldData)
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", err
	}

	// Build informative result message
	oldLines := strings.Count(oldContent, "\n") + 1
	newLines := strings.Count(content, "\n") + 1
	if !existed {
		return fmt.Sprintf("✅ Created %s (%d lines, %d bytes)", path, newLines, len(content)), nil
	}
	if oldContent == content {
		return fmt.Sprintf("⏭️ %s unchanged", path), nil
	}
	diffLines := newLines - oldLines
	diffStr := fmt.Sprintf("%+d", diffLines)
	return fmt.Sprintf("✏️ Modified %s: %d→%d lines (%s), %d→%d bytes", path, oldLines, newLines, diffStr, len(oldContent), len(content)), nil
}
