package ai

import (
	"testing"
)

func TestParseBashCommands(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int // number of tool calls expected
		toolName string
	}{
		{
			name:     "find command",
			input:    `find . -name "*.go"`,
			expected: 1,
			toolName: "glob_files",
		},
		{
			name:     "find with cd prefix",
			input:    `cd F:\code\goby && find . -name "*_test.go" | head -20`,
			expected: 1,
			toolName: "glob_files",
		},
		{
			name:     "grep command",
			input:    `grep -rn "ioutil.ReadDir" .`,
			expected: 1,
			toolName: "search_files",
		},
		{
			name:     "grep with path and include",
			input:    `grep -rn "const\s|var\s" internal/agent/agent_engine.go | head -10`,
			expected: 1,
			toolName: "search_files",
		},
		{
			name:     "grep with cd prefix",
			input:    `cd F:\code\goby && grep -rn "ioutil\.ReadDir" internal/ --include="*.go"`,
			expected: 1,
			toolName: "search_files",
		},
		{
			name:     "cat command",
			input:    `cat main.go`,
			expected: 1,
			toolName: "read_file",
		},
		{
			name:     "cat with cd prefix",
			input:    `cd F:\code\goby && cat configs/config.yaml 2>/dev/null | head -100`,
			expected: 1,
			toolName: "read_file",
		},
		{
			name:     "ls command",
			input:    `ls -la configs/`,
			expected: 1,
			toolName: "list_directory",
		},
		{
			name:     "cd && command",
			input:    `cd internal && grep -rn "Close()" .`,
			expected: 1,
			toolName: "search_files",
		},
		{
			name:     "natural language not detected",
			input:    `好的，我来审查一下代码。`,
			expected: 0,
		},
		{
			name:     "markdown not detected",
			input:    `## 分析结果`,
			expected: 0,
		},
		{
			name:     "mixed content",
			input:    "首先我来查看项目结构：\nfind . -name \"*.go\" | head -20\n然后搜索关键代码：\ngrep -rn \"func main\" .",
			expected: 2,
		},
		{
			name:     "dir command (Windows)",
			input:    `dir /s /b F:\code\goby\configs\skills\SKILL.md`,
			expected: 1,
			toolName: "list_directory",
		},
		{
			name:     "dir /b with pattern (Windows)",
			input:    `dir /b F:\code\goby\configs\skills\*.md`,
			expected: 1,
			toolName: "glob_files",
		},
		{
			name:     "dir bare (Windows)",
			input:    `dir`,
			expected: 1,
			toolName: "list_directory",
		},
		{
			name:     "dir path (Windows)",
			input:    `dir F:\code\goby\configs\skills`,
			expected: 1,
			toolName: "list_directory",
		},
		{
			name:     "type command (Windows)",
			input:    `type main.go`,
			expected: 1,
			toolName: "read_file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := parseBashCommands(tt.input)
			if len(calls) != tt.expected {
				t.Errorf("parseBashCommands(%q) returned %d calls, expected %d", tt.input, len(calls), tt.expected)
				for i, c := range calls {
					t.Logf("  call[%d]: %s %s", i, c.Function.Name, c.Function.Arguments)
				}
			}
			if tt.expected > 0 && len(calls) > 0 && tt.toolName != "" {
				if calls[0].Function.Name != tt.toolName {
					t.Errorf("expected tool %q, got %q", tt.toolName, calls[0].Function.Name)
				}
			}
		})
	}
}

func TestIsLikelyNotCommand(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"# 标题", true},
		{"- 列表项", true},
		{"1. 第一项", true},
		{"我来分析一下", true},
		{"首先查看文件", true},
		{"Let me check", true},
		{"First, read the file", true},
		{"find . -name \"*.go\"", false},
		{"grep -rn \"pattern\" .", false},
		{"cat main.go", false},
		{"ls -la", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isLikelyNotCommand(tt.input)
			if result != tt.expected {
				t.Errorf("isLikelyNotCommand(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDSLMToolCalls(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		toolName string
	}{
		{
			name:     "single invoke",
			input:    `<| DSLM | tool_calls> <| DSLM | invoke name="glob_files"> <| DSLM | parameter name="pattern" string="true">*/.go<| DSLM | parameter /> <| DSLM | invoke /> <| DSLM | tool_calls>`,
			expected: 1,
			toolName: "glob_files",
		},
		{
			name:     "multiple invokes",
			input:    `<| DSLM | tool_calls> <| DSLM | invoke name="glob_files"> <| DSLM | parameter name="pattern" string="true">**/*.go<| DSLM | parameter /> <| DSLM | invoke /> <| DSLM | invoke name="read_file"> <| DSLM | parameter name="path" string="true">main.go<| DSLM | parameter /> <| DSLM | invoke /> <| DSLM | tool_calls>`,
			expected: 2,
			toolName: "glob_files",
		},
		{
			name:     "no DSLM",
			input:    "This is normal text without DSLM",
			expected: 0,
		},
		{
			name:     "real example from screenshot",
			input:    `<| DSLM | tool_calls> <| DSLM | invoke name="glob_files"> <| DSLM | parameter name="pattern" string="true">*/.go<| DSLM | parameter /> <| DSLM | invoke /> <| DSLM | invoke name="glob_files"> <| DSLM | parameter name="pattern" string="true">/config.yaml<| DSLM | parameter /> <| DSLM | invoke /> <| DSLM | tool_calls>`,
			expected: 2,
			toolName: "glob_files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := parseDSLMToolCalls(tt.input)
			if len(calls) != tt.expected {
				t.Errorf("parseDSLMToolCalls(%q) returned %d calls, expected %d", tt.input, len(calls), tt.expected)
				for i, c := range calls {
					t.Logf("  call[%d]: %s %s", i, c.Function.Name, c.Function.Arguments)
				}
			}
			if tt.expected > 0 && len(calls) > 0 && tt.toolName != "" {
				if calls[0].Function.Name != tt.toolName {
					t.Errorf("expected tool %q, got %q", tt.toolName, calls[0].Function.Name)
				}
			}
		})
	}
}
