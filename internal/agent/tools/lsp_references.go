package tools

import (
	"context"
	"fmt"
	"strings"

	"StarCore/internal/agent"
)

type LSPReferencesTool struct{}

func NewLSPReferencesTool() *LSPReferencesTool { return &LSPReferencesTool{} }

func (t *LSPReferencesTool) ID() string             { return "lsp_references" }
func (t *LSPReferencesTool) Name() string           { return "LSP References" }
func (t *LSPReferencesTool) RequiresApproval() bool { return false }

func (t *LSPReferencesTool) Description() string {
	return "使用 LSP 查找符号的所有引用。比 grep 更精确，能找到所有使用位置。"
}

func (t *LSPReferencesTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path":                {Type: "string", Description: "文件路径"},
			"line":                {Type: "number", Description: "行号（从0开始）"},
			"column":              {Type: "number", Description: "列号（从0开始）"},
			"include_declaration": {Type: "boolean", Description: "是否包含声明位置（默认true）"},
		},
		Required: []string{"path", "line", "column"},
	}
}

func (t *LSPReferencesTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if LSPManager == nil {
		return "", fmt.Errorf("LSP 未初始化")
	}

	path, _ := args["path"].(string)
	line, _ := args["line"].(float64)
	col, _ := args["column"].(float64)
	includeDecl := true
	if v, ok := args["include_declaration"].(bool); ok {
		includeDecl = v
	}

	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	locations, err := LSPManager.GetReferences(path, int(line), int(col), includeDecl)
	if err != nil {
		return "", fmt.Errorf("LSP references failed: %w", err)
	}

	if len(locations) == 0 {
		return "未找到引用", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 个引用:\n", len(locations)))
	for i, loc := range locations {
		if i >= 50 {
			sb.WriteString(fmt.Sprintf("... 还有 %d 个引用\n", len(locations)-50))
			break
		}
		sb.WriteString(fmt.Sprintf("%s:%d:%d\n", loc.FilePath, loc.Line+1, loc.Col+1))
	}
	return sb.String(), nil
}
