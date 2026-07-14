package ai

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- DAG Pipeline Plan ---
//
// A Plan is a Directed Acyclic Graph of execution steps (nodes).
// Unlike linear execution (agent loop), a DAG allows:
//   - Parallel execution of independent nodes
//   - Conditional branching (different nodes based on blackboard state)
//   - Clear dependency declarations (node A needs node B's output)
//   - Fallback nodes (if primary fails, try alternative)
//   - Early termination (skip remaining if critical failure)
//
// The DAG is built from the Understander's output + TaskRouter's decomposition.
// It can be visualized in the frontend (Phase 7).

// NodeStatus represents the current state of a DAG node.
type NodeStatus int

const (
	NodePending   NodeStatus = iota // not started
	NodeRunning                     // executing
	NodeCompleted                   // finished successfully
	NodeFailed                      // errored
	NodeSkipped                     // skipped (condition not met or dependency failed)
)

func (s NodeStatus) String() string {
	switch s {
	case NodePending:
		return "pending"
	case NodeRunning:
		return "running"
	case NodeCompleted:
		return "completed"
	case NodeFailed:
		return "failed"
	case NodeSkipped:
		return "skipped"
	}
	return "unknown"
}

// Node is a single unit of work in the DAG.
type Node struct {
	ID           string                 // unique within plan
	Label        string                 // human-readable label
	Description  string                 // what this node does
	AgentID      string                 // which agent to use (empty = any)
	Mode         string                 // chat/plan/build
	Task         string                 // task description for the agent
	SystemPrompt string                 // override system prompt
	Tools        []string               // tool allowlist (empty = inherit from agent)
	Dependencies []string               // IDs of nodes that must complete before this
	Condition    func(*Blackboard) bool //:nil = always execute
	FallbackID   string                 // node to run if this fails
	OnFailure    string                 // "stop" | "skip" | "continue"

	// Runtime state (set during execution)
	Status    NodeStatus
	Output    string
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

// Duration returns how long the node took to execute.
func (n *Node) Duration() time.Duration {
	if n.StartTime.IsZero() {
		return 0
	}
	if n.EndTime.IsZero() {
		return time.Since(n.StartTime)
	}
	return n.EndTime.Sub(n.StartTime)
}

// Plan is a DAG of nodes.
type Plan struct {
	ID          string
	Name        string
	Description string
	Nodes       []*Node
	convID      string

	// Runtime
	mu         sync.Mutex
	status     PlanStatus
	startTime  time.Time
	endTime    time.Time
	blackboard *Blackboard
	tracer     Tracer
}

// PlanStatus represents the overall plan execution state.
type PlanStatus int

const (
	PlanPending PlanStatus = iota
	PlanRunning
	PlanCompleted
	PlanFailed
	PlanPartial // some nodes failed but plan continued
)

func (s PlanStatus) String() string {
	switch s {
	case PlanPending:
		return "pending"
	case PlanRunning:
		return "running"
	case PlanCompleted:
		return "completed"
	case PlanFailed:
		return "failed"
	case PlanPartial:
		return "partial"
	}
	return "unknown"
}

// NewPlan creates a new plan.
func NewPlan(id, name, description string) *Plan {
	return &Plan{
		ID:          id,
		Name:        name,
		Description: description,
		Nodes:       make([]*Node, 0),
		status:      PlanPending,
	}
}

// AddNode adds a node to the plan.
func (p *Plan) AddNode(node *Node) {
	p.Nodes = append(p.Nodes, node)
}

// Validate checks the plan for cycles, invalid dependencies, etc.
func (p *Plan) Validate() error {
	// Check for duplicate IDs
	ids := make(map[string]bool)
	for _, n := range p.Nodes {
		if ids[n.ID] {
			return fmt.Errorf("duplicate node ID: %s", n.ID)
		}
		ids[n.ID] = true
	}

	// Check dependencies exist
	for _, n := range p.Nodes {
		for _, dep := range n.Dependencies {
			if !ids[dep] {
				return fmt.Errorf("node %s depends on non-existent node: %s", n.ID, dep)
			}
		}
	}

	// Check for cycles using DFS.
	// Build adjacency list: node → nodes that depend on it (forward edges).
	// A cycle exists if following forward edges revisits a node on the current path.
	adj := make(map[string][]string)
	for _, n := range p.Nodes {
		for _, dep := range n.Dependencies {
			adj[dep] = append(adj[dep], n.ID)
		}
	}

	visited := make(map[string]int) // 0=unvisited, 1=in-progress, 2=done
	var visit func(id string) error
	visit = func(id string) error {
		state := visited[id]
		if state == 1 {
			return fmt.Errorf("cycle detected at node: %s", id)
		}
		if state == 2 {
			return nil
		}
		visited[id] = 1

		// Visit dependents (nodes that depend on this one)
		for _, next := range adj[id] {
			if err := visit(next); err != nil {
				return err
			}
		}

		visited[id] = 2
		return nil
	}

	for _, n := range p.Nodes {
		if visited[n.ID] == 0 {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

// ReadyNodes returns nodes whose dependencies are all completed (or whose
// failed dependencies are tolerated by the OnFailure policy).
func (p *Plan) ReadyNodes() []*Node {
	p.mu.Lock()
	defer p.mu.Unlock()

	var ready []*Node
	for _, n := range p.Nodes {
		if n.Status != NodePending {
			continue
		}

		// Evaluate each dependency
		allDepsMet := true
		anyDepFailed := false
		for _, depID := range n.Dependencies {
			depNode := p.findNode(depID)
			if depNode == nil {
				allDepsMet = false
				break
			}
			switch depNode.Status {
			case NodeCompleted:
				// Good — this dep is satisfied
			case NodeFailed, NodeSkipped:
				anyDepFailed = true
				// If OnFailure is "continue", failed deps still count as met
				if n.OnFailure != "continue" {
					allDepsMet = false
				}
			default:
				// Pending or running — not met yet
				allDepsMet = false
			}
		}

		if !allDepsMet {
			continue
		}

		// Handle failure: if deps failed but policy allows continuation, proceed
		if anyDepFailed && n.OnFailure != "continue" {
			// Policy is "stop" or empty → skip this node
			n.Status = NodeSkipped
			continue
		}

		// Check condition (do NOT permanently skip — conditions may change as
		// other nodes write to the blackboard)
		if n.Condition != nil && p.blackboard != nil {
			if !n.Condition(p.blackboard) {
				continue // not ready yet, but may become ready later
			}
		}

		ready = append(ready, n)
	}

	return ready
}

// findNode finds a node by ID.
func (p *Plan) findNode(id string) *Node {
	for _, n := range p.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// CompletedCount returns the number of completed nodes.
func (p *Plan) CompletedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, n := range p.Nodes {
		if n.Status == NodeCompleted {
			count++
		}
	}
	return count
}

// TotalCount returns total nodes.
func (p *Plan) TotalCount() int {
	return len(p.Nodes)
}

// StatusSummary returns a compact status string for the frontend.
func (p *Plan) StatusSummary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	counts := make(map[NodeStatus]int)
	for _, n := range p.Nodes {
		counts[n.Status]++
	}

	var parts []string
	for _, s := range []NodeStatus{NodeRunning, NodeCompleted, NodeFailed, NodeSkipped, NodePending} {
		if counts[s] > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", s, counts[s]))
		}
	}
	return strings.Join(parts, " ")
}

// Duration returns total plan execution time.
func (p *Plan) Duration() time.Duration {
	if p.startTime.IsZero() {
		return 0
	}
	if p.endTime.IsZero() {
		return time.Since(p.startTime)
	}
	return p.endTime.Sub(p.startTime)
}

// Summary returns a compact text summary of the plan (for frontend display).
func (p *Plan) Summary() map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()

	nodeSummaries := make([]map[string]any, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodeSummaries = append(nodeSummaries, map[string]any{
			"id":     n.ID,
			"label":  n.Label,
			"status": n.Status.String(),
			"mode":   n.Mode,
			"deps":   n.Dependencies,
		})
	}

	return map[string]any{
		"planID": p.ID,
		"name":   p.Name,
		"total":  len(p.Nodes),
		"nodes":  nodeSummaries,
	}
}

// Snapshot returns a serializable view of the plan (for tracing/frontend).
func (p *Plan) Snapshot() PlanSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	nodes := make([]NodeSnapshot, 0, len(p.Nodes))
	for _, n := range p.Nodes {
		nodes = append(nodes, NodeSnapshot{
			ID:       n.ID,
			Label:    n.Label,
			Status:   n.Status.String(),
			Duration: n.Duration().Milliseconds(),
			AgentID:  n.AgentID,
		})
	}

	return PlanSnapshot{
		ID:        p.ID,
		Name:      p.Name,
		Status:    p.status.String(),
		Nodes:     nodes,
		Total:     len(p.Nodes),
		Completed: countCompleted(p.Nodes),
	}
}

// PlanSnapshot is a serializable view of a plan.
type PlanSnapshot struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Nodes     []NodeSnapshot `json:"nodes"`
	Total     int            `json:"total"`
	Completed int            `json:"completed"`
}

// NodeSnapshot is a serializable view of a node.
type NodeSnapshot struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Duration int64  `json:"duration_ms"`
	AgentID  string `json:"agent_id"`
}

func countCompleted(nodes []*Node) int {
	count := 0
	for _, n := range nodes {
		if n.Status == NodeCompleted {
			count++
		}
	}
	return count
}

// --- Builder helpers ---

// SimpleLinearPlan creates a simple linear chain of nodes.
func SimpleLinearPlan(id, name string, tasks []string) *Plan {
	p := NewPlan(id, name, "线性执行计划")
	for i, task := range tasks {
		node := &Node{
			ID:          fmt.Sprintf("step_%d", i+1),
			Label:       fmt.Sprintf("Step %d", i+1),
			Description: task,
			Task:        task,
			Mode:        "build",
		}
		if i > 0 {
			node.Dependencies = []string{fmt.Sprintf("step_%d", i)}
		}
		p.AddNode(node)
	}
	return p
}

// FanOutPlan creates a plan: one analyze node → N parallel build nodes.
func FanOutPlan(id, name, analyzeTask string, buildTasks []string) *Plan {
	p := NewPlan(id, name, "扇出执行计划")

	// Analyze node
	analyzeNode := &Node{
		ID:          "analyze",
		Label:       "分析",
		Description: analyzeTask,
		Task:        analyzeTask,
		Mode:        "plan",
	}
	p.AddNode(analyzeNode)

	// Build nodes (all depend on analyze)
	for i, task := range buildTasks {
		node := &Node{
			ID:           fmt.Sprintf("build_%d", i+1),
			Label:        fmt.Sprintf("实现 %d", i+1),
			Description:  task,
			Task:         task,
			Mode:         "build",
			Dependencies: []string{"analyze"},
			OnFailure:    "continue",
		}
		p.AddNode(node)
	}

	return p
}

// ReviewPlan creates a plan: build → review → fix (conditional).
func ReviewPlan(id, name, buildTask, reviewSpec string) *Plan {
	p := NewPlan(id, name, "构建-审查-修复计划")

	p.AddNode(&Node{
		ID:          "build",
		Label:       "构建",
		Description: buildTask,
		Task:        buildTask,
		Mode:        "build",
	})

	p.AddNode(&Node{
		ID:           "review",
		Label:        "审查",
		Description:  reviewSpec,
		Task:         reviewSpec,
		Mode:         "plan",
		Dependencies: []string{"build"},
		OnFailure:    "continue",
	})

	// Fix node: conditionally runs if review found issues
	p.AddNode(&Node{
		ID:           "fix",
		Label:        "修复",
		Description:  "根据审查结果修复问题",
		Task:         "根据审查结果修复发现的问题",
		Mode:         "build",
		Dependencies: []string{"review"},
		Condition: func(bb *Blackboard) bool {
			result := bb.ReadString("review:result")
			return strings.Contains(result, "问题") || strings.Contains(result, "issue") ||
				strings.Contains(result, "需要修复") || strings.Contains(result, "must fix")
		},
		OnFailure: "continue",
	})

	return p
}

// BuildPlanFromSubTasks creates a DAG plan from decomposed sub-tasks.
// This bridges the existing TaskRouter decomposition with the new DAG pipeline.
func BuildPlanFromSubTasks(route *TaskRoute, convID string) *Plan {
	if route == nil || len(route.SubTasks) == 0 {
		return nil
	}

	planID := fmt.Sprintf("plan_%s", convID)
	p := NewPlan(planID, "任务分解执行计划", fmt.Sprintf("来自 %d 个子任务", len(route.SubTasks)))

	for i, subTask := range route.SubTasks {
		node := &Node{
			ID:          fmt.Sprintf("subtask_%d", i+1),
			Label:       fmt.Sprintf("子任务 %d", i+1),
			Description: subTask.Description,
			Task:        subTask.Description,
			AgentID:     subTask.AgentID,
			Mode:        "build",
			OnFailure:   "continue",
		}
		// Wire up dependencies from the sub-task spec
		for _, depIdx := range subTask.DependsOn {
			if depIdx >= 0 && depIdx < len(route.SubTasks) {
				node.Dependencies = append(node.Dependencies, fmt.Sprintf("subtask_%d", depIdx+1))
			}
		}
		p.AddNode(node)
	}

	return p
}
