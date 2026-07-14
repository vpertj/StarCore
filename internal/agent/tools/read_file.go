package tools

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"StarCore/internal/agent"
)

type ReadFileTool struct{}

func NewReadFileTool() *ReadFileTool { return &ReadFileTool{} }

func (t *ReadFileTool) ID() string             { return "read_file" }
func (t *ReadFileTool) Name() string           { return "Read File" }
func (t *ReadFileTool) RequiresApproval() bool { return false }

func (t *ReadFileTool) Description() string {
	return "读取文件内容。可指定 start_line/end_line 读取部分。修改文件前先读取了解当前代码。"
}

func (t *ReadFileTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":       {Type: "string", Description: "File path to read"},
			"start_line": {Type: "integer", Description: "Start line number (1-indexed, optional)"},
			"end_line":   {Type: "integer", Description: "End line number (optional)"},
		},
		Required: []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	_ = ctx
	path, ok := args["path"].(string)
	if !ok || path == "" {
		// Try common project files first
		tryFiles := []string{"main.go", "app.go", "go.mod", "package.json", "README.md", "index.html", "Cargo.toml", "pyproject.toml", "composer.json", "Gemfile", "Makefile", "main.py", "main.rs", "server.js", "Dockerfile"}
		var foundFiles []string
		for _, f := range tryFiles {
			if _, err := os.ReadFile(f); err == nil {
				foundFiles = append(foundFiles, f)
			}
		}
		if len(foundFiles) > 0 {
			// Read the first found file
			data, _ := os.ReadFile(foundFiles[0])
			result := fmt.Sprintf("Auto-detected %s:\n\n```\n%s\n```\n\nOther files: %s", foundFiles[0], string(data), strings.Join(foundFiles[1:], ", "))
			return result, nil
		}
		// List all files
		files, _ := os.ReadDir(".")
		var list string
		for _, f := range files {
			if f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
				list += fmt.Sprintf("  [dir]  %s/\n", f.Name())
			} else if !f.IsDir() && !strings.HasPrefix(f.Name(), ".") {
				list += fmt.Sprintf("  [file] %s\n", f.Name())
			}
		}
		return "", fmt.Errorf("No file path specified. Files in project:\n%s\nPlease specify one.", list)
	}

	path = strings.TrimSpace(path)

	if cfg := GetSandboxConfig(); cfg != nil {
		if err := cfg.ValidatePath(path); err != nil {
			return "", fmt.Errorf("path validation failed: %w", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")

	startLine := 1
	endLine := len(lines)

	if v, ok := args["start_line"]; ok {
		if n, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil && n > 0 {
			startLine = n
		}
	}
	if v, ok := args["end_line"]; ok {
		if n, err := strconv.Atoi(fmt.Sprintf("%v", v)); err == nil && n > 0 {
			endLine = n
		}
	}

	if startLine < 1 {
		startLine = 1
	}
	if endLine > len(lines) {
		endLine = len(lines)
	}
	if startLine > endLine {
		startLine = endLine
	}

	return strings.Join(lines[startLine-1:endLine], "\n"), nil
}
