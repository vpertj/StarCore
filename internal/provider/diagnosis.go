package provider

import (
	"fmt"
	"strings"
)

type ErrorDiagnosis struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Action      string `json:"action"`
	ActionLabel string `json:"actionLabel"`
	Retryable   bool   `json:"retryable"`
}

func DiagnoseError(err error) ErrorDiagnosis {
	if err == nil {
		return ErrorDiagnosis{Type: "none", Retryable: false}
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "401") || strings.Contains(lower, "403") ||
		strings.Contains(lower, "unauthorized") || strings.Contains(lower, "api key") {
		return ErrorDiagnosis{
			Type: "auth", Title: "API密钥无效",
			Message: "AI提供商认证失败，请检查API密钥配置。",
			Action: "settings", ActionLabel: "前往设置", Retryable: false,
		}
	}

	if strings.Contains(lower, "429") || strings.Contains(lower, "rate limit") {
		return ErrorDiagnosis{
			Type: "rate_limit", Title: "请求频率限制",
			Message: "AI服务请求过于频繁，请稍后重试。",
			Action: "retry", ActionLabel: "稍后重试", Retryable: true,
		}
	}

	if strings.Contains(lower, "context_length") || strings.Contains(lower, "token limit") {
		return ErrorDiagnosis{
			Type: "context", Title: "对话上下文超限",
			Message: "对话过长超出模型限制，建议开始新对话。",
			Action: "new_chat", ActionLabel: "开始新对话", Retryable: false,
		}
	}

	if strings.Contains(lower, "500") || strings.Contains(lower, "502") ||
		strings.Contains(lower, "503") || strings.Contains(lower, "504") ||
		strings.Contains(lower, "server error") {
		return ErrorDiagnosis{
			Type: "service", Title: "AI服务暂时不可用",
			Message: "AI提供商服务异常，请稍后重试或切换提供商。",
			Action: "retry", ActionLabel: "重试", Retryable: true,
		}
	}

	if strings.Contains(lower, "network") || strings.Contains(lower, "timeout") ||
		strings.Contains(lower, "connection refused") || strings.Contains(lower, "dns") ||
		strings.Contains(lower, "no such host") {
		return ErrorDiagnosis{
			Type: "network", Title: "网络连接失败",
			Message: "无法连接到AI服务，请检查网络设置。",
			Action: "retry", ActionLabel: "重试", Retryable: true,
		}
	}

	return ErrorDiagnosis{
		Type: "unknown", Title: "AI请求失败",
		Message: fmt.Sprintf("错误: %s", truncate(msg, 200)),
		Action: "retry", ActionLabel: "重试", Retryable: true,
	}
}

func ClassifyProviderError(err error) string {
	d := DiagnoseError(err)
	if d.Type == "none" {
		return ""
	}
	if d.Action != "none" && d.ActionLabel != "" {
		return fmt.Sprintf("%s\n\n💡 %s", d.Message, d.ActionLabel)
	}
	return d.Message
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
