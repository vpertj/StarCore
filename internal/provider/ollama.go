package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type OllamaProvider struct {
	config ProviderConfig
}

func NewOllamaProvider() *OllamaProvider {
	return &OllamaProvider{
		config: ProviderConfig{
			ID:       "ollama",
			Name:     "Ollama",
			Endpoint: "http://localhost:11434",
			Enabled:  false,
		},
	}
}

func (p *OllamaProvider) ID() string                      { return "ollama" }
func (p *OllamaProvider) Name() string                    { return "Ollama" }
func (p *OllamaProvider) SetConfig(config ProviderConfig) { p.config = config }
func (p *OllamaProvider) GetConfig() ProviderConfig       { return p.config }

type ollamaChatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Role      string     `json:"role"`
		Content   string     `json:"content"`
		ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	} `json:"message"`
	Done bool `json:"done"`
}

type ollamaModel struct {
	Name string `json:"name"`
}

type ollamaModelsResponse struct {
	Models []ollamaModel `json:"models"`
}

func (p *OllamaProvider) getEndpoint() string {
	if p.config.Endpoint != "" {
		return p.config.Endpoint
	}
	return "http://localhost:11434"
}

func (p *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := ollamaChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
		Tools:    req.Tools,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.getEndpoint() + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := NewHTTPClient(300 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))
		return nil, fmt.Errorf("ollama returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk ollamaChatResponse
		if json.Unmarshal([]byte(line), &chunk) == nil {
			result.WriteString(chunk.Message.Content)
			if chunk.Done {
				break
			}
		}
	}

	return &ChatResponse{Content: result.String(), Provider: p.ID(), Model: req.Model}, nil
}

func (p *OllamaProvider) ChatStream(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 64)

	body := ollamaChatRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   true,
		Tools:    req.Tools,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to marshal request: %w", err)
	}

	endpoint := p.getEndpoint() + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(string(jsonBody)))
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := NewHTTPClient(0)
	resp, err := client.Do(httpReq)
	if err != nil {
		close(ch)
		return ch, fmt.Errorf("request failed: %w", err)
	}

	go func() {
		defer close(ch)
		defer resp.Body.Close()

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
			var chunk ollamaChatResponse
			if json.Unmarshal([]byte(line), &chunk) != nil {
				continue
			}
			if chunk.Message.Content != "" {
				ch <- StreamEvent{Type: "data", Content: chunk.Message.Content}
			}
			if len(chunk.Message.ToolCalls) > 0 {
				ch <- StreamEvent{Type: "tool_call", ToolCalls: chunk.Message.ToolCalls}
			}
			if chunk.Done {
				ch <- StreamEvent{Type: "done"}
				break
			}
		}
	}()

	return ch, nil
}

func (p *OllamaProvider) Completion(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	messages := []Message{
		{Role: "system", Content: "Continue the code. Return only code."},
		{Role: "user", Content: req.Content},
	}
	chatReq := ChatRequest{Model: req.Model, Messages: messages, Temperature: pickTemp(req.Temperature, 0.2), MaxTokens: 256}
	resp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}
	return &CompletionResponse{Text: resp.Content}, nil
}

func (p *OllamaProvider) ListModels(ctx context.Context) ([]Model, error) {
	endpoint := p.getEndpoint() + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	client := NewHTTPClient(5 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[ListModels] ollama request failed: %v", err)
		return []Model{}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[ListModels] read body failed: %v", err)
		return []Model{}, nil
	}

	var modelsResp ollamaModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		log.Printf("[ListModels] decode failed: %v", err)
		return []Model{}, nil
	}

	models := make([]Model, 0, len(modelsResp.Models))
	for _, m := range modelsResp.Models {
		models = append(models, Model{
			ID:            m.Name,
			Name:          m.Name,
			ProviderID:    "ollama",
			MaxTokens:     32000,
			ContextWindow: 32000,
		})
	}
	return models, nil
}

func (p *OllamaProvider) Validate(ctx context.Context) error {
	endpoint := p.getEndpoint() + "/api/tags"
	httpReq, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return err
	}
	client := NewHTTPClient(5 * time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("ollama not running: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}
	return nil
}
