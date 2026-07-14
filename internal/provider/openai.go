package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OpenAIProvider struct {
	config ProviderConfig
	id     string
	name   string
}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		id:   "openai",
		name: "OpenAI",
		config: ProviderConfig{
			ID:       "openai",
			Name:     "OpenAI",
			Endpoint: "https://api.openai.com/v1/chat/completions",
			Enabled:  true,
		},
	}
}

func NewGenericOpenAIProvider(id, name string) *OpenAIProvider {
	return &OpenAIProvider{
		id:   id,
		name: name,
		config: ProviderConfig{
			ID:   id,
			Name: name,
		},
	}
}

func (p *OpenAIProvider) ID() string                   { return p.id }
func (p *OpenAIProvider) Name() string                 { return p.name }
func (p *OpenAIProvider) SetConfig(cfg ProviderConfig) { p.config = cfg }
func (p *OpenAIProvider) GetConfig() ProviderConfig    { return p.config }

type openAIRequestBody struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Temperature float64          `json:"temperature,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Stream      bool             `json:"stream"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			ToolCalls        []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		Message struct {
			Content   string     `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message,omitempty"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails *struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// captureDeltaExtra parses the raw delta to find any provider-specific fields
// (e.g. reasoning_content) not captured by named struct fields, and returns them
// so they can be round-tripped on subsequent requests.
func captureDeltaExtra(raw json.RawMessage) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	known := map[string]bool{"content": true, "tool_calls": true, "role": true}
	var extra map[string]json.RawMessage
	for k, v := range m {
		if !known[k] {
			if extra == nil {
				extra = make(map[string]json.RawMessage)
			}
			extra[k] = v
		}
	}
	return extra
}

// resolveChatEndpoint ensures the endpoint includes the chat completions path.
// Users may configure just the base URL (e.g. "https://api.deepseek.com")
// without the /v1/chat/completions suffix.
func resolveChatEndpoint(endpoint string) string {
	if endpoint == "" {
		return "https://api.openai.com/v1/chat/completions"
	}
	// Already includes the path — use as-is
	if strings.Contains(endpoint, "/chat/completions") || strings.Contains(endpoint, "/messages") {
		return endpoint
	}
	// Trim trailing slashes
	endpoint = strings.TrimRight(endpoint, "/")
	// Anthropic-style endpoint
	if strings.Contains(endpoint, "anthropic.com") {
		return endpoint + "/v1/messages"
	}
	// OpenAI-compatible: append /v1/chat/completions
	// If endpoint already has /v1 or /api/v3 etc, just append /chat/completions
	if strings.HasSuffix(endpoint, "/v1") || strings.Contains(endpoint, "/api/v") {
		return endpoint + "/chat/completions"
	}
	return endpoint + "/v1/chat/completions"
}

func (p *OpenAIProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = EstimateContextWindow(req.Model)
		if maxTokens > 65536 {
			maxTokens = 16384
		}
	}

	if req.Tools != nil && len(req.Tools) == 0 {
		req.Tools = nil
	}

	body := openAIRequestBody{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
		Stream:      false,
		Tools:       req.Tools,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := resolveChatEndpoint(p.config.Endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	client := NewHTTPClient(120 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	msg := apiResp.Choices[0].Message

	var usage *TokenUsage
	if apiResp.Usage != nil {
		usage = &TokenUsage{
			PromptTokens:     apiResp.Usage.PromptTokens,
			CompletionTokens: apiResp.Usage.CompletionTokens,
			TotalTokens:      apiResp.Usage.TotalTokens,
		}
		if apiResp.Usage.PromptTokensDetails != nil {
			usage.CachedTokens = apiResp.Usage.PromptTokensDetails.CachedTokens
		}
	}

	return &ChatResponse{
		Content:  msg.Content,
		Provider: p.ID(),
		Model:    req.Model,
		Usage:    usage,
	}, nil
}

func (p *OpenAIProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 64)

	if p.config.APIKey == "" {
		close(ch)
		return ch, fmt.Errorf("OpenAI API key is required")
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = EstimateContextWindow(req.Model)
		if maxTokens > 65536 {
			maxTokens = 16384
		}
	}

	if req.Tools != nil && len(req.Tools) == 0 {
		req.Tools = nil
	}

	body := openAIRequestBody{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   maxTokens,
		Stream:      true,
		Tools:       req.Tools,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := resolveChatEndpoint(p.config.Endpoint)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	client := NewHTTPClient(300 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("request failed: %w", err)
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ch <- StreamEvent{Type: "error", Content: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))}
			return
		}

		// Read raw delta bytes as well so we can capture extra fields.
		type enrichedChunk struct {
			chunk openAIStreamChunk
			raw   json.RawMessage
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
		var pendingToolCalls []*ToolCall
		var chunkCount int
		firstChunk := true
		var lastUsage *TokenUsage

		for scanner.Scan() {
			line := scanner.Text()

			if line == "data: [DONE]" {
				if len(pendingToolCalls) > 0 {
					tcs := make([]ToolCall, 0, len(pendingToolCalls))
					for _, tc := range pendingToolCalls {
						if tc != nil && tc.Function.Name != "" {
							tcs = append(tcs, *tc)
						}
					}
					if len(tcs) > 0 {
						ch <- StreamEvent{Type: "tool_call", ToolCalls: tcs}
					}
				}
				ch <- StreamEvent{Type: "done"}
				return
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil || len(chunk.Choices) == 0 {
				continue
			}

			if firstChunk {
				firstChunk = false
			}
			chunkCount++

			if chunk.Error != nil {
				ch <- StreamEvent{Type: "error", Content: chunk.Error.Message}
				return
			}

			// Capture usage from chunk if available
			if chunk.Usage != nil {
				lastUsage = &TokenUsage{
					PromptTokens:     chunk.Usage.PromptTokens,
					CompletionTokens: chunk.Usage.CompletionTokens,
					TotalTokens:      chunk.Usage.TotalTokens,
				}
				if chunk.Usage.PromptTokensDetails != nil {
					lastUsage.CachedTokens = chunk.Usage.PromptTokensDetails.CachedTokens
				}
			}

			delta := chunk.Choices[0].Delta

			if delta.Content != "" {
				ch <- StreamEvent{Type: "data", Content: delta.Content}
			}
			if delta.ReasoningContent != "" {
				ch <- StreamEvent{Type: "thinking", Content: delta.ReasoningContent}
			}

			for _, tc := range delta.ToolCalls {
				idx := tc.Index
				for len(pendingToolCalls) <= idx {
					pendingToolCalls = append(pendingToolCalls, nil)
				}
				if tc.ID != "" {
					pendingToolCalls[idx] = &ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: ToolCallFunc{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				} else if pendingToolCalls[idx] != nil {
					if tc.Function.Name != "" {
						pendingToolCalls[idx].Function.Name = tc.Function.Name
					}
					pendingToolCalls[idx].Function.Arguments += tc.Function.Arguments
				}
			}

			if chunk.Choices[0].FinishReason != nil {
				reason := *chunk.Choices[0].FinishReason
				if reason == "tool_calls" && len(pendingToolCalls) > 0 {
					tcs := make([]ToolCall, 0, len(pendingToolCalls))
					for _, tc := range pendingToolCalls {
						if tc != nil && tc.Function.Name != "" {
							tcs = append(tcs, *tc)
						}
					}
					if len(tcs) > 0 {
						ch <- StreamEvent{Type: "tool_call", ToolCalls: tcs}
					}
					pendingToolCalls = make([]*ToolCall, 0)
				}
			}

			// Capture any provider-specific delta fields for round-tripping.
			// We re-parse the delta as raw JSON to find fields not in the struct.
			var rawChunk struct {
				Choices []struct {
					Delta json.RawMessage `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &rawChunk) == nil && len(rawChunk.Choices) > 0 {
				if extra := captureDeltaExtra(rawChunk.Choices[0].Delta); len(extra) > 0 {
					ch <- StreamEvent{Type: "extra", Extra: extra}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamEvent{Type: "error", Content: err.Error()}
			return
		}

		// Flush remaining tool calls
		if len(pendingToolCalls) > 0 {
			tcs := make([]ToolCall, 0, len(pendingToolCalls))
			for _, tc := range pendingToolCalls {
				if tc != nil && tc.Function.Name != "" {
					tcs = append(tcs, *tc)
				}
			}
			if len(tcs) > 0 {
				ch <- StreamEvent{Type: "tool_call", ToolCalls: tcs}
			}
		}

		ch <- StreamEvent{Type: "done", Usage: lastUsage}
	}()

	return ch, nil
}

func (p *OpenAIProvider) Completion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	content := req.Content
	cursor := req.CursorPos
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(content) {
		cursor = len(content)
	}
	before := content[:cursor]
	after := content[cursor:]

	// Trim context windows: keep last 2000 chars before cursor, first 500 after.
	if len(before) > 2000 {
		before = before[len(before)-2000:]
	}
	afterCtx := after
	if len(afterCtx) > 500 {
		afterCtx = afterCtx[:500]
	}

	messages := []Message{
		{Role: "system", Content: `你是一个世界级的代码补全引擎，集成在 IDE 中。你的任务是根据光标前后的代码上下文，预测开发者最可能输入的下一个代码片段。

规则：
- 只返回要插入的代码，不要任何解释、注释或 markdown
- 如果上下文已经完整（如括号已闭合、语句已结束、空行后），则不要补全，返回空
- 补全可以是片段（如一个函数调用的剩余参数）、整行、或连续多行（如完整的方法体、if 块、循环体）
- 保持与周围代码一致的缩进风格（tabs/spaces）
- 如果是补全函数参数，推断最可能的参数值（如变量名、常量、字面量）
- 如果是补全方法调用，参考该文件中已有的调用模式
- 不要重复光标前已有的代码`},
		{Role: "user", Content: fmt.Sprintf("文件: %s\n语言: %s\n\n光标前的代码:\n%s\n\n光标后的代码:\n%s\n\n请补全光标处的代码（只输出要插入的内容）:", req.File, req.Language, before, afterCtx)},
	}

	maxTokens := EstimateContextWindow(req.Model)
	if maxTokens > 4096 {
		maxTokens = 512
	}
	if maxTokens < 64 {
		maxTokens = 256
	}

	chatReq := ChatRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	resp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	return &CompletionResponse{Text: resp.Content}, nil
}

func (p *OpenAIProvider) ListModels(ctx context.Context) ([]Model, error) {
	if p.config.APIKey == "" {
		return defaultModels(p), nil
	}

	endpoint := p.config.Endpoint
	// Volcengine uses a non-standard path; models are at /api/v3/models
	if strings.Contains(endpoint, "volces.com") {
		u, _ := strings.CutPrefix(endpoint, "https://")
		u, _, _ = strings.Cut(u, "/")
		endpoint = "https://" + u + "/api/v3/models"
	} else {
		endpoint = strings.TrimSuffix(endpoint, "/chat/completions")
		endpoint = strings.TrimSuffix(endpoint, "/v1")
		endpoint = endpoint + "/v1/models"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		log.Printf("[ListModels] request creation failed: %v", err)
		return defaultModels(p), nil
	}

	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	client := NewHTTPClient(10 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[ListModels] request failed: %v", err)
		return defaultModels(p), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ListModels] unexpected status %d", resp.StatusCode)
		return defaultModels(p), nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("[ListModels] decode failed: %v", err)
		return defaultModels(p), nil
	}

	models := make([]Model, 0, len(result.Data))
	for _, m := range result.Data {
		cw := EstimateContextWindow(m.ID)
		models = append(models, Model{
			ID:               m.ID,
			Name:             m.ID,
			ProviderID:       p.ID(),
			SupportsTool:     true,
			SupportsVision:   false,
			SupportsThinking: true,
			MaxTokens:        cw,
			ContextWindow:    cw,
		})
	}
	if len(models) == 0 {
		return defaultModels(p), nil
	}
	return models, nil
}

func defaultModels(p *OpenAIProvider) []Model {
	return []Model{
		{ID: "gpt-4o", Name: "GPT-4o", ProviderID: p.ID(), MaxTokens: 200000, ContextWindow: 200000, SupportsTool: true, SupportsVision: true},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", ProviderID: p.ID(), MaxTokens: 128000, ContextWindow: 128000, SupportsTool: true},
		{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", ProviderID: p.ID(), MaxTokens: 128000, ContextWindow: 128000, SupportsTool: true},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", ProviderID: p.ID(), MaxTokens: 16385, ContextWindow: 16385, SupportsTool: true},
	}
}

func (p *OpenAIProvider) Validate(ctx context.Context) error {
	if p.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	_, err := p.ListModels(ctx)
	return err
}

// ---- shared helpers ----

func EstimateContextWindow(model string) int {
	return GetModelCapabilities(model).ContextWindow
}

func pickTemp(reqTemp, defaultTemp float64) float64 {
	if reqTemp > 0 {
		return reqTemp
	}
	return defaultTemp
}
