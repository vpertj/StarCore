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
	return "Make a precise edit to a file by replacing an exact string match. " +
		"Prefer this over write_file for targeted changes. " +
		"The old_string must uniquely match the text to replace (including whitespace). " +
		"If the string is not unique, the edit fails — provide more surrounding context to make it unique."
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
	oldStr, _ := args["old_string"].(string)
	newStr, _ := args["new_string"].(string)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if oldStr == "" && newStr == "" {
		return "", fmt.Errorf("old_string or new_string is required")
	}

	if SandboxConfig != nil {
		if err := SandboxConfig.ValidatePath(path); err != nil {
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

	if oldStr == newStr {
		return fmt.Sprintf("⏭️ %s unchanged", path), nil
	}
	if oldLines == 0 && newLines == 0 {
		return fmt.Sprintf("✏️ %s: replaced \"%s\" → \"%s\"", path, oldPreview, newPreview), nil
	}
	return fmt.Sprintf("✏️ %s: replaced %d→%d lines", path, oldLines+1, newLines+1), nil
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
