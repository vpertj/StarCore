package tools

import (
	"context"
	"fmt"
	"strings"

	"StarCore/internal/agent"
	"StarCore/internal/lsp"
)

// LSPManager is set by app.go to give LSP tools access to the language server.
var LSPManager *lsp.Manager

type LSPDefinitionTool struct{}

func NewLSPDefinitionTool() *LSPDefinitionTool { return &LSPDefinitionTool{} }

func (t *LSPDefinitionTool) ID() string             { return "lsp_definition" }
func (t *LSPDefinitionTool) Name() string           { return "LSP Definition" }
func (t *LSPDefinitionTool) RequiresApproval() bool { return false }

func (t *LSPDefinitionTool) Description() string {
	return "使用 LSP 跳转到符号定义。比 grep 更精确，支持所有语言。"
}

func (t *LSPDefinitionTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":   {Type: "string", Description: "文件路径"},
			"line":   {Type: "number", Description: "行号（从0开始）"},
			"column": {Type: "number", Description: "列号（从0开始）"},
		},
		Required: []string{"path", "line", "column"},
	}
}

func (t *LSPDefinitionTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if LSPManager == nil {
		return "", fmt.Errorf("LSP 未初始化")
	}

	path, _ := args["path"].(string)
	line, _ := args["line"].(float64)
	col, _ := args["column"].(float64)

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	locations, err := LSPManager.GetDefinition(path, int(line), int(col))
	if err != nil {
		return "", fmt.Errorf("LSP definition failed: %w", err)
	}

	if len(locations) == 0 {
		return "未找到定义", nil
	}

	var sb strings.Builder
	for _, loc := range locations {
		filePath := strings.TrimPrefix(loc.URI, "file:///")
		filePath = strings.TrimPrefix(filePath, "/")
		sb.WriteString(fmt.Sprintf("%s:%d:%d\n", filePath, loc.Range.Start.Line+1, loc.Range.Start.Character+1))
	}
	return sb.String(), nil
}
