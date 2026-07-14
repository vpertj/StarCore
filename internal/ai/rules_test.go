package ai

import (
	"testing"
	"time"

	agentTools "StarCore/internal/agent/tools"
)

func TestMaxLoopsHardLimitRule(t *testing.T) {
	r := maxLoopsHardLimitRule

	// Below threshold: no decision
	snap := &Snapshot{CurrentLoop: 50, MaxLoops: 60}
	if d := r(snap); d != nil {
		t.Errorf("expected nil at loop 50, got %+v", d)
	}

	// At threshold: fires
	snap = &Snapshot{CurrentLoop: 100, MaxLoops: 100}
	if d := r(snap); d == nil {
		t.Fatal("expected decision at loop 100")
	} else if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}

	// Above threshold: fires
	snap = &Snapshot{CurrentLoop: 150, MaxLoops: 100}
	if d := r(snap); d == nil {
		t.Fatal("expected decision above loop 100")
	}
}

func TestAutoContinueExhaustedRule(t *testing.T) {
	r := autoContinueExhaustedRule

	// Auto-continued 3x but still stagnant
	snap := &Snapshot{AutoCont: 3, StagnantRounds: 3}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}

	// Auto-continued 3x but progressing (not stagnant)
	snap = &Snapshot{AutoCont: 3, StagnantRounds: 0}
	if d := r(snap); d != nil {
		t.Errorf("expected nil when progressing, got %+v", d)
	}

	// Stagnant but haven't auto-continued enough
	snap = &Snapshot{AutoCont: 1, StagnantRounds: 5}
	if d := r(snap); d != nil {
		t.Errorf("expected nil when autoCont<3, got %+v", d)
	}
}

func TestAllToolsFailed3xRule(t *testing.T) {
	r := allToolsFailed3xRule

	snap := &Snapshot{AllToolsFailed: true, ToolErrors: 3}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionEscalate {
		t.Errorf("expected escalate, got %s", d.Action)
	}

	// Not all failed
	snap = &Snapshot{AllToolsFailed: false, ToolErrors: 5}
	if d := r(snap); d != nil {
		t.Errorf("expected nil when not all failed, got %+v", d)
	}

	// All failed but only 2 errors
	snap = &Snapshot{AllToolsFailed: true, ToolErrors: 2}
	if d := r(snap); d != nil {
		t.Errorf("expected nil below threshold, got %+v", d)
	}
}

func TestStagnation8Rule(t *testing.T) {
	r := stagnation8Rule

	snap := &Snapshot{StagnantRounds: 8}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}

	snap = &Snapshot{StagnantRounds: 7}
	if d := r(snap); d != nil {
		t.Errorf("expected nil at 7 rounds, got %+v", d)
	}
}

func TestStagnation5WithNudgeRule(t *testing.T) {
	r := stagnation5WithNudgeRule

	// At threshold, few nudges: fires nudge
	snap := &Snapshot{StagnantRounds: 5, NudgeCount: 2}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionNudge {
		t.Errorf("expected nudge, got %s", d.Action)
	}

	// Already nudged 5 times: no more nudges
	snap = &Snapshot{StagnantRounds: 6, NudgeCount: 5}
	if d := r(snap); d != nil {
		t.Errorf("expected nil when nudgeCount>=5, got %+v", d)
	}

	// Below threshold
	snap = &Snapshot{StagnantRounds: 4, NudgeCount: 0}
	if d := r(snap); d != nil {
		t.Errorf("expected nil at 4 rounds, got %+v", d)
	}
}

func TestRepetition3xRule(t *testing.T) {
	r := repetition3xRule

	snap := &Snapshot{Repetition: true, StagnantRounds: 3}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionNudge {
		t.Errorf("expected nudge, got %s", d.Action)
	}

	// No repetition
	snap = &Snapshot{Repetition: false, StagnantRounds: 5}
	if d := r(snap); d != nil {
		t.Errorf("expected nil without repetition, got %+v", d)
	}
}

func TestHighErrorRateRule(t *testing.T) {
	r := highErrorRateRule

	// 3/4 failed = 75% > 50%
	snap := &Snapshot{ToolCallsTotal: 1, ToolErrors: 3}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionNudge {
		t.Errorf("expected nudge, got %s", d.Action)
	}

	// 2/4 failed = 50% → not > 50%
	snap = &Snapshot{ToolCallsTotal: 2, ToolErrors: 2}
	if d := r(snap); d != nil {
		t.Errorf("expected nil at 50%%, got %+v", d)
	}

	// Total < 4: not enough data
	snap = &Snapshot{ToolCallsTotal: 0, ToolErrors: 2}
	if d := r(snap); d != nil {
		t.Errorf("expected nil with <4 calls, got %+v", d)
	}
}

func TestTokenBudgetWarningRule(t *testing.T) {
	r := tokenBudgetWarningRule

	// Above threshold: fires (but action is continue — informational)
	snap := &Snapshot{TokenIn: 500000, TokenOut: 400000}
	if d := r(snap); d == nil {
		t.Fatal("expected decision")
	} else if d.Action != ActionContinue {
		t.Errorf("expected continue (info), got %s", d.Action)
	}

	// Below threshold
	snap = &Snapshot{TokenIn: 100000, TokenOut: 50000}
	if d := r(snap); d != nil {
		t.Errorf("expected nil below threshold, got %+v", d)
	}
}

func TestDecisionFrontendMessage(t *testing.T) {
	tests := map[RuleAction]string{
		ActionForceStop:    "🛑 test",
		ActionNudge:        "💡 test",
		ActionEscalate:     "⚠️ test",
		ActionAutoContinue: "🔄 test",
		ActionContinue:     "test",
	}

	for action, expected := range tests {
		d := &Decision{Action: action, Reason: "test"}
		if got := d.FrontendMessage(); got != expected {
			t.Errorf("action %s: expected %q, got %q", action, expected, got)
		}
	}
}

func TestDecisionWailsEvent(t *testing.T) {
	d := &Decision{
		Action:   ActionForceStop,
		Severity: SevCritical,
		Reason:   "test reason",
		Detail:   "detail info",
	}
	event, data := d.WailsEvent()
	if event != "ai:supervisor:decision" {
		t.Errorf("unexpected event name: %s", event)
	}
	if data["action"] != "force_stop" {
		t.Errorf("unexpected action: %v", data["action"])
	}
}

// --- Integration: Supervisor evaluates rules correctly ---

func TestSupervisorBasicFlow(t *testing.T) {
	s := NewSupervisor()
	s.SetEnabled(true)

	// Normal loop: no decision
	d := s.OnLoopEnd(5, 60, 0, 10, 0, 3, 0, 0, false, false, 0, 0)
	if d != nil {
		t.Errorf("expected nil for normal loop, got %+v", d)
	}

	// Stagnation 8: force stop
	d = s.OnLoopEnd(20, 60, 0, 10, 0, 3, 0, 8, false, false, 0, 0)
	if d == nil {
		t.Fatal("expected decision for stagnation 8")
	}
	if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}
}

func TestSupervisorDisabled(t *testing.T) {
	s := NewSupervisor()
	s.SetEnabled(false)

	// Would fire if enabled
	d := s.OnLoopEnd(20, 60, 0, 0, 0, 0, 0, 10, false, false, 0, 0)
	if d != nil {
		t.Errorf("expected nil when disabled, got %+v", d)
	}
}

func TestSupervisorCustomRules(t *testing.T) {
	// A custom rule that fires when toolCalls == 42
	customRule := rule("magic_42", func(snap *Snapshot) *Decision {
		if snap.ToolCallsTotal == 42 {
			return &Decision{Action: ActionForceStop, Reason: "magic 42", Severity: SevInfo}
		}
		return nil
	})

	s := NewSupervisorWithRules([]Rule{customRule})

	d := s.OnLoopEnd(10, 60, 0, 40, 0, 2, 0, 0, false, false, 0, 0)
	if d != nil {
		t.Errorf("expected nil at 40 calls, got %+v", d)
	}

	d = s.OnLoopEnd(10, 60, 0, 42, 0, 2, 0, 0, false, false, 0, 0)
	if d == nil {
		t.Fatal("expected decision at 42 calls")
	}
	if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}
}

func TestSupervisorLastDecision(t *testing.T) {
	s := NewSupervisor()

	// No decision yet
	if d := s.LastDecision(); d != nil {
		t.Errorf("expected nil initially, got %+v", d)
	}

	// Trigger stagnation 8
	s.OnLoopEnd(20, 60, 0, 0, 0, 0, 0, 8, false, false, 0, 0)

	d := s.LastDecision()
	if d == nil {
		t.Fatal("expected last decision to be set")
	}
	if d.Action != ActionForceStop {
		t.Errorf("expected force_stop, got %s", d.Action)
	}
}

func TestBuildSnapshot(t *testing.T) {
	ls := agentTools.NewLoopState()
	ls.AddFileTouched("a.go")
	ls.AddFileTouched("b.go")
	ls.RecordToolCall("read_file", map[string]any{"path": "a.go"}, 1)

	snap := BuildSnapshot(ls, 5, 60, 0, 3, 1, 0, false, false, 100, 200, time.Now())
	if snap.CurrentLoop != 5 {
		t.Errorf("expected loop 5, got %d", snap.CurrentLoop)
	}
	if snap.FilesTouched != 2 {
		t.Errorf("expected 2 files, got %d", snap.FilesTouched)
	}
	if snap.ToolCallsTotal != 3 {
		t.Errorf("expected 3 calls, got %d", snap.ToolCallsTotal)
	}
}
