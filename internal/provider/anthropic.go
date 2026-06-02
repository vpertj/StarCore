package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Error      *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
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
		role := msg.Role
		if role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		result = append(result, anthropicMessage{Role: role, Content: msg.Content})
	}
	return systemPrompt, result
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
		System:      systemPrompt,
		Temperature: req.Temperature,
		Stream:      false,
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

	client := &http.Client{Timeout: p.timeout()}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
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

	return &ChatResponse{Content: content, Provider: p.ID(), Model: req.Model}, nil
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
		System:      systemPrompt,
		Temperature: req.Temperature,
		Stream:      true,
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

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(httpReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("request failed: %w", err)
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := ioutil.ReadAll(resp.Body)
			ch <- StreamEvent{Type: "error", Content: fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(respBody))}
			return
		}

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF && !strings.Contains(err.Error(), "use of closed network") {
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
			case "content_block_delta":
				if event.Delta != nil && event.Delta.Text != "" {
					ch <- StreamEvent{Type: "data", Content: event.Delta.Text}
				}
			case "message_stop":
				ch <- StreamEvent{Type: "done"}
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
		return []Model{}, nil
	}
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthroVersion)

	client := &http.Client{Timeout: p.timeout() / 4}
	resp, err := client.Do(httpReq)
	if err != nil {
		return []Model{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []Model{}, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Model{}, nil
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return []Model{}, nil
	}

	models := make([]Model, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, Model{
			ID:           m.ID,
			Name:         m.ID,
			ProviderID:   "anthropic",
			MaxTokens:    200000,
			SupportsTool: true,
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
	client := &http.Client{Timeout: p.timeout() / 12}
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
