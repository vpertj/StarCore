package completion

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"StarCore/internal/provider"
)

type CompletionType string

const (
	TypeLine  CompletionType = "line"
	TypeBlock CompletionType = "block"
	TypeFIM   CompletionType = "fim"
)

type Suggestion struct {
	Text string         `json:"text"`
	Type CompletionType `json:"type"`
	Rank int            `json:"rank"`
}

type FIMRequest struct {
	BeforeCursor string `json:"beforeCursor"`
	AfterCursor  string `json:"afterCursor"`
	FileName     string `json:"fileName"`
	Language     string `json:"language"`
	MaxTokens    int    `json:"maxTokens,omitempty"`
}

type Service struct {
	providerMgr *provider.Manager
	cache       *completionCache
	mu          sync.Mutex
}

type completionCache struct {
	entries map[string]*cacheEntry
	mu      sync.RWMutex
}

type cacheEntry struct {
	suggestion *Suggestion
	createdAt  time.Time
}

func NewService(providerMgr *provider.Manager) *Service {
	return &Service{
		providerMgr: providerMgr,
		cache: &completionCache{
			entries: make(map[string]*cacheEntry),
		},
	}
}

func (s *Service) Complete(ctx context.Context, req FIMRequest) (*Suggestion, error) {
	cacheKey := fmt.Sprintf("%s:%d:%s", req.FileName, len(req.BeforeCursor), lastLine(req.BeforeCursor))
	if cached := s.cache.get(cacheKey); cached != nil {
		return cached, nil
	}

	p := s.providerMgr.GetDefaultProvider()
	if p == nil {
		return nil, fmt.Errorf("no provider configured")
	}

	cfg := p.GetConfig()
	if cfg.APIKey == "" && p.ID() != "ollama" {
		return s.fallbackChatCompletion(ctx, req)
	}

	switch p.ID() {
	case "openai", "deepseek":
		return s.openaiFIM(ctx, cfg, req)
	default:
		return s.fallbackChatCompletion(ctx, req)
	}
}

func (s *Service) CompleteMultiLine(ctx context.Context, req FIMRequest) (*Suggestion, error) {
	req.MaxTokens = 256
	sug, err := s.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	if sug == nil {
		return nil, nil
	}

	text := sug.Text
	if !strings.Contains(text, "\n") {
		lines := strings.Split(text, "\n")
		if len(lines) > 1 {
			text = strings.Join(lines[:min(len(lines), 5)], "\n")
		}
	}
	sug.Text = text
	sug.Type = TypeBlock
	return sug, nil
}

func (s *Service) openaiFIM(ctx context.Context, cfg provider.ProviderConfig, req FIMRequest) (*Suggestion, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimRight(endpoint, "/")

	fimURL := endpoint + "/completions"

	before := req.BeforeCursor
	if len(before) > 3000 {
		before = before[len(before)-3000:]
	}
	after := req.AfterCursor
	if len(after) > 1000 {
		after = after[:1000]
	}

	prompt := before + "<|FIM_SUFFIX|>" + after + "<|FIM_PREFIX|>" + before

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 128
	}

	body := map[string]any{
		"model":       "code-completion",
		"prompt":      prompt,
		"max_tokens":  maxTokens,
		"temperature": 0.1,
		"stop":        []string{"<|FIM_SUFFIX|>", "<|FIM_PREFIX|>", "\n\n"},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fimURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return s.fallbackChatCompletion(ctx, req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return s.fallbackChatCompletion(ctx, req)
	}

	var result struct {
		Choices []struct {
			Text string `json:"text"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 || result.Choices[0].Text == "" {
		return nil, nil
	}

	sug := &Suggestion{
		Text: strings.TrimSpace(result.Choices[0].Text),
		Type: TypeFIM,
		Rank: 1,
	}

	cacheKey := fmt.Sprintf("%s:%d:%s", req.FileName, len(req.BeforeCursor), lastLine(req.BeforeCursor))
	s.cache.put(cacheKey, sug)

	return sug, nil
}

func (s *Service) openaiFIMStream(ctx context.Context, cfg provider.ProviderConfig, req FIMRequest) (*Suggestion, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimRight(endpoint, "/")

	fimURL := endpoint + "/completions"

	before := req.BeforeCursor
	if len(before) > 3000 {
		before = before[len(before)-3000:]
	}
	after := req.AfterCursor
	if len(after) > 1000 {
		after = after[:1000]
	}

	prompt := before + "<|FIM_SUFFIX|>" + after + "<|FIM_PREFIX|>" + before

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 128
	}

	body := map[string]any{
		"model":       "code-completion",
		"prompt":      prompt,
		"max_tokens":  maxTokens,
		"temperature": 0.1,
		"stream":      true,
		"stop":        []string{"<|FIM_SUFFIX|>", "<|FIM_PREFIX|>", "\n\n"},
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", fimURL, strings.NewReader(string(bodyJSON)))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return s.fallbackChatCompletion(ctx, req)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return s.fallbackChatCompletion(ctx, req)
	}

	var resultText strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024*64), 1024*64)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Text string `json:"text"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			resultText.WriteString(chunk.Choices[0].Text)
		}
	}

	text := strings.TrimSpace(resultText.String())
	if text == "" {
		return nil, nil
	}

	sug := &Suggestion{
		Text: text,
		Type: TypeFIM,
		Rank: 1,
	}

	cacheKey := fmt.Sprintf("%s:%d:%s", req.FileName, len(req.BeforeCursor), lastLine(req.BeforeCursor))
	s.cache.put(cacheKey, sug)

	return sug, nil
}

func (s *Service) fallbackChatCompletion(ctx context.Context, req FIMRequest) (*Suggestion, error) {
	p := s.providerMgr.GetDefaultProvider()
	if p == nil {
		return nil, fmt.Errorf("no provider configured")
	}

	before := req.BeforeCursor
	if len(before) > 2000 {
		before = before[len(before)-2000:]
	}
	after := req.AfterCursor
	if len(after) > 500 {
		after = after[:500]
	}

	systemPrompt := `你是一个代码补全引擎。根据光标前后的代码上下文，补全光标处的代码。
规则：
- 只返回要插入的代码，不要任何解释、注释或markdown
- 保持与周围代码一致的缩进风格
- 不要重复光标前已有的代码
- 如果上下文已完整，返回空
- 优先补全当前行，其次是代码块`

	userMsg := fmt.Sprintf("文件: %s\n语言: %s\n\n光标前:\n%s\n\n光标后:\n%s\n\n补全:", req.FileName, req.Language, before, after)

	maxTokens := 128
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	chatReq := provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMsg},
		},
		Temperature: 0.1,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	resp, err := p.Chat(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(resp.Content)
	if text == "" {
		return nil, nil
	}

	sug := &Suggestion{
		Text: text,
		Type: TypeLine,
		Rank: 1,
	}

	cacheKey := fmt.Sprintf("%s:%d:%s", req.FileName, len(req.BeforeCursor), lastLine(req.BeforeCursor))
	s.cache.put(cacheKey, sug)

	return sug, nil
}

func (c *completionCache) get(key string) *Suggestion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	if time.Since(entry.createdAt) > 3*time.Second {
		return nil
	}
	return entry.suggestion
}

func (c *completionCache) put(key string, sug *Suggestion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) > 500 {
		for k := range c.entries {
			delete(c.entries, k)
			if len(c.entries) <= 250 {
				break
			}
		}
	}
	c.entries[key] = &cacheEntry{
		suggestion: sug,
		createdAt:  time.Now(),
	}
}

func lastLine(text string) string {
	if text == "" {
		return ""
	}
	idx := strings.LastIndex(text, "\n")
	if idx < 0 {
		return text
	}
	line := text[idx+1:]
	if len(line) > 80 {
		return line[:80]
	}
	return line
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func discardBody(body io.ReadCloser) {
	io.Copy(io.Discard, body)
	body.Close()
}
