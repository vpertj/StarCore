package tools

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

// SubAgentToolExec is set by app.go to give the SubAgent tool access to the tool executor.
var SubAgentToolExec *agent.ToolExecutor
var SubAgentProviderMgr *provider.Manager
var SubAgentMemoryStore *memory.Store
var SubAgentCurrentProviderID string // set before each request by the caller

type SubAgentTool struct{}

func NewSubAgentTool() *SubAgentTool { return &SubAgentTool{} }

func (t *SubAgentTool) ID() string             { return "sub_agent" }
func (t *SubAgentTool) Name() string           { return "Sub Agent" }
func (t *SubAgentTool) RequiresApproval() bool { return false }

func (t *SubAgentTool) Description() string {
	return "Spawn an independent read-only sub-agent to analyze a specific task with its own context window. The sub-agent can read files, search code, and list directories. Use this to delegate focused investigation work without polluting the main conversation context."
}

func (t *SubAgentTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"task":    {Type: "string", Description: "Detailed description of what to analyze/investigate"},
			"context": {Type: "string", Description: "Optional: file paths, code snippets, or background info"},
		},
		Required: []string{"task"},
	}
}

const subAgentMaxLoops = 6

func (t *SubAgentTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if SubAgentProviderMgr == nil {
		return "", fmt.Errorf("sub-agent: provider not available")
	}

	task, ok := args["task"].(string)
	if !ok || task == "" {
		return "", fmt.Errorf("task description is required")
	}
	extContext, _ := args["context"].(string)

	log.Printf("[SUB-AGENT] spawned: %s", truncateStr(task, 80))

	prompt := task
	if extContext != "" {
		prompt += "\n\nContext:\n" + extContext
	}

	providerID := SubAgentCurrentProviderID
	if providerID == "" && SubAgentProviderMgr != nil {
		dp := SubAgentProviderMgr.GetDefaultProvider()
		if dp != nil {
			providerID = dp.ID()
		}
	}
	if providerID == "" {
		return "", fmt.Errorf("sub-agent: no provider configured")
	}

	req := provider.ChatRequest{
		ProviderID: providerID,
		Messages: []provider.Message{
			{Role: "system", Content: "You are a focused analysis sub-agent. Use read_file, search_files, list_directory, get_git_diff to investigate. Be thorough. After analysis, write a clear summary with specific findings, file paths, and code. Do NOT write or modify any files. Be concise."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   0,
	}

	allTools := []string{"read_file", "glob_files", "search_files", "list_directory", "get_git_diff"}
	req.Tools = buildSubAgentToolDefs(allTools)

	var result strings.Builder
	currentReq := req

	for loop := 0; loop < subAgentMaxLoops; loop++ {
		roundCtx, cancel := context.WithTimeout(ctx, 90*time.Second)

		eventCh, err := SubAgentProviderMgr.ChatStream(roundCtx, currentReq)
		if err != nil {
			cancel()
			break
		}

		var content string
		var toolCalls []provider.ToolCall
		hasTools := false

		for event := range eventCh {
			switch event.Type {
			case "data":
				content += event.Content
			case "error":
				cancel()
				return result.String(), nil
			case "tool_call":
				if len(event.ToolCalls) > 0 {
					toolCalls = append(toolCalls, event.ToolCalls...)
					hasTools = true
				} else if event.Name != "" {
					toolCalls = append(toolCalls, provider.ToolCall{
						ID:   fmt.Sprintf("sa_%d", time.Now().UnixNano()),
						Type: "function",
						Function: provider.ToolCallFunc{
							Name:      event.Name,
							Arguments: event.Args,
						},
					})
					hasTools = true
				}
			case "done":
				if !hasTools {
					result.WriteString(content)
					cancel()
					return strings.TrimSpace(result.String()), nil
				}
			}
		}

		// Record token usage for this sub-agent round
		if SubAgentMemoryStore != nil {
			tokensIn := 0
			for _, msg := range currentReq.Messages {
				tokensIn += estimateSubTokens(msg.Content)
			}
			tokensOut := estimateSubTokens(content)
			if tokensIn > 0 || tokensOut > 0 {
				go SubAgentMemoryStore.SaveTokenUsage(&memory.TokenUsageEntry{
					ID:             fmt.Sprintf("sa_%d", time.Now().UnixNano()),
					ConversationID: "",
					ProviderID:     currentReq.ProviderID,
					Model:          currentReq.Model,
					TokensIn:       tokensIn,
					TokensOut:      tokensOut,
					Cost:           0,
					CreatedAt:      time.Now().Format(time.RFC3339),
				})
			}
		}

		cancel() // done consuming events, release context

		if !hasTools {
			result.WriteString(content)
			return strings.TrimSpace(result.String()), nil
		}

		assistantMsg := provider.Message{Role: "assistant", Content: content}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}
		currentReq.Messages = append(currentReq.Messages, assistantMsg)

		for _, tc := range toolCalls {
			call := agent.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: make(map[string]any),
			}
			if tc.Function.Arguments != "" {
				json.Unmarshal([]byte(tc.Function.Arguments), &call.Args)
			}
			if SubAgentToolExec != nil {
				toolResult, err := SubAgentToolExec.Execute(ctx, call)
				if err != nil {
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role: "tool", Content: fmt.Sprintf("Error: %v", err), ToolCallID: tc.ID, Name: tc.Function.Name,
					})
				} else if toolResult != nil {
					rc := toolResult.Result
					if toolResult.Error != "" {
						rc = "Error: " + toolResult.Error
					}
					if len(rc) > 5000 {
						rc = rc[:5000] + "..."
					}
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role: "tool", Content: rc, ToolCallID: tc.ID, Name: tc.Function.Name,
					})
				}
			}
		}

	}

	summary := strings.TrimSpace(result.String())
	if summary == "" {
		summary = "(子智能体未产生有效输出)"
	}
	log.Printf("[SUB-AGENT] completed: %s", truncateStr(summary, 120))
	return "子智能体分析结果:\n" + summary, nil
}

func estimateSubTokens(text string) int {
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

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func buildSubAgentToolDefs(toolIDs []string) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(toolIDs))
	for _, id := range toolIDs {
		var desc string
		var params agent.ToolParameters
		if SubAgentToolExec != nil {
			if t, ok := SubAgentToolExec.Get(id); ok {
				desc = t.Description()
				params = t.Parameters()
			}
		}
		if desc == "" {
			desc = "Read-only tool: " + id
		}
		if params.Properties == nil {
			params = agent.ToolParameters{Type: "object", Properties: map[string]agent.ToolParamProp{}}
		}
		defs = append(defs, provider.ToolDefinition{
			Type: "function",
			Function: provider.ToolFunction{
				Name:        id,
				Description: desc,
				Parameters:  params,
			},
		})
	}
	return defs
}
