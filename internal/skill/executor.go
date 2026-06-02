package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/provider"
)

type Executor struct {
	registry    *Registry
	providerMgr *provider.Manager
	toolExec    *agent.ToolExecutor
}

func NewExecutor(registry *Registry, providerMgr *provider.Manager, toolExec *agent.ToolExecutor) *Executor {
	return &Executor{
		registry:    registry,
		providerMgr: providerMgr,
		toolExec:    toolExec,
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
		MaxTokens:  0,
		Stream:     true,
		Tools:       e.buildReadOnlyToolDefs(),
	}

	return e.providerMgr.ChatStream(ctx, req)
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
		MaxTokens:  0,
		Stream:     true,
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
		defer roundCancel()
		eventCh, err := e.providerMgr.ChatStream(roundCtx, currentReq)
		if err != nil {
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
					return
				}
			}
		}

		if !hasToolCalls {
			ch <- provider.StreamEvent{Type: "done"}
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
