package ai

import (
	"context"
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

	// tracer collects full-chain execution events (Phase 1+).
	// Set just before runAgentLoop; may be nil (tracing disabled).
	tracer Tracer

	// traceSink is the persistent storage for traces (Phase 6+).
	// If set, traces are saved to SQLite; otherwise NoopTraceSink is used.
	traceSink TraceSink

	// supervisor is the rule-engine watchdog (Phase 2+).
	// Monitors loop progress and intervenes when patterns warrant.
	supervisor *Supervisor

	// understander is the deeper intent analyzer (Phase 3+).
	// Detects ambiguity, extracts entities, decides on clarification.
	understander *Understander

	// router is the DAG plan executor (Phase 4+).
	// Runs multi-step plans with parallel node execution.
	router *Router
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
	agentTools.SetLoopStateRef(ls)
	s := &Service{
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
		supervisor:       NewSupervisor(),
		understander:     NewUnderstander(),
	}
	// Initialize router after struct creation (needs service references).
	s.router = NewRouter(agentReg, providerMgr, toolExec, memoryStore)
	return s
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
			if len(trimmed) > 50 {
				return false
			}
			lower := strings.ToLower(trimmed)
			techIndicators := []string{"fix", "add", "refactor", "write", "create", "implement", "test", "deploy", "build",
				"修复", "添加", "重构", "写", "创建", "实现", "测试", "部署", "构建", "改", "删", "优化",
				"read", "search", "edit", "analyze", "run", "install", "config", "debug", "compile",
				"读取", "搜索", "修改", "分析", "运行", "安装", "配置", "调试", "编译",
				"问题", "错误", "bug", "报错", "改进", "建议", "审查", "检查"}
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
	if len(trimmed) > 50 {
		return ""
	}
	lower := strings.ToLower(trimmed)
	techIndicators := []string{"fix", "add", "refactor", "write", "create", "implement", "test", "deploy", "build",
		"修复", "添加", "重构", "写", "创建", "实现", "测试", "部署", "构建", "改", "删", "优化",
		"read", "search", "edit", "analyze", "run", "install", "config", "debug", "compile",
		"读取", "搜索", "修改", "分析", "运行", "安装", "配置", "调试", "编译",
		"问题", "错误", "bug", "报错", "改进", "建议", "审查", "检查"}
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
// 对标 Claude Code Build Mode：自主编程智能体。
// 工具是按需使用的能力，不是每条回复的强制要求。
const buildModePrompt = `
=== 构建模式 ===
你是自主编程智能体。你可以调用工具来完成编码任务，也可以纯文字回答。

## 何时使用工具
- 用户要求修改/创建/删除代码 → 调用 read_file / edit_file / write_file 等
- 用户要求运行测试/构建 → 调用 execute_command
- 用户询问具体代码位置 → 调用 search_files / glob_files

## 何时纯文字回答即可
- 问候、聊天 → 直接回答
- 询问能力范围 → 直接列举
- 概念解释 → 直接说明
- 简单确认 → 简短回复

## 回复原则
- 直接行动，不要输出前言（"好的，我来..."、"让我分析..."）
- 不要重复内容
- 不要使用 bash 命令（find, grep, cat, ls, cd 等），用专用工具代替

## 工具调用格式
如果模型支持 function calling，使用工具调用格式。
如果不支持，使用文本格式：[TOOL: tool_name {"param": "value"}]
每条调用单独一行。
`

// --- 规划模式 ---
// 对标 Cursor Plan Mode：分析并输出结构化方案，按需使用工具。
const planModePrompt = `
=== 规划模式 ===
你负责分析项目并输出结构化实施方案。工具辅助你的分析，但不强制使用。

## 何时使用工具
- 需要了解具体代码内容 → read_file
- 需要查找文件 → glob_files / search_files
- 需要验证假设 → 相关只读工具

## 何时不需要工具
- 用户仅要求概述/思路 → 直接文字回答
- 概念解释 → 直接说明

## 回复原则
- 直接输出分析，不要前言
- 不要使用 bash 命令，用专用工具代替
- 分析完成后自然结束

## 工具调用格式
如果模型支持 function calling，使用工具调用格式。
如果不支持，使用文本格式：[TOOL: tool_name {"param": "value"}]
注意：禁止使用 write_file 和 execute_command。
`

// --- 构建模式（简单消息）---
// 用于问候、感谢等非技术消息 — 简短直接回复，不需要工具。
const buildModePromptSimple = `
=== 构建模式 ===
用户发送了简单的问候或非技术消息。直接简短回复，不需要调用任何工具。
`

// --- 规划模式（简单消息）---
const planModePromptSimple = `
=== 规划模式 ===
用户发送了简单的问候或非技术消息。直接简短回复，不需要调用任何工具。
`

// --- 对话模式 ---
// 对标 Claude Code Chat Mode：只读分析，精准解答。
// 注意：Chat 模式以对话为主，不强制要求每条回复都调用工具。
// 只有用户明确要求操作文件/代码时才需要工具调用。
const chatModePrompt = `
=== 对话模式 ===
你可以通过工具辅助回答，但对于一般性问题（问候、介绍、概念解释等），直接文字回答即可。

## 何时使用工具
- 用户询问特定文件内容 → 用 read_file 读取再回答
- 用户询问代码位置 → 用 search_files 或 glob_files 查找
- 用户要求修改代码 → 用 edit_file 或 write_file
- 用户要求执行命令 → 用 execute_command

## 何时不需要工具
- 问候、聊天 → 直接回答
- 询问能力范围 → 直接列举
- 概念解释 → 直接说明
- 简单确认 → 简短回复

## 回复风格
- 简洁直接，不要输出前言式文本
- 不要列出"我将采取的步骤"再行动
- 如果需要调用工具，直接调用

## 工具调用格式
如果模型支持 function calling，使用工具调用格式。
如果不支持，在回复中使用以下格式：
[TOOL: read_file {"path": "文件路径"}]
[TOOL: search_files {"query": "搜索内容"}]
[TOOL: glob_files {"pattern": "**/*.go"}]
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
		if s.supervisor != nil {
			s.supervisor.Reset()
		}
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
	var understanding *Understanding
	if len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if req.Messages[i].Role == "user" {
				lastMsg := req.Messages[i].Content
				// Phase 3+: Use Understander for deeper analysis
				if s.understander != nil {
					understanding = s.understander.Understand(lastMsg)
					intent = &agent.IntentResult{
						Intent:     understanding.Intent,
						Confidence: understanding.Confidence,
						Keywords:   understanding.Keywords,
						Language:   understanding.Language,
					}
				} else {
					intent = s.intentClassifier.Classify(lastMsg)
				}
				break
			}
		}
	}

	if intent != nil {
		s.loopState.SetDetectedIntent(intent)
	}

	// Phase 3+: Understander result event + clarification gating
	if understanding != nil {
		event, data := understanding.ToWailsEvent()
		s.emitFn(event, data)

		// If high ambiguity and not explicitly requesting a chat, surface clarification
		if understanding.Ambiguity >= AmbiguityMedium && understanding.Clarification != nil &&
			intent.Intent != agent.IntentChat {
			s.emitFn("ai:clarification:needed", map[string]any{
				"question": understanding.Clarification.Question,
				"options":  understanding.Clarification.Options,
				"context":  understanding.Clarification.Context,
				"priority": understanding.Clarification.Priority,
			})
		}
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

			// Phase 4+: Build a DAG plan from decomposed sub-tasks
			if s.router != nil {
				plan := BuildPlanFromSubTasks(route, req.ConversationID)
				plan.tracer = s.tracer
				s.emitFn("ai:dag:plan", plan.Summary())
			}
		}
	}

	if req.AgentID != "" {
		ag, ok := s.agentReg.Get(req.AgentID)

		// Normalize mode: default to "build" for maximum capability
		mode := req.Mode
		if mode != "plan" && mode != "build" && mode != "chat" {
			mode = "build"
		}
		// Check if this is a simple non-technical message
		simpleMsg := isSimpleMessage(req.Messages)

		if ok && ag.SystemPrompt != "" {
			modePrompt := ""
			switch mode {
			case "plan":
				if simpleMsg {
					modePrompt = planModePromptSimple
				} else {
					modePrompt = planModePrompt
				}
			case "build":
				if simpleMsg {
					modePrompt = buildModePromptSimple
				} else {
					modePrompt = buildModePrompt
				}
			default: // chat
				modePrompt = chatModePrompt
			}
			if mode == "build" && !simpleMsg {
				modePrompt += buildToolSuppressHint(req.Messages)
			}
			// Phase 5+: Inject role system hint (permission-aware persona)
			roleHint := ""
			for _, role := range ag.Roles {
				if hint := ag.GetSystemHint(role); hint != "" {
					roleHint += "\n\n" + hint
				}
			}
			// Build context message and merge it INTO the system prompt
			// This ensures tool instructions come FIRST, context comes SECOND
			contextMsg := s.buildContext(req)
			systemContent := ag.SystemPrompt + roleHint + getLanguageHint(req.Model) + modePrompt
			if contextMsg != "" {
				systemContent += "\n\n" + contextMsg
			}
			systemMsg := provider.Message{Role: "system", Content: systemContent}
			req.Messages = append([]provider.Message{systemMsg}, req.Messages...)
		}
		if ok && len(ag.Tools) > 0 {
			tools := ag.Tools
			// Phase 5+: Filter tools by agent role (permission matrix)
			tools = s.filterToolsByRole(tools, ag)
			if mode == "chat" || (mode == "plan" && simpleMsg) {
				tools = []string{}
				for _, t := range ag.Tools {
					if tool, ok := s.toolExec.Get(t); ok && !tool.RequiresApproval() {
						tools = append(tools, t)
					}
				}
			}
			if mode == "plan" && !simpleMsg {
				// plan mode: read-only tools
				tools = []string{}
				for _, t := range ag.Tools {
					if tool, ok := s.toolExec.Get(t); ok && !tool.RequiresApproval() {
						tools = append(tools, t)
					}
				}
			}
			if mode == "build" && simpleMsg {
				tools = []string{}
			}
			if len(tools) > 0 {
				// Check if model supports function calling
				modelCaps := provider.GetModelCapabilities(req.Model)
				if modelCaps.SupportsTool {
					// Model supports function calling — send tools in request
					req.Tools = s.buildToolDefinitions(tools)
				} else {
					// Model doesn't support function calling — don't send tools
					// Rely on text-based tool parsing and bash command detection
					req.Tools = nil
				}
				// Always inject tool instructions into system prompt
				// This enables text-based tool calling for non-function-calling models
				// Use a softer tone for models that don't support function calling —
				// they cannot use [TOOL: ...] format and will output natural language instead
				supportsFC := modelCaps.SupportsTool
				toolHint := buildToolUsageHint(req.Tools, "", supportsFC)
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

	// Create tracer for this conversation (Phase 1+, Phase 6+ with SQLite)
	sink := s.traceSink
	if sink == nil {
		sink = NoopTraceSink{}
	}
	s.tracer = NewTracer(req.ConversationID, sink)

	go s.runAgentLoop(req, agentCtx)

	return nil
}

// SetTraceSink sets the persistent trace storage (Phase 6+).
// Call once during initialization, before any ChatStream calls.
func (s *Service) SetTraceSink(sink TraceSink) {
	s.traceSink = sink
}

// GetTraces retrieves trace headers for a conversation (Phase 6+).
// Returns nil if no SQLite trace sink is configured.
func (s *Service) GetTraces(convID string, limit int) ([]TraceHeader, error) {
	if s.traceSink == nil {
		return nil, nil
	}
	if sqlite, ok := s.traceSink.(*SQLiteTraceSink); ok {
		return sqlite.GetTraces(convID, limit)
	}
	return nil, nil
}

// GetTraceEvents retrieves events for a specific trace (Phase 6+).
func (s *Service) GetTraceEvents(traceID string) ([]TraceEvent, error) {
	if s.traceSink == nil {
		return nil, nil
	}
	if sqlite, ok := s.traceSink.(*SQLiteTraceSink); ok {
		return sqlite.GetTraceEvents(traceID)
	}
	return nil, nil
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

// filterToolsByRole filters a tool ID list based on the agent's roles.
// If the agent has explicit Tools defined, those are further filtered by role.
// If no roles match, returns the original list (fail-open for backward compatibility).
func (s *Service) filterToolsByRole(toolIDs []string, agentDef agent.AgentDef) []string {
	if len(agentDef.Roles) == 0 {
		// No roles specified → use tools as-is (backward compatible)
		return toolIDs
	}

	var filtered []string
	for _, id := range toolIDs {
		for _, role := range agentDef.Roles {
			if agent.CanRoleUseTool(role, id) {
				filtered = append(filtered, id)
				break
			}
		}
	}

	if len(filtered) == 0 {
		// Fail-open: if role filtering removes everything, return original
		return toolIDs
	}
	return filtered
}

// --- RunAgentLoop and helpers moved to agent_loop.go ---
// The following were extracted to agent_loop.go on 2026-07-10 (Phase 1):
//   runAgentLoop, retryableChatStream, isRepeatedLoop, learnPreferences,
//   parseToolArgs, extractFileMeta, parseTextToolCalls, parseKeyValueArgs,
//   detectTextRepetition, detectTextRepetitionN, pruneMessages.

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

// buildToolUsageHint generates explicit tool calling instructions for all models.
// Includes both function calling format and text-based format for maximum compatibility.
// controls tone: false for models that don't support function calling (they'll output natural
// language, not [TOOL: ...] format).
func buildToolUsageHint(tools []provider.ToolDefinition, lastContent string, supportsFC bool) string {
	if len(tools) == 0 {
		return ""
	}

	var hint strings.Builder
	hint.WriteString("\n## 可用工具\n")
	if !supportsFC {
		// Soft tone for non-function-calling models — they can only output text
		hint.WriteString("你可以使用以下工具描述来辅助分析。如果你需要获取文件内容，请直接说明你想了解什么。\n")
	} else {
		hint.WriteString("你必须使用工具来完成任务，不要只输出文字描述。\n")
	}
	hint.WriteString("❌ 禁止使用 bash 命令（find, grep, cat, ls, cd 等）\n")
	hint.WriteString("✅ 必须使用专用工具代替 bash 命令\n\n")

	hint.WriteString("### 工具对照表\n")
	hint.WriteString("| ❌ 禁止的 bash | ✅ 必须使用的工具 |\n")
	hint.WriteString("|---|---|\n")
	hint.WriteString("| find . -name \"*.go\" | glob_files {\"pattern\": \"**/*.go\"} |\n")
	hint.WriteString("| grep -rn \"pattern\" . | search_files {\"query\": \"pattern\"} |\n")
	hint.WriteString("| cat file.go | read_file {\"path\": \"file.go\"} |\n")
	hint.WriteString("| ls -la dir/ | list_directory {\"path\": \"dir/\"} |\n")
	hint.WriteString("| grep找定义 | lsp_definition {\"path\":\"f.go\",\"line\":N,\"column\":N} |\n")
	hint.WriteString("| grep找引用 | lsp_references {\"path\":\"f.go\",\"line\":N,\"column\":N} |\n")
	hint.WriteString("| 读文件看结构 | lsp_symbols {\"path\":\"f.go\"} |\n\n")

	hint.WriteString("### 代码分析优先使用 LSP 工具\n")
	hint.WriteString("- 需要跳转到定义 → lsp_definition\n")
	hint.WriteString("- 需要查找所有引用 → lsp_references\n")
	hint.WriteString("- 需要查看文件大纲 → lsp_symbols\n")
	hint.WriteString("- LSP 比 grep/search 更精确，支持所有语言\n\n")

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
	if supportsFC {
		hint.WriteString("请使用 function calling 格式调用工具。\n")
	} else {
		hint.WriteString("请在回复中使用以下格式（每行一个工具调用）：\n")
	}
	hint.WriteString("[TOOL: 工具名 {\"参数名\": \"参数值\"}]\n\n")
	hint.WriteString("示例：\n")
	hint.WriteString("[TOOL: read_file {\"path\": \"main.go\"}]\n")
	hint.WriteString("[TOOL: search_files {\"query\": \"func main\"}]\n")
	hint.WriteString("[TOOL: execute_command {\"command\": \"go build ./...\"}]\n\n")
	hint.WriteString("重要：必须实际调用工具，不要只说\"我来读取\"。\n")

	return hint.String()
}
