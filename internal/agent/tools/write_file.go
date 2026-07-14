package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"StarCore/internal/agent"
)

type WriteFileTool struct{}

func NewWriteFileTool() *WriteFileTool { return &WriteFileTool{} }

func (t *WriteFileTool) ID() string             { return "write_file" }
func (t *WriteFileTool) Name() string           { return "Write File" }
func (t *WriteFileTool) RequiresApproval() bool { return true }

func (t *WriteFileTool) Description() string {
	return "创建或覆写文件。使用 edit_file 做局部修改，write_file 适合创建新文件或完全重写。自动创建父目录。"
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
	path = strings.TrimSpace(path)
	content, ok := args["content"].(string)
	if !ok {
		return "", fmt.Errorf("content is required")
	}

	if cfg := GetSandboxConfig(); cfg != nil {
		if err := cfg.ValidatePath(path); err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	// Read old content before overwriting (for diff) — do this BEFORE atomic write
	oldData, readErr := os.ReadFile(path)
	var oldContent string
	existed := readErr == nil
	if existed {
		oldContent = string(oldData)
	}

	// Atomic write: write to temp file in same dir, then rename
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".starcore-write-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Write content to temp file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename (same filesystem — safe because tmp is in same dir as target)
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to atomically replace file: %w", err)
	}

	// Build informative result message
	oldLines := strings.Count(oldContent, "\n") + 1
	newLines := strings.Count(content, "\n") + 1

	var resultMsg string
	if !existed {
		resultMsg = fmt.Sprintf("✅ Created %s (%d lines, %d bytes)", path, newLines, len(content))
	} else if oldContent == content {
		resultMsg = fmt.Sprintf("⏭️ %s unchanged", path)
	} else {
		diffLines := newLines - oldLines
		diffStr := fmt.Sprintf("%+d", diffLines)
		resultMsg = fmt.Sprintf("✏️ Modified %s: %d→%d lines (%s), %d→%d bytes", path, oldLines, newLines, diffStr, len(oldContent), len(content))
	}

	// Quick post-write syntax check
	if syntaxErr := QuickSyntaxCheck(path); syntaxErr != "" {
		resultMsg += "\n" + syntaxErr
	}

	// File modification rate limit check
	if ls := loopStateRef.Load(); ls != nil {
		if rateMsg := ls.CheckFileRateLimit(path); rateMsg != "" {
			resultMsg += "\n" + rateMsg
		}
	}

	return resultMsg, nil
}
