package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"StarCore/internal/agent"
)

type ListDirectoryTool struct{}

func NewListDirectoryTool() *ListDirectoryTool { return &ListDirectoryTool{} }

func (t *ListDirectoryTool) ID() string             { return "list_directory" }
func (t *ListDirectoryTool) Name() string           { return "List Directory" }
func (t *ListDirectoryTool) RequiresApproval() bool { return false }

func (t *ListDirectoryTool) Description() string {
	return "List files and directories in a given path."
}

func (t *ListDirectoryTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path": {Type: "string", Description: "Directory path to list"},
		},
		Required: []string{"path"},
	}
}

func (t *ListDirectoryTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	path, ok := args["path"].(string)
	if !ok || path == "" {
		path = "."
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var result string
	var keyFiles []string
	for _, f := range files {
		prefix := "  "
		if f.IsDir() {
			prefix = "D "
		}
		name := f.Name()
		result += fmt.Sprintf("%s %s\n", prefix, filepath.Join(path, name))
		// Highlight key project files
		if !f.IsDir() && (name == "main.go" || name == "app.go" || name == "go.mod" || name == "package.json" || name == "README.md" || name == "spec.md" || name == "index.html") {
			keyFiles = append(keyFiles, name)
		}
	}
	if len(keyFiles) > 0 {
		result += fmt.Sprintf("\nKey files to examine: %s\nUse read_file with one of these paths.", strings.Join(keyFiles, ", "))
	}

	return result, nil
}
