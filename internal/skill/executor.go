package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

type Executor struct {
	registry    *Registry
	providerMgr *provider.Manager
	toolExec    *agent.ToolExecutor
	memoryStore *memory.Store
}

func NewExecutor(registry *Registry, providerMgr *provider.Manager, toolExec *agent.ToolExecutor, memoryStore *memory.Store) *Executor {
	return &Executor{
		registry:    registry,
		providerMgr: providerMgr,
		toolExec:    toolExec,
		memoryStore: memoryStore,
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

func recordSkillToken(e *Executor, req provider.ChatRequest, assistantContent string) {
	if e.memoryStore == nil {
		return
	}
	tokensIn := 0
	for _, msg := range req.Messages {
		tokensIn += estimateTokens(msg.Content)
	}
	tokensOut := estimateTokens(assistantContent)
	if tokensIn > 0 || tokensOut > 0 {
		go e.memoryStore.SaveTokenUsage(&memory.TokenUsageEntry{
			ID:             fmt.Sprintf("sk_%d", time.Now().UnixNano()),
			ConversationID: "",
			ProviderID:     req.ProviderID,
			Model:          req.Model,
			TokensIn:       tokensIn,
			TokensOut:      tokensOut,
			Cost:           0,
			CreatedAt:      time.Now().Format(time.RFC3339),
		})
	}
}

// ExecuteSingle runs a skill as a single-shot request without tool calling.
// Used by the Skill tool to invoke other skills without recursion.
func (e *Executor) ExecuteSingle(ctx context.Context, skillID string, sctx SkillContext, providerID string, model string) (<-chan provider.StreamEvent, error) {
	sk, ok := e.registry.Get(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	filledPrompt := fillTemplate(sk.PromptTemplate, sctx)
	resolvedProviderID := e.resolveProviderID(providerID)

	systemPrompt := buildSkillSystemPrompt(sk)
	req := provider.ChatRequest{
		ProviderID: resolvedProviderID,
		Model:      model,
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: filledPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   0,
		Stream:      true,
		Tools:       e.buildReadOnlyToolDefs(),
	}

	eventCh, err := e.providerMgr.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}
	// Wrap to capture content for token recording
	ch := make(chan provider.StreamEvent, 64)
	go func() {
		defer close(ch)
		var content string
		for event := range eventCh {
			if event.Type == "data" {
				content += event.Content
			}
			ch <- event
		}
		recordSkillToken(e, req, content)
	}()
	return ch, nil
}

// Execute runs a skill with tool calling support (mini agent loop).
// This allows skills like "using-superpowers" to call other skills via the Skill tool.
func (e *Executor) Execute(ctx context.Context, skillID string, sctx SkillContext, providerID string, model string) (<-chan provider.StreamEvent, error) {
	sk, ok := e.registry.Get(skillID)
	if !ok {
		return nil, fmt.Errorf("skill not found: %s", skillID)
	}

	filledPrompt := fillTemplate(sk.PromptTemplate, sctx)
	resolvedProviderID := e.resolveProviderID(providerID)

	systemPrompt := buildSkillSystemPrompt(sk)

	req := provider.ChatRequest{
		ProviderID: resolvedProviderID,
		Model:      model,
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: filledPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   0,
		Stream:      true,
		Tools:       e.buildToolDefs(),
	}

	ch := make(chan provider.StreamEvent, 64)
	go e.runSkillLoop(ctx, req, ch)
	return ch, nil
}

func (e *Executor) resolveProviderID(providerID string) string {
	if providerID == "" {
		dp := e.providerMgr.GetDefaultProvider()
		if dp != nil {
			return dp.ID()
		}
	}
	return providerID
}

func (e *Executor) buildToolDefs() []provider.ToolDefinition {
	if e.toolExec == nil {
		return nil
	}
	allTools := e.toolExec.List()
	defs := make([]provider.ToolDefinition, 0, len(allTools))
	for _, t := range allTools {
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

func (e *Executor) buildReadOnlyToolDefs() []provider.ToolDefinition {
	if e.toolExec == nil {
		return nil
	}
	allTools := e.toolExec.List()
	defs := make([]provider.ToolDefinition, 0, len(allTools))
	for _, t := range allTools {
		if t.RequiresApproval() {
			continue // skip write/execute tools
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

const maxSkillLoops = 50

func (e *Executor) runSkillLoop(ctx context.Context, req provider.ChatRequest, ch chan<- provider.StreamEvent) {
	defer close(ch)

	currentReq := req
	for loop := 0; loop < maxSkillLoops; loop++ {

		select {
		case <-ctx.Done():
			ch <- provider.StreamEvent{Type: "done"}
			return
		default:
		}

		roundCtx, roundCancel := context.WithTimeout(ctx, 120*time.Second)
		eventCh, err := e.providerMgr.ChatStream(roundCtx, currentReq)
		if err != nil {
			roundCancel()
			ch <- provider.StreamEvent{Type: "error", Content: err.Error()}
			return
		}

		var assistantContent string
		var toolCalls []provider.ToolCall
		hasToolCalls := false

		for event := range eventCh {
			switch event.Type {
			case "data":
				assistantContent += event.Content
				ch <- event
			case "thinking":
				ch <- event
			case "error":
				ch <- event
				roundCancel()
				return
			case "tool_call":
				if len(event.ToolCalls) > 0 {
					toolCalls = append(toolCalls, event.ToolCalls...)
					hasToolCalls = true
				} else if event.Name != "" {
					toolCalls = append(toolCalls, provider.ToolCall{
						ID:   fmt.Sprintf("tc_%d", time.Now().UnixNano()),
						Type: "function",
						Function: provider.ToolCallFunc{
							Name:      event.Name,
							Arguments: event.Args,
						},
					})
					hasToolCalls = true
				}
			case "done":
				if !hasToolCalls {
					ch <- provider.StreamEvent{Type: "done"}
					roundCancel()
					return
				}
			}
		}

		recordSkillToken(e, currentReq, assistantContent)

		if !hasToolCalls {
			ch <- provider.StreamEvent{Type: "done"}
			roundCancel()
			return
		}

		// Add assistant message with tool calls
		assistantMsg := provider.Message{Role: "assistant", Content: assistantContent}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}
		currentReq.Messages = append(currentReq.Messages, assistantMsg)

		// Execute tool calls and add results
		for _, tc := range toolCalls {
			call := agent.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: make(map[string]any),
			}
			if tc.Function.Arguments != "" {
				json.Unmarshal([]byte(tc.Function.Arguments), &call.Args)
			}

			if e.toolExec != nil {
				result, err := e.toolExec.Execute(ctx, call)
				if err != nil {
					log.Printf("Skill tool error: %s(%s): %v", call.Name, tc.Function.Arguments, err)
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role: "tool", Content: fmt.Sprintf("Error: %v", err), ToolCallID: tc.ID, Name: tc.Function.Name,
					})
				} else {
					rc := result.Result
					if result.Error != "" {
						rc = "Error: " + result.Error
					}
					if len(rc) > 5000 {
						rc = rc[:5000] + "... [truncated]"
					}
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role: "tool", Content: rc, ToolCallID: tc.ID, Name: tc.Function.Name,
					})
				}
			} else {
				currentReq.Messages = append(currentReq.Messages, provider.Message{
					Role: "tool", Content: "Tool executor not available", ToolCallID: tc.ID, Name: tc.Function.Name,
				})
			}
		}

		roundCancel()
	}

	ch <- provider.StreamEvent{Type: "done"}
}

func buildSkillSystemPrompt(skill SkillDef) string {
	switch skill.ResultType {
	case "code":
		return "You are a code generation assistant. Output ONLY the requested code without any markdown fences or explanations. Be precise and follow the user's requirements exactly."
	case "diff":
		return "You are a code refactoring assistant. Show changes in unified diff format. Be specific about what changes and why."
	default:
		return "You are a skilled software engineering assistant. Follow the user's instructions carefully and provide a direct, actionable response. Be concise and specific."
	}
}

func fillTemplate(template string, ctx SkillContext) string {
	r := strings.NewReplacer(
		"{code}", ctx.SelectedCode,
		"{file}", ctx.FilePath,
		"{content}", ctx.FileContent,
		"{error}", strings.Join(ctx.Diagnostics, "\n"),
		"{language}", ctx.Language,
		"{input}", ctx.UserInput,
	)
	return r.Replace(template)
}

func (e *Executor) ExecutePipeline(ctx context.Context, pipeline SkillPipeline, skillCtx SkillContext) (*PipelineExecutionResult, error) {
	result := &PipelineExecutionResult{
		PipelineID: pipeline.ID,
		Steps:      make([]PipelineStepResult, 0, len(pipeline.Steps)),
		Success:    true,
	}

	var previousOutput string

	for i, step := range pipeline.Steps {
		stepResult := PipelineStepResult{
			StepIndex: i,
			SkillID:   step.SkillID,
		}

		if step.Condition != "" {
			if !evaluateCondition(step.Condition, previousOutput, skillCtx) {
				stepResult.Skipped = true
				result.Steps = append(result.Steps, stepResult)
				continue
			}
		}

		if step.InputFrom != "" && previousOutput != "" {
			skillCtx.UserInput = previousOutput
		}

		_, ok := e.registry.Get(step.SkillID)
		if !ok {
			stepResult.Error = fmt.Sprintf("skill %s not found", step.SkillID)
			result.Steps = append(result.Steps, stepResult)
			if !step.Optional {
				result.Success = false
				result.Error = stepResult.Error
				return result, fmt.Errorf(stepResult.Error)
			}
			continue
		}

		eventCh, err := e.Execute(ctx, step.SkillID, skillCtx, "", "")
		if err != nil {
			stepResult.Error = err.Error()
			result.Steps = append(result.Steps, stepResult)
			if !step.Optional {
				result.Success = false
				result.Error = err.Error()
				return result, err
			}
			continue
		}

		var content strings.Builder
		for event := range eventCh {
			if event.Type == "data" {
				content.WriteString(event.Content)
			} else if event.Type == "error" {
				stepResult.Error = event.Content
				break
			}
		}

		sr := &SkillResult{
			SkillID:    step.SkillID,
			Content:    content.String(),
			ResultType: "text",
		}
		stepResult.Result = sr
		previousOutput = sr.Content
		result.Steps = append(result.Steps, stepResult)

		if e.memoryStore != nil && skillCtx.ProjectPath != "" {
			go e.memoryStore.SaveKnowledge(&memory.Knowledge{
				ID:          fmt.Sprintf("skill_pipe_%s_%d_%d", pipeline.ID, i, time.Now().UnixNano()),
				ProjectPath: skillCtx.ProjectPath,
				Category:    "skill_pipeline",
				Key:         fmt.Sprintf("%s_step_%d", pipeline.ID, i),
				Value:       sr.Content,
				Source:      "auto",
				UpdatedAt:   time.Now().Format(time.RFC3339),
			})
		}
	}

	return result, nil
}

func evaluateCondition(condition string, previousOutput string, ctx SkillContext) bool {
	switch condition {
	case "has_errors":
		return len(ctx.Diagnostics) > 0
	case "no_errors":
		return len(ctx.Diagnostics) == 0
	case "has_code":
		return ctx.SelectedCode != ""
	case "has_previous_output":
		return previousOutput != ""
	default:
		return true
	}
}
