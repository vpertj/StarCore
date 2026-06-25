package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"StarCore/internal/agent"
	agentTools "StarCore/internal/agent/tools"
	"StarCore/internal/memory"
	"StarCore/internal/provider"
	"StarCore/internal/sandbox"
)

const maxToolResultChars = 8000

// calcMaxAgentLoops computes the maximum agent loop iterations based on mode and model.
// chat: 10 (lightweight Q&A, rarely needs tools)
// plan: 25 (read-only analysis, needs several file reads)
// build: 15-60 (autonomous coding, depends on context window)
func calcMaxAgentLoops(mode, model string) int {
	ctxWindow := provider.EstimateContextWindow(model)
	if ctxWindow <= 0 {
		ctxWindow = 128000
	}

	switch mode {
	case "chat":
		return 15
	case "plan":
		return 35
	case "build":
		switch {
		case ctxWindow >= 500000:
			return 80
		case ctxWindow >= 200000:
			return 65
		case ctxWindow >= 128000:
			return 50
		case ctxWindow >= 64000:
			return 35
		default:
			return 20
		}
	default:
		switch {
		case ctxWindow >= 200000:
			return 50
		case ctxWindow >= 128000:
			return 40
		default:
			return 25
		}
	}
}

// EmitFunc emits events to the Wails frontend.
type EmitFunc func(event string, data interface{})

// ContextBuilder builds context messages for chat requests.
type ContextBuilder func(req provider.ChatRequest) string

// Compressor compresses messages to fit context windows.
type Compressor func(msgs []provider.Message, maxTokens int, providerID string, conversationID string) ([]provider.Message, bool)

// ContextWindowEstimator estimates the context window size for a model.
type ContextWindowEstimator func(providerID, modelID string) int

// ContextProvider returns the Wails application context.
type ContextProvider func() context.Context

// VerifyFunc runs verification on modified files and returns a summary.
type VerifyFunc func(ctx context.Context, filePaths []string) string

// Service manages AI chat, streaming, and agent loop execution.
type Service struct {
	providerMgr *provider.Manager
	toolExec    *agent.ToolExecutor
	memoryStore *memory.Store
	agentReg    *agent.Registry

	emitFn        EmitFunc
	buildContext  ContextBuilder
	compress      Compressor
	contextWindow ContextWindowEstimator
	appCtx        ContextProvider

	loopState *agentTools.LoopState
	verifyFn  VerifyFunc
	cb        *CircuitBreaker
	sem       chan struct{}

	mu                sync.Mutex
	cancel            context.CancelFunc
	fingerprints      []string
	currentConvID     string
	continueCh        chan int
	autoContinueCount int // frontend can send extra loops via this channel

	// Token budget tracking
	totalTokensUsed int
	contextWindowFn ContextWindowEstimator

	// File change history for undo/redo
	fileHistory *agentTools.FileHistory

	// Agent loop progress tracker
	progress *AgentProgress

	intentClassifier *agent.IntentClassifier
	toolRouter       *agent.ToolRouter
	taskRouter       *TaskRouter
}

// AgentProgress tracks progress within a single agent loop session.
type AgentProgress struct {
	filesModified    map[string]bool // files that have been written/edited
	toolCallCount    int             // total tool calls made
	successfulCalls  int             // tool calls that succeeded
	consecutiveEmpty int             // consecutive rounds with no tool calls
	lastFileCount    int             // file count at last nudge
}

func newAgentProgress() *AgentProgress {
	return &AgentProgress{
		filesModified: make(map[string]bool),
	}
}

// NewService creates a new AI service.
func NewService(
	providerMgr *provider.Manager,
	toolExec *agent.ToolExecutor,
	memoryStore *memory.Store,
	agentReg *agent.Registry,
	emitFn EmitFunc,
	buildContext ContextBuilder,
	compress Compressor,
	contextWindow ContextWindowEstimator,
	appCtx ContextProvider,
	verifyFn VerifyFunc,
) *Service {
	ls := agentTools.NewLoopState()
	agentTools.LoopStateRef = ls
	return &Service{
		providerMgr:      providerMgr,
		toolExec:         toolExec,
		memoryStore:      memoryStore,
		agentReg:         agentReg,
		emitFn:           emitFn,
		buildContext:     buildContext,
		compress:         compress,
		contextWindow:    contextWindow,
		contextWindowFn:  contextWindow,
		appCtx:           appCtx,
		loopState:        ls,
		verifyFn:         verifyFn,
		continueCh:       make(chan int, 1),
		cb:               NewCircuitBreaker(10, 60*time.Second),
		sem:              make(chan struct{}, 3),
		fileHistory:      agentTools.NewFileHistory(),
		progress:         newAgentProgress(),
		intentClassifier: agent.NewIntentClassifier(),
		toolRouter:       agent.NewToolRouter(),
		taskRouter:       NewTaskRouter(),
	}
}

func estimateTokens(text string) int {
	return estimateTokensWithModel(text, "")
}

// estimateTokensWithModel estimates token count with model-specific ratios.
// When model is known, uses provider-specific CJK/ASCII ratios for better accuracy.
// Falls back to cl100k-based defaults for unknown models.
func estimateTokensWithModel(text string, model string) int {
	if len(text) == 0 {
		return 0
	}
	cjk := 0
	asciiWords := 0
	inWord := false
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
			r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
			r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
			r >= 0xAC00 && r <= 0xD7AF {
			cjk++
			inWord = false
		} else if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			if !inWord {
				asciiWords++
				inWord = true
			}
		} else {
			inWord = false
		}
	}

	// Model-specific ratios (tokens per CJK character and tokens per ASCII word)
	cjkRatio := 1.5  // default: cl100k_base
	wordRatio := 1.3 // default: cl100k_base

	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "deepseek"):
		cjkRatio = 1.3 // DeepSeek tokenizer is more efficient for Chinese
		wordRatio = 1.25
	case strings.Contains(m, "qwen"):
		cjkRatio = 1.4 // Qwen tokenizer
		wordRatio = 1.3
	case strings.Contains(m, "claude"):
		cjkRatio = 1.6 // Claude tokenizer — slightly less efficient for CJK
		wordRatio = 1.35
	case strings.Contains(m, "gpt-4o") || strings.Contains(m, "o1") || strings.Contains(m, "o3"):
		cjkRatio = 1.5 // o200k_base — similar to cl100k for CJK
		wordRatio = 1.3
	case strings.Contains(m, "gpt-") || strings.Contains(m, "o1-") || strings.Contains(m, "o3-"):
		cjkRatio = 1.5 // cl100k_base
		wordRatio = 1.3
	case strings.Contains(m, "gemini"):
		cjkRatio = 1.55 // Gemini tokenizer
		wordRatio = 1.3
	}

	nonWordChars := 0
	for _, r := range text {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') &&
			!(r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
				r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
				r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
				r >= 0xAC00 && r <= 0xD7AF) {
			nonWordChars++
		}
	}
	return int(float64(cjk)*cjkRatio+float64(asciiWords)*wordRatio+float64(nonWordChars)*0.4) + 1
}

func executeToolWithTimeout(ctx context.Context, executor *agent.ToolExecutor, call agent.ToolCall, timeout time.Duration) (*agent.ToolResult, error) {
	type toolResponse struct {
		result *agent.ToolResult
		err    error
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ch := make(chan toolResponse, 1)
	go func() {
		r, e := executor.Execute(toolCtx, call)
		select {
		case ch <- toolResponse{r, e}:
		case <-toolCtx.Done():
		}
	}()
	select {
	case resp := <-ch:
		return resp.result, resp.err
	case <-toolCtx.Done():
		return nil, fmt.Errorf("tool execution timed out after %v", timeout)
	}
}

func preCheckProvider(providerMgr *provider.Manager, providerID string) error {
	p, err := providerMgr.Get(providerID)
	if err != nil {
		return fmt.Errorf("%s: %s", provider.T("no_provider"), providerID)
	}
	cfg := p.GetConfig()
	_, isOllama := p.(*provider.OllamaProvider)
	if cfg.APIKey == "" && !isOllama {
		return fmt.Errorf("%s %s", provider.T("auth_failed"), providerID)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("%s %s", provider.T("no_endpoint"), providerID)
	}
	return nil
}

func isSimpleMessage(msgs []provider.Message) bool {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			trimmed := strings.TrimSpace(msgs[i].Content)
			if len(trimmed) > 10 {
				return false
			}
			lower := strings.ToLower(trimmed)
			techIndicators := []string{"fix", "add", "refactor", "write", "create", "implement", "test", "deploy", "build",
				"修复", "添加", "重构", "写", "创建", "实现", "测试", "部署", "构建", "改", "删", "优化"}
			for _, kw := range techIndicators {
				if strings.Contains(lower, kw) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func buildToolSuppressHint(msgs []provider.Message) string {
	if len(msgs) == 0 {
		return ""
	}
	lastUserMsg := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			lastUserMsg = msgs[i].Content
			break
		}
	}
	if lastUserMsg == "" {
		return ""
	}
	trimmed := strings.TrimSpace(lastUserMsg)
	if len(trimmed) > 10 {
		return ""
	}
	lower := strings.ToLower(trimmed)
	techIndicators := []string{"fix", "add", "refactor", "write", "create", "implement", "test", "deploy", "build",
		"修复", "添加", "重构", "写", "创建", "实现", "测试", "部署", "构建", "改", "删", "优化"}
	for _, kw := range techIndicators {
		if strings.Contains(lower, kw) {
			return ""
		}
	}
	return prompt("suppress_hint")
}

// getLanguageHint returns a model-aware language instruction.
// DeepSeek/Chinese models: strong Chinese instruction (they respond natively in Chinese)
// GPT/Claude: match user language (they default to English for reasoning)
func getLanguageHint(model string) string {
	m := strings.ToLower(model)

	// Chinese-native models — use strong Chinese instruction
	if strings.Contains(m, "deepseek") || strings.Contains(m, "qwen") || strings.Contains(m, "yi") {
		return "❗ 语言要求：始终使用中文回复。包括思考过程、代码注释、提交信息、错误分析等所有内容都必须使用中文。唯一例外是代码本身（变量名、函数名等使用英文）。\n"
	}

	// English-primary models — match user language
	if prompt("language_hint") != "language_hint" {
		return prompt("language_hint")
	}
	return "用和用户相同的语言回答。用户用中文提问就用中文回答，用户用英文提问就用英文回答。\n"
}

// --- 构建模式 ---
// 对标 Claude Code Build Mode：自主编程智能体，完整的工程纪律。
const buildModePrompt = `
=== 构建模式 ===
❗ 核心规则：你必须使用工具完成任务。只输出文字而不调用工具是错误的行为。

## 工具调用方式
如果模型支持 function calling，使用工具调用格式。
如果不支持，在回复中使用以下格式（每个工具调用单独一行）：
[TOOL: 工具名 {"参数名": "参数值"}]

示例：
[TOOL: read_file {"path": "main.go"}]
[TOOL: search_files {"query": "func main"}]
[TOOL: execute_command {"command": "go build ./..."}]

## 执行流程
1. 用 read_file 读取相关文件
2. 用 edit_file 或 write_file 修改代码
3. 用 execute_command 运行验证
4. 简要总结变更

## 规则
- 每次回复必须包含工具调用，否则是错误的
- 不要只说"我来读取"，要实际调用 read_file
- 不要只说"我来修复"，要实际调用 edit_file
- 输出完成后立即结束，不要重复
`

// --- 规划模式 ---
// 对标 Cursor Plan Mode：只读分析，输出结构化的实施方案。
const planModePrompt = `
=== 规划模式 ===
❗ 核心规则：你必须使用工具读取文件来分析。只输出文字而不读取文件是错误的行为。

## 工具调用方式
如果模型支持 function calling，使用工具调用格式。
如果不支持，在回复中使用以下格式：
[TOOL: read_file {"path": "文件路径"}]
[TOOL: search_files {"query": "搜索内容"}]

## 工作流程
1. 用 read_file 读取 1-3 个关键文件
2. 基于文件内容输出分析方案

## 规则
- 每次回复必须包含工具调用
- 不要只说"我来读取"，要实际调用 read_file
- 分析完成后以 --- 规划完成 --- 结尾
- 禁止使用 write_file、execute_command
`

// --- 对话模式 ---
// 对标 Claude Code Chat Mode：只读分析，精准解答。
const chatModePrompt = `
=== 对话模式 ===
❗ 核心规则：你必须使用工具读取文件来回答问题。只输出文字而不读取文件是错误的行为。

## 工具调用方式
如果模型支持 function calling，使用工具调用格式。
如果不支持，在回复中使用以下格式：
[TOOL: read_file {"path": "文件路径"}]
[TOOL: search_files {"query": "搜索内容"}]

## 工作流程
1. 用 search_files 或 read_file 定位相关代码
2. 基于代码给出分析

## 规则
- 每次回复必须包含工具调用
- 不要凭记忆猜测，要用工具验证
- 回答要简洁直接
`

// ChatStream initiates a streaming AI chat with agent support.
func (s *Service) ChatStream(req provider.ChatRequest) error {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
	}
	agentCtx, cancel := context.WithCancel(s.appCtx())
	s.cancel = cancel
	s.mu.Unlock()

	select {
	case s.sem <- struct{}{}:
	default:
		s.emitFn("ai:stream:error", provider.T("concurrency_limit"))
		cancel()
		return fmt.Errorf("concurrency limit reached")
	}
	defer func() { <-s.sem }()

	// Reset loop state when starting a new conversation
	if req.ConversationID != "" && req.ConversationID != s.currentConvID {
		s.mu.Lock()
		s.currentConvID = req.ConversationID
		s.loopState.Reset()
		s.fingerprints = nil
		s.autoContinueCount = 0
		s.mu.Unlock()
	}

	if req.ProviderID == "" {
		defProvider := s.providerMgr.GetDefaultProvider()
		if defProvider != nil {
			req.ProviderID = defProvider.ID()
		} else {
			s.emitFn("ai:stream:error", provider.T("no_provider"))
			return fmt.Errorf("no provider configured")
		}
	}

	if err := preCheckProvider(s.providerMgr, req.ProviderID); err != nil {
		s.emitFn("ai:stream:error", err.Error())
		return err
	}

	var intent *agent.IntentResult
	if len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				intent = s.intentClassifier.Classify(req.Messages[i].Content)
				break
			}
		}
	}

	if intent != nil {
		s.loopState.SetDetectedIntent(intent)
	}

	if req.AgentID == "" && intent != nil && intent.Confidence >= agent.HighConfidence {
		if suggestedAgent := s.agentReg.FindByIntent(intent.Intent); suggestedAgent != "" {
			req.AgentID = suggestedAgent
			s.emitFn("ai:agent:suggested", map[string]any{
				"agentID": suggestedAgent,
				"intent":  string(intent.Intent),
			})
		}
	}

	if intent != nil {
		route := s.taskRouter.Route(req.Messages, intent)
		if route.Route == "decompose" && len(route.SubTasks) > 0 {
			s.emitFn("ai:task:decomposed", map[string]any{
				"complexity": int(route.Complexity),
				"subtasks":   len(route.SubTasks),
			})
		}
	}

	if req.AgentID != "" {
		ag, ok := s.agentReg.Get(req.AgentID)
		if ok && ag.SystemPrompt != "" {
			modePrompt := ""
			switch req.Mode {
			case "plan":
				modePrompt = planModePrompt
			case "build":
				modePrompt = buildModePrompt
			default:
				modePrompt = chatModePrompt
			}
			if req.Mode == "build" {
				modePrompt += buildToolSuppressHint(req.Messages)
			}
			// Build context message and merge it INTO the system prompt
			// This ensures tool instructions come FIRST, context comes SECOND
			contextMsg := s.buildContext(req)
			systemContent := ag.SystemPrompt + getLanguageHint(req.Model) + modePrompt
			if contextMsg != "" {
				systemContent += "\n\n" + contextMsg
			}
			systemMsg := provider.Message{Role: "system", Content: systemContent}
			req.Messages = append([]provider.Message{systemMsg}, req.Messages...)
		}
		if ok && len(ag.Tools) > 0 {
			tools := ag.Tools
			if req.Mode == "chat" || req.Mode == "plan" {
				tools = []string{}
				for _, t := range ag.Tools {
					if tool, ok := s.toolExec.Get(t); ok && !tool.RequiresApproval() {
						tools = append(tools, t)
					}
				}
			}
			if req.Mode == "build" && isSimpleMessage(req.Messages) {
				tools = []string{}
			}
			if len(tools) > 0 {
				req.Tools = s.buildToolDefinitions(tools)
				// Inject tool instructions into the system prompt (not as separate message)
				toolHint := buildToolUsageHint(req.Tools, "")
				if toolHint != "" {
					// Prepend tool hint to the first system message
					if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
						req.Messages[0].Content = toolHint + "\n" + req.Messages[0].Content
					} else {
						req.Messages = append([]provider.Message{{Role: "system", Content: toolHint}}, req.Messages...)
					}
				}
			} else {
				req.Tools = nil
			}
			for _, t := range ag.Tools {
				s.toolExec.SetAutoApprove(t, true)
			}
		}
	} else {
		// No agent selected — still build context and merge into a system message
		contextMsg := s.buildContext(req)
		if contextMsg != "" {
			req.Messages = append([]provider.Message{{Role: "system", Content: contextMsg}}, req.Messages...)
		}
	}

	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			risk := sandbox.DetectPromptInjection(req.Messages[i].Content)
			if risk.Detected {
				s.emitFn("ai:stream:injection_warning", risk)
			}
			req.Messages[i].Content = sandbox.SanitizeUserInput(req.Messages[i].Content)
			break
		}
	}

	if len(req.Attachments) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				images := make([]provider.ImageContent, 0, len(req.Attachments))
				for _, att := range req.Attachments {
					if att.Type == "image" {
						img := provider.ImageContent{Type: "image"}
						if att.URL != "" {
							img.URL = att.URL
						} else if att.Data != "" {
							img.Data = att.Data
							img.MediaType = att.MimeType
							if img.MediaType == "" {
								img.MediaType = "image/png"
							}
						}
						if img.URL != "" || img.Data != "" {
							images = append(images, img)
						}
					}
				}
				if len(images) > 0 {
					req.Messages[i].Images = images
				}
				break
			}
		}
	}

	modelCtxWindow := s.contextWindow(req.ProviderID, req.Model)
	maxContextTokens := int(float64(modelCtxWindow) * 0.8)
	compressed, didSummarize := s.compress(req.Messages, maxContextTokens, req.ProviderID, req.ConversationID)
	req.Messages = compressed
	if didSummarize {
		s.emitFn("ai:context:summarized", "上下文已自动压缩，旧消息摘要已保留")
	}

	// Token budget warning — check if we're approaching the limit
	totalMsgTokens := 0
	for _, msg := range req.Messages {
		totalMsgTokens += estimateTokensWithModel(msg.Content, req.Model)
	}
	s.totalTokensUsed += totalMsgTokens
	usagePercent := float64(totalMsgTokens) / float64(maxContextTokens) * 100
	if usagePercent > 70 {
		s.emitFn("ai:context:warning", fmt.Sprintf("上下文使用率 %.0f%%，接近上限。建议开始新对话或等待自动压缩。", usagePercent))
	}

	// Start ask_user notification forwarder
	go s.forwardAskUserRequests(agentCtx)

	go s.runAgentLoop(req, agentCtx)

	return nil
}

// Stop cancels the current agent run.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// SetFileHistorySavePath configures disk persistence for undo/redo history.
func (s *Service) SetFileHistorySavePath(dir string) {
	s.fileHistory.SetSavePath(dir)
	// Try to load any existing history from disk
	if s.fileHistory.Load() {
		log.Printf("Loaded file history from %s", dir)
	}
}

// ContinueLoop adds extra iterations to the running agent loop.
// extraLoops is the number of additional loops to allow (e.g. 10 or 20).
func (s *Service) ContinueLoop(extraLoops int) {
	if extraLoops < 1 {
		extraLoops = 10
	}
	select {
	case s.continueCh <- extraLoops:
	default:
	}
}

// Chat performs a non-streaming AI chat.
func (s *Service) Chat(req provider.ChatRequest) (string, error) {
	if req.ProviderID == "" {
		defProvider := s.providerMgr.GetDefaultProvider()
		if defProvider != nil {
			req.ProviderID = defProvider.ID()
		} else {
			return "", fmt.Errorf("no provider configured")
		}
	}

	if req.AgentID != "" {
		ag, ok := s.agentReg.Get(req.AgentID)
		if ok && ag.SystemPrompt != "" {
			systemMsg := provider.Message{Role: "system", Content: ag.SystemPrompt + getLanguageHint(req.Model)}
			req.Messages = append([]provider.Message{systemMsg}, req.Messages...)
		}
	}

	contextMsg := s.buildContext(req)
	if contextMsg != "" {
		req.Messages = append([]provider.Message{{Role: "user", Content: contextMsg}}, req.Messages...)
	}

	resp, err := s.providerMgr.Chat(s.appCtx(), req)
	if err != nil {
		return "", err
	}
	// Record token usage for non-streaming Chat calls
	if s.memoryStore != nil {
		tokensIn := 0
		for _, msg := range req.Messages {
			tokensIn += estimateTokens(msg.Content)
		}
		tokensOut := estimateTokens(resp.Content)
		if tokensIn > 0 || tokensOut > 0 {
			go s.memoryStore.SaveTokenUsage(&memory.TokenUsageEntry{
				ID:             fmt.Sprintf("tu_%d", time.Now().UnixNano()),
				ConversationID: req.ConversationID,
				ProviderID:     req.ProviderID,
				Model:          req.Model,
				TokensIn:       tokensIn,
				TokensOut:      tokensOut,
				Cost:           0,
				CreatedAt:      time.Now().Format(time.RFC3339),
			})
		}
	}
	return resp.Content, nil
}

// Completion performs a code completion request.
func (s *Service) Completion(providerID string, req provider.CompletionRequest) (string, error) {
	resp, err := s.providerMgr.Completion(s.appCtx(), providerID, req)
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}

// buildToolDefinitions builds tool definitions from tool IDs.
func (s *Service) buildToolDefinitions(toolIDs []string) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(toolIDs))
	for _, id := range toolIDs {
		t, ok := s.toolExec.Get(id)
		if !ok {
			continue
		}
		defs = append(defs, provider.ToolDefinition{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        t.ID(),
				Description: t.Description(),
				Parameters:  t.Parameters(),
			},
		})
	}
	return defs
}

// isRepeatedLoop checks if current tool calls match the previous round's.
func (s *Service) isRepeatedLoop(current []provider.ToolCall) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	fps := make([]string, len(current))
	for i, tc := range current {
		fps[i] = tc.Function.Name + ":" + tc.Function.Arguments
	}

	if len(fps) == 0 || len(s.fingerprints) != len(fps) {
		s.fingerprints = fps
		return false
	}

	for i := range fps {
		if fps[i] != s.fingerprints[i] {
			s.fingerprints = fps
			return false
		}
	}
	return true
}

// retryableChatStream performs a ChatStream with retry on transient failures.
func (s *Service) retryableChatStream(roundCtx context.Context, req provider.ChatRequest) (<-chan provider.StreamEvent, error) {
	if !s.cb.Allow() {
		return nil, fmt.Errorf("AI服务断路器已打开，请稍后重试（连续失败%d次后自动恢复）", s.cb.MaxFailures)
	}

	maxRetries := 5
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		eventCh, err := s.providerMgr.ChatStream(roundCtx, req)
		if err == nil {
			return eventCh, nil
		}

		diag := provider.DiagnoseError(err)
		if !diag.Retryable || attempt == maxRetries {
			s.cb.RecordFailure()
			return nil, err
		}

		log.Printf("ChatStream attempt %d failed (retryable): %v, retrying in %v", attempt+1, err, delays[attempt])
		s.emitFn("ai:stream:data", fmt.Sprintf("⏳ %s，第%d次重试（%v后）...\n", diag.Title, attempt+1, delays[attempt]))

		select {
		case <-roundCtx.Done():
			return nil, roundCtx.Err()
		case <-time.After(delays[attempt]):
		}
	}

	s.cb.RecordFailure()
	return nil, fmt.Errorf("重试%d次后仍然失败", maxRetries)
}

// runAgentLoop is the core agent loop that handles tool calls, parallel execution, and loop detection.
func (s *Service) runAgentLoop(req provider.ChatRequest, ctx context.Context) {
	currentReq := req

	var doneMu sync.Mutex
	doneEmitted := false
	setDone := func() {
		doneMu.Lock()
		doneEmitted = true
		doneMu.Unlock()
	}
	isDone := func() bool {
		doneMu.Lock()
		defer doneMu.Unlock()
		return doneEmitted
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("runAgentLoop panic: %v", r)
			s.emitFn("ai:stream:error", fmt.Sprintf("内部错误: %v", r))
			setDone()
		}
		agentTools.SubAgentProgressFn = nil // clear progress callback
		if !isDone() {
			s.emitFn("ai:stream:done", "")
		}
	}()

	// Wire up sub-agent progress reporting to frontend.
	// Sub-agent progress events (ai:stream:data) reset the frontend's 4-minute
	// stream timeout, so long-running sub-agents (up to 22 min) won't trigger
	// false timeout errors as long as each round completes within 4 minutes.
	agentTools.SubAgentProgressFn = func(round, maxRounds int, task string) {
		preview := task
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		s.emitFn("ai:stream:data", fmt.Sprintf("\n🔄 子Agent进度 [%d/%d]: %s\n", round, maxRounds, preview))
	}

	var prevMsgCount int
	var nudgeCount int
	s.progress = newAgentProgress()
	maxLoops := calcMaxAgentLoops(req.Mode, req.Model)
	warningAt := maxLoops - 3
	if warningAt < 1 {
		warningAt = 1
	}

	// Set original goal for anti-drift tracking
	if len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				s.loopState.SetOriginalGoal(req.Messages[i].Content)
				break
			}
		}
	}

	for loop := 0; loop < maxLoops; loop++ {
		select {
		case <-ctx.Done():
			s.emitFn("ai:stream:done", "cancelled")
			setDone()
			return
		case extra := <-s.continueCh:
			maxLoops = loop + extra
			warningAt = maxLoops - 3
			if warningAt < loop+1 {
				warningAt = maxLoops
			}
			s.emitFn("ai:stream:data", fmt.Sprintf("\n\n*已追加%d轮执行，当前上限%d轮。*", extra, maxLoops))
		default:
		}

		// Inject project state (todo list, files touched, decisions) at start of each iteration
		if stateSummary := s.loopState.ProjectStateSummary(); stateSummary != "" {
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role: "system", Content: stateSummary,
			})
		}

		// Anti-drift: re-inject original goal every 10 rounds or when stagnating
		if loop > 0 && loop%10 == 0 {
			if reminder := s.loopState.AntiDriftReminder(); reminder != "" {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role: "system", Content: reminder,
				})
			}
		}
		if s.loopState.IsStagnant(5) {
			if reminder := s.loopState.AntiDriftReminder(); reminder != "" {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role: "system", Content: reminder,
				})
			}
		}

		// Context pruning: if messages exceed threshold, keep system + last N messages
		if len(currentReq.Messages) > 80 {
			currentReq.Messages = pruneMessages(currentReq.Messages, 60)
		}

		roundCtx, roundCancel := context.WithTimeout(ctx, 300*time.Second)
		eventCh, err := s.retryableChatStream(roundCtx, currentReq)
		if err != nil {
			s.cb.RecordFailure()
			userMsg := provider.ClassifyProviderError(err)
			if userMsg == "" {
				userMsg = err.Error()
			}
			s.emitFn("ai:stream:error", userMsg)
			roundCancel()
			setDone()
			return
		}

		var assistantContent string
		var reasoningContent string
		var accumulatedExtra map[string]json.RawMessage
		var toolCalls []provider.ToolCall
		toolCallsSeen := false
		streamReceivedAny := false
		streamRetryCount := 0
		streamInterrupted := false

		var streamUsage *provider.TokenUsage

		// Streaming-level repetition detection thresholds
		const maxTextWithoutTools = 1500    // Max chars of text before forcing tool call (lowered from 3000)
		const repetitionCheckInterval = 200 // Check for repetition every N chars (lowered from 500)
		const repetitionThreshold = 2       // Trigger after 2 repetitions (lowered from 3)

		for event := range eventCh {
			streamReceivedAny = true
			switch event.Type {
			case "data":
				assistantContent += event.Content

				// Streaming-level interrupt: detect planning loops and repetition
				if !toolCallsSeen && !streamInterrupted {
					// Check 1: Text length threshold - model is outputting too much without tools
					if len(assistantContent) > maxTextWithoutTools {
						streamInterrupted = true
						s.emitFn("ai:stream:data", "\n\n*[系统: 检测到模型输出大量文本但未调用工具，正在中断并重新引导...]*")
						roundCancel()
						break
					}

					// Check 2: Repetition detection - same phrases appearing multiple times
					if len(assistantContent) > repetitionCheckInterval {
						if detectTextRepetitionN(assistantContent, repetitionThreshold) {
							streamInterrupted = true
							s.emitFn("ai:stream:data", "\n\n*[系统: 检测到重复输出，正在中断并重新引导...]*")
							roundCancel()
							break
						}
					}
				}

				s.emitFn("ai:stream:data", event.Content)
			case "thinking":
				reasoningContent += event.Content
				s.emitFn("ai:stream:thinking", event.Content)
			case "extra":
				if accumulatedExtra == nil {
					accumulatedExtra = make(map[string]json.RawMessage)
				}
				for k, v := range event.Extra {
					if prev, ok := accumulatedExtra[k]; ok {
						// Streamed string fields arrive incrementally — concatenate.
						var prevStr, newStr string
						if json.Unmarshal(prev, &prevStr) == nil && json.Unmarshal(v, &newStr) == nil {
							merged, _ := json.Marshal(prevStr + newStr)
							accumulatedExtra[k] = merged
							continue
						}
					}
					accumulatedExtra[k] = v
				}
			case "error":
				userMsg := provider.ClassifyProviderError(fmt.Errorf("%s", event.Content))
				if userMsg == "" {
					userMsg = event.Content
				}
				if streamRetryCount < 1 {
					streamRetryCount++
					s.emitFn("ai:stream:data", map[string]any{"content": "\n⚠️ 流式传输中断，正在重试...\n"})
					roundCancel()
					retryEventCh, retryErr := s.retryableChatStream(ctx, currentReq)
					if retryErr == nil && retryEventCh != nil {
						eventCh = retryEventCh
						continue
					}
				}
				s.emitFn("ai:stream:error", userMsg)
				roundCancel()
				setDone()
				return
			case "tool_call":
				if len(event.ToolCalls) > 0 {
					toolCalls = append(toolCalls, event.ToolCalls...)
					toolCallsSeen = true
				} else if event.Name != "" {
					toolCalls = append(toolCalls, provider.ToolCall{
						ID:   fmt.Sprintf("tc_%d", time.Now().UnixNano()),
						Type: "function",
						Function: provider.ToolCallFunc{
							Name:      event.Name,
							Arguments: event.Args,
						},
					})
					toolCallsSeen = true
				}
			case "done":
				// Capture usage from stream
				if event.Usage != nil {
					streamUsage = event.Usage
				}
				// Stream completed normally. If no tool calls and no content,
				// try a non-streaming fallback. Otherwise just mark for exit.
				if !toolCallsSeen && assistantContent == "" {
					noToolReq := currentReq
					noToolReq.Tools = nil
					noToolReq.Stream = false
					fbCtx, fbCancel := context.WithTimeout(ctx, 45*time.Second)
					resp, chatErr := s.providerMgr.Chat(fbCtx, noToolReq)
					fbCancel()
					if chatErr == nil && resp != nil && resp.Content != "" {
						assistantContent = resp.Content
						s.emitFn("ai:stream:data", resp.Content)
					}
					// Whether fallback succeeded or not, we'll exit after this
					// (the post-loop no-tool-calls check handles it)
				}
			}
		}

		// If stream was interrupted due to repetition or length threshold,
		// force tool-call nudge immediately without further processing
		if streamInterrupted && !toolCallsSeen {
			s.progress.recordEmptyRound()
			nudgeCount++
			nudgeContent := "你的输出被中断了，因为检测到重复内容或过长文本但未调用工具。你必须立即使用工具开始工作，不要输出规划性文本。"
			if s.loopState.GetOriginalGoal() != "" {
				nudgeContent += fmt.Sprintf("\n原始目标: %s\n", s.loopState.GetOriginalGoal())
			}
			if intent := s.loopState.GetDetectedIntent(); intent != nil {
				if suggestion := s.toolRouter.SuggestTools(s.loopState.GetOriginalGoal()); suggestion != nil {
					nudgeContent += fmt.Sprintf("\n建议使用的工具: %s\n%s", suggestion.PrimaryTool, suggestion.Hint)
				}
			}
			nudgeContent += buildToolUsageHint(currentReq.Tools, "")
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "assistant",
				Content: assistantContent,
			})
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "user",
				Content: nudgeContent,
			})
			continue
		}

		// Record token usage for THIS round only (delta, not cumulative history)
		if s.memoryStore != nil && streamReceivedAny {
			var tokensIn, tokensOut int
			var cachedTokens int

			if streamUsage != nil && streamUsage.PromptTokens > 0 {
				// Use actual API usage data
				tokensIn = streamUsage.PromptTokens
				tokensOut = streamUsage.CompletionTokens
				cachedTokens = streamUsage.CachedTokens
			} else {
				// Fall back to estimation
				currentMsgs := currentReq.Messages
				for i := prevMsgCount; i < len(currentMsgs) && i >= 0; i++ {
					tokensIn += estimateTokensWithModel(currentMsgs[i].Content, currentReq.Model)
				}
				tokensOut = estimateTokensWithModel(assistantContent, currentReq.Model)
			}
			prevMsgCount = len(currentReq.Messages)

			if tokensIn > 0 || tokensOut > 0 {
				cost := provider.CalculateCost(currentReq.Model, &provider.TokenUsage{
					PromptTokens:     tokensIn,
					CompletionTokens: tokensOut,
					CachedTokens:     cachedTokens,
				})
				cacheSavings := provider.CalculateCacheSavings(currentReq.Model, &provider.TokenUsage{
					PromptTokens: tokensIn,
					CachedTokens: cachedTokens,
				})

				usageEntry := &memory.TokenUsageEntry{
					ID:             fmt.Sprintf("tu_%d", time.Now().UnixNano()),
					ConversationID: currentReq.ConversationID,
					ProviderID:     currentReq.ProviderID,
					Model:          currentReq.Model,
					TokensIn:       tokensIn,
					TokensOut:      tokensOut,
					Cost:           cost,
					CreatedAt:      time.Now().Format(time.RFC3339),
				}
				_ = cacheSavings // TODO: store in entry when schema is extended
				go s.memoryStore.SaveTokenUsage(usageEntry)
			}
		}

		roundCancel()

		if !streamReceivedAny {
			s.cb.RecordFailure()
			s.emitFn("ai:stream:error", provider.T("no_response"))
			setDone()
			return
		}

		// If no function-calling tool calls, try parsing text-based tool calls
		if !toolCallsSeen && assistantContent != "" {
			textCalls := parseTextToolCalls(assistantContent)
			if len(textCalls) > 0 {
				toolCalls = append(toolCalls, textCalls...)
				toolCallsSeen = true
			}
		}

		if !toolCallsSeen {
			s.cb.RecordSuccess()
			if assistantContent == "" {
				fallbackCtx, fbCancel := context.WithTimeout(ctx, 60*time.Second)
				noToolReq := currentReq
				noToolReq.Tools = nil
				noToolReq.Stream = false
				resp, chatErr := s.providerMgr.Chat(fallbackCtx, noToolReq)
				fbCancel()
				if chatErr == nil && resp != nil && resp.Content != "" {
					assistantContent = resp.Content
					s.emitFn("ai:stream:data", resp.Content)
				} else {
					s.emitFn("ai:stream:error", provider.T("no_content"))
					setDone()
					return
				}
			}

			if req.Mode == "build" || req.Mode == "plan" {
				// Check if this is a non-technical exchange (greeting, simple question)
				// that doesn't require tool calls — exit immediately
				isNonTechnical := false
				lastUserMsg := ""
				for i := len(currentReq.Messages) - 1; i >= 0; i-- {
					if currentReq.Messages[i].Role == "user" {
						lastUserMsg = strings.ToLower(currentReq.Messages[i].Content)
						break
					}
				}
				nonTechnicalPatterns := []string{
					"你好", "hello", "hi ", "嗨", "hey",
					"谢谢", "thanks", "thank you",
				}
				for _, pattern := range nonTechnicalPatterns {
					if strings.Contains(lastUserMsg, pattern) {
						isNonTechnical = true
						break
					}
				}
				if isNonTechnical {
					donePayload := map[string]interface{}{}
					if streamUsage != nil {
						donePayload["usage"] = streamUsage
					}
					s.emitFn("ai:stream:done", donePayload)
					setDone()
					return
				}

				// If no tool calls were made, check if AI declared task done or nudge it
				if !toolCallsSeen && assistantContent != "" {
					s.progress.recordEmptyRound()

					// Check if AI explicitly declared task completion — stop immediately
					lowerContent := strings.ToLower(assistantContent)
					explicitDone := strings.Contains(lowerContent, "任务已完成") ||
						strings.Contains(lowerContent, "all tasks completed") ||
						strings.Contains(lowerContent, "所有任务已完成")
					if explicitDone {
						donePayload := map[string]interface{}{}
						if streamUsage != nil {
							donePayload["usage"] = streamUsage
						}
						s.emitFn("ai:stream:done", donePayload)
						setDone()
						return
					}

					maxNudges := s.progress.calculateMaxNudges()
					// If model keeps outputting text without tools for 2+ rounds, stop immediately
					// This model likely doesn't support function calling
					if nudgeCount >= 2 {
						s.emitFn("ai:stream:data", "\n\n*[系统: 模型连续未调用工具，可能不支持 function calling。建议切换到 GPT-4、Claude 或 Gemini 等支持工具调用的模型。]*")
						donePayload := map[string]interface{}{}
						if streamUsage != nil {
							donePayload["usage"] = streamUsage
						}
						s.emitFn("ai:stream:done", donePayload)
						setDone()
						return
					}
					if nudgeCount < maxNudges {
						nudgeCount++

						// Build tool usage hint for models that don't generate tool_calls
						toolHint := buildToolUsageHint(currentReq.Tools, assistantContent)

						// Build adaptive nudge based on what's been tried
						nudgeContent := fmt.Sprintf("你的回复中没有调用工具。你必须使用工具来完成任务，而不是只输出文字。")
						if s.loopState.GetOriginalGoal() != "" {
							nudgeContent += fmt.Sprintf("\n原始目标: %s\n", s.loopState.GetOriginalGoal())
						}
						if files := s.loopState.GetFilesTouched(); len(files) > 0 {
							nudgeContent += fmt.Sprintf("已修改 %d 个文件: %s\n", len(files), strings.Join(files, ", "))
						}
						nudgeContent += toolHint

						if intent := s.loopState.GetDetectedIntent(); intent != nil {
							if suggestion := s.toolRouter.SuggestTools(s.loopState.GetOriginalGoal()); suggestion != nil {
								nudgeContent += fmt.Sprintf("\n建议使用的工具: %s\n%s", suggestion.PrimaryTool, suggestion.Hint)
							}
						}

						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "assistant",
							Content: assistantContent,
						})
						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "user",
							Content: nudgeContent,
						})
						s.emitFn("ai:stream:data", "\n\n*[系统: 等待工具调用...]*")
						continue
					}
					// Nudge exhausted — emit done with warning
					s.emitFn("ai:stream:data", "\n\n*[系统: 已达到最大提示次数，请手动发送「继续」以继续执行]*")
					donePayload := map[string]interface{}{}
					if streamUsage != nil {
						donePayload["usage"] = streamUsage
					}
					s.emitFn("ai:stream:done", donePayload)
					setDone()
					return
				}

				donePayload := map[string]interface{}{}
				if streamUsage != nil {
					donePayload["usage"] = streamUsage
				}
				s.emitFn("ai:stream:done", donePayload)
				setDone()

				// Learn user preferences from this conversation (async, non-blocking)
				if s.memoryStore != nil && req.ProjectPath != "" {
					go s.learnPreferences(req, assistantContent, toolCalls)
				}

				return
			}

			assistantMsg := provider.Message{Role: "assistant", Content: assistantContent, Extra: accumulatedExtra}
			if len(toolCalls) > 0 {
				assistantMsg.ToolCalls = toolCalls
			}
			currentReq.Messages = append(currentReq.Messages, assistantMsg)

			// All tools execute automatically — mode already controls which tools are available.
			s.progress.resetEmptyRounds()
			var calls []agent.ToolCall
			for _, tc := range toolCalls {
				call := agent.ToolCall{
					ID: tc.ID, Name: tc.Function.Name, Args: make(map[string]any),
				}
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &call.Args)
				}
				calls = append(calls, call)
			}

			agentTools.SetSubAgentProviderID(currentReq.ProviderID)

			// Inject parent agent context into sub-agents so they know what's been done
			if parentSummary := s.loopState.ProjectStateSummary(); parentSummary != "" {
				agentTools.SubAgentParentContext = parentSummary
			}

			// Record tool calls for repetition detection
			for _, call := range calls {
				s.loopState.RecordToolCall(call.Name, call.Args, loop+1)
			}

			if len(calls) > 0 {
				type toolRes struct {
					call   agent.ToolCall
					result *agent.ToolResult
					err    error
				}
				ch := make(chan toolRes, len(calls))
				for _, call := range calls {
					emitData := map[string]any{
						"id": call.ID, "name": call.Name, "args": call.Args, "loop": loop + 1,
					}
					if meta := extractFileMeta(call.Name, call.Args); meta != nil {
						emitData["fileMeta"] = meta
					}
					s.emitFn("ai:stream:tool_call", emitData)
					// Check if this tool needs approval
					needsApproval := false
					if t, ok := s.toolExec.Get(call.Name); ok {
						needsApproval = t.RequiresApproval() && !s.toolExec.IsAutoApproved(call.Name)
					}
					if needsApproval {
						s.emitFn("ai:stream:tool_approval", map[string]any{
							"id": call.ID, "name": call.Name, "args": call.Args,
						})
					}
					go func(c agent.ToolCall) {
						timeout := 60 * time.Second
						if c.Name == "ask_user" {
							timeout = 6 * time.Minute
						}
						if t, ok := s.toolExec.Get(c.Name); ok && t.RequiresApproval() {
							timeout = 6 * time.Minute // allow time for user to approve
						}
						r, e := executeToolWithTimeout(ctx, s.toolExec, c, timeout)
						ch <- toolRes{c, r, e}
					}(call)
				}
				toolFailures := 0
				for i := 0; i < len(calls); i++ {
					tr := <-ch
					if tr.err != nil {
						toolFailures++
						classified := agent.ClassifyToolError(tr.call.Name, tr.err)
						s.loopState.RecordToolFailure(tr.call.Name)
						errData := map[string]string{"callId": tr.call.ID, "name": tr.call.Name, "error": tr.err.Error(), "category": classified.Category}
						s.emitFn("ai:stream:tool_result", errData)
						errorMsg := agent.FormatClassifiedError(classified)
						currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: errorMsg, ToolCallID: tr.call.ID, Name: tr.call.Name})
					} else {
						if tr.result.FileMeta != nil {
							resultMap := map[string]any{
								"callId":   tr.result.CallID,
								"name":     tr.result.Name,
								"result":   tr.result.Result,
								"fileMeta": tr.result.FileMeta,
							}
							s.emitFn("ai:stream:tool_result", resultMap)
						} else {
							s.emitFn("ai:stream:tool_result", tr.result)
						}
						rc := tr.result.Result
						estimatedUsed := estimateContextUsed(currentReq.Messages)
						budget := calcToolResultBudget(estimatedUsed, 100000)
						rc = smartTruncateToolResult(tr.call.Name, rc, budget)
						currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: rc, ToolCallID: tr.call.ID, Name: tr.call.Name})
						// Track progress
						filePath := ""
						if tr.result.FileMeta != nil {
							filePath = tr.result.FileMeta.FilePath
						}
						s.progress.recordToolCall(tr.call.Name, true, filePath)
						s.loopState.ResetToolFailure(tr.call.Name)
					}
				}

				// Circuit breaker: if ALL tool calls in this round failed, record a failure
				if toolFailures > 0 && toolFailures == len(calls) {
					s.cb.RecordFailure()
				} else if len(calls) > 0 {
					s.cb.RecordSuccess()
				}

				for _, call := range calls {
					if failures := s.loopState.GetConsecutiveFailures(call.Name); failures >= 3 {
						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "system",
							Content: fmt.Sprintf("工具 %s 已连续失败 %d 次。请换一种方法完成任务，或向用户说明困难。", call.Name, failures),
						})
						break
					}
				}
			}
		}

		// Track files touched by write/edit/move/delete tools
		var modifiedFiles []string
		for _, tc := range toolCalls {
			switch tc.Function.Name {
			case "write_file", "edit_file", "create_directory":
				var args map[string]any
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				if path, ok := args["path"].(string); ok && path != "" {
					s.loopState.AddFileTouched(path)
					modifiedFiles = append(modifiedFiles, path)
				}
			case "move_file":
				var args map[string]any
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				if dest, ok := args["dest"].(string); ok && dest != "" {
					s.loopState.AddFileTouched(dest)
					modifiedFiles = append(modifiedFiles, dest)
				}
			case "delete_file":
				var args map[string]any
				if tc.Function.Arguments != "" {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				if path, ok := args["path"].(string); ok && path != "" {
					s.loopState.AddFileTouched(path)
					modifiedFiles = append(modifiedFiles, path)
				}
			}
		}

		// Auto-verify after file modifications in build mode
		if len(modifiedFiles) > 0 && s.verifyFn != nil && req.Mode == "build" {
			verifyCtx, verifyCancel := context.WithTimeout(ctx, 60*time.Second)
			verifySummary := s.verifyFn(verifyCtx, modifiedFiles)
			verifyCancel()
			if verifySummary != "" {
				s.emitFn("ai:stream:verify", map[string]any{
					"files":   modifiedFiles,
					"summary": verifySummary,
				})
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role:    "system",
					Content: fmt.Sprintf("[自动验证结果]\n%s\n如果验证失败，请分析错误原因并修复代码。", verifySummary),
				})
			}
		}

		// Record round for stagnation tracking
		s.loopState.RecordRound(len(toolCalls), len(modifiedFiles))

		// Semantic repetition detection: if agent is repeating the same approach
		if len(toolCalls) > 0 && s.loopState.CheckSemanticRepetition(3) {
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: "你似乎在重复相同的操作但没有取得进展。请回顾原始目标，尝试完全不同的方法：换一个文件、换一种搜索策略、或者先停下来分析为什么当前方法无效。",
			})
		}

		// Stagnation alert: no progress for 5+ rounds
		if s.loopState.IsStagnant(5) {
			s.emitFn("ai:stream:data", "\n\n*[系统: 已连续5轮无实质进展，请调整策略]*")
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: "⚠️ 已连续5轮无实质进展（无工具调用或文件修改）。请：1) 回顾原始目标；2) 分析当前卡在哪里；3) 尝试完全不同的方法；4) 如果无法继续，总结已完成的工作并结束。",
			})
		}

		if loop == 10 && s.progress != nil && len(s.progress.filesModified) > 8 {
			s.emitFn("ai:task:complexity_warning", map[string]any{
				"message":       "任务涉及大量文件修改，建议拆分为多个子任务以提高效率",
				"filesModified": len(s.progress.filesModified),
			})
		}

		if assistantContent == "" && len(toolCalls) > 0 {
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: "你调用了工具但没有解释在做什么。请在工具调用前说明你的推理，简要总结你做了什么以及为什么。",
			})
		}

		if len(toolCalls) > 0 && assistantContent == "" && s.isRepeatedLoop(toolCalls) {
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: "你似乎陷入了重复模式。请回顾你的目标，尝试不同的方式：读取其他文件、换一个搜索策略、或者先分析当前进展再决定下一步。",
			})
		}

		// Approaching loop limit — nudge AI to wrap up
		if loop+1 >= warningAt && loop+1 < maxLoops {
			remaining := maxLoops - loop - 1
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: fmt.Sprintf("[系统提醒] 还剩%d轮执行。如果任务尚未完成，请：1) 总结当前进度和已完成的变更；2) 说明剩余工作；3) 尽快完成最关键的操作。如果无法完成，给出清晰的续接指引。", remaining),
			})
		}

		// Auto-continue when loop limit reached (up to 3 times)
		if loop+1 >= maxLoops && s.autoContinueCount < 3 {
			s.autoContinueCount++
			extra := 20
			maxLoops = maxLoops + extra
			warningAt = maxLoops - 3
			s.emitFn("ai:stream:data", fmt.Sprintf("\n\n*循环达到上限，自动追加%d轮继续执行（第%d次自动继续，最多3次）*", extra, s.autoContinueCount))
		}
	}

	// Agent loop exhausted — save progress and notify frontend
	progressSummary := s.loopState.ProjectStateSummary()
	if progressSummary != "" && s.memoryStore != nil && req.ProjectPath != "" {
		s.memoryStore.SaveKnowledge(&memory.Knowledge{
			ID:          fmt.Sprintf("loop_progress_%d", time.Now().UnixNano()),
			ProjectPath: req.ProjectPath,
			Category:    "progress",
			Key:         "loop_exhausted_progress",
			Value:       progressSummary,
			Source:      "auto",
			UpdatedAt:   time.Now().Format(time.RFC3339),
		})
	}

	s.emitFn("ai:stream:loop_exhausted", map[string]any{
		"maxLoops":      maxLoops,
		"mode":          req.Mode,
		"progress":      progressSummary,
		"autoContinued": s.autoContinueCount,
	})
	s.emitFn("ai:stream:done", "")
	setDone()
}

// learnPreferences analyzes the conversation and saves observed user preferences.
func (s *Service) learnPreferences(req provider.ChatRequest, assistantContent string, toolCalls []provider.ToolCall) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("learnPreferences panic: %v", r)
		}
	}()

	// Observe coding style preferences from tool arguments
	for _, tc := range toolCalls {
		if tc.Function.Name == "write_file" || tc.Function.Name == "edit_file" {
			// Check if user uses specific formatting patterns
			if args, err := parseToolArgs(tc.Function.Arguments); err == nil {
				if content, ok := args["content"].(string); ok {
					// Detect indentation preference
					if strings.Contains(content, "\t") {
						s.memoryStore.LearnPreference(req.ProjectPath, "indentation", "tabs", "observed")
					} else if strings.Contains(content, "    ") {
						s.memoryStore.LearnPreference(req.ProjectPath, "indentation", "4 spaces", "observed")
					}
					// Detect line ending preference
					if strings.Contains(content, "\r\n") {
						s.memoryStore.LearnPreference(req.ProjectPath, "line_endings", "crlf", "observed")
					}
				}
			}
		}
	}

	// Observe project structure patterns from the assistant's response
	if assistantContent != "" {
		// Detect if the project uses specific frameworks
		frameworks := []string{"react", "vue", "svelte", "angular", "next", "nuxt", "express", "fastapi", "gin", "echo"}
		for _, fw := range frameworks {
			if strings.Contains(strings.ToLower(assistantContent), fw) {
				s.memoryStore.LearnPattern(req.ProjectPath, "framework:"+fw, "Detected in conversation")
			}
		}
	}
}

// parseToolArgs parses tool call arguments from JSON string.
func parseToolArgs(argsStr string) (map[string]any, error) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsStr), &args); err != nil {
		return nil, err
	}
	return args, nil
}

// forwardAskUserRequests monitors the ask_user notification channel and
// emits events to the Wails frontend so the user sees the question.
func (s *Service) forwardAskUserRequests(ctx context.Context) {
	ch := agentTools.PollAskUserRequests()
	for {
		select {
		case req := <-ch:
			s.emitFn("ai:stream:ask_user", req)
		case <-ctx.Done():
			// Drain remaining items to prevent stale requests from surfacing in the next conversation
			for {
				select {
				case <-ch:
				default:
					return
				}
			}
		}
	}
}

// RespondToAsk is called from app.go (Wails) when the user answers an ask_user prompt.
func (s *Service) RespondToAsk(response agentTools.AskUserResponse) bool {
	return agentTools.AskUserReg.Respond(response)
}

// extractFileMeta extracts file operation metadata from tool call args.
func extractFileMeta(name string, args map[string]any) *agent.FileMeta {
	switch name {
	case "read_file":
		path, _ := args["path"].(string)
		if path == "" {
			return nil
		}
		return &agent.FileMeta{Operation: "read", FilePath: path}
	case "write_file":
		path, _ := args["path"].(string)
		if path == "" {
			return nil
		}
		return &agent.FileMeta{Operation: "write", FilePath: path}
	case "edit_file":
		path, _ := args["path"].(string)
		if path == "" {
			return nil
		}
		return &agent.FileMeta{Operation: "edit", FilePath: path}
	case "search_files":
		path, _ := args["path"].(string)
		query, _ := args["query"].(string)
		return &agent.FileMeta{Operation: "search", FilePath: path, Summary: query}
	case "glob_files":
		pattern, _ := args["pattern"].(string)
		path, _ := args["path"].(string)
		return &agent.FileMeta{Operation: "glob", FilePath: path, Summary: pattern}
	case "execute_command":
		cmd, _ := args["command"].(string)
		if len(cmd) > 60 {
			cmd = cmd[:60] + "..."
		}
		return &agent.FileMeta{Operation: "exec", Summary: cmd}
	default:
		return nil
	}
}

// UndoFileChange reverts the last file change.
func (s *Service) UndoFileChange() (string, error) {
	change, err := s.fileHistory.Undo()
	if err != nil {
		return "", err
	}
	if change == nil {
		return "", nil
	}
	return fmt.Sprintf("Reverted %s (%s)", change.FilePath, change.Description), nil
}

// RedoFileChange re-applies the last undone file change.
func (s *Service) RedoFileChange() (string, error) {
	change, err := s.fileHistory.Redo()
	if err != nil {
		return "", err
	}
	if change == nil {
		return "", nil
	}
	return fmt.Sprintf("Re-applied %s (%s)", change.FilePath, change.Description), nil
}

// CanUndoFileChange returns whether undo is possible.
func (s *Service) CanUndoFileChange() bool {
	return s.fileHistory.CanUndo()
}

// CanRedoFileChange returns whether redo is possible.
func (s *Service) CanRedoFileChange() bool {
	return s.fileHistory.CanRedo()
}

// GetFileHistory returns the file change history.
func (s *Service) GetFileHistory() []agentTools.FileChange {
	return s.fileHistory.GetHistory()
}

// --- Agent Progress Tracking ---

// recordToolCall tracks a tool call for progress analysis.
func (p *AgentProgress) recordToolCall(name string, success bool, filePath string) {
	p.toolCallCount++
	if success {
		p.successfulCalls++
	}
	if filePath != "" {
		p.filesModified[filePath] = true
	}
}

// recordEmptyRound tracks a round with no tool calls.
func (p *AgentProgress) recordEmptyRound() {
	p.consecutiveEmpty++
}

// resetEmptyRounds resets the consecutive empty counter when tools are called.
func (p *AgentProgress) resetEmptyRounds() {
	p.consecutiveEmpty = 0
}

// fileCount returns the number of files modified.
func (p *AgentProgress) fileCount() int {
	return len(p.filesModified)
}

// hasProgress returns true if the agent has made meaningful progress.
func (p *AgentProgress) hasProgress() bool {
	return p.fileCount() > 0 || p.successfulCalls > 3
}

// isStagnant returns true if the agent hasn't made progress in recent rounds.
func (p *AgentProgress) isStagnant() bool {
	return p.consecutiveEmpty >= 3 && !p.hasProgress()
}

// calculateMaxNudges dynamically determines the max nudge count based on task context.
func (p *AgentProgress) calculateMaxNudges() int {
	base := 3

	// More files touched = more complex task = allow more rounds
	if p.fileCount() > 5 {
		base += 2
	} else if p.fileCount() > 2 {
		base += 1
	}

	// More successful tool calls = more complex task
	if p.successfulCalls > 10 {
		base += 2
	} else if p.successfulCalls > 5 {
		base += 1
	}

	// If stagnant (no progress + no empty rounds), reduce limit
	if p.isStagnant() {
		base = 2
	}

	// Cap at reasonable limits
	if base < 2 {
		base = 2
	}
	if base > 10 {
		base = 10
	}

	return base
}

// shouldStop returns true if the agent loop should stop.
func (p *AgentProgress) shouldStop(nudgeCount int) bool {
	maxNudges := p.calculateMaxNudges()
	return nudgeCount >= maxNudges
}

// getProgressSummary returns a human-readable summary of progress.
func (p *AgentProgress) getProgressSummary() string {
	return fmt.Sprintf("files=%d, toolCalls=%d, success=%d, empty=%d",
		p.fileCount(), p.toolCallCount, p.successfulCalls, p.consecutiveEmpty)
}

// pruneMessages keeps system messages + the most recent messages to prevent context overflow.
// System messages are trimmed too: first keepSystemPrefix (stable cache prefix) and
// last keepSystemSuffix (recent state updates) are preserved; middle system messages are dropped.
// This prevents accumulation of outdated compression markers, state summaries, etc.
func pruneMessages(msgs []provider.Message, keepRecent int) []provider.Message {
	if len(msgs) <= keepRecent {
		return msgs
	}

	const keepSystemPrefix = 6 // stable prefix: Rules, Structure, Knowledge, RAG, ContextFiles, ActiveFile
	const keepSystemSuffix = 4 // recent state: project state, anti-drift, compression markers, verify results

	var systemMsgs []provider.Message
	var otherMsgs []provider.Message

	for _, m := range msgs {
		if m.Role == "system" {
			systemMsgs = append(systemMsgs, m)
		} else {
			otherMsgs = append(otherMsgs, m)
		}
	}

	// Keep recent non-system messages
	if len(otherMsgs) > keepRecent {
		otherMsgs = otherMsgs[len(otherMsgs)-keepRecent:]
	}

	// Trim system messages: keep prefix + suffix, drop middle duplicates
	var trimmedSystem []provider.Message
	if len(systemMsgs) > keepSystemPrefix+keepSystemSuffix {
		trimmedSystem = append(trimmedSystem, systemMsgs[:keepSystemPrefix]...)
		// Insert a marker for dropped system messages
		dropped := len(systemMsgs) - keepSystemPrefix - keepSystemSuffix
		trimmedSystem = append(trimmedSystem, provider.Message{
			Role:    "system",
			Content: fmt.Sprintf("[已省略 %d 条中间系统消息以节省上下文]", dropped),
		})
		trimmedSystem = append(trimmedSystem, systemMsgs[len(systemMsgs)-keepSystemSuffix:]...)
	} else {
		trimmedSystem = systemMsgs
	}

	// Reconstruct: system messages first, then recent messages
	result := make([]provider.Message, 0, len(trimmedSystem)+len(otherMsgs)+1)
	result = append(result, trimmedSystem...)

	// Add a summary marker if we pruned messages
	if len(msgs) > keepRecent+len(systemMsgs) {
		result = append(result, provider.Message{
			Role:    "system",
			Content: fmt.Sprintf("[上下文已压缩：保留了最近 %d 条消息]", keepRecent),
		})
	}

	result = append(result, otherMsgs...)
	return result
}

// buildToolUsageHint generates explicit tool calling instructions for all models.
// Includes both function calling format and text-based format for maximum compatibility.
func buildToolUsageHint(tools []provider.ToolDefinition, lastContent string) string {
	if len(tools) == 0 {
		return ""
	}

	var hint strings.Builder
	hint.WriteString("\n## 可用工具\n")
	hint.WriteString("你必须使用工具来完成任务，不要只输出文字描述。\n\n")

	for _, t := range tools {
		hint.WriteString(fmt.Sprintf("### %s\n%s\n", t.Function.Name, t.Function.Description))
		if params, ok := t.Function.Parameters.(agent.ToolParameters); ok {
			if len(params.Required) > 0 {
				hint.WriteString(fmt.Sprintf("必需参数: %s\n", strings.Join(params.Required, ", ")))
			}
		}
		hint.WriteString("\n")
	}

	hint.WriteString("## 调用方式\n")
	hint.WriteString("如果模型支持 function calling，请使用工具调用格式。\n")
	hint.WriteString("如果不支持，请在回复中使用以下格式（每行一个工具调用）：\n")
	hint.WriteString("[TOOL: 工具名 {\"参数名\": \"参数值\"}]\n\n")
	hint.WriteString("示例：\n")
	hint.WriteString("[TOOL: read_file {\"path\": \"main.go\"}]\n")
	hint.WriteString("[TOOL: search_files {\"query\": \"func main\"}]\n")
	hint.WriteString("[TOOL: execute_command {\"command\": \"go build ./...\"}]\n\n")
	hint.WriteString("重要：必须实际调用工具，不要只说\"我来读取\"。\n")

	return hint.String()
}

// parseTextToolCalls extracts tool calls from text output using [TOOL: name {args}] format.
func parseTextToolCalls(content string) []provider.ToolCall {
	var calls []provider.ToolCall
	// Match [TOOL: name {json}] or [TOOL:name {json}]
	re := regexp.MustCompile(`\[TOOL:\s*(\w+)\s*\{([^}]+)\}\]`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		toolName := match[1]
		argsStr := match[2]

		// Try to parse as JSON
		var args map[string]any
		if err := json.Unmarshal([]byte("{"+argsStr+"}"), &args); err != nil {
			// If JSON parsing fails, try key:value format
			args = parseKeyValueArgs(argsStr)
		}

		argsJSON, _ := json.Marshal(args)
		calls = append(calls, provider.ToolCall{
			ID:   fmt.Sprintf("tc_%d", time.Now().UnixNano()),
			Type: "function",
			Function: provider.ToolCallFunc{
				Name:      toolName,
				Arguments: string(argsJSON),
			},
		})
	}
	return calls
}

// parseKeyValueArgs parses "key:value, key2:value2" format into a map.
func parseKeyValueArgs(s string) map[string]any {
	args := make(map[string]any)
	pairs := strings.Split(s, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Remove surrounding quotes
			val = strings.Trim(val, "\"'")
			args[key] = val
		}
	}
	return args
}

// detectTextRepetition checks if the assistant's output contains repetitive phrases.
// Returns true if the same phrase (>=20 chars) appears 3+ times.
func detectTextRepetition(content string) bool {
	return detectTextRepetitionN(content, 3)
}

// detectTextRepetitionN checks if the assistant's output contains repetitive phrases.
// Returns true if the same phrase appears threshold+ times.
// Uses three strategies: exact line match, exact sentence match, and N-gram prefix match.
func detectTextRepetitionN(content string, threshold int) bool {
	if len(content) < 80 {
		return false
	}

	// Strategy 1: Exact line repetition
	lines := strings.Split(content, "\n")
	lineCounts := make(map[string]int)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) >= 10 {
			lineCounts[line]++
			if lineCounts[line] >= threshold {
				return true
			}
		}
	}

	// Strategy 2: Exact sentence repetition (split by punctuation)
	sentences := regexp.MustCompile(`[。！？.!?\n]+`).Split(content, -1)
	sentenceCounts := make(map[string]int)

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) >= 10 {
			sentenceCounts[sent]++
			if sentenceCounts[sent] >= threshold {
				return true
			}
		}
	}

	// Strategy 3: N-gram prefix detection — catches variant phrases
	// e.g., "好的，我来全面审查" and "好的，我来全面检查" share prefix "好的，我来"
	prefixCounts := make(map[string]int)
	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len([]rune(sent)) >= 5 {
			// Use first 5 chars as prefix (catches "好的，我来" variants)
			prefix := string([]rune(sent)[:5])
			prefixCounts[prefix]++
			if prefixCounts[prefix] >= threshold {
				return true
			}
		}
	}

	// Strategy 4: Sliding window N-gram detection
	// Check if any 8-char substring appears threshold+ times
	runes := []rune(content)
	if len(runes) >= 16 {
		ngramCounts := make(map[string]int)
		ngramSize := 8
		for i := 0; i <= len(runes)-ngramSize; i++ {
			ngram := string(runes[i : i+ngramSize])
			ngramCounts[ngram]++
			if ngramCounts[ngram] >= threshold {
				return true
			}
		}
	}

	return false
}
