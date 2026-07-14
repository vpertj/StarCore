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

type AnthropicProvider struct {
	config ProviderConfig
}

const anthroVersion = "2023-06-01"

func NewAnthropicProvider() *AnthropicProvider {
	return &AnthropicProvider{
		config: ProviderConfig{
			ID:       "anthropic",
			Name:     "Anthropic",
			Endpoint: "https://api.anthropic.com/v1/messages",
			Enabled:  false,
		},
	}
}

func (p *AnthropicProvider) ID() string                      { return "anthropic" }
func (p *AnthropicProvider) Name() string                    { return "Anthropic" }
func (p *AnthropicProvider) SetConfig(config ProviderConfig) { p.config = config }
func (p *AnthropicProvider) GetConfig() ProviderConfig       { return p.config }

func (p *AnthropicProvider) timeout() time.Duration {
	if p.config.TimeoutSecs > 0 {
		return time.Duration(p.config.TimeoutSecs) * time.Second
	}
	return 120 * time.Second
}

type anthropicContent struct {
	Type      string                `json:"type"`
	Text      string                `json:"text,omitempty"`
	ID        string                `json:"id,omitempty"`
	Name      string                `json:"name,omitempty"`
	Input     json.RawMessage       `json:"input,omitempty"`
	ToolUseID string                `json:"tool_use_id,omitempty"`
	Content   json.RawMessage       `json:"content,omitempty"`
	Source    *anthropicImageSource `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type anthropicCacheControl struct {
	Type string `json:"type"` // "ephemeral"
}

type anthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      interface{}        `json:"system,omitempty"` // string or []anthropicSystemBlock
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicMessage struct {
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta"`
	ContentBlock *struct {
		Type string `json:"type"`
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"content_block"`
	Usage *struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
	} `json:"usage,omitempty"`
	Message *anthropicResponse `json:"message"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func convertToAnthropicMessages(messages []Message) (string, []anthropicMessage) {
	var systemPrompt string
	var result []anthropicMessage
	for _, msg := range messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		role := "user"
		var content []anthropicContent

		switch msg.Role {
		case "assistant":
			role = "assistant"
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					if msg.Content != "" {
						content = append(content, anthropicContent{Type: "text", Text: msg.Content})
					}
					content = append(content, anthropicContent{
						Type:  "tool_use",
						ID:    tc.ID,
						Name:  tc.Function.Name,
						Input: json.RawMessage(tc.Function.Arguments),
					})
				}
			} else {
				content = append(content, anthropicContent{Type: "text", Text: msg.Content})
			}
		case "tool":
			role = "user"
			content = append(content, anthropicContent{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   json.RawMessage(`"` + msg.Content + `"`),
			})
		default:
			content = append(content, anthropicContent{Type: "text", Text: msg.Content})
			for _, img := range msg.Images {
				if img.Data != "" {
					mediaType := img.MediaType
					if mediaType == "" {
						mediaType = "image/png"
					}
					content = append(content, anthropicContent{
						Type: "image",
						Source: &anthropicImageSource{
							Type:      "base64",
							MediaType: mediaType,
							Data:      img.Data,
						},
					})
				} else if img.URL != "" {
					content = append(content, anthropicContent{Type: "text", Text: fmt.Sprintf("[Image: %s]", img.URL)})
				}
			}
		}
		result = append(result, anthropicMessage{Role: role, Content: content})
	}
	return systemPrompt, result
}

// buildAnthropicSystem builds the system prompt with cache_control markers.
// The system prompt is sent as an array of content blocks with cache_control
// to enable Anthropic's prompt caching (90% discount on cache reads).
func buildAnthropicSystem(systemPrompt string) []anthropicSystemBlock {
	if systemPrompt == "" {
		return nil
	}
	return []anthropicSystemBlock{
		{
			Type: "text",
			Text: systemPrompt,
			CacheControl: &anthropicCacheControl{
				Type: "ephemeral",
			},
		},
	}
}

func convertAnthropicTools(tools []ToolDefinition) []anthropicTool {
	result := make([]anthropicTool, 0, len(tools))
	for _, t := range tools {
		result = append(result, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: t.Function.Parameters,
		})
	}
	return result
}

func (p *AnthropicProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if p.config.APIKey == "" {
		return nil, fmt.Errorf("Anthropic API key is required")
	}

	systemPrompt, messages := convertToAnthropicMessages(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = EstimateContextWindow(req.Model)
		if maxTokens > 65536 {
			maxTokens = 16384
		}
	}

	body := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Messages:    messages,
		System:      buildAnthropicSystem(systemPrompt),
		Temperature: req.Temperature,
		Stream:      false,
		Tools:       convertAnthropicTools(req.Tools),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthroVersion)

	client := NewHTTPClient(p.timeout())
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	var content string
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	var usage *TokenUsage
	if apiResp.Usage != nil {
		usage = &TokenUsage{
			PromptTokens:        apiResp.Usage.InputTokens,
			CompletionTokens:    apiResp.Usage.OutputTokens,
			CacheCreationTokens: apiResp.Usage.CacheCreationInputTokens,
			CacheReadTokens:     apiResp.Usage.CacheReadInputTokens,
		}
	}

	return &ChatResponse{Content: content, Provider: p.ID(), Model: req.Model, Usage: usage}, nil
}

func (p *AnthropicProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 64)

	if p.config.APIKey == "" {
		close(ch)
		return ch, fmt.Errorf("Anthropic API key is required")
	}

	systemPrompt, messages := convertToAnthropicMessages(req.Messages)
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = EstimateContextWindow(req.Model)
		if maxTokens > 65536 {
			maxTokens = 16384
		}
	}

	body := anthropicRequest{
		Model:       req.Model,
		MaxTokens:   maxTokens,
		Messages:    messages,
		System:      buildAnthropicSystem(systemPrompt),
		Temperature: req.Temperature,
		Stream:      true,
		Tools:       convertAnthropicTools(req.Tools),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthroVersion)
	httpReq.Header.Set("Accept", "text/event-stream")

	client := NewHTTPClient(0)
	resp, err := client.Do(httpReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("request failed: %w", err)
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			ch <- StreamEvent{Type: "error", Content: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBody))}
			return
		}

		// Track multiple pending tool calls (Anthropic can return parallel tool_use blocks)
		type pendingTool struct {
			id   string
			name string
			args strings.Builder
		}
		pendingTools := make(map[string]*pendingTool)
		var activeToolID string
		receivedDone := false
		var lastUsage *TokenUsage

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if !receivedDone {
						ch <- StreamEvent{Type: "error", Content: "stream ended unexpectedly (EOF without message_stop)"}
					}
				} else if !strings.Contains(err.Error(), "use of closed network") {
					ch <- StreamEvent{Type: "error", Content: err.Error()}
				}
				break
			}

			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Error != nil {
				ch <- StreamEvent{Type: "error", Content: event.Error.Message}
				break
			}

			switch event.Type {
			case "content_block_start":
				if event.ContentBlock != nil && event.ContentBlock.Type == "tool_use" {
					activeToolID = event.ContentBlock.ID
					pendingTools[activeToolID] = &pendingTool{
						id:   event.ContentBlock.ID,
						name: event.ContentBlock.Name,
					}
				}
			case "content_block_delta":
				if event.Delta != nil {
					if event.Delta.Type == "text" && event.Delta.Text != "" {
						ch <- StreamEvent{Type: "data", Content: event.Delta.Text}
					} else if event.Delta.Type == "input_json_delta" && event.Delta.PartialJSON != "" {
						if tool, ok := pendingTools[activeToolID]; ok {
							tool.args.WriteString(event.Delta.PartialJSON)
						}
					}
				}
			case "message_delta":
				if event.Usage != nil {
					lastUsage = &TokenUsage{
						PromptTokens:     event.Usage.InputTokens,
						CompletionTokens: event.Usage.OutputTokens,
						CacheReadTokens:  event.Usage.CacheReadInputTokens,
					}
				}
			case "message_stop":
				receivedDone = true
				// Flush ALL pending tool calls (supports parallel tools)
				if len(pendingTools) > 0 {
					toolCalls := make([]ToolCall, 0, len(pendingTools))
					for _, tool := range pendingTools {
						toolCalls = append(toolCalls, ToolCall{
							ID:   tool.id,
							Type: "function",
							Function: ToolCallFunc{
								Name:      tool.name,
								Arguments: tool.args.String(),
							},
						})
					}
					ch <- StreamEvent{Type: "tool_call", ToolCalls: toolCalls}
				}
				ch <- StreamEvent{Type: "done", Usage: lastUsage}
				return
			}
		}
	}()

	return ch, nil
}

func (p *AnthropicProvider) Completion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	messages := []Message{
		{Role: "system", Content: "You are a code completion assistant. Return only the code to insert."},
		{Role: "user", Content: fmt.Sprintf("File: %s\nLanguage: %s\nCode:\n%s\nComplete at position %d:", req.File, req.Language, req.Content, req.CursorPos)},
	}
	chatReq := ChatRequest{Model: req.Model, Messages: messages, Temperature: pickTemp(req.Temperature, 0.2), MaxTokens: 256}
	resp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return &CompletionResponse{Text: resp.Content}, nil
}

func (p *AnthropicProvider) ListModels(ctx context.Context) ([]Model, error) {
	if p.config.APIKey == "" {
		return []Model{}, nil
	}
	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}
	modelsURL := strings.TrimRight(endpoint, "/")
	modelsURL = strings.TrimSuffix(modelsURL, "/messages")
	modelsURL += "/models"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		log.Printf("[ListModels] request creation failed: %v", err)
		return []Model{}, nil
	}
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthroVersion)

	client := NewHTTPClient(p.timeout() / 4)
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[ListModels] request failed: %v", err)
		return []Model{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ListModels] unexpected status %d", resp.StatusCode)
		return []Model{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ListModels] read body failed: %v", err)
		return []Model{}, nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		log.Printf("[ListModels] decode failed: %v", err)
		return []Model{}, nil
	}

	models := make([]Model, 0, len(result.Data))
	for _, m := range result.Data {
		cw := EstimateContextWindow(m.ID)
		models = append(models, Model{
			ID:            m.ID,
			Name:          m.ID,
			ProviderID:    "anthropic",
			MaxTokens:     200000,
			ContextWindow: cw,
			SupportsTool:  true,
		})
	}
	return models, nil
}

func (p *AnthropicProvider) Validate(ctx context.Context) error {
	if p.config.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	endpoint := p.config.Endpoint
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	}
	modelsURL := strings.TrimRight(endpoint, "/")
	modelsURL = strings.TrimSuffix(modelsURL, "/messages")
	modelsURL += "/models"
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", anthroVersion)
	client := NewHTTPClient(p.timeout() / 12)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid API key (status %d)", resp.StatusCode)
	}
	return nil
}
