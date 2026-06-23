package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"StarCore/internal/agent"
)

type MoveFileTool struct{}

func NewMoveFileTool() *MoveFileTool { return &MoveFileTool{} }

func (t *MoveFileTool) ID() string             { return "move_file" }
func (t *MoveFileTool) Name() string           { return "Move/Rename File" }
func (t *MoveFileTool) RequiresApproval() bool { return true }

func (t *MoveFileTool) Description() string {
	return "Move or rename a file. Works across directories. Creates parent directories of the destination if needed."
}

func (t *MoveFileTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"source": {Type: "string", Description: "Current file path"},
			"dest":   {Type: "string", Description: "New file path"},
		},
		Required: []string{"source", "dest"},
	}
}

func (t *MoveFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	source, _ := args["source"].(string)
	source = strings.TrimSpace(source)
	dest, _ := args["dest"].(string)
	dest = strings.TrimSpace(dest)

	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if dest == "" {
		return "", fmt.Errorf("dest is required")
	}

	if SandboxConfig != nil {
		if err := SandboxConfig.ValidatePath(source); err != nil {
			return "", fmt.Errorf("source path validation failed: %w", err)
		}
		if err := SandboxConfig.ValidatePath(dest); err != nil {
			return "", fmt.Errorf("dest path validation failed: %w", err)
		}
	}

	if _, err := os.Stat(source); err != nil {
		return "", fmt.Errorf("source file not found: %s", source)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return "", fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := os.Rename(source, dest); err != nil {
		return "", fmt.Errorf("failed to move %s → %s: %w", source, dest, err)
	}

	return fmt.Sprintf("📦 Moved %s → %s", source, dest), nil
}
