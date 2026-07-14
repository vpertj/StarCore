package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"StarCore/internal/agent"
)

type EditFileTool struct{}

func NewEditFileTool() *EditFileTool { return &EditFileTool{} }

func (t *EditFileTool) ID() string             { return "edit_file" }
func (t *EditFileTool) Name() string           { return "Edit File" }
func (t *EditFileTool) RequiresApproval() bool { return true }

func (t *EditFileTool) Description() string {
	return "精确替换文件中的文本。old_string 必须唯一匹配（包括空格）。比 write_file 更安全，适合修改已有文件的局部内容。"
}

func (t *EditFileTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":       {Type: "string", Description: "File path to edit"},
			"old_string": {Type: "string", Description: "Exact text to find and replace"},
			"new_string": {Type: "string", Description: "Replacement text"},
		},
		Required: []string{"path", "old_string", "new_string"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	path, _ := args["path"].(string)
	path = strings.TrimSpace(path)
	oldStr, _ := args["old_string"].(string)
	newStr, _ := args["new_string"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldStr == "" && newStr == "" {
		return "", fmt.Errorf("old_string or new_string is required")
	}

	if cfg := GetSandboxConfig(); cfg != nil {
		if err := cfg.ValidatePath(path); err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", path, err)
	}
	content := string(data)

	count := strings.Count(content, oldStr)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", path)
	}
	if count > 1 {
		// Show line numbers for each occurrence to help the AI provide more context.
		var lines []string
		offset := 0
		lineNum := 1
		for i, ch := range content {
			if i >= offset && strings.HasPrefix(content[i:], oldStr) {
				lines = append(lines, fmt.Sprintf("  line %d: ...%s...", lineNum, snippetAround(content, i, len(oldStr))))
				offset = i + len(oldStr)
			}
			if ch == '\n' {
				lineNum++
			}
		}
		return "", fmt.Errorf("old_string appears %d times in %s — provide more surrounding context:\n%s",
			count, path, strings.Join(lines, "\n"))
	}

	newContent := strings.Replace(content, oldStr, newStr, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write %s: %w", path, err)
	}

	// Compute a concise diff summary.
	oldLines := strings.Count(oldStr, "\n")
	newLines := strings.Count(newStr, "\n")
	oldPreview := truncate(oldStr, 60)
	newPreview := truncate(newStr, 60)

	var resultMsg string
	if oldStr == newStr {
		resultMsg = fmt.Sprintf("⏭️ %s unchanged", path)
	} else if oldLines == 0 && newLines == 0 {
		resultMsg = fmt.Sprintf("✏️ %s: replaced \"%s\" → \"%s\"", path, oldPreview, newPreview)
	} else {
		resultMsg = fmt.Sprintf("✏️ %s: replaced %d→%d lines", path, oldLines+1, newLines+1)
	}

	// Quick post-edit syntax check
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

func snippetAround(content string, pos int, matchLen int) string {
	start := pos - 15
	if start < 0 {
		start = 0
	}
	end := pos + matchLen + 15
	if end > len(content) {
		end = len(content)
	}
	s := content[start:end]
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
