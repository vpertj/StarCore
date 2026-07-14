package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"StarCore/internal/agent"
)

type CreateDirectoryTool struct{}

func NewCreateDirectoryTool() *CreateDirectoryTool { return &CreateDirectoryTool{} }

func (t *CreateDirectoryTool) ID() string             { return "create_directory" }
func (t *CreateDirectoryTool) Name() string           { return "Create Directory" }
func (t *CreateDirectoryTool) RequiresApproval() bool { return true }

func (t *CreateDirectoryTool) Description() string {
	return "创建目录（自动创建父目录）。用于搭建项目结构。"
}

func (t *CreateDirectoryTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path": {Type: "string", Description: "Directory path to create (e.g. 'src/components')"},
		},
		Required: []string{"path"},
	}
}

func (t *CreateDirectoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
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

	if _, err := os.Stat(path); err == nil {
		return fmt.Sprintf("⏭️ Directory already exists: %s", path), nil
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	return fmt.Sprintf("✅ Created directory: %s", path), nil
}
