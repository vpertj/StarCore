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
	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

// --- DAG Router ---
//
// The Router is the execution engine for DAG plans. It:
//  1. Takes a Plan (built from Understander + TaskRouter decomposition)
//  2. Schedules ready nodes for parallel execution
//  3. Runs each node via a sub-agent call
//  4. Writes results to the Blackboard
//  5. Detects completion, handles failures via fallback nodes
//  6. Emits trace events for visualization
//
// Design principles:
//   - Max parallelism: concurrent nodes respect a semaphore
//   - Timeout: each node has a configurable timeout
//   - Blackboard: all node outputs go to the shared blackboard
//   - Failure handling: skip / fallback / stop per-node policy
//   - Traceable: every scheduling decision emits a trace event

// Router executes DAG plans.
type Router struct {
	agentReg       *agent.Registry
	providerMgr    *provider.Manager
	subAgentExec   *agent.ToolExecutor
	memoryStore    *memory.Store
	maxConcurrency int
	defaultTimeout time.Duration
}

// RouterOption configures a Router.
type RouterOption func(*Router)

// WithConcurrency sets the max parallel node execution.
func WithConcurrency(n int) RouterOption {
	return func(r *Router) {
		if n > 0 {
			r.maxConcurrency = n
		}
	}
}

// WithNodeTimeout sets the default node execution timeout.
func WithNodeTimeout(d time.Duration) RouterOption {
	return func(r *Router) {
		r.defaultTimeout = d
	}
}

// NewRouter creates a Router.
func NewRouter(agentReg *agent.Registry, providerMgr *provider.Manager, subAgentExec *agent.ToolExecutor, memoryStore *memory.Store, opts ...RouterOption) *Router {
	r := &Router{
		agentReg:       agentReg,
		providerMgr:    providerMgr,
		subAgentExec:   subAgentExec,
		memoryStore:    memoryStore,
		maxConcurrency: 3,
		defaultTimeout: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Execute runs a DAG plan to completion.
// Returns: final plan status, aggregated error (if any).
func (r *Router) Execute(ctx context.Context, plan *Plan, convID string) (PlanStatus, error) {
	// Validate first
	if err := plan.Validate(); err != nil {
		plan.status = PlanFailed
		return PlanFailed, fmt.Errorf("plan validation failed: %w", err)
	}

	// Initialize
	plan.status = PlanRunning
	plan.startTime = time.Now()
	plan.convID = convID
	plan.blackboard = GetBlackboard(convID)

	if plan.tracer != nil {
		plan.tracer.Event(EventDAGStart, StageRoute, convID, fmt.Sprintf("Plan '%s' started (%d nodes)", plan.Name, len(plan.Nodes)))
	}

	log.Printf("[DAG] Plan '%s' started: %d nodes", plan.Name, len(plan.Nodes))

	// Main scheduling loop
	sem := make(chan struct{}, r.maxConcurrency)

	for {
		select {
		case <-ctx.Done():
			plan.endTime = time.Now()
			plan.status = PlanFailed
			if plan.tracer != nil {
				plan.tracer.Event(EventDAGDone, StageRoute, convID, "cancelled")
			}
			return PlanFailed, ctx.Err()
		default:
		}

		// Check if all nodes are done (completed/failed/skipped)
		if r.allNodesSettled(plan) {
			break
		}

		// Get ready nodes
		ready := plan.ReadyNodes()
		if len(ready) == 0 {
			// No ready nodes → check for deadlock or wait
			if r.allNodesSettled(plan) {
				break
			}
			if r.hasDeadlock(plan) {
				plan.status = PlanFailed
				plan.endTime = time.Now()
				if plan.tracer != nil {
					plan.tracer.Event(EventDAGDone, StageRoute, convID, "deadlock detected")
				}
				return PlanFailed, fmt.Errorf("DAG deadlock: pending nodes with unsatisfiable dependencies")
			}
			// Some nodes still running → wait before re-checking
			time.Sleep(200 * time.Millisecond)
			continue
		}

		// Execute ready nodes in parallel
		var wg sync.WaitGroup
		for _, node := range ready {
			wg.Add(1)
			sem <- struct{}{} // acquire slot
			go func(n *Node) {
				defer wg.Done()
				defer func() { <-sem }() // release slot
				r.executeNode(ctx, plan, n)
			}(node)
		}
		wg.Wait()
	}

	// Determine final status
	plan.endTime = time.Now()
	status := r.determineFinalStatus(plan)
	plan.status = status

	if plan.tracer != nil {
		plan.tracer.Event(EventDAGDone, StageRoute, convID, fmt.Sprintf("Plan '%s' finished: %s", plan.Name, status))
	}

	log.Printf("[DAG] Plan '%s' completed in %v: %s", plan.Name, plan.Duration(), status)
	return status, nil
}

// executeNode runs a single node of the DAG.
func (r *Router) executeNode(ctx context.Context, plan *Plan, node *Node) {
	node.Status = NodeRunning
	node.StartTime = time.Now()

	if plan.tracer != nil {
		plan.tracer.Event(EventNodeStart, StageExecute, node.ID, node.Label)
	}

	log.Printf("[DAG] Node '%s' (%s) started", node.ID, node.Label)

	// Build the task for this node
	task := node.Task
	if task == "" {
		task = node.Description
	}

	// Inject blackboard context so the agent knows what other nodes have done
	if plan.blackboard != nil {
		bbSummary := plan.blackboard.Summary()
		if bbSummary != "" {
			task = fmt.Sprintf("## 上下文 (来自其他节点的发现)\n%s\n\n## 你的任务\n%s", bbSummary, task)
		}
	}

	// Determine tools for this node
	tools := node.Tools
	if len(tools) == 0 {
		switch node.Mode {
		case "build":
			tools = []string{"read_file", "write_file", "edit_file", "multi_edit", "glob_files",
				"search_files", "list_directory", "get_git_diff", "execute_command", "skill"}
		case "plan":
			tools = []string{"read_file", "glob_files", "search_files", "list_directory", "get_git_diff", "skill"}
		default:
			tools = []string{"read_file", "glob_files", "search_files", "list_directory", "skill"}
		}
	}

	// Execute via sub-agent
	timeout := r.defaultTimeout
	nodeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := r.runNodeSubAgent(nodeCtx, node, task, tools)

	node.EndTime = time.Now()

	if result.err != nil {
		node.Status = NodeFailed
		node.Error = result.err
		log.Printf("[DAG] Node '%s' FAILED: %v", node.ID, result.err)

		if plan.tracer != nil {
			plan.tracer.Event(EventNodeFailed, StageExecute, node.ID, result.err.Error())
		}

		// Try fallback node if configured
		if node.FallbackID != "" && plan.findNode(node.FallbackID) != nil {
			fallback := plan.findNode(node.FallbackID)
			if fallback.Status == NodePending {
				log.Printf("[DAG] Node '%s' trying fallback: %s", node.ID, node.FallbackID)
				r.executeNode(ctx, plan, fallback)
			}
		}
	} else {
		node.Status = NodeCompleted
		node.Output = result.output

		// Write result to blackboard
		if plan.blackboard != nil {
			plan.blackboard.Write(fmt.Sprintf("node:%s:result", node.ID), result.output, node.ID, []string{"result", node.Mode})
			plan.blackboard.Write(fmt.Sprintf("node:%s:summary", node.ID), truncateForBB(result.output, 500),
				node.ID, []string{"summary", node.Mode})
		}

		if plan.tracer != nil {
			plan.tracer.Event(EventNodeDone, StageExecute, node.ID, truncateForBB(result.output, 100))
		}

		log.Printf("[DAG] Node '%s' completed in %v", node.ID, node.Duration())
	}
}

// nodeResult holds the result of a node execution.
type nodeResult struct {
	output string
	err    error
}

// runNodeSubAgent runs a single node via a focused sub-agent call.
func (r *Router) runNodeSubAgent(ctx context.Context, node *Node, task string, tools []string) nodeResult {
	systemPrompt := node.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = r.buildNodeSystemPrompt(node)
	}

	// Resolve provider
	providerID := ""
	if r.providerMgr != nil {
		if dp := r.providerMgr.GetDefaultProvider(); dp != nil {
			providerID = dp.ID()
		}
	}
	if providerID == "" {
		return nodeResult{err: fmt.Errorf("no provider available for node execution")}
	}

	req := provider.ChatRequest{
		ProviderID: providerID,
		Messages: []provider.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: task},
		},
		Temperature: 0.2,
		MaxTokens:   0,
	}

	// Build tool definitions
	req.Tools = r.buildToolDefs(tools)

	// Run a focused loop
	return r.runFocusedLoop(ctx, req)
}

// runFocusedLoop runs a limited agent loop for a single node.
func (r *Router) runFocusedLoop(ctx context.Context, req provider.ChatRequest) nodeResult {
	maxLoops := 10

	currentReq := req
	var result strings.Builder

	for loop := 0; loop < maxLoops; loop++ {
		roundCtx, cancel := context.WithTimeout(ctx, 90*time.Second)

		eventCh, err := r.providerMgr.ChatStream(roundCtx, currentReq)
		if err != nil {
			cancel()
			return nodeResult{output: result.String(), err: err}
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
				return nodeResult{output: result.String(), err: fmt.Errorf("stream error")}
			case "tool_call":
				if len(event.ToolCalls) > 0 {
					toolCalls = append(toolCalls, event.ToolCalls...)
					hasTools = true
				}
			case "done":
				if !hasTools {
					result.WriteString(content)
					cancel()
					return nodeResult{output: strings.TrimSpace(result.String())}
				}
			}
		}
		cancel()

		if !hasTools {
			result.WriteString(content)
			return nodeResult{output: strings.TrimSpace(result.String())}
		}

		// Append assistant message + execute tools
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

			if r.subAgentExec != nil {
				toolResult, err := r.subAgentExec.Execute(ctx, call)
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
		summary = "(节点执行完成但未产生输出)"
	}
	return nodeResult{output: summary}
}

// buildNodeSystemPrompt builds a system prompt for a node.
func (r *Router) buildNodeSystemPrompt(node *Node) string {
	var sb strings.Builder
	sb.WriteString("You are executing a single step of a multi-step plan. ")
	sb.WriteString("Focus ONLY on this step. Be thorough but concise.\n\n")
	sb.WriteString(fmt.Sprintf("Step: %s\n", node.Label))
	if node.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", node.Description))
	}
	sb.WriteString("\nAfter completing your step, summarize what you did and any important findings. ")
	sb.WriteString("Do NOT attempt to do work beyond this step — other steps will be handled separately.\n")

	switch node.Mode {
	case "build":
		sb.WriteString("\nYou can read, write, and edit files as needed. Verify your changes work.")
	case "plan":
		sb.WriteString("\nThis is a READ-ONLY step. Do NOT write or modify files.")
	}

	return sb.String()
}

// buildToolDefs builds tool definitions for available tools.
func (r *Router) buildToolDefs(toolIDs []string) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(toolIDs))
	for _, id := range toolIDs {
		var desc string
		var params agent.ToolParameters
		if r.subAgentExec != nil {
			if t, ok := r.subAgentExec.Get(id); ok {
				desc = t.Description()
				params = t.Parameters()
			}
		}
		if desc == "" {
			desc = "Tool: " + id
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

// allNodesSettled returns true if no nodes are pending or running.
func (r *Router) allNodesSettled(plan *Plan) bool {
	for _, n := range plan.Nodes {
		if n.Status == NodePending || n.Status == NodeRunning {
			return false
		}
	}
	return true
}

// hasDeadlock detects if pending nodes have unmeetable dependencies.
func (r *Router) hasDeadlock(plan *Plan) bool {
	for _, n := range plan.Nodes {
		if n.Status != NodePending {
			continue
		}
		// A pending node with failed/skipped dependencies that doesn't have
		// "continue" policy means it can never run
		for _, depID := range n.Dependencies {
			depNode := plan.findNode(depID)
			if depNode != nil && (depNode.Status == NodeFailed || depNode.Status == NodeSkipped) {
				if n.OnFailure != "continue" {
					return true
				}
			}
		}
	}
	return false
}

// determineFinalStatus computes the plan's final status.
func (r *Router) determineFinalStatus(plan *Plan) PlanStatus {
	completed := 0
	failed := 0
	total := len(plan.Nodes)

	for _, n := range plan.Nodes {
		switch n.Status {
		case NodeCompleted:
			completed++
		case NodeFailed:
			failed++
		}
	}

	if completed == total {
		return PlanCompleted
	}
	if failed > 0 && completed > 0 {
		return PlanPartial
	}
	if failed > 0 && completed == 0 {
		return PlanFailed
	}
	return PlanCompleted
}

// SetTracer sets the tracer for plan execution.
func (r *Router) SetTracer(t Tracer) {
	// Router stores tracer; plans can access it via plan.tracer
	// This is a no-op at router level — plans carry their own tracer
	_ = t
}

// --- Helper functions ---

func truncateForBB(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ResolveProvider picks a provider for node execution.
// For now returns the default provider. Phase 5+ will use per-agent providers.
func ResolveProvider(providerMgr *provider.Manager) string {
	if providerMgr == nil {
		return ""
	}
	if dp := providerMgr.GetDefaultProvider(); dp != nil {
		return dp.ID()
	}
	return ""
}
