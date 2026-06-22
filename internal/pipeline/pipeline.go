package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"StarCore/internal/agent"
	"StarCore/internal/memory"
	"StarCore/internal/provider"
)

type StageStatus string

const (
	StagePending   StageStatus = "pending"
	StageRunning   StageStatus = "running"
	StageCompleted StageStatus = "completed"
	StageFailed    StageStatus = "failed"
	StageSkipped   StageStatus = "skipped"
)

type PipelineStatus string

const (
	PipelinePending   PipelineStatus = "pending"
	PipelineRunning   PipelineStatus = "running"
	PipelineCompleted PipelineStatus = "completed"
	PipelineFailed    PipelineStatus = "failed"
	PipelineCancelled PipelineStatus = "cancelled"
)

type StageResult struct {
	StageID   string      `json:"stageId"`
	AgentID   string      `json:"agentId"`
	Status    StageStatus `json:"status"`
	Output    string      `json:"output"`
	Artifacts []Artifact  `json:"artifacts,omitempty"`
	Error     string      `json:"error,omitempty"`
	StartedAt string      `json:"startedAt,omitempty"`
	EndedAt   string      `json:"endedAt,omitempty"`
	TokensIn  int         `json:"tokensIn"`
	TokensOut int         `json:"tokensOut"`
}

type Artifact struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Path    string `json:"path,omitempty"`
}

type Stage struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	AgentID      string   `json:"agentId"`
	Description  string   `json:"description"`
	Mode         string   `json:"mode"`
	DependsOn    []string `json:"dependsOn,omitempty"`
	Parallel     bool     `json:"parallel,omitempty"`
	MaxLoops     int      `json:"maxLoops,omitempty"`
	Optional     bool     `json:"optional,omitempty"`
	RequiresGate bool     `json:"requiresGate,omitempty"`
}

type Pipeline struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Stages      []Stage `json:"stages"`
}

type PipelineRun struct {
	PipelineID   string                  `json:"pipelineId"`
	Status       PipelineStatus          `json:"status"`
	StageResults map[string]*StageResult `json:"stageResults"`
	StartedAt    string                  `json:"startedAt,omitempty"`
	EndedAt      string                  `json:"endedAt,omitempty"`
	Error        string                  `json:"error,omitempty"`
	Snapshot     []byte                  `json:"snapshot,omitempty"`
	mu           sync.Mutex
}

func (r *PipelineRun) SetStageResult(id string, result *StageResult) {
	r.mu.Lock()
	r.StageResults[id] = result
	r.mu.Unlock()
}

func (r *PipelineRun) GetStageResult(id string) *StageResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.StageResults[id]
}

func (r *PipelineRun) GetAllStageResults() map[string]*StageResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make(map[string]*StageResult, len(r.StageResults))
	for k, v := range r.StageResults {
		cp[k] = v
	}
	return cp
}

type GateStatus string

const (
	GatePending  GateStatus = "pending"
	GateApproved GateStatus = "approved"
	GateRejected GateStatus = "rejected"
)

type GateRequest struct {
	StageID   string     `json:"stageId"`
	StageName string     `json:"stageName"`
	Output    string     `json:"output"`
	Status    GateStatus `json:"status"`
}

type EmitFunc func(event string, data interface{})

type Executor struct {
	providerMgr *provider.Manager
	toolExec    *agent.ToolExecutor
	agentReg    *agent.Registry
	memoryStore *memory.Store
	emitFn      EmitFunc
	mu          sync.Mutex
	cancel      context.CancelFunc
	gateCh      chan GateRequest
	persistDir  string
}

func NewExecutor(
	providerMgr *provider.Manager,
	toolExec *agent.ToolExecutor,
	agentReg *agent.Registry,
	memoryStore *memory.Store,
	emitFn EmitFunc,
) *Executor {
	return &Executor{
		providerMgr: providerMgr,
		toolExec:    toolExec,
		agentReg:    agentReg,
		memoryStore: memoryStore,
		emitFn:      emitFn,
		gateCh:      make(chan GateRequest, 16),
	}
}

func (e *Executor) SetPersistDir(dir string) {
	e.persistDir = dir
}

func (e *Executor) GateChannel() <-chan GateRequest {
	return e.gateCh
}

func (e *Executor) ApproveGate(stageID string) {
	e.gateCh <- GateRequest{StageID: stageID, Status: GateApproved}
}

func (e *Executor) RejectGate(stageID string) {
	e.gateCh <- GateRequest{StageID: stageID, Status: GateRejected}
}

func (e *Executor) Run(ctx context.Context, pipeline Pipeline, userInput string, projectPath string) (*PipelineRun, error) {
	e.mu.Lock()
	pipelineCtx, cancel := context.WithCancel(ctx)
	e.cancel = cancel
	e.mu.Unlock()

	run := &PipelineRun{
		PipelineID:   pipeline.ID,
		Status:       PipelineRunning,
		StageResults: make(map[string]*StageResult),
		StartedAt:    time.Now().Format(time.RFC3339),
	}

	for _, stage := range pipeline.Stages {
		run.StageResults[stage.ID] = &StageResult{
			StageID: stage.ID,
			AgentID: stage.AgentID,
			Status:  StagePending,
		}
	}

	e.emitFn("pipeline:start", map[string]any{
		"pipelineId": pipeline.ID,
		"name":       pipeline.Name,
		"stages":     len(pipeline.Stages),
	})

	defer func() {
		if r := recover(); r != nil {
			log.Printf("Pipeline panic: %v", r)
			run.Status = PipelineFailed
			run.Error = fmt.Sprintf("内部错误: %v", r)
		}
		run.EndedAt = time.Now().Format(time.RFC3339)
		e.persistRun(run)
		e.emitFn("pipeline:done", run)
	}()

	for i := 0; i < len(pipeline.Stages); {
		select {
		case <-pipelineCtx.Done():
			run.Status = PipelineCancelled
			return run, pipelineCtx.Err()
		default:
		}

		readyStages := e.findReadyStages(pipeline.Stages, run, i)
		if len(readyStages) == 0 {
			if e.allStagesDone(pipeline.Stages, run) {
				break
			}
			remaining := e.countRemaining(pipeline.Stages, run)
			if remaining > 0 {
				run.Status = PipelineFailed
				run.Error = "流水线死锁：存在未满足依赖的阶段"
				return run, fmt.Errorf("pipeline deadlock")
			}
			break
		}

		if len(readyStages) == 1 && !readyStages[0].Parallel {
			stage := readyStages[0]
			result := e.executeStage(pipelineCtx, stage, run, userInput, projectPath)
			run.StageResults[stage.ID] = result
			e.emitFn("pipeline:stage_done", result)
			if result.Status == StageFailed && !stage.Optional {
				run.Status = PipelineFailed
				run.Error = fmt.Sprintf("阶段 %s (%s) 失败: %s", stage.ID, stage.Name, result.Error)
				e.cancelRemaining(pipeline.Stages, run)
				return run, fmt.Errorf(run.Error)
			}
			i++
		} else {
			var wg sync.WaitGroup
			var resultsMu sync.Mutex
			for _, stage := range readyStages {
				wg.Add(1)
				go func(s Stage) {
					defer wg.Done()
					result := e.executeStage(pipelineCtx, s, run, userInput, projectPath)
					resultsMu.Lock()
					run.StageResults[s.ID] = result
					resultsMu.Unlock()
					e.emitFn("pipeline:stage_done", result)
				}(stage)
			}
			wg.Wait()

			for _, stage := range readyStages {
				result := run.StageResults[stage.ID]
				if result.Status == StageFailed && !stage.Optional {
					run.Status = PipelineFailed
					run.Error = fmt.Sprintf("阶段 %s (%s) 失败: %s", stage.ID, stage.Name, result.Error)
					e.cancelRemaining(pipeline.Stages, run)
					return run, fmt.Errorf(run.Error)
				}
			}
			i += len(readyStages)
		}
	}

	if e.allStagesCompleted(pipeline.Stages, run) {
		run.Status = PipelineCompleted
	} else {
		run.Status = PipelineFailed
		run.Error = "部分阶段未完成"
	}

	return run, nil
}

func (e *Executor) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
		e.cancel = nil
	}
}

func (e *Executor) findReadyStages(stages []Stage, run *PipelineRun, minIndex int) []Stage {
	var ready []Stage
	for _, stage := range stages {
		sr, exists := run.StageResults[stage.ID]
		if !exists || sr.Status != StagePending {
			continue
		}
		allDepsMet := true
		for _, depID := range stage.DependsOn {
			depResult, depExists := run.StageResults[depID]
			if !depExists || depResult.Status != StageCompleted {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			ready = append(ready, stage)
		}
	}
	return ready
}

func (e *Executor) executeStage(ctx context.Context, stage Stage, run *PipelineRun, userInput string, projectPath string) *StageResult {
	result := &StageResult{
		StageID:   stage.ID,
		AgentID:   stage.AgentID,
		Status:    StageRunning,
		StartedAt: time.Now().Format(time.RFC3339),
	}

	e.emitFn("pipeline:stage_start", map[string]any{
		"stageId": stage.ID,
		"name":    stage.Name,
		"agentId": stage.AgentID,
	})

	ag, ok := e.agentReg.Get(stage.AgentID)
	if !ok {
		result.Status = StageFailed
		result.Error = fmt.Sprintf("Agent %s not found", stage.AgentID)
		result.EndedAt = time.Now().Format(time.RFC3339)
		return result
	}

	stagePrompt := e.buildStagePrompt(stage, run, userInput)

	providerID := ""
	dp := e.providerMgr.GetDefaultProvider()
	if dp != nil {
		providerID = dp.ID()
	}
	if providerID == "" {
		result.Status = StageFailed
		result.Error = "no provider configured"
		result.EndedAt = time.Now().Format(time.RFC3339)
		return result
	}

	mode := stage.Mode
	if mode == "" {
		mode = "build"
	}

	maxLoops := stage.MaxLoops
	if maxLoops == 0 {
		maxLoops = 30
	}

	req := provider.ChatRequest{
		ProviderID: providerID,
		Messages: []provider.Message{
			{Role: "system", Content: ag.SystemPrompt},
			{Role: "user", Content: stagePrompt},
		},
		Mode:        mode,
		Stream:      true,
		ProjectPath: projectPath,
		Temperature: 0.3,
	}

	tools := ag.Tools
	if mode == "chat" || mode == "plan" {
		tools = []string{}
		for _, t := range ag.Tools {
			if tool, ok := e.toolExec.Get(t); ok && !tool.RequiresApproval() {
				tools = append(tools, t)
			}
		}
	}
	if len(tools) > 0 {
		req.Tools = e.buildToolDefinitions(tools)
	}

	var output strings.Builder
	var totalTokensIn, totalTokensOut int

	for loop := 0; loop < maxLoops; loop++ {
		select {
		case <-ctx.Done():
			result.Status = StageFailed
			result.Error = "cancelled"
			result.Output = output.String()
			result.EndedAt = time.Now().Format(time.RFC3339)
			return result
		default:
		}

		roundCtx, roundCancel := context.WithTimeout(ctx, 180*time.Second)
		eventCh, err := e.providerMgr.ChatStream(roundCtx, req)
		if err != nil {
			roundCancel()
			if loop == 0 {
				result.Status = StageFailed
				result.Error = err.Error()
				result.EndedAt = time.Now().Format(time.RFC3339)
				return result
			}
			break
		}

		var content string
		var toolCalls []provider.ToolCall
		hasTools := false

		for event := range eventCh {
			switch event.Type {
			case "data":
				content += event.Content
				output.WriteString(event.Content)
				e.emitFn("pipeline:stage_data", map[string]any{
					"stageId": stage.ID,
					"content": event.Content,
				})
			case "tool_call":
				if len(event.ToolCalls) > 0 {
					toolCalls = append(toolCalls, event.ToolCalls...)
					hasTools = true
				} else if event.Name != "" {
					toolCalls = append(toolCalls, provider.ToolCall{
						ID:   fmt.Sprintf("p_%d", time.Now().UnixNano()),
						Type: "function",
						Function: provider.ToolCallFunc{
							Name:      event.Name,
							Arguments: event.Args,
						},
					})
					hasTools = true
				}
			case "error":
				roundCancel()
				result.Status = StageFailed
				result.Error = event.Content
				result.Output = output.String()
				result.EndedAt = time.Now().Format(time.RFC3339)
				return result
			case "done":
				if !hasTools && content == "" {
					break
				}
			}
		}

		totalTokensIn += estimateTokensForMessages(req.Messages)
		totalTokensOut += estimateTokensSimple(content)

		roundCancel()

		if !hasTools {
			break
		}

		assistantMsg := provider.Message{Role: "assistant", Content: content}
		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
		}
		req.Messages = append(req.Messages, assistantMsg)

		for _, tc := range toolCalls {
			call := agent.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: make(map[string]any),
			}
			if tc.Function.Arguments != "" {
				json.Unmarshal([]byte(tc.Function.Arguments), &call.Args)
			}
			toolResult, err := e.toolExec.Execute(ctx, call)
			if err != nil {
				req.Messages = append(req.Messages, provider.Message{
					Role: "tool", Content: fmt.Sprintf("Error: %v", err), ToolCallID: tc.ID, Name: tc.Function.Name,
				})
			} else if toolResult != nil {
				rc := toolResult.Result
				if toolResult.Error != "" {
					rc = "Error: " + toolResult.Error
				}
				if len(rc) > 8000 {
					rc = rc[:8000] + "... [truncated]"
				}
				req.Messages = append(req.Messages, provider.Message{
					Role: "tool", Content: rc, ToolCallID: tc.ID, Name: tc.Function.Name,
				})
			}
		}
	}

	artifacts := e.extractArtifacts(output.String())

	if stage.RequiresGate {
		gateReq := GateRequest{
			StageID:   stage.ID,
			StageName: stage.Name,
			Output:    output.String(),
			Status:    GatePending,
		}
		e.emitFn("pipeline:gate", gateReq)
		select {
		case e.gateCh <- gateReq:
		default:
		}
		gateTimeout := time.NewTimer(10 * time.Minute)
		defer gateTimeout.Stop()
	gateLoop:
		for {
			select {
			case <-ctx.Done():
				result.Status = StageFailed
				result.Error = "cancelled during gate review"
				result.Output = output.String()
				result.EndedAt = time.Now().Format(time.RFC3339)
				return result
			case resp := <-e.gateCh:
				if resp.StageID == stage.ID {
					if resp.Status == GateRejected {
						result.Status = StageFailed
						result.Error = "gate rejected by user"
						result.Output = output.String()
						result.EndedAt = time.Now().Format(time.RFC3339)
						return result
					}
					break gateLoop
				}
			case <-gateTimeout.C:
				result.Status = StageFailed
				result.Error = "gate review timed out"
				result.Output = output.String()
				result.EndedAt = time.Now().Format(time.RFC3339)
				return result
			}
		}
	}

	artifacts = e.extractArtifacts(output.String())

	result.Status = StageCompleted
	result.Output = output.String()
	result.Artifacts = artifacts
	result.EndedAt = time.Now().Format(time.RFC3339)
	result.TokensIn = totalTokensIn
	result.TokensOut = totalTokensOut

	if e.memoryStore != nil {
		go e.memoryStore.SaveTokenUsage(&memory.TokenUsageEntry{
			ID:             fmt.Sprintf("pipe_%d", time.Now().UnixNano()),
			ConversationID: fmt.Sprintf("pipeline_%s_%s", run.PipelineID, stage.ID),
			ProviderID:     providerID,
			TokensIn:       totalTokensIn,
			TokensOut:      totalTokensOut,
			Cost:           0,
			CreatedAt:      time.Now().Format(time.RFC3339),
		})
	}

	return result
}

func (e *Executor) buildStagePrompt(stage Stage, run *PipelineRun, userInput string) string {
	prompt := fmt.Sprintf("## 任务: %s\n\n%s\n\n### 用户需求\n%s", stage.Name, stage.Description, userInput)

	if len(stage.DependsOn) > 0 {
		prompt += "\n\n### 前序阶段输出\n"
		for _, depID := range stage.DependsOn {
			if depResult, ok := run.StageResults[depID]; ok && depResult.Status == StageCompleted {
				prompt += fmt.Sprintf("\n**阶段 %s 输出:**\n%s\n", depID, depResult.Output)
				if len(depResult.Artifacts) > 0 {
					prompt += "\n**产出物:**\n"
					for _, a := range depResult.Artifacts {
						prompt += fmt.Sprintf("- %s (%s)\n", a.Name, a.Type)
						if a.Content != "" {
							maxLen := 4000
							content := a.Content
							if len(content) > maxLen {
								content = content[:maxLen] + "\n... [truncated]"
							}
							prompt += fmt.Sprintf("```\n%s\n```\n", content)
						}
					}
				}
			}
		}
	}

	return prompt
}

func (e *Executor) buildToolDefinitions(toolIDs []string) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(toolIDs))
	for _, id := range toolIDs {
		t, ok := e.toolExec.Get(id)
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

func (e *Executor) extractArtifacts(output string) []Artifact {
	var artifacts []Artifact
	codeBlockRe := regexp.MustCompile("```(?:markdown|md)?\\s*(?:#(?:.*?))?\n(.*?)```")
	matches := codeBlockRe.FindAllStringSubmatch(output, -1)
	for i, m := range matches {
		if len(m) >= 2 && len(m[1]) > 100 {
			artifacts = append(artifacts, Artifact{
				Type:    "document",
				Name:    fmt.Sprintf("artifact_%d", i+1),
				Content: m[1],
			})
		}
		if len(artifacts) >= 5 {
			break
		}
	}
	return artifacts
}

func (e *Executor) allStagesDone(stages []Stage, run *PipelineRun) bool {
	for _, stage := range stages {
		sr, exists := run.StageResults[stage.ID]
		if !exists {
			return false
		}
		if sr.Status == StagePending || sr.Status == StageRunning {
			return false
		}
	}
	return true
}

func (e *Executor) allStagesCompleted(stages []Stage, run *PipelineRun) bool {
	for _, stage := range stages {
		sr, exists := run.StageResults[stage.ID]
		if !exists || sr.Status != StageCompleted {
			if stage.Optional && (sr.Status == StageSkipped || sr.Status == StageFailed) {
				continue
			}
			return false
		}
	}
	return true
}

func (e *Executor) countRemaining(stages []Stage, run *PipelineRun) int {
	count := 0
	for _, stage := range stages {
		sr, exists := run.StageResults[stage.ID]
		if !exists || sr.Status == StagePending || sr.Status == StageRunning {
			count++
		}
	}
	return count
}

func (e *Executor) cancelRemaining(stages []Stage, run *PipelineRun) {
	for _, stage := range stages {
		sr, exists := run.StageResults[stage.ID]
		if exists && sr.Status == StagePending {
			run.StageResults[stage.ID].Status = StageSkipped
		}
	}
}

func estimateTokensSimple(text string) int {
	if len(text) == 0 {
		return 0
	}
	return len(text)/4 + 1
}

func estimateTokensForMessages(msgs []provider.Message) int {
	total := 0
	for _, msg := range msgs {
		total += estimateTokensSimple(msg.Content)
	}
	return total
}

func (e *Executor) persistRun(run *PipelineRun) {
	if e.persistDir == "" {
		return
	}
	os.MkdirAll(e.persistDir, 0755)
	data, err := json.Marshal(run)
	if err != nil {
		log.Printf("Pipeline persist marshal error: %v", err)
		return
	}
	safeTime := strings.ReplaceAll(run.StartedAt, ":", "-")
	safeTime = strings.ReplaceAll(safeTime, "T", "_")
	path := filepath.Join(e.persistDir, fmt.Sprintf("pipeline_%s_%s.json", run.PipelineID, safeTime))
	if err := os.WriteFile(path, data, 0644); err != nil {
		log.Printf("Pipeline persist write error: %v", err)
	}
}

func (e *Executor) LoadRun(pipelineID, startedAt string) (*PipelineRun, error) {
	if e.persistDir == "" {
		return nil, fmt.Errorf("persist dir not configured")
	}
	safeTime := strings.ReplaceAll(startedAt, ":", "-")
	safeTime = strings.ReplaceAll(safeTime, "T", "_")
	path := filepath.Join(e.persistDir, fmt.Sprintf("pipeline_%s_%s.json", pipelineID, safeTime))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var run PipelineRun
	if err := json.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}
