package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
type Compressor func(msgs []provider.Message, maxTokens int, providerID string) ([]provider.Message, bool)

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
		providerMgr:   providerMgr,
		toolExec:      toolExec,
		memoryStore:   memoryStore,
		agentReg:      agentReg,
		emitFn:        emitFn,
		buildContext:  buildContext,
		compress:      compress,
		contextWindow: contextWindow,
		appCtx:        appCtx,
		loopState:     ls,
		verifyFn:      verifyFn,
		continueCh:    make(chan int, 1),
		cb:            NewCircuitBreaker(10, 60*time.Second),
		sem:           make(chan struct{}, 3),
	}
}

func estimateTokens(text string) int {
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
	// CJK: ~1.5 tokens per char (cl100k base)
	// ASCII words: ~1.3 tokens per word (average English word)
	// Whitespace/punctuation: ~0.5 tokens per char for remaining
	remaining := len(text) - cjk*3 // rough byte adjustment
	if remaining < 0 {
		remaining = 0
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
	return int(float64(cjk)*1.5+float64(asciiWords)*1.3+float64(nonWordChars)*0.4) + 1
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

var languageHint = prompt("language_hint")

// --- 规划模式 ---
// 对标 Cursor Plan Mode：只读分析，输出结构化的实施方案。
const planModePrompt = `
=== 规划模式 ===
你是一个资深软件架构师，负责分析需求并制定精准的实施方案。
你的职责是分析——不是实现。禁止写文件、禁止执行命令。

## 工作流程
1. 广度优先探索：先了解项目整体结构（入口文件、目录树、关键配置），再深入相关模块
2. 逐文件精读：对每个相关文件，理解其职责、接口和依赖关系
3. 对照分析：将用户需求与现有代码对照，找出差距和约束
4. 输出方案：按固定格式输出结构化实施计划

## 输出格式（必须严格遵守）
### 当前状态
基于代码阅读总结的现状（引用文件路径:行号）

### 目标
明确本次要达成的结果

### 实施计划
1. **步骤标题** — 文件: path/to/file:行号 — 复杂度: 低/中/高
   - 具体做什么
   - 为什么这样做
   - 潜在影响和注意事项

### 风险与依赖
- 风险点及缓解措施
- 依赖的其他模块或前提条件

## 规则
- 每次读取文件后必须写出分析发现，不要只列文件名
- 引用代码时使用 文件路径:行号 格式
- 如果用户需求模糊，先列出澄清问题再继续
- 以 --- 规划完成 --- 作为结尾标记
- 此模式下绝对禁止使用 write_file、execute_command 等修改性工具
- 输出完整方案后立即结束，不要等待系统提示
`

// --- 构建模式 ---
// 对标 Claude Code Build Mode：自主编程智能体，完整的工程纪律。
const buildModePrompt = `
=== 构建模式 ===
你是一个拥有完整文件系统访问权限的自主编程智能体。你的目标是精准、安全、高质量地完成每一个编程任务。

**❗ 核心铁律：任务未完成之前绝对不要结束对话。** 实现代码后必须立即运行验证命令。验证失败 → 分析错误原因 → 修复代码 → 重新验证 → 反复循环直到通过。跳过验证步骤直接报告完成是绝对禁止的。只有所有改动验证通过、且用户需求全部实现后，你才能报告"已完成"。

## 执行纪律（每个任务严格遵循）
1. **探索**：先阅读相关文件，理解当前实现。写出你的发现——不要只列文件名
2. **规划**：说明你打算改什么、为什么这样改。对于非简单任务，先列出步骤再动手
3. **实现**：使用工具完成修改。每次改动后简要说明改了什么
4. **验证**：改完代码后必须运行验证命令。Go 项目跑 (go build ./...) 和 (go vet ./...)；前端项目跑 'npm run build'；有测试则跑测试。如果验证失败，分析错误并修复，直到通过
5. **报告**：任务完成后总结所有变更

## 编码规范
- 优先使用 Edit（精确替换）而不是 Write（整体覆写），减少出错范围
- 不要写不必要的注释。代码通过好的命名自解释。只在 WHY 不明显时才加注释（隐蔽约束、特殊 workaround、会让人惊讶的行为）
- 三行相似的代码好过一套过早的抽象。不要为假想的未来需求做设计
- 不要引入半成品。要么完整实现，要么不做
- 删除死代码，不要注释掉或标记 deprecated。如果确信某段代码没用，直接删掉
- 不要做向后兼容的 hack（如重命名后留 _var、re-export 类型、// removed 注释等）

## 安全底线
- 绝不引入命令注入、XSS、SQL 注入等 OWASP 漏洞
- 用户输入和外部 API 数据必须校验和净化
- 文件路径必须防目录穿越
- 不记录敏感数据（密钥、token 等）
- 不硬编码密钥——使用环境变量或配置文件

## 工具使用策略
- 独立操作可以并行调用多个工具，有依赖关系则串联
- 读取文件时一次读取完整内容，不要分段读取同一文件
- 搜索优先用 Grep（ripgrep），不要用 Bash 跑 find/grep
- 需要探索项目结构时一次性发起多个搜索，不要反复询问
- 工具调用超时或失败时分析原因，不要盲目重试
- **子智能体（sub_agent）**：对于需要大量阅读和搜索的独立子任务（如代码审查、安全审计、跨文件追踪），使用 sub_agent 工具派生子智能体去完成。子智能体会独立运行、有自己的上下文窗口，不会污染主对话。把子智能体当成"派一个助手去做专项调研"。主智能体收到子智能体的报告后，根据结果决定下一步。复杂任务应该被分解，主智能体做决策，子智能体做调研。

## 效率原则
- 不要为了修一个小 bug 而重构整个模块
- 不要添加任务范围外的"顺便改"优化（除非是明显的安全问题）
- 一次 commit 做一件事，不要混在一起
- 修改完立即验证，不要等所有改动做完再一起测

## 何时结束
- 所有代码改动已验证通过
- 用户需求已全部实现
- 此时直接输出完成总结，不要等待系统提示
- 如果你认为任务已完成但系统要求继续，确认是否还有遗漏的工作
`

// --- 对话模式 ---
// 对标 Claude Code Chat Mode：只读分析，精准解答。
const chatModePrompt = `
=== 对话模式 ===
你是一个资深软件工程师，正在与开发者进行技术对话。
你的角色是理解问题、阅读代码、给出精准的答案和建议。

## 可用能力
- 读取文件、搜索代码、浏览目录——完整的代码阅读权限
- 禁止写文件、禁止执行命令——你只能读，不能改

## 工作流程
1. 明确问题：确保你理解了用户在问什么。如果不清楚，主动追问
2. 定位代码：用搜索和文件阅读定位相关代码，不要凭记忆猜测
3. 分析回答：基于实际代码给出分析，引用文件路径和行号
4. 给出建议：提供可执行的、具体的方案，说明利弊权衡

## 输出要求
- 展示代码时使用 markdown 代码块，注明文件路径:行号
- 分析结果要有结构：问题 → 原因 → 方案（如果有多个方案，说明各自的优劣）
- 不要给出模糊的建议如"可以优化一下性能"——指出具体哪个函数、怎么优化
- 如果一个问题可以有多种解法，列出主要方案并标注推荐项
- 简洁直接：回答问题的核心，不要铺垫背景知识（除非用户明显需要）

## 何时主动追问
- 用户的描述可以用多种方式理解时，先确认意图
- 修复一个问题的方案会影响其他模块时，先指出让用户决策
- 用户说"改一下 X"但 X 不够具体时，追问细节
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
			systemMsg := provider.Message{Role: "system", Content: ag.SystemPrompt + languageHint + modePrompt}
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
			} else {
				req.Tools = nil
			}
			for _, t := range ag.Tools {
				s.toolExec.SetAutoApprove(t, true)
			}
		}
	}

	contextMsg := s.buildContext(req)
	if contextMsg != "" {
		req.Messages = append([]provider.Message{{Role: "system", Content: contextMsg}}, req.Messages...)
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
	compressed, didSummarize := s.compress(req.Messages, maxContextTokens, req.ProviderID)
	req.Messages = compressed
	if didSummarize {
		s.emitFn("ai:context:summarized", "上下文已自动压缩，旧消息摘要已保留")
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
			systemMsg := provider.Message{Role: "system", Content: ag.SystemPrompt + languageHint}
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

	var doneEmitted bool
	defer func() {
		if r := recover(); r != nil {
			log.Printf("runAgentLoop panic: %v", r)
			s.emitFn("ai:stream:error", fmt.Sprintf("内部错误: %v", r))
			doneEmitted = true
		}
		if !doneEmitted {
			s.emitFn("ai:stream:done", "")
		}
	}()

	var prevMsgCount int
	var nudgeCount int
	const maxNudges = 2
	maxLoops := calcMaxAgentLoops(req.Mode, req.Model)
	warningAt := maxLoops - 3
	if warningAt < 1 {
		warningAt = 1
	}

	for loop := 0; loop < maxLoops; loop++ {
		select {
		case <-ctx.Done():
			s.emitFn("ai:stream:done", "cancelled")
			doneEmitted = true
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
			doneEmitted = true
			return
		}

		var assistantContent string
		var reasoningContent string
		var accumulatedExtra map[string]json.RawMessage
		var toolCalls []provider.ToolCall
		toolCallsSeen := false
		streamReceivedAny := false
		streamRetryCount := 0

		var streamUsage *provider.TokenUsage

		for event := range eventCh {
			streamReceivedAny = true
			switch event.Type {
			case "data":
				assistantContent += event.Content
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
				doneEmitted = true
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
					tokensIn += estimateTokens(currentMsgs[i].Content)
				}
				tokensOut = estimateTokens(assistantContent)
			}
			prevMsgCount = len(currentReq.Messages)

			if tokensIn > 0 || tokensOut > 0 {
				cost := provider.CalculateCost(currentReq.Model, &provider.TokenUsage{
					PromptTokens:     tokensIn,
					CompletionTokens: tokensOut,
					CachedTokens:     cachedTokens,
				})
				cacheSavings := provider.CalculateCacheSavings(currentReq.Model, &provider.TokenUsage{
					PromptTokens:  tokensIn,
					CachedTokens:  cachedTokens,
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
			doneEmitted = true
			return
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
					doneEmitted = true
					return
				}
			}

			if req.Mode == "build" || req.Mode == "plan" {
				// Check if AI seems to have completed its task
				completionSignals := []string{
					"已完成", "完成", "任务完成", "总结", "总结如下", "以上是",
					"done", "completed", "finished", "summary", "in summary",
					"---", "规划完成", "实施计划", "风险与依赖",
				}
				lowerContent := strings.ToLower(assistantContent)
				isComplete := false
				for _, signal := range completionSignals {
					if strings.Contains(lowerContent, signal) {
						isComplete = true
						break
					}
				}

				// Check if this is a non-technical response (greeting, question, etc.)
				// that doesn't require tool calls
				isNonTechnical := false
				lastUserMsg := ""
				for i := len(currentReq.Messages) - 1; i >= 0; i-- {
					if currentReq.Messages[i].Role == "user" {
						lastUserMsg = strings.ToLower(currentReq.Messages[i].Content)
						break
					}
				}
				nonTechnicalPatterns := []string{
					"你好", "hello", "hi", "嗨", "hey",
					"谢谢", "thanks", "thank you",
					"什么是", "怎么理解", "解释一下", "是什么意思",
					"what is", "how does", "explain", "what does",
				}
				for _, pattern := range nonTechnicalPatterns {
					if strings.Contains(lastUserMsg, pattern) {
						isNonTechnical = true
						break
					}
				}

				// Also check if response is short and non-actionable (<200 chars, no file paths)
				isShortResponse := len(assistantContent) < 200 && !strings.Contains(assistantContent, "/") && !strings.Contains(assistantContent, "```")

				// If AI seems to have completed, or this is a non-technical exchange, don't push for tools
				if isComplete || isNonTechnical || isShortResponse {
					donePayload := map[string]interface{}{}
					if streamUsage != nil {
						donePayload["usage"] = streamUsage
					}
					s.emitFn("ai:stream:done", donePayload)
					doneEmitted = true
					return
				}

				// Only nudge if we haven't nudged too many times
				if !toolCallsSeen && assistantContent != "" && nudgeCount < maxNudges {
					nudgeCount++
					toolNames := make([]string, 0, len(currentReq.Tools))
					for _, t := range currentReq.Tools {
						toolNames = append(toolNames, t.Function.Name)
					}
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role:    "assistant",
						Content: assistantContent,
					})
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role:    "user",
						Content: fmt.Sprintf("你的回复中没有调用工具。如果任务尚未完成，请直接调用工具继续执行（可用工具: %s）。如果任务已完成，请在回复中明确说明。", strings.Join(toolNames, ", ")),
					})
					s.emitFn("ai:stream:data", "\n\n*[系统: 等待工具调用...]*")
					continue
				}
			}

			donePayload := map[string]interface{}{}
			if streamUsage != nil {
				donePayload["usage"] = streamUsage
			}
			s.emitFn("ai:stream:done", donePayload)
			doneEmitted = true
			return
		}

		assistantMsg := provider.Message{Role: "assistant", Content: assistantContent, Extra: accumulatedExtra}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}
		currentReq.Messages = append(currentReq.Messages, assistantMsg)

		// All tools execute automatically — mode already controls which tools are available.
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
			for i := 0; i < len(calls); i++ {
				tr := <-ch
				if tr.err != nil {
					errData := map[string]string{"callId": tr.call.ID, "name": tr.call.Name, "error": tr.err.Error()}
					s.emitFn("ai:stream:tool_result", errData)
					currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: fmt.Sprintf("Error: %s", tr.err.Error()), ToolCallID: tr.call.ID, Name: tr.call.Name})
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
					if len(rc) > maxToolResultChars {
						rc = rc[:maxToolResultChars] + "... [truncated]"
					}
					currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: rc, ToolCallID: tr.call.ID, Name: tr.call.Name})
				}
			}
		}

		// Track files touched by write/edit tools
		var modifiedFiles []string
		for _, tc := range toolCalls {
			switch tc.Function.Name {
			case "write_file", "edit_file":
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

		if len(toolCalls) > 0 && assistantContent == "" && s.isRepeatedLoop(toolCalls) {
			currentReq.Messages = append(currentReq.Messages, provider.Message{
				Role:    "system",
				Content: "ä½ ä¼¼ä¹Žé™·å…¥äº†é‡å¤æ¨¡å¼ã€‚è¯·å›žé¡¾ä½ çš„ç›®æ ‡ï¼Œå°è¯•ä¸åŒçš„æ–¹å¼ï¼šè¯»å–å…¶ä»–æ–‡ä»¶ã€æ¢ä¸€ä¸ªæœç´¢ç–¥ç•¥ã€æˆ–è€…å…ˆåˆ†æžå½“å‰è¿›å±•å†å†³å®šä¸‹ä¸€æ¥ã€‚",
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
	doneEmitted = true
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
