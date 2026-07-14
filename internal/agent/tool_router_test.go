package agent

import (
	"testing"
)

func TestToolRouter_KeywordMatch(t *testing.T) {
	router := NewToolRouter()

	tests := []struct {
		name        string
		message     string
		primaryTool string
	}{
		{"edit Chinese", "修改 main.go 中的函数", "edit_file"},
		{"edit English", "edit the function", "edit_file"},
		{"execute", "运行测试", "execute_command"},
		{"search", "搜索所有使用 printf 的地方", "search_files"},
		{"read", "读取配置文件", "read_file"},
		{"git", "提交代码", "git_commit"},
		{"debug", "修复这个 bug", "get_diagnostics"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := router.SuggestTools(tt.message)
			if result == nil {
				t.Fatal("expected suggestion, got nil")
			}
			if result.PrimaryTool != tt.primaryTool {
				t.Errorf("expected %s, got %s", tt.primaryTool, result.PrimaryTool)
			}
		})
	}
}

func TestToolRouter_NoMatch(t *testing.T) {
	router := NewToolRouter()
	result := router.SuggestTools("今天天气怎么样")
	if result != nil {
		t.Errorf("expected nil for unrelated message, got %+v", result)
	}
}

func TestToolRouter_EmptyMessage(t *testing.T) {
	router := NewToolRouter()
	result := router.SuggestTools("")
	if result != nil {
		t.Errorf("expected nil for empty message, got %+v", result)
	}
}

func TestBuildToolMappingHint(t *testing.T) {
	hint := BuildToolMappingHint()
	if hint == "" {
		t.Error("expected non-empty hint")
	}
}
