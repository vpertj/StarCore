package agent

import (
	"strings"
)

type RoutingSuggestion struct {
	PrimaryTool   string
	FallbackTools []string
	Hint          string
}

type routingRule struct {
	intentKeywords []string
	primaryTool    string
	fallbackTools  []string
	hintTemplate   string
}

type ToolRouter struct {
	rules []routingRule
}

func NewToolRouter() *ToolRouter {
	return &ToolRouter{
		rules: buildDefaultRoutingRules(),
	}
}

func buildDefaultRoutingRules() []routingRule {
	return []routingRule{
		{
			intentKeywords: []string{"修改", "添加", "删除", "编辑", "change", "edit", "add", "remove", "update"},
			primaryTool:    "edit_file",
			fallbackTools:  []string{"write_file"},
			hintTemplate:   "用户要求修改代码。请使用 edit_file 工具进行精确修改，或 write_file 重写整个文件。",
		},
		{
			intentKeywords: []string{"运行", "执行", "测试", "构建", "run", "execute", "test", "build"},
			primaryTool:    "execute_command",
			fallbackTools:  nil,
			hintTemplate:   "用户要求执行命令。请使用 execute_command 工具。",
		},
		{
			intentKeywords: []string{"搜索", "查找", "定位", "search", "find", "locate", "where"},
			primaryTool:    "search_files",
			fallbackTools:  []string{"glob_files"},
			hintTemplate:   "用户要求搜索内容。请使用 search_files 搜索文件内容，或 glob_files 按文件名搜索。",
		},
		{
			intentKeywords: []string{"读取", "查看", "打开", "read", "view", "open", "show"},
			primaryTool:    "read_file",
			fallbackTools:  nil,
			hintTemplate:   "用户要求查看文件。请使用 read_file 工具。",
		},
		{
			intentKeywords: []string{"提交", "commit", "push", "pull", "git"},
			primaryTool:    "git_commit",
			fallbackTools:  []string{"get_git_diff", "execute_command"},
			hintTemplate:   "用户要求 Git 操作。请使用 git_commit/git_pull/git_push 工具。",
		},
		{
			intentKeywords: []string{"解释", "分析", "理解", "explain", "analyze", "understand", "what"},
			primaryTool:    "read_file",
			fallbackTools:  []string{"search_files"},
			hintTemplate:   "用户要求解释代码。请先使用 read_file 读取相关文件，然后分析解释。",
		},
		{
			intentKeywords: []string{"修复", "bug", "错误", "报错", "fix", "debug", "error"},
			primaryTool:    "read_file",
			fallbackTools:  []string{"search_files", "get_diagnostics"},
			hintTemplate:   "用户要求修复 bug。请先读取相关文件和错误信息，分析根因，然后使用 edit_file 修复。",
		},
	}
}

func (r *ToolRouter) SuggestTools(userMessage string) *RoutingSuggestion {
	if userMessage == "" {
		return nil
	}
	msg := strings.ToLower(userMessage)

	for _, rule := range r.rules {
		for _, kw := range rule.intentKeywords {
			if strings.Contains(msg, strings.ToLower(kw)) {
				return &RoutingSuggestion{
					PrimaryTool:   rule.primaryTool,
					FallbackTools: rule.fallbackTools,
					Hint:          rule.hintTemplate,
				}
			}
		}
	}
	return nil
}

func BuildToolMappingHint() string {
	return `
## 任务→工具映射
- 修改代码 → edit_file（精确修改）或 write_file（重写文件）
- 运行命令 → execute_command
- 搜索内容 → search_files（内容搜索）或 glob_files（文件名搜索）
- 查看文件 → read_file
- 修复 bug → read_file → 分析 → edit_file
- Git 操作 → get_git_diff / git_commit / git_pull / git_push
`
}
