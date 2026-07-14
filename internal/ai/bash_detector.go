package ai

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"StarCore/internal/provider"
)

// bashPattern defines a regex pattern and its conversion to a tool call.
type bashPattern struct {
	re      *regexp.Regexp
	convert func(matches []string) *provider.ToolCall
}

// Bash detector patterns — converts common bash commands to tool calls.
var bashPatterns []bashPattern

func init() {
	bashPatterns = []bashPattern{
		// find . -name "*.go" → glob_files
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?find\s+\.\s+-name\s+["']([^"']+)["']`),
			convert: func(m []string) *provider.ToolCall {
				pattern := m[1]
				// Convert shell glob to doublestar: *.go → **/*.go
				if !strings.Contains(pattern, "/") {
					pattern = "**/" + pattern
				}
				return toolCall("glob_files", map[string]any{"pattern": pattern})
			},
		},
		// find . -name "*.go" | head -N → glob_files (limited)
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?find\s+\.\s+-name\s+["']([^"']+)["']\s*\|\s*head\s+-?\d*`),
			convert: func(m []string) *provider.ToolCall {
				pattern := m[1]
				if !strings.Contains(pattern, "/") {
					pattern = "**/" + pattern
				}
				return toolCall("glob_files", map[string]any{"pattern": pattern})
			},
		},
		// grep -rn "pattern" . --include="*.go" → search_files
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?grep\s+-rn?\s+["']([^"']+)["']\s+\S*(?:\s+--include=(?:["']([^"']+)["']|\S+))?`),
			convert: func(m []string) *provider.ToolCall {
				query := m[1]
				include := ""
				if len(m) > 2 && m[2] != "" {
					include = m[2]
				}
				args := map[string]any{"query": query}
				if include != "" {
					args["include_pattern"] = include
				}
				return toolCall("search_files", args)
			},
		},
		// grep -rn "pattern" internal/ --include="*.go" → search_files with path
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?grep\s+-rn?\s+["']([^"']+)["']\s+(\S+)\s+(?:--include=(?:["']([^"']+)["']|\S+))?`),
			convert: func(m []string) *provider.ToolCall {
				query := m[1]
				path := m[2]
				include := ""
				if len(m) > 3 && m[3] != "" {
					include = m[3]
				}
				args := map[string]any{"query": query, "path": path}
				if include != "" {
					args["include_pattern"] = include
				}
				return toolCall("search_files", args)
			},
		},
		// cat file.go → read_file
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?cat\s+([^\s|>]+)`),
			convert: func(m []string) *provider.ToolCall {
				path := strings.Trim(m[1], "\"'")
				return toolCall("read_file", map[string]any{"path": path})
			},
		},
		// cat file.go | head -100 → read_file (with implicit limit)
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?cat\s+([^\s|]+)\s*\|\s*head\s+-?\d+`),
			convert: func(m []string) *provider.ToolCall {
				path := strings.Trim(m[1], "\"'")
				return toolCall("read_file", map[string]any{"path": path})
			},
		},
		// ls -la dir/ → list_directory
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?ls\s+(?:-[a-zA-Z]+\s+)?([^\s|>]+)`),
			convert: func(m []string) *provider.ToolCall {
				path := strings.Trim(m[1], "\"'")
				if path == "" || path == "." {
					path = "."
				}
				return toolCall("list_directory", map[string]any{"path": path})
			},
		},
		// ls → list_directory (current dir)
		{
			re: regexp.MustCompile(`(?i)^\s*ls\s*$`),
			convert: func(m []string) *provider.ToolCall {
				return toolCall("list_directory", map[string]any{"path": "."})
			},
		},
		// dir /s /b path → glob_files (Windows)
		{
			re: regexp.MustCompile(`(?i)^\s*dir\s+/s\s+/b\s+(.+)`),
			convert: func(m []string) *provider.ToolCall {
				pattern := strings.TrimSpace(m[1])
				// Convert to glob pattern: if it has a wildcard, treat as glob
				if strings.ContainsAny(pattern, "*?") {
					return toolCall("glob_files", map[string]any{"pattern": pattern})
				}
				return toolCall("list_directory", map[string]any{"path": pattern})
			},
		},
		// dir /b path → list_directory or glob_files (Windows)
		{
			re: regexp.MustCompile(`(?i)^\s*dir\s+/b\s+(.+)`),
			convert: func(m []string) *provider.ToolCall {
				pattern := strings.TrimSpace(m[1])
				if strings.ContainsAny(pattern, "*?") {
					return toolCall("glob_files", map[string]any{"pattern": pattern})
				}
				return toolCall("list_directory", map[string]any{"path": pattern})
			},
		},
		// dir path → list_directory (Windows)
		{
			re: regexp.MustCompile(`(?i)^\s*dir\s+([^\s|]+)`),
			convert: func(m []string) *provider.ToolCall {
				path := strings.Trim(m[1], "\"'")
				return toolCall("list_directory", map[string]any{"path": path})
			},
		},
		// dir → list_directory (current dir, Windows)
		{
			re: regexp.MustCompile(`(?i)^\s*dir\s*$`),
			convert: func(m []string) *provider.ToolCall {
				return toolCall("list_directory", map[string]any{"path": "."})
			},
		},
		// type file.go → read_file (Windows)
		{
			re: regexp.MustCompile(`(?i)^\s*(?:cd\s+\S+\s*&&\s*)?type\s+([^\s|>]+)`),
			convert: func(m []string) *provider.ToolCall {
				path := strings.Trim(m[1], "\"'")
				return toolCall("read_file", map[string]any{"path": path})
			},
		},
		// cd dir && command → execute_command with cwd
		{
			re: regexp.MustCompile(`(?i)^cd\s+(\S+)\s*&&\s*(.+)$`),
			convert: func(m []string) *provider.ToolCall {
				cwd := strings.Trim(m[1], "\"'")
				cmd := strings.TrimSpace(m[2])
				return toolCall("execute_command", map[string]any{"command": cmd, "cwd": cwd})
			},
		},
	}
}

// parseBashCommands detects common bash commands in text and converts them to tool calls.
// This handles models that don't support function calling and output raw bash.
func parseBashCommands(content string) []provider.ToolCall {
	var calls []provider.ToolCall

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Skip lines that are clearly not commands (markdown, explanations, etc.)
		if isLikelyNotCommand(trimmed) {
			continue
		}

		for _, pattern := range bashPatterns {
			if matches := pattern.re.FindStringSubmatch(trimmed); matches != nil {
				if tc := pattern.convert(matches); tc != nil {
					calls = append(calls, *tc)
					break // one match per line
				}
			}
		}
	}

	return calls
}

// isLikelyNotCommand returns true if the line is probably natural language, not a command.
func isLikelyNotCommand(line string) bool {
	// Markdown headers
	if strings.HasPrefix(line, "#") {
		return true
	}
	// Bullet points
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "• ") {
		return true
	}
	// Numbered lists
	if len(line) > 2 && line[0] >= '0' && line[0] <= '9' && line[1] == '.' {
		return true
	}
	// Common natural language patterns
	lower := strings.ToLower(line)
	naturalPatterns := []string{
		"我来", "让我", "首先", "然后", "接下来", "最后",
		"let me", "first", "then", "next", "finally",
		"好的", "ok", "yes", "no", "是的", "不是",
		"the ", "this ", "that ", "it ", "we ",
		"根据", "分析", "发现", "建议", "应该",
		"based on", "analysis", "suggest", "should",
	}
	for _, p := range naturalPatterns {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	// Lines starting with Chinese characters (likely prose)
	if len(line) > 0 {
		r := rune(line[0])
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

// toolCall creates a ToolCall with the given name and args.
func toolCall(name string, args map[string]any) *provider.ToolCall {
	argsJSON := "{}"
	if args != nil {
		if b, err := json.Marshal(args); err == nil {
			argsJSON = string(b)
		}
	}
	return &provider.ToolCall{
		ID:   fmt.Sprintf("tc_%d", len(argsJSON)), // placeholder ID, will be overwritten
		Type: "function",
		Function: provider.ToolCallFunc{
			Name:      name,
			Arguments: argsJSON,
		},
	}
}

// parseDSLMToolCalls parses DSLM format tool calls from model output.
// Format: <| DSLM | invoke name="tool_name"> <| DSLM | parameter name="param" string="true">value<| DSLM | parameter />
func parseDSLMToolCalls(content string) []provider.ToolCall {
	var calls []provider.ToolCall

	// Check if content contains DSLM markers
	if !strings.Contains(content, "DSLM") {
		return calls
	}

	// Extract all invoke blocks
	invokeRe := regexp.MustCompile(`<\|\s*DSLM\s*\|\s*invoke\s+name="([^"]+)"[^>]*>`)
	paramRe := regexp.MustCompile(`<\|\s*DSLM\s*\|\s*parameter\s+name="([^"]+)"(?:\s+string="true")?[^>]*>([^<]*)<\|\s*DSLM\s*\|\s*parameter\s*/>`)

	// Find all invoke positions
	invokeMatches := invokeRe.FindAllStringSubmatchIndex(content, -1)
	if len(invokeMatches) == 0 {
		return calls
	}

	for i, invokeMatch := range invokeMatches {
		if len(invokeMatch) < 4 {
			continue
		}
		toolName := content[invokeMatch[2]:invokeMatch[3]]

		// Find the end of this invoke block (next invoke or end of content)
		invokeStart := invokeMatch[1]
		invokeEnd := len(content)
		if i+1 < len(invokeMatches) {
			invokeEnd = invokeMatches[i+1][0]
		}

		// Extract parameters from this invoke block
		paramContent := content[invokeStart:invokeEnd]
		args := make(map[string]any)
		paramMatches := paramRe.FindAllStringSubmatch(paramContent, -1)
		for _, pm := range paramMatches {
			if len(pm) >= 4 {
				paramName := pm[1]
				paramValue := pm[2]
				args[paramName] = paramValue
			}
		}

		if tc := toolCall(toolName, args); tc != nil {
			calls = append(calls, *tc)
		}
	}

	return calls
}
