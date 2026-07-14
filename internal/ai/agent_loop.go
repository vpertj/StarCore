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
)

// runAgentLoop is the core agent loop that handles tool calls, parallel execution,
// repetition detection, nudging, and auto-continue. It is launched as a goroutine
// by ChatStream via: go s.runAgentLoop(req, agentCtx).
//
// Loop execution flow:
//  1. Inject project state + anti-drift reminders
//  2. Call retryableChatStream → stream LLM response
//  3. Parse tool calls (function calling → text [TOOL:] → DSLM → bash)
//  4. If no tools: nudge (build mode) or accept text (plan/chat mode)
//  5. Execute tools in parallel → collect results → append to messages
//  6. Repeat until: no tool calls, nudge exhausted, auto-continue, or max loops
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

	// Wire supervisor to tracer so it can observe trace events
	if s.supervisor != nil && s.tracer != nil {
		s.supervisor.SetTracer(s.tracer)
	}

	var prevMsgCount int
	var nudgeCount int
	s.progress = newAgentProgress()

	// Normalize mode: default to "build"
	mode := req.Mode
	if mode != "plan" && mode != "build" && mode != "chat" {
		mode = "build"
	}

	maxLoops := calcMaxAgentLoops(mode, req.Model)
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

	totalToolCalls := 0
	totalToolErrors := 0

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
			if s.tracer != nil {
				s.tracer.Event(EventLoopAutoCont, StageExecute, "", fmt.Sprintf("追加%d轮，上限%d", extra, maxLoops))
			}
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
		const maxTextWithoutTools = 2000   // Max chars of text before interrupting — generous to allow real answers
		const repetitionCheckInterval = 50 // Check for repetition every N chars (was 100)
		const repetitionThreshold = 2      // Trigger after 2 repetitions

		// In chat/plan mode or when tools are suppressed, text-only responses are fine
		chatMode := mode == "chat" || mode == "plan" || (mode == "build" && isSimpleMessage(req.Messages))

		// Check if user message is purely non-technical (greeting, thanks) — skip repetition detection
		// Must match the ENTIRE message pattern, not just contain a keyword
		isNonTechMsg := false
		{
			lastUMsg := ""
			for i := len(currentReq.Messages) - 1; i >= 0; i-- {
				if currentReq.Messages[i].Role == "user" {
					lastUMsg = strings.TrimSpace(currentReq.Messages[i].Content)
					break
				}
			}
			// Only skip for very short, purely social messages
			lower := strings.ToLower(lastUMsg)
			nonTechPatterns := []string{
				"^你好$", "^hello$", "^hi$", "^嗨$", "^hey$",
				"^谢谢$", "^thanks$", "^thank you$", "^再见$", "^bye$",
				"^你好[！!]*$", "^hello[!]*$",
			}
			for _, pattern := range nonTechPatterns {
				if matched, _ := regexp.MatchString(pattern, lower); matched {
					isNonTechMsg = true
					break
				}
			}
			// Also match very short messages (<=6 chars) that are clearly social
			if !isNonTechMsg && len([]rune(lastUMsg)) <= 6 {
				shortKws := []string{"你好", "hello", "hi", "嗨", "hey", "谢谢", "thanks", "bye", "再见"}
				for _, kw := range shortKws {
					if strings.Contains(lower, kw) {
						isNonTechMsg = true
						break
					}
				}
			}
		}

		for event := range eventCh {
			streamReceivedAny = true
			switch event.Type {
			case "data":
				assistantContent += event.Content

				// Streaming-level interrupt: detect planning loops and repetition
				// Skip for non-technical messages (greetings don't need tool calls)
				// Skip for chat mode (text-only responses are acceptable)
				if !toolCallsSeen && !streamInterrupted && !isNonTechMsg && !chatMode {
					// Check if model is currently outputting a tool call
					// If text contains [TO or [TOOL: or {, it's likely mid-tool-call
					isOutputtingToolCall := strings.Contains(assistantContent, "[TO") ||
						strings.Contains(assistantContent, "[TOOL:") ||
						strings.HasSuffix(strings.TrimSpace(assistantContent), "{") ||
						strings.HasSuffix(strings.TrimSpace(assistantContent), ":")

					// Only interrupt if NOT outputting a tool call
					if !isOutputtingToolCall {
						// Check 1: Text length threshold — model is outputting too much without tools
						if len(assistantContent) > maxTextWithoutTools {
							streamInterrupted = true
							s.emitFn("ai:stream:data", "\n\n*[系统: 检测到模型输出大量文本但未调用工具，正在中断并重新引导...]*")
							if s.tracer != nil {
								s.tracer.Event(EventStreamInterrupt, StageExecute, "", "文本过长未调用工具")
							}
							roundCancel()
							break
						}

						// Check 2: Repetition detection — same phrases appearing multiple times
						if len(assistantContent) > repetitionCheckInterval {
							if detectTextRepetitionN(assistantContent, repetitionThreshold) {
								streamInterrupted = true
								s.emitFn("ai:stream:data", "\n\n*[系统: 检测到重复输出，正在中断并重新引导...]*")
								if s.tracer != nil {
									s.tracer.Event(EventRepetition, StageExecute, "", "流式重复检测触发")
								}
								roundCancel()
								break
							}
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
		// handle consistently with the unified nudge logic below.
		// Do NOT force a separate aggressive nudge here — it causes infinite loops
		// when the model genuinely cannot produce tool calls.
		if streamInterrupted && !toolCallsSeen {
			// Fall through to the unified no-tool handling below.
			// Mark as empty round but don't inject the old aggressive prompt.
			s.progress.recordEmptyRound()
			// Don't increment nudgeCount here — let the unified logic handle counting
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

		// If still no tool calls, try parsing DSLM format (used by some models)
		if !toolCallsSeen && assistantContent != "" {
			dslmCalls := parseDSLMToolCalls(assistantContent)
			if len(dslmCalls) > 0 {
				toolCalls = append(toolCalls, dslmCalls...)
				toolCallsSeen = true
			}
		}

		// If still no tool calls, try detecting bash commands in the output
		// This handles models that don't support function calling and output raw bash
		if !toolCallsSeen && assistantContent != "" {
			bashCalls := parseBashCommands(assistantContent)
			if len(bashCalls) > 0 {
				toolCalls = append(toolCalls, bashCalls...)
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

			if req.Mode == "build" || req.Mode == "plan" || req.Mode == "chat" {
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

				// Unified no-tool-call handling for ALL modes.
				// Philosophy: tools are capabilities, not obligations.
				// The model should decide whether to use them based on the user's actual need.
				// Text-only responses are ALWAYS acceptable — only nudge after repeated empty rounds.
				if !toolCallsSeen && assistantContent != "" {
					s.progress.recordEmptyRound()

					// Check if AI explicitly declared task completion — stop immediately
					lowerContent := strings.ToLower(assistantContent)
					explicitDone := strings.Contains(lowerContent, "任务已完成") ||
						strings.Contains(lowerContent, "all tasks completed") ||
						strings.Contains(lowerContent, "所有任务已完成") ||
						strings.Contains(lowerContent, "规划完成") ||
						strings.Contains(lowerContent, "分析完成")
					if explicitDone {
						donePayload := map[string]interface{}{}
						if streamUsage != nil {
							donePayload["usage"] = streamUsage
						}
						s.emitFn("ai:stream:done", donePayload)
						setDone()
						return
					}

					// After 3+ consecutive rounds without tools, accept the text response and stop.
					// The model likely cannot or does not need to use tools for this request.
					if nudgeCount >= 3 {
						s.emitFn("ai:stream:data", "\n\n*[系统: 已生成文字回复。如需操作文件，请明确说明需求。]*")
						donePayload := map[string]interface{}{}
						if streamUsage != nil {
							donePayload["usage"] = streamUsage
						}
						s.emitFn("ai:stream:done", donePayload)
						setDone()
						return
					}

					// Nudge #1 and #2: gently remind the model tools are available
					if nudgeCount < 2 {
						nudgeCount++

						// Build a gentle reminder that tools are available, not a command
						nudgeContent := "提示：你可以使用工具来完成更复杂的任务（读取文件、搜索代码、执行命令等）。如果需要，请调用工具；如果不需要，直接回答即可。"
						if s.loopState.GetOriginalGoal() != "" {
							nudgeContent += fmt.Sprintf("\n原始目标: %s", s.loopState.GetOriginalGoal())
						}

						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "assistant",
							Content: assistantContent,
						})
						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "user",
							Content: nudgeContent,
						})

						// First nudge: inject example tool calls as demonstration
						if nudgeCount == 1 {
							exampleCalls := buildExampleToolCalls(currentReq.Tools)
							if exampleCalls != "" {
								currentReq.Messages = append(currentReq.Messages, provider.Message{
									Role:    "assistant",
									Content: exampleCalls,
								})
							}
						}

						if s.tracer != nil {
							s.tracer.Event(EventNudge, StageExecute, "", fmt.Sprintf("模式%s第%d次温和引导", req.Mode, nudgeCount))
						}
						continue
					}
					// Nudge #2 exhausted — accept text and stop (next iteration hits the >= 3 check)
					s.emitFn("ai:stream:data", "\n\n*[系统: 等待工具调用...]*")
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
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &call.Args); err != nil {
						log.Printf("WARNING: tool %q has invalid arguments JSON (loop %d): %v", tc.Function.Name, loop+1, err)
						continue
					}
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

			toolFailures := 0
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
				totalToolCalls += len(calls)
				totalToolErrors += toolFailures

				// Circuit breaker: if ALL tool calls in this round failed, record a failure
				if toolFailures > 0 && toolFailures == len(calls) {
					s.cb.RecordFailure()
					if s.tracer != nil {
						s.tracer.Event(EventCircuitBreaker, StageExecute, "", fmt.Sprintf("第%d轮全部工具调用失败", loop+1))
					}
				} else if toolFailures < len(calls) {
					s.cb.RecordSuccess()
				}
			}

			// Track files touched by write/edit/move/delete tools
			var modifiedFiles []string
			for _, tc := range toolCalls {
				switch tc.Function.Name {
				case "write_file", "edit_file", "create_directory":
					var args map[string]any
					if tc.Function.Arguments != "" {
						json.Unmarshal([]byte(tc.Function.Arguments), &args) // best-effort: missing path just means no tracking
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
			if len(modifiedFiles) > 0 && s.verifyFn != nil && mode == "build" {
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
				if s.tracer != nil {
					s.tracer.Event(EventRepetition, StageExecute, "", "语义重复检测触发")
				}
			}

			// Stagnation alert: no progress for 5+ rounds
			if s.loopState.IsStagnant(5) {
				s.emitFn("ai:stream:data", "\n\n*[系统: 已连续5轮无实质进展，请调整策略]*")
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role:    "system",
					Content: "⚠️ 已连续5轮无实质进展（无工具调用或文件修改）。请：1) 回顾原始目标；2) 分析当前卡在哪里；3) 尝试完全不同的方法；4) 如果无法继续，总结已完成的工作并结束。",
				})
				if s.tracer != nil {
					s.tracer.Event(EventStagnation, StageExecute, "", "连续5轮无进展停滞检测")
				}
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
				if s.tracer != nil {
					s.tracer.Event(EventLoopAutoCont, StageExecute, "", fmt.Sprintf("第%d次自动继续，追加%d轮", s.autoContinueCount, extra))
				}
			}

			// Supervisor checkpoint — rule-engine watchdog evaluates loop health
			if s.supervisor != nil {
				allFailed := len(calls) > 0 && toolFailures > 0 && toolFailures == len(calls)
				if decision := s.supervisorCheck(loop, maxLoops, totalToolCalls, totalToolErrors, nudgeCount, allFailed); decision != nil {
					switch decision.Action {
					case ActionForceStop:
						s.emitFn("ai:stream:data", "\n\n"+decision.FrontendMessage())
						if s.tracer != nil {
							s.tracer.Event(EventStagnation, StageSupervise, "", "supervisor: force_stop: "+decision.Reason)
						}
						donePayload := map[string]interface{}{}
						s.emitFn("ai:stream:done", donePayload)
						setDone()
						return
					case ActionNudge:
						s.emitFn("ai:stream:data", "\n\n"+decision.FrontendMessage())
						nudgeCount++
						currentReq.Messages = append(currentReq.Messages, provider.Message{
							Role:    "system",
							Content: fmt.Sprintf("[Supervisor引导] %s", decision.Reason),
						})
					case ActionEscalate:
						s.emitFn("ai:stream:data", "\n\n"+decision.FrontendMessage())
						if event, data := decision.WailsEvent(); event != "" {
							s.emitFn(event, data)
						}
					}
				}
			}
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

	if s.tracer != nil {
		s.tracer.Event(EventLoopExhausted, StageExecute, "", fmt.Sprintf("循环耗尽: %d轮, %d工具, %d错误", maxLoops, totalToolCalls, totalToolErrors))
		s.tracer.Finish(maxLoops, totalToolCalls, totalToolErrors)
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
		if s.tracer != nil {
			s.tracer.Event(EventLLMCall, StageExecute, "", fmt.Sprintf("LLM调用失败(attempt %d): %s", attempt, diag.Title))
		}
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

// parseTextToolCalls extracts tool calls from text output using [TOOL: name {args}] format.
func parseTextToolCalls(content string) []provider.ToolCall {
	var calls []provider.ToolCall

	// Strategy 1: Complete format [TOOL: name {json}]
	re := regexp.MustCompile(`\[TOOL:\s*(\w+)\s*\{([^}]+)\}\]`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		toolName := match[1]
		argsStr := match[2]
		var args map[string]any
		if err := json.Unmarshal([]byte("{"+argsStr+"}"), &args); err != nil {
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
	if len(calls) > 0 {
		return calls
	}

	// Strategy 2: Incomplete format — tool name without closing bracket
	// [TOOL: execute_command {"command": "go build"}]
	// [TOOL: execute_command
	re2 := regexp.MustCompile(`\[TOOL:\s*(\w+)\s*(?:\{([^}]*))?`)
	matches2 := re2.FindAllStringSubmatch(content, -1)
	for _, match := range matches2 {
		toolName := match[1]
		argsStr := match[2]
		if argsStr == "" {
			// No arguments provided, skip
			continue
		}
		var args map[string]any
		if err := json.Unmarshal([]byte("{"+argsStr+"}"), &args); err != nil {
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
// Ignores tool call lines (lines containing [TOOL: or function calling patterns).
func detectTextRepetitionN(content string, threshold int) bool {
	if len([]rune(content)) < 30 {
		return false
	}

	// Strategy 1: Exact line repetition (skip tool call lines)
	lines := strings.Split(content, "\n")
	lineCounts := make(map[string]int)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip tool call lines — they are intentional repetitions
		if strings.Contains(line, "[TOOL:") || strings.Contains(line, "[TO") ||
			strings.Contains(line, "function") || strings.Contains(line, "invoke") {
			continue
		}
		if len(line) >= 10 {
			lineCounts[line]++
			if lineCounts[line] >= threshold {
				return true
			}
		}
	}

	// Strategy 2: Exact sentence repetition (split by punctuation, skip tool calls)
	sentences := regexp.MustCompile(`[。！？.!?\n]+`).Split(content, -1)
	sentenceCounts := make(map[string]int)

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		// Skip tool call content
		if strings.Contains(sent, "[TOOL:") || strings.Contains(sent, "[TO") {
			continue
		}
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
		// Skip tool call content
		if strings.Contains(sent, "[TOOL:") || strings.Contains(sent, "[TO") {
			continue
		}
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

// supervisorCheck builds a Snapshot and asks the supervisor to evaluate rules.
// Returns a non-nil Decision if a rule fires, or nil to continue normally.
func (s *Service) supervisorCheck(loop, maxLoops int, toolCalls, toolErrors, nudgeCount int, allToolsFailed bool) *Decision {
	// Read cumulative token usage from the tracer
	tokenIn, tokenOut := 0, 0
	if s.tracer != nil {
		t := s.tracer.GetTrace()
		if t != nil {
			tokenIn = t.TokenIn
			tokenOut = t.TokenOut
		}
	}

	return s.supervisor.OnLoopEnd(
		loop, maxLoops, s.autoContinueCount,
		toolCalls, toolErrors, len(s.loopState.GetFilesTouched()),
		nudgeCount, s.loopState.GetStagnantRounds(),
		allToolsFailed, false,
		tokenIn, tokenOut,
	)
}

// buildExampleToolCalls generates example tool call messages that demonstrate
// the expected [TOOL: ...] format. This teaches non-function-calling models
// by example — most models learn patterns better from demonstrations than from
// instructions.
func buildExampleToolCalls(tools []provider.ToolDefinition) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("以下是我将要执行的工具调用：\n\n")

	// Pick up to 3 relevant tools to demo
	demoCount := 0
	for _, t := range tools {
		if demoCount >= 3 {
			break
		}
		name := t.Function.Name
		switch name {
		case "read_file":
			sb.WriteString("[TOOL: read_file {\"path\": \"main.go\"}]\n")
		case "search_files":
			sb.WriteString("[TOOL: search_files {\"query\": \"func main\"}]\n")
		case "glob_files":
			sb.WriteString("[TOOL: glob_files {\"pattern\": \"**/*.go\"}]\n")
		case "execute_command":
			sb.WriteString("[TOOL: execute_command {\"command\": \"go build ./...\"}]\n")
		case "list_directory":
			sb.WriteString("[TOOL: list_directory {\"path\": \".\"}]\n")
		case "edit_file":
			sb.WriteString("[TOOL: edit_file {\"path\": \"main.go\", \"old_string\": \"old\", \"new_string\": \"new\"}]\n")
		case "write_file":
			sb.WriteString("[TOOL: write_file {\"path\": \"new.go\", \"content\": \"package main\"}]\n")
		case "get_diagnostics":
			sb.WriteString("[TOOL: get_diagnostics {\"path\": \"main.go\"}]\n")
		case "get_context":
			sb.WriteString("[TOOL: get_context {\"query\": \"项目结构\"}]\n")
		default:
			// Generic example for any other tool
			sb.WriteString(fmt.Sprintf("[TOOL: %s {\"param\": \"value\"}]\n", name))
		}
		demoCount++
	}

	return sb.String()
}
