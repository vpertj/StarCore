package ai

import (
	"strconv"
	"strings"

	"StarCore/internal/provider"
)

func calcToolResultBudget(contextUsed int, contextMax int) int {
	remaining := contextMax - contextUsed
	if remaining < 0 {
		remaining = 0
	}
	budget := remaining * 30 / 100
	if budget < 2000 {
		return 2000
	}
	if budget > 12000 {
		return 12000
	}
	return budget
}

func smartTruncateToolResult(toolName string, result string, budget int) string {
	if len(result) <= budget {
		return result
	}
	switch toolName {
	case "execute_command":
		return truncateCommandOutput(result, budget)
	case "read_file":
		return truncateHeadTail(result, budget, 75, 25)
	case "search_files", "glob_files":
		return truncateSearchResults(result, budget)
	case "get_git_diff":
		return truncateGitDiff(result, budget)
	case "web_fetch", "http_request":
		return truncateHeadTail(result, budget, 75, 25)
	default:
		return truncateHeadTail(result, budget, 75, 25)
	}
}

func truncateCommandOutput(output string, budget int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= 20 {
		return output
	}

	var errorLines []string
	errorKeywords := []string{"error", "fail", "panic", "fatal", "exception"}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, kw := range errorKeywords {
			if strings.Contains(lower, kw) {
				errorLines = append(errorLines, line)
				break
			}
		}
	}

	tailStart := len(lines) - 30
	if tailStart < 0 {
		tailStart = 0
	}
	tailLines := lines[tailStart:]

	var result strings.Builder
	result.WriteString("[Command output truncated]\n")
	if len(errorLines) > 0 {
		result.WriteString("Error lines:\n")
		for _, el := range errorLines {
			result.WriteString(el + "\n")
		}
		result.WriteString("\n")
	}
	result.WriteString("Last 30 lines:\n")
	result.WriteString(strings.Join(tailLines, "\n"))

	final := result.String()
	if len(final) > budget {
		return final[:budget] + "\n... [truncated]"
	}
	return final
}

func truncateSearchResults(content string, budget int) string {
	lines := strings.Split(content, "\n")
	var statsLine string
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "match") || strings.Contains(lower, "result") {
			statsLine = line
			break
		}
	}

	var kept []string
	if statsLine != "" {
		kept = append(kept, statsLine)
	}
	count := 0
	for _, line := range lines {
		if line == statsLine {
			continue
		}
		kept = append(kept, line)
		count++
		if count >= 50 {
			break
		}
	}

	result := strings.Join(kept, "\n")
	if len(result) > budget {
		return result[:budget] + "\n... [truncated]"
	}
	return result
}

func truncateGitDiff(diff string, budget int) string {
	lines := strings.Split(diff, "\n")
	var statsLines []string
	var contentLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++") {
			statsLines = append(statsLines, line)
		} else {
			contentLines = append(contentLines, line)
		}
	}

	var result strings.Builder
	if len(statsLines) > 0 {
		result.WriteString("File changes:\n")
		limit := len(statsLines)
		if limit > 10 {
			limit = 10
		}
		result.WriteString(strings.Join(statsLines[:limit], "\n"))
		result.WriteString("\n\n")
	}

	remaining := budget - result.Len()
	if remaining > 0 && len(contentLines) > 0 {
		content := strings.Join(contentLines, "\n")
		if len(content) > remaining {
			content = content[:remaining] + "\n... [truncated]"
		}
		result.WriteString(content)
	}
	return result.String()
}

func truncateHeadTail(content string, budget int, headPct, tailPct int) string {
	if len(content) <= budget {
		return content
	}
	headSize := budget * headPct / 100
	tailSize := budget * tailPct / 100

	head := content[:headSize]
	if idx := strings.LastIndex(head, "\n"); idx > 0 {
		head = head[:idx]
	}

	tail := content[len(content)-tailSize:]
	if idx := strings.Index(tail, "\n"); idx > 0 {
		tail = tail[idx+1:]
	}

	omitted := len(content) - len(head) - len(tail)
	return head + "\n\n... [omitted " + strconv.Itoa(omitted) + " chars] ...\n\n" + tail
}

func estimateContextUsed(messages []provider.Message) int {
	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
	}
	return total
}
