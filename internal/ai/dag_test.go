package ai

import (
	"testing"
)

// --- Blackboard Tests ---

func TestBlackboard_WriteRead(t *testing.T) {
	bb := NewBlackboard("test-conv-1")
	bb.Write("key1", "value1", "agent1", []string{"tag1", "tag2"})

	entry, ok := bb.Read("key1")
	if !ok {
		t.Fatal("entry should exist")
	}
	if entry.Value != "value1" {
		t.Errorf("expected value1, got %s", entry.Value)
	}
	if entry.Author != "agent1" {
		t.Errorf("expected author agent1, got %s", entry.Author)
	}
	if entry.Version != 1 {
		t.Errorf("expected version 1, got %d", entry.Version)
	}
}

func TestBlackboard_VersionIncrement(t *testing.T) {
	bb := NewBlackboard("test-conv-2")
	bb.Write("key1", "v1", "a1", nil)
	bb.Write("key1", "v2", "a1", nil)

	entry, ok := bb.Read("key1")
	if !ok {
		t.Fatal("entry should exist")
	}
	if entry.Version != 2 {
		t.Errorf("expected version 2, got %d", entry.Version)
	}
}

func TestBlackboard_Has(t *testing.T) {
	bb := NewBlackboard("test-conv-3")
	bb.Write("exists", "yes", "a", nil)

	if !bb.Has("exists") {
		t.Error("Has should return true for existing key")
	}
	if bb.Has("nonexistent") {
		t.Error("Has should return false for missing key")
	}
}

func TestBlackboard_Delete(t *testing.T) {
	bb := NewBlackboard("test-conv-4")
	bb.Write("to-delete", "value", "a", nil)
	bb.Delete("to-delete")

	if bb.Has("to-delete") {
		t.Error("entry should be deleted")
	}
}

func TestBlackboard_Query(t *testing.T) {
	bb := NewBlackboard("test-conv-5")
	bb.Write("a", "val-a", "agent1", []string{"analysis", "auth"})
	bb.Write("b", "val-b", "agent2", []string{"analysis", "security"})
	bb.Write("c", "val-c", "agent1", []string{"implementation"})

	// Query for "analysis" tag
	results := bb.Query([]string{"analysis"})
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'analysis', got %d", len(results))
	}

	// Query for both tags
	results = bb.Query([]string{"analysis", "auth"})
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'analysis'+'auth', got %d", len(results))
	}

	// Query with no tags → all
	results = bb.Query(nil)
	if len(results) != 3 {
		t.Errorf("expected 3 results for nil query, got %d", len(results))
	}
}

func TestBlackboard_QueryPrefix(t *testing.T) {
	bb := NewBlackboard("test-conv-6")
	bb.Write("file:main.go:analysis", "result1", "a", nil)
	bb.Write("file:utils.go:analysis", "result2", "a", nil)
	bb.Write("node:1:result", "result3", "a", nil)

	results := bb.QueryPrefix("file:")
	if len(results) != 2 {
		t.Errorf("expected 2 file: results, got %d", len(results))
	}
}

func TestBlackboard_ReadString(t *testing.T) {
	bb := NewBlackboard("test-conv-7")
	bb.Write("greeting", "hello world", "a", nil)

	val := bb.ReadString("greeting")
	if val != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", val)
	}

	val = bb.ReadString("missing")
	if val != "" {
		t.Errorf("expected empty string for missing key, got '%s'", val)
	}
}

func TestBlackboard_Size(t *testing.T) {
	bb := NewBlackboard("test-conv-8")
	if bb.Size() != 0 {
		t.Errorf("new blackboard should be empty")
	}
	bb.Write("a", "1", "a", nil)
	bb.Write("b", "2", "a", nil)
	if bb.Size() != 2 {
		t.Errorf("expected size 2, got %d", bb.Size())
	}
}

func TestBlackboard_Summary(t *testing.T) {
	bb := NewBlackboard("test-conv-9")
	bb.Write("result", "some analysis output here", "agent1", []string{"analysis"})

	summary := bb.Summary()
	if summary == "" {
		t.Error("summary should not be empty when entries exist")
	}
}

func TestBlackboard_Snapshot(t *testing.T) {
	bb := NewBlackboard("test-conv-10")
	bb.Write("k1", "v1", "a", nil)
	snap := bb.Snapshot()
	if len(snap) != 1 {
		t.Errorf("snapshot should have 1 entry, got %d", len(snap))
	}
}

func TestBlackboard_Clear(t *testing.T) {
	bb := NewBlackboard("test-conv-11")
	bb.Write("k1", "v1", "a", nil)
	bb.Write("k2", "v2", "a", nil)
	bb.Clear()
	if bb.Size() != 0 {
		t.Error("blackboard should be empty after Clear")
	}
}

func TestBlackboard_ConcurrentAccess(t *testing.T) {
	bb := NewBlackboard("test-conv-concurrent")
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			bb.Write("k", "v", "a", nil)
			_ = bb.Size()
			_, _ = bb.Read("k")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}
	// If we reach here without deadlock/panic, concurrent access works
}

func TestGetBlackboard(t *testing.T) {
	bb1 := GetBlackboard("conv-get-1")
	bb2 := GetBlackboard("conv-get-1")
	if bb1 != bb2 {
		t.Error("same convID should return same blackboard")
	}

	bb3 := GetBlackboard("conv-get-2")
	if bb1 == bb3 {
		t.Error("different convID should return different blackboard")
	}

	DeleteBlackboard("conv-get-1")
	DeleteBlackboard("conv-get-2")
}

// --- Plan / DAG Tests ---

func TestNewPlan(t *testing.T) {
	p := NewPlan("plan1", "Test Plan", "A test plan")
	if p.ID != "plan1" {
		t.Errorf("expected plan ID plan1, got %s", p.ID)
	}
	if len(p.Nodes) != 0 {
		t.Error("new plan should have no nodes")
	}
	if p.status != PlanPending {
		t.Errorf("new plan should be pending, got %s", p.status)
	}
}

func TestPlan_Validate_NoCycle(t *testing.T) {
	p := NewPlan("p1", "No Cycle", "")
	p.AddNode(&Node{ID: "a", Dependencies: []string{"b"}})
	p.AddNode(&Node{ID: "b", Dependencies: []string{"c"}})
	p.AddNode(&Node{ID: "c"}) // no deps

	if err := p.Validate(); err != nil {
		t.Errorf("linear chain should be valid, got: %v", err)
	}
}

func TestPlan_Validate_Cycle(t *testing.T) {
	p := NewPlan("p2", "Has Cycle", "")
	p.AddNode(&Node{ID: "a", Dependencies: []string{"b"}})
	p.AddNode(&Node{ID: "b", Dependencies: []string{"a"}}) // cycle!

	err := p.Validate()
	if err == nil {
		t.Error("should detect cycle")
	}
}

func TestPlan_Validate_DuplicateID(t *testing.T) {
	p := NewPlan("p3", "Duplicate ID", "")
	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{ID: "a"}) // duplicate

	err := p.Validate()
	if err == nil {
		t.Error("should detect duplicate IDs")
	}
}

func TestPlan_Validate_MissingDependency(t *testing.T) {
	p := NewPlan("p4", "Missing Dep", "")
	p.AddNode(&Node{ID: "a", Dependencies: []string{"nonexistent"}})

	err := p.Validate()
	if err == nil {
		t.Error("should detect missing dependency")
	}
}

func TestPlan_ReadyNodes(t *testing.T) {
	p := NewPlan("p5", "Ready Test", "")
	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{ID: "b", Dependencies: []string{"a"}})
	p.AddNode(&Node{ID: "c", Dependencies: []string{"a"}})

	// Initially: only "a" should be ready
	ready := p.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "a" {
		t.Errorf("expected only 'a' ready, got %v", nodeIDs(ready))
	}

	// Complete "a", now "b" and "c" should be ready
	p.findNode("a").Status = NodeCompleted
	ready = p.ReadyNodes()
	if len(ready) != 2 {
		t.Errorf("expected 2 ready nodes, got %d: %v", len(ready), nodeIDs(ready))
	}
}

func TestPlan_ReadyNodes_SkipOnFailure(t *testing.T) {
	p := NewPlan("p6", "Skip on Failure", "")
	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{ID: "b", Dependencies: []string{"a"}, OnFailure: "skip"})

	p.findNode("a").Status = NodeFailed

	ready := p.ReadyNodes()
	// "b" should be skipped because "a" failed
	for _, n := range ready {
		if n.ID == "b" {
			t.Error("node b should be skipped when dependency fails")
		}
	}
}

func TestPlan_ReadyNodes_ContinueOnFailure(t *testing.T) {
	p := NewPlan("p7", "Continue on Failure", "")
	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{ID: "b", Dependencies: []string{"a"}, OnFailure: "continue"})

	p.findNode("a").Status = NodeFailed

	ready := p.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "b" {
		t.Errorf("expected 'b' to be ready (continue policy), got %v", nodeIDs(ready))
	}
}

func TestPlan_ReadyNodes_Condition(t *testing.T) {
	p := NewPlan("p8", "Condition Test", "")
	p.blackboard = NewBlackboard("cond-test")

	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{
		ID:           "b",
		Dependencies: []string{"a"},
		Condition: func(bb *Blackboard) bool {
			return bb.Has("trigger")
		},
	})

	p.findNode("a").Status = NodeCompleted

	// Condition not met → b should be skipped
	ready := p.ReadyNodes()
	for _, n := range ready {
		if n.ID == "b" {
			t.Error("node b should be skipped when condition is false")
		}
	}

	// Set condition → b should be ready
	p.blackboard.Write("trigger", "yes", "test", nil)
	ready = p.ReadyNodes()
	if len(ready) != 1 || ready[0].ID != "b" {
		t.Errorf("expected 'b' to be ready after condition met, got %v", nodeIDs(ready))
	}
}

func TestPlan_CompletedCount(t *testing.T) {
	p := NewPlan("p9", "Completion", "")
	p.AddNode(&Node{ID: "a"})
	p.AddNode(&Node{ID: "b"})
	p.AddNode(&Node{ID: "c"})

	p.findNode("a").Status = NodeCompleted
	p.findNode("b").Status = NodeCompleted
	p.findNode("c").Status = NodeFailed

	if p.CompletedCount() != 2 {
		t.Errorf("expected 2 completed, got %d", p.CompletedCount())
	}
}

func TestPlan_Snapshot(t *testing.T) {
	p := NewPlan("p10", "Snapshot Test", "")
	p.AddNode(&Node{ID: "a", Label: "Step A", Mode: "build"})
	p.AddNode(&Node{ID: "b", Label: "Step B", Mode: "plan"})

	snap := p.Snapshot()
	if snap.ID != "p10" {
		t.Errorf("snapshot ID should be p10, got %s", snap.ID)
	}
	if snap.Total != 2 {
		t.Errorf("snapshot should have 2 nodes, got %d", snap.Total)
	}
	if len(snap.Nodes) != 2 {
		t.Errorf("snapshot should have 2 node snapshots, got %d", len(snap.Nodes))
	}
}

// --- Builder helpers tests ---

func TestSimpleLinearPlan(t *testing.T) {
	p := SimpleLinearPlan("lin", "Linear", []string{"step1", "step2", "step3"})
	if len(p.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(p.Nodes))
	}

	// Check dependencies chain
	if len(p.Nodes[0].Dependencies) != 0 {
		t.Error("first node should have no deps")
	}
	if len(p.Nodes[1].Dependencies) != 1 || p.Nodes[1].Dependencies[0] != "step_1" {
		t.Errorf("second node should depend on step_1, got %v", p.Nodes[1].Dependencies)
	}

	if err := p.Validate(); err != nil {
		t.Errorf("linear plan should be valid: %v", err)
	}
}

func TestFanOutPlan(t *testing.T) {
	p := FanOutPlan("fan", "Fan Out", "analyze code", []string{"fix a", "fix b", "fix c"})
	if len(p.Nodes) != 4 { // 1 analyze + 3 build
		t.Errorf("expected 4 nodes, got %d", len(p.Nodes))
	}

	// First node is analyze
	if p.Nodes[0].ID != "analyze" {
		t.Errorf("first node should be 'analyze', got %s", p.Nodes[0].ID)
	}

	// All build nodes depend on analyze
	for i := 1; i < len(p.Nodes); i++ {
		if len(p.Nodes[i].Dependencies) != 1 || p.Nodes[i].Dependencies[0] != "analyze" {
			t.Errorf("build node %d should depend on 'analyze', got %v", i, p.Nodes[i].Dependencies)
		}
	}

	if err := p.Validate(); err != nil {
		t.Errorf("fan-out plan should be valid: %v", err)
	}
}

func TestReviewPlan(t *testing.T) {
	p := ReviewPlan("rev", "Review", "implement auth", "review the code")
	if len(p.Nodes) != 3 {
		t.Errorf("expected 3 nodes (build, review, fix), got %d", len(p.Nodes))
	}

	// Verify structure: build → review → fix
	if p.Nodes[0].ID != "build" {
		t.Errorf("first node should be 'build', got %s", p.Nodes[0].ID)
	}
	if p.Nodes[1].ID != "review" {
		t.Errorf("second node should be 'review', got %s", p.Nodes[1].ID)
	}
	if p.Nodes[2].ID != "fix" {
		t.Errorf("third node should be 'fix', got %s", p.Nodes[2].ID)
	}

	// Fix node has a condition
	if p.Nodes[2].Condition == nil {
		t.Error("fix node should have a condition")
	}

	if err := p.Validate(); err != nil {
		t.Errorf("review plan should be valid: %v", err)
	}
}

func TestNodeDuration(t *testing.T) {
	n := &Node{}
	if n.Duration() != 0 {
		t.Error("duration should be 0 for unstarted node")
	}
}

func TestBuildPlanFromSubTasks(t *testing.T) {
	route := &TaskRoute{
		Complexity: ComplexityComplex,
		Route:      "decompose",
		SubTasks: []SubTaskSpec{
			{Description: "分析代码结构", AgentID: "architect"},
			{Description: "修改 main.go", DependsOn: []int{0}},
			{Description: "修改 utils.go", DependsOn: []int{0}},
		},
	}

	plan := BuildPlanFromSubTasks(route, "conv-1")
	if plan == nil {
		t.Fatal("plan should not be nil")
	}
	if len(plan.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(plan.Nodes))
	}

	// First node has no deps
	if len(plan.Nodes[0].Dependencies) != 0 {
		t.Error("first node should have no deps")
	}

	// Second and third depend on first
	if len(plan.Nodes[1].Dependencies) != 1 || plan.Nodes[1].Dependencies[0] != "subtask_1" {
		t.Errorf("node 2 should depend on subtask_1, got %v", plan.Nodes[1].Dependencies)
	}
	if len(plan.Nodes[2].Dependencies) != 1 || plan.Nodes[2].Dependencies[0] != "subtask_1" {
		t.Errorf("node 3 should depend on subtask_1, got %v", plan.Nodes[2].Dependencies)
	}

	// Validate the plan
	if err := plan.Validate(); err != nil {
		t.Errorf("plan should be valid: %v", err)
	}
}

func TestBuildPlanFromSubTasks_Nil(t *testing.T) {
	plan := BuildPlanFromSubTasks(nil, "conv")
	if plan != nil {
		t.Error("nil route should produce nil plan")
	}

	plan = BuildPlanFromSubTasks(&TaskRoute{}, "conv")
	if plan != nil {
		t.Error("empty route should produce nil plan")
	}
}

func TestPlanSummary(t *testing.T) {
	p := SimpleLinearPlan("sum", "Summary", []string{"a", "b"})
	summary := p.Summary()
	if summary["total"] != 2 {
		t.Errorf("expected total=2, got %v", summary["total"])
	}
	if summary["name"] != "Summary" {
		t.Errorf("expected name=Summary, got %v", summary["name"])
	}
}

// --- Helper ---

func nodeIDs(nodes []*Node) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}
