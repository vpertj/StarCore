package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

// SubAgentToolExec is set by app.go to give the SubAgent tool access to the tool executor.
var SubAgentToolExec *agent.ToolExecutor
var SubAgentProviderMgr *provider.Manager
var SubAgentMemoryStore *memory.Store

var subAgentProviderMu sync.RWMutex
var subAgentProviderID string

func SetSubAgentProviderID(id string) {
	subAgentProviderMu.Lock()
	subAgentProviderID = id
	subAgentProviderMu.Unlock()
}

func GetSubAgentProviderID() string {
	subAgentProviderMu.RLock()
	defer subAgentProviderMu.RUnlock()
	return subAgentProviderID
}

type SubAgentTool struct{}

func NewSubAgentTool() *SubAgentTool { return &SubAgentTool{} }

func (t *SubAgentTool) ID() string             { return "sub_agent" }
func (t *SubAgentTool) Name() string           { return "Sub Agent" }
func (t *SubAgentTool) RequiresApproval() bool { return false }

func (t *SubAgentTool) Description() string {
	return "派生子智能体处理独立任务。子智能体有自己的上下文，不会污染主对话。适合：代码审查、安全审计、跨文件追踪等需要大量阅读的独立任务。"
}

func (t *SubAgentTool) Parameters() agent.ToolParameters {
	return agent.ToolParameters{
		Type: "object",
		Properties: map[string]agent.ToolParamProp{
			"task":     {Type: "string", Description: "Description of what to analyze/implement. For parallel tasks, describe each task separated by '---'"},
			"context":  {Type: "string", Description: "Optional: file paths, code snippets, or background info"},
			"mode":     {Type: "string", Description: "Optional: 'analyze' (read-only, default) or 'build' (can write/execute)"},
			"parallel": {Type: "boolean", Description: "Optional: set true to split task by '---' and run sub-agents in parallel"},
		},
		Required: []string{"task"},
	}
}

const subAgentMaxLoops = 6

var (
	subAgentPoolMu    sync.Mutex
	subAgentActive    int
	subAgentMaxActive = 4
)

func canSpawnSubAgent() bool {
	subAgentPoolMu.Lock()
	defer subAgentPoolMu.Unlock()
	return subAgentActive < subAgentMaxActive
}

func acquireSubAgentSlot() bool {
	subAgentPoolMu.Lock()
	defer subAgentPoolMu.Unlock()
	if subAgentActive >= subAgentMaxActive {
		return false
	}
	subAgentActive++
	return true
}

func releaseSubAgentSlot() {
	subAgentPoolMu.Lock()
	defer subAgentPoolMu.Unlock()
	if subAgentActive > 0 {
		subAgentActive--
	}
}

func SetSubAgentMaxConcurrency(n int) {
	subAgentPoolMu.Lock()
	defer subAgentPoolMu.Unlock()
	if n > 0 {
		subAgentMaxActive = n
	}
}

func (t *SubAgentTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	if SubAgentProviderMgr == nil {
		return "", fmt.Errorf("sub-agent: provider not available")
	}

	task, ok := args["task"].(string)
	task = strings.TrimSpace(task)
	if !ok || task == "" {
		return "", fmt.Errorf("task description is required")
	}
	extContext, _ := args["context"].(string)
	mode, _ := args["mode"].(string)
	if mode == "" {
		mode = "analyze"
	}
	parallel, _ := args["parallel"].(bool)

	providerID := GetSubAgentProviderID()
	if providerID == "" && SubAgentProviderMgr != nil {
		dp := SubAgentProviderMgr.GetDefaultProvider()
		if dp != nil {
			providerID = dp.ID()
		}
	}
	if providerID == "" {
		return "", fmt.Errorf("sub-agent: no provider configured")
	}

	if parallel && strings.Contains(task, "---") {
		tasks := strings.Split(task, "---")
		type taskResult struct {
			index  int
			result string
			err    error
		}
		ch := make(chan taskResult, len(tasks))
		for i, t := range tasks {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			go func(idx int, taskDesc string) {
				result, err := runSingleSubAgent(ctx, taskDesc, extContext, mode, providerID)
				ch <- taskResult{index: idx, result: result, err: err}
			}(i, t)
		}

		results := make([]string, len(tasks))
		for i := 0; i < len(tasks); i++ {
			if strings.TrimSpace(tasks[i]) == "" {
				continue
			}
			tr := <-ch
			if tr.err != nil {
				results[tr.index] = fmt.Sprintf("(子智能体%d错误: %v)", tr.index+1, tr.err)
			} else {
				results[tr.index] = tr.result
			}
		}

		var combined strings.Builder
		combined.WriteString("并行子智能体执行结果:\n\n")
		for i, r := range results {
			if r == "" {
				continue
			}
			combined.WriteString(fmt.Sprintf("### 子智能体 %d\n%s\n\n", i+1, r))
		}
		return combined.String(), nil
	}

	return runSingleSubAgent(ctx, task, extContext, mode, providerID)
}

func runSingleSubAgent(ctx context.Context, task string, extContext string, mode string, providerID string) (string, error) {
	if !acquireSubAgentSlot() {
		return "", fmt.Errorf("sub-agent: concurrency limit reached (%d active)", subAgentMaxActive)
	}
	defer releaseSubAgentSlot()

	log.Printf("[SUB-AGENT] spawned (mode=%s): %s", mode, truncateStr(task, 80))

	prompt := task
	if extContext != "" {
		prompt += "\n\nContext:\n" + extContext
	}

	var systemPrompt string
	var tools []string

	switch mode {
	case "build":
		systemPrompt = "You are a focused implementation sub-agent. You can read files, write files, edit files, search code, execute commands, and invoke skills. Implement the task precisely. After implementation, verify your changes work correctly. Be thorough and concise."
		tools = []string{"read_file", "write_file", "edit_file", "glob_files", "search_files", "list_directory", "get_git_diff", "execute_command", "skill"}
	default:
		systemPrompt = "You are a focused analysis sub-agent. Use read_file, search_files, list_directory, get_git_diff to investigate. You can also invoke skills for specialized analysis. Be thorough. After analysis, write a clear summary with specific findings, file paths, and code. Do NOT write or modify any files. Be concise."
		tools = []string{"read_file", "glob_files", "search_files", "list_directory", "get_git_diff", "skill"}
	}

	req := provider.ChatRequest{
		ProviderID: providerID,
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
		MaxTokens:   0,
	}

	req.Tools = buildSubAgentToolDefs(tools)

	var result strings.Builder
	currentReq := req

	maxLoops := subAgentMaxLoops
	if mode == "build" {
		maxLoops = 15
	}

	for loop := 0; loop < maxLoops; loop++ {
		roundCtx, cancel := context.WithTimeout(ctx, 90*time.Second)

		// Log progress (sub-agent progress is visible in parent's tool result)
		if loop > 0 {
			taskPreview := task
			if len(taskPreview) > 50 {
				taskPreview = taskPreview[:50] + "..."
			}
			log.Printf("[sub-agent] round %d/%d: %s", loop+1, maxLoops, taskPreview)
		}

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

		cancel()

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
			// Check approval for dangerous tools in sub-agents
			needsApproval := tc.Function.Name == "write_file" || tc.Function.Name == "edit_file" ||
				tc.Function.Name == "execute_command" || tc.Function.Name == "git_commit"
			if needsApproval && SubAgentToolExec != nil {
				if !SubAgentToolExec.IsAutoApproved(tc.Function.Name) {
					// Sub-agent write/exec requires parent approval
					currentReq.Messages = append(currentReq.Messages, provider.Message{
						Role:    "tool",
						Content: fmt.Sprintf("Tool '%s' requires approval. Please ask the user to approve this action.", tc.Function.Name),
						ToolCallID: tc.ID,
						Name:    tc.Function.Name,
					})
					continue
				}
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
					maxChars := 5000
					if mode == "build" {
						maxChars = 8000
					}
					if len(rc) > maxChars {
						rc = rc[:maxChars] + "..."
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
