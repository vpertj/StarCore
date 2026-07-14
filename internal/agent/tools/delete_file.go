package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"StarCore/internal/agent"
)

type DeleteFileTool struct{}

func NewDeleteFileTool() *DeleteFileTool { return &DeleteFileTool{} }

func (t *DeleteFileTool) ID() string             { return "delete_file" }
func (t *DeleteFileTool) Name() string           { return "Delete File" }
func (t *DeleteFileTool) RequiresApproval() bool { return true }

func (t *DeleteFileTool) Description() string {
	return "删除文件。不能删除目录（需用 execute_command + rm -r）。"
}

func (t *DeleteFileTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path": {Type: "string", Description: "File path to delete"},
		},
		Required: []string{"path"},
	}
}

func (t *DeleteFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	path, ok := args["path"].(string)
	if !ok || path == "" {
		return "", fmt.Errorf("path is required")
	}
	path = strings.TrimSpace(path)

	if cfg := GetSandboxConfig(); cfg != nil {
		if err := cfg.ValidatePath(path); err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, not a file. Use 'execute_command' with 'rm -r' to delete directories.", path)
	}

	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("failed to delete %s: %w", path, err)
	}

	return fmt.Sprintf("🗑️ Deleted %s (%d bytes)", path, info.Size()), nil
}
