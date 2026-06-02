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
)

const maxAgentLoops = 100
const maxToolResultChars = 8000

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

	mu           sync.Mutex
	cancel       context.CancelFunc
	fingerprints []string
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
) *Service {
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
	}
}

func estimateTokens(text string) int {
	cjk := 0
	other := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF || r >= 0x3400 && r <= 0x4DBF ||
			r >= 0x3000 && r <= 0x303F || r >= 0xFF00 && r <= 0xFFEF ||
			r >= 0x3040 && r <= 0x309F || r >= 0x30A0 && r <= 0x30FF ||
			r >= 0xAC00 && r <= 0xD7AF {
			cjk++
		} else {
			other++
		}
	}
	return int(float64(cjk)*1.5 + float64(other)*0.25)
}

func executeToolWithTimeout(ctx context.Context, executor *agent.ToolExecutor, call agent.ToolCall, timeout time.Duration) (*agent.ToolResult, error) {
	type toolResponse struct {
		result *agent.ToolResult
		err    error
	}
	ch := make(chan toolResponse, 1)
	go func() {
		r, e := executor.Execute(ctx, call)
		ch <- toolResponse{r, e}
	}()
	select {
	case resp := <-ch:
		return resp.result, resp.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("tool execution timed out after %v", timeout)
	}
}

func preCheckProvider(providerMgr *provider.Manager, providerID string) error {
	p, err := providerMgr.Get(providerID)
	if err != nil {
		return fmt.Errorf("未找到AI提供商: %s", providerID)
	}
	cfg := p.GetConfig()
	if cfg.APIKey == "" && providerID != "ollama" {
		return fmt.Errorf("API密钥未配置，请前往设置配置 %s 的API密钥", providerID)
	}
	if cfg.Endpoint == "" {
		return fmt.Errorf("API端点未配置，请前往设置配置 %s 的端点地址", providerID)
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
	return "\n\n注意：用户的消息是简单的问候或提问，直接用文字回复，不要调用任何工具或读取文件，对话式回答即可。"
}

const languageHint = `用和用户相同的语言回答。用户用中文提问就用中文回答，用户用英文提问就用英文回答。
`

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
`

// --- 构建模式 ---
// 对标 Claude Code Build Mode：自主编程智能体，完整的工程纪律。
const buildModePrompt = `
=== 构建模式 ===
你是一个拥有完整文件系统访问权限的自主编程智能体。你的目标是精准、安全、高质量地完成每一个编程任务。

## 执行纪律（每个任务严格遵循）
1. **探索**：先阅读相关文件，理解当前实现。写出你的发现——不要只列文件名
2. **规划**：说明你打算改什么、为什么这样改。对于非简单任务，先列出步骤再动手
3. **实现**：使用工具完成修改。每次改动后简要说明改了什么
4. **验证**：必须回读修改后的文件片段来确认改动正确
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

	if req.ProviderID == "" {
		defProvider := s.providerMgr.GetDefaultProvider()
		if defProvider != nil {
			req.ProviderID = defProvider.ID()
		} else {
			s.emitFn("ai:stream:error", "未配置AI提供商，请先在设置中配置API密钥")
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
		req.Messages = append([]provider.Message{{Role: "user", Content: contextMsg}}, req.Messages...)
	}

	modelCtxWindow := s.contextWindow(req.ProviderID, req.Model)
	maxContextTokens := int(float64(modelCtxWindow) * 0.8)
	compressed, didSummarize := s.compress(req.Messages, maxContextTokens, req.ProviderID)
	req.Messages = compressed
	if didSummarize {
		s.emitFn("ai:context:summarized", "上下文已自动压缩，旧消息摘要已保留")
	}

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
	maxRetries := 5
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		eventCh, err := s.providerMgr.ChatStream(roundCtx, req)
		if err == nil {
			return eventCh, nil
		}

		diag := provider.DiagnoseError(err)
		if !diag.Retryable || attempt == maxRetries {
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


	for loop := 0; loop < maxAgentLoops; loop++ {
		select {
		case <-ctx.Done():
			s.emitFn("ai:stream:done", "cancelled")
			doneEmitted = true
			return
		default:
		}
		roundCtx, roundCancel := context.WithTimeout(ctx, 180*time.Second)
		eventCh, err := s.retryableChatStream(roundCtx, currentReq)
		if err != nil {
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
				if !toolCallsSeen {
					if assistantContent == "" {
						noToolReq := currentReq
						noToolReq.Tools = nil
						noToolReq.Stream = false
						fbCtx, fbCancel := context.WithTimeout(ctx, 45*time.Second)
						resp, chatErr := s.providerMgr.Chat(fbCtx, noToolReq)
						fbCancel()
						if chatErr == nil && resp != nil && resp.Content != "" {
							assistantContent = resp.Content
							s.emitFn("ai:stream:data", resp.Content)
						} else {
							s.emitFn("ai:stream:error", "AI未返回任何内容。请检查网络连接或尝试换个问题。")
							roundCancel()
							doneEmitted = true
							return
						}
					}
					estimatedTokensIn := 0
					for _, msg := range currentReq.Messages {
						estimatedTokensIn += estimateTokens(msg.Content)
					}
					if s.memoryStore != nil && estimatedTokensIn > 0 {
						usageEntry := &memory.TokenUsageEntry{
							ID:             fmt.Sprintf("tu_%d", time.Now().UnixNano()),
							ConversationID: "",
							ProviderID:     currentReq.ProviderID,
							Model:          currentReq.Model,
							TokensIn:       estimatedTokensIn,
							TokensOut:      estimateTokens(assistantContent),
							Cost:           0,
							CreatedAt:      time.Now().Format(time.RFC3339),
						}
						go s.memoryStore.SaveTokenUsage(usageEntry)
					}
					s.emitFn("ai:stream:done", "")
					roundCancel()
					doneEmitted = true
					return
				}
			}
		}

		roundCancel()

		if !streamReceivedAny {
			s.emitFn("ai:stream:error", "AI服务未返回任何响应，请检查网络连接或API配置")
			doneEmitted = true
			return
		}

		if !toolCallsSeen {
			if assistantContent == "" {
				fallbackCtx, fbCancel := context.WithTimeout(ctx, 60*time.Second)
				resp, chatErr := s.providerMgr.Chat(fallbackCtx, currentReq)
				fbCancel()
				if chatErr == nil && resp != nil && resp.Content != "" {
					assistantContent = resp.Content
					s.emitFn("ai:stream:data", resp.Content)
				} else {
					s.emitFn("ai:stream:error", "AI未返回任何内容。请检查网络连接或尝试换个问题。")
					doneEmitted = true
					return
				}
			}
			s.emitFn("ai:stream:done", "")
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

		agentTools.SubAgentCurrentProviderID = currentReq.ProviderID

		if len(calls) > 0 {
			type toolRes struct {
				call   agent.ToolCall
				result *agent.ToolResult
				err    error
			}
			ch := make(chan toolRes, len(calls))
			for _, call := range calls {
				s.emitFn("ai:stream:tool_call", map[string]any{
					"id": call.ID, "name": call.Name, "args": call.Args, "loop": loop + 1,
				})
				go func(c agent.ToolCall) {
					r, e := executeToolWithTimeout(ctx, s.toolExec, c, 60*time.Second)
					ch <- toolRes{c, r, e}
				}(call)
			}
			for i := 0; i < len(calls); i++ {
				tr := <-ch
				if tr.err != nil {
					s.emitFn("ai:stream:tool_result", map[string]string{"callId": tr.call.ID, "name": tr.call.Name, "error": tr.err.Error()})
					currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: fmt.Sprintf("Error: %s", tr.err.Error()), ToolCallID: tr.call.ID, Name: tr.call.Name})
				} else {
					s.emitFn("ai:stream:tool_result", tr.result)
					rc := tr.result.Result
					if len(rc) > maxToolResultChars {
						rc = rc[:maxToolResultChars] + "... [truncated]"
					}
					currentReq.Messages = append(currentReq.Messages, provider.Message{Role: "tool", Content: rc, ToolCallID: tr.call.ID, Name: tr.call.Name})
				}
			}
		}

		if assistantContent == "" && len(toolCalls) > 0 {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role:    "system",
					Content: "你调用了工具但没有解释在做什么。请在工具调用前后说明你的推理，简要总结你做了什么以及为什么。",
				})
			}

			// Soft nudges for potential issues — never terminate, just guide.
			if assistantContent == "" && !toolCallsSeen {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role:    "system",
					Content: "上一轮没有产生任何输出。请检查工具执行结果，继续推进任务。如果任务已经完成，请向用户报告总结。",
				})
			}

			if len(toolCalls) > 0 && assistantContent == "" && s.isRepeatedLoop(toolCalls) {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role:    "system",
					Content: "你似乎陷入了重复模式。请回顾你的目标，尝试不同的方式：读取其他文件、换一个搜索策略、或者先分析当前进展再决定下一步。",
				})
			}
		}

		// Agent loop exhausted (safety net) — deliver a graceful ending.
		s.emitFn("ai:stream:data", "\n\n---\n*任务已达到最大执行轮次。如果任务未完成，请在新对话中继续。*")
		s.emitFn("ai:stream:done", "")
		doneEmitted = true
	}
