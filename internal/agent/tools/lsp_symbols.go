package tools

import (
	"context"
	"fmt"
	"strings"

	"StarCore/internal/agent"
	"StarCore/internal/lsp"
)

type LSPSymbolsTool struct{}

func NewLSPSymbolsTool() *LSPSymbolsTool { return &LSPSymbolsTool{} }

func (t *LSPSymbolsTool) ID() string             { return "lsp_symbols" }
func (t *LSPSymbolsTool) Name() string           { return "LSP Symbols" }
func (t *LSPSymbolsTool) RequiresApproval() bool { return false }

func (t *LSPSymbolsTool) Description() string {
	return "使用 LSP 获取文件的大纲（函数、类、变量等）。比读取整个文件更高效。"
}

func (t *LSPSymbolsTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"path": {Type: "string", Description: "文件路径"},
		},
		Required: []string{"path"},
	}
}

func (t *LSPSymbolsTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if LSPManager == nil {
		return "", fmt.Errorf("LSP 未初始化")
	}

	path, _ := args["path"].(string)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	symbols, err := LSPManager.GetDocumentSymbols(path)
	if err != nil {
		return "", fmt.Errorf("LSP symbols failed: %w", err)
	}

	if len(symbols) == 0 {
		return "未找到符号", nil
	}

	var sb strings.Builder
	formatSymbols(symbols, &sb, 0)
	return sb.String(), nil
}

func formatSymbols(symbols []lsp.DocumentSymbol, sb *strings.Builder, depth int) {
	indent := strings.Repeat("  ", depth)
	for _, sym := range symbols {
		kind := symbolKindName(sym.Kind)
		line := sym.Range.Start.Line + 1
		sb.WriteString(fmt.Sprintf("%s%s %s (L%d)\n", indent, kind, sym.Name, line))
		if len(sym.Children) > 0 {
			formatSymbols(sym.Children, sb, depth+1)
		}
	}
}

func symbolKindName(kind int) string {
	switch kind {
	case 1:
		return "File"
	case 2:
		return "Module"
	case 3:
		return "Namespace"
	case 4:
		return "Package"
	case 5:
		return "Class"
	case 6:
		return "Method"
	case 7:
		return "Property"
	case 8:
		return "Field"
	case 9:
		return "Constructor"
	case 10:
		return "Enum"
	case 11:
		return "Interface"
	case 12:
		return "Function"
	case 13:
		return "Variable"
	case 14:
		return "Constant"
	case 15:
		return "String"
	case 16:
		return "Number"
	case 17:
		return "Boolean"
	case 18:
		return "Array"
	case 19:
		return "Object"
	case 20:
		return "Key"
	case 21:
		return "Null"
	case 22:
		return "EnumMember"
	case 23:
		return "Struct"
	case 24:
		return "Event"
	case 25:
		return "Operator"
	case 26:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}
