package agent

import (
	"strings"
)

type ErrorSeverity int

const (
	ErrorRetryable ErrorSeverity = iota
	ErrorNeedsLLM
	ErrorFatal
)

type ClassifiedError struct {
	Severity   ErrorSeverity
	Category   string
	Message    string
	Suggestion string
	AutoRetry  bool
}

func ClassifyToolError(toolName string, err error) *ClassifiedError {
	if err == nil {
		return &ClassifiedError{Severity: ErrorNeedsLLM, Category: "none"}
	}
	msg := strings.ToLower(err.Error())

	if strings.Contains(msg, "file not found") || strings.Contains(msg, "no such file") ||
		strings.Contains(msg, "cannot find") || strings.Contains(msg, "does not exist") {
		return &ClassifiedError{
			Severity:   ErrorRetryable,
			Category:   "file_not_found",
			Message:    err.Error(),
			Suggestion: "文件未找到。请检查文件路径是否正确，或使用 glob_files/search_files 搜索文件。",
			AutoRetry:  false,
		}
	}

	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded") {
		return &ClassifiedError{
			Severity:   ErrorRetryable,
			Category:   "timeout",
			Message:    err.Error(),
			Suggestion: "操作超时。考虑拆分操作或使用更简短的命令。",
			AutoRetry:  true,
		}
	}

	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "access denied") {
		return &ClassifiedError{
			Severity:   ErrorNeedsLLM,
			Category:   "permission",
			Message:    err.Error(),
			Suggestion: "权限不足。检查文件权限或使用其他路径。",
			AutoRetry:  false,
		}
	}

	if strings.Contains(msg, "syntax error") || strings.Contains(msg, "parse error") ||
		strings.Contains(msg, "compile error") || strings.Contains(msg, "invalid syntax") {
		return &ClassifiedError{
			Severity:   ErrorNeedsLLM,
			Category:   "syntax",
			Message:    err.Error(),
			Suggestion: "代码有语法错误。请使用 get_diagnostics 获取详细错误信息。",
			AutoRetry:  false,
		}
	}

	if strings.Contains(msg, "path traversal") || strings.Contains(msg, "sandbox") ||
		strings.Contains(msg, "security") || strings.Contains(msg, "not allowed") {
		return &ClassifiedError{
			Severity:   ErrorFatal,
			Category:   "security",
			Message:    err.Error(),
			Suggestion: "此操作被安全策略阻止。请在项目目录内操作。",
			AutoRetry:  false,
		}
	}

	return &ClassifiedError{
		Severity:   ErrorNeedsLLM,
		Category:   "unknown",
		Message:    err.Error(),
		Suggestion: "操作失败，请分析错误信息并尝试其他方法。",
		AutoRetry:  false,
	}
}

func FormatClassifiedError(classified *ClassifiedError) string {
	var sb strings.Builder
	sb.WriteString("[工具执行失败]\n")
	sb.WriteString("错误: " + classified.Message + "\n")
	sb.WriteString("建议: " + classified.Suggestion + "\n")
	if classified.Severity == ErrorFatal {
		sb.WriteString("注意: 此错误不可重试，请换一种方法。\n")
	}
	return sb.String()
}

func ExtractFilePathFromToolArgs(args map[string]any) string {
	if path, ok := args["path"].(string); ok && path != "" {
		return path
	}
	if source, ok := args["source"].(string); ok && source != "" {
		return source
	}
	return ""
}
