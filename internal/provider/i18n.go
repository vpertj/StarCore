package provider

import "sync"

var errorMessages = map[string]map[string]string{
	"zh": {
		"auth_failed":       "API密钥无效或未配置，请前往设置配置",
		"rate_limit":        "请求频率超限，请稍后重试",
		"context_too_long":  "对话过长，超出上下文窗口限制，请开始新对话",
		"server_error":      "AI服务暂时不可用，请稍后重试",
		"network_error":     "网络连接失败，请检查网络设置",
		"no_provider":       "未配置AI提供商，请先在设置中配置API密钥",
		"no_endpoint":       "API端点未配置，请前往设置配置端点地址",
		"no_response":       "AI服务未返回任何响应，请检查网络连接或API配置",
		"no_content":        "AI未返回任何内容，请检查网络连接或尝试换个问题",
		"circuit_open":      "AI服务断路器已打开，请稍后重试",
		"concurrency_limit": "请求过于频繁，请等待当前请求完成后再试",
	},
	"en": {
		"auth_failed":       "API key invalid or not configured, please check settings",
		"rate_limit":        "Rate limit exceeded, please retry later",
		"context_too_long":  "Conversation too long, exceeds context window limit",
		"server_error":      "AI service temporarily unavailable, please retry later",
		"network_error":     "Network connection failed, please check network settings",
		"no_provider":       "No AI provider configured, please set up API key first",
		"no_endpoint":       "API endpoint not configured, please check settings",
		"no_response":       "AI service returned no response, check network or API config",
		"no_content":        "AI returned no content, check network or try a different query",
		"circuit_open":      "AI service circuit breaker is open, please retry later",
		"concurrency_limit": "Too many concurrent requests, please wait for current request to finish",
	},
}

var (
	currentLang = "zh"
	langMu      sync.RWMutex
)

func SetErrorLanguage(lang string) {
	langMu.Lock()
	defer langMu.Unlock()
	if lang == "en" {
		currentLang = "en"
	} else {
		currentLang = "zh"
	}
}

func GetErrorLanguage() string {
	langMu.RLock()
	defer langMu.RUnlock()
	return currentLang
}

func T(key string) string {
	langMu.RLock()
	lang := currentLang
	langMu.RUnlock()
	if msgs, ok := errorMessages[lang]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	if msgs, ok := errorMessages["zh"]; ok {
		if msg, ok := msgs[key]; ok {
			return msg
		}
	}
	return key
}
