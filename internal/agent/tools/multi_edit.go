package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"StarCore/internal/agent"
)

// MultiEditTool edits multiple files atomically.
type MultiEditTool struct{}

func NewMultiEditTool() *MultiEditTool { return &MultiEditTool{} }

func (t *MultiEditTool) ID() string             { return "multi_edit" }
func (t *MultiEditTool) Name() string           { return "Multi Edit" }
func (t *MultiEditTool) RequiresApproval() bool { return true }

func (t *MultiEditTool) Description() string {
	return "Edit multiple files atomically. All edits are applied together or none are applied. " +
		"Use this when changes to multiple files are dependent on each other."
}

func (t *MultiEditTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"edits": {Type: "array", Description: "Array of file edits, each with path, old_string, and new_string"},
		},
		Required: []string{"edits"},
	}
}

func (t *MultiEditTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx

	editsRaw, ok := args["edits"].([]any)
	if !ok || len(editsRaw) == 0 {
		return "", fmt.Errorf("edits array is required")
	}

	// Phase 1: Read all files and validate edits
	type fileEdit struct {
		path     string
		oldStr   string
		newStr   string
		content  string
	}
	var edits []fileEdit

	for i, editRaw := range editsRaw {
		editMap, ok := editRaw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("edit %d: invalid format", i)
		}

		path, _ := editMap["path"].(string)
		oldStr, _ := editMap["old_string"].(string)
		newStr, _ := editMap["new_string"].(string)

		if path == "" || oldStr == "" {
			return "", fmt.Errorf("edit %d: path and old_string are required", i)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("edit %d: failed to read %s: %w", i, path, err)
		}

		content := string(data)
		if !strings.Contains(content, oldStr) {
			return "", fmt.Errorf("edit %d: old_string not found in %s", i, path)
		}

		// Check for uniqueness (multiple occurrences)
		count := strings.Count(content, oldStr)
		if count > 1 {
			return "", fmt.Errorf("edit %d: old_string found %d times in %s (must be unique)", i, count, path)
		}

		edits = append(edits, fileEdit{
			path:    path,
			oldStr:  oldStr,
			newStr:  newStr,
			content: content,
		})
	}

	// Phase 2: Apply all edits
	var applied []string
	for i, edit := range edits {
		newContent := strings.Replace(edit.content, edit.oldStr, edit.newStr, 1)
		if err := os.WriteFile(edit.path, []byte(newContent), 0644); err != nil {
			// Rollback previously applied edits
			for j := i - 1; j >= 0; j-- {
				origContent := strings.Replace(edits[j].content, edits[j].newStr, edits[j].oldStr, 1)
				os.WriteFile(edits[j].path, []byte(origContent), 0644)
			}
			return "", fmt.Errorf("edit %d: failed to write %s: %w (rolled back %d edits)", i, edit.path, err, i)
		}
		applied = append(applied, edit.path)
	}

	return fmt.Sprintf("Successfully edited %d files: %s", len(applied), strings.Join(applied, ", ")), nil
}
