package ai

import (
	"fmt"
	"sync"
	"time"

	agentTools "StarCore/internal/agent/tools"
)

// Supervisor observes the agent loop via trace events and rule evaluations.
// It is the "watchdog" agent described in the architecture: a rule engine
// that monitors execution and intervenes when patterns warrant, without
// ever calling the LLM (zero cost, deterministic).
//
// Lifecycle:
//   - Created once per ChatStream call (via NewSupervisor)
//   - Receives event notifications from the Tracer (callback wiring)
//   - Evaluates rules at checkpoints (round end, stagnation, etc.)
//   - Returns Decisions to the agent loop via the Checkpoint method
//
// Thread-safety: the supervisor's mutable state (snapshot) is protected by mu.
// Rule evaluation reads the snapshot under lock, then releases it before any
// decision actions (which may call back into the loop).
type Supervisor struct {
	mu       sync.Mutex
	rules    []Rule
	snap     Snapshot
	tracer   Tracer
	start    time.Time
	decision *Decision // last non-continue decision (for trace/log)
	enabled  bool      // can be disabled for testing
}

// NewSupervisor creates a supervisor with the default rule set.
func NewSupervisor() *Supervisor {
	return &Supervisor{
		rules:   allRules(),
		enabled: true,
		start:   time.Now(),
	}
}

// NewSupervisorWithRules creates a supervisor with a custom rule set (for tests).
func NewSupervisorWithRules(rules []Rule) *Supervisor {
	return &Supervisor{
		rules:   rules,
		enabled: true,
		start:   time.Now(),
	}
}

// SetTracer wires the supervisor to observe trace events.
func (s *Supervisor) SetTracer(t Tracer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tracer = t
}

// Reset clears all state for a new session.
func (s *Supervisor) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap = Snapshot{}
	s.decision = nil
	s.start = time.Now()
}

// Disabled controls whether the supervisor intervenes (for tests).
func (s *Supervisor) SetEnabled(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = v
}

// Snapshot returns a copy of the current snapshot (for tracing/debugging).
func (s *Supervisor) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap
}

// LastDecision returns the last non-continue decision made.
func (s *Supervisor) LastDecision() *Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.decision == nil {
		return nil
	}
	cp := *s.decision
	return &cp
}

// OnLoopEnd is called at the end of each loop iteration to update the snapshot
// and evaluate rules. It returns a Decision if a rule fires, or nil to continue.
//
// This is the primary integration point with the agent loop. The loop calls
// this after each round's tool execution (or nudge/skip).
func (s *Supervisor) OnLoopEnd(
	currentLoop, maxLoops, autoCont int,
	toolCalls, toolErrors, filesTouched, nudgeCount, stagnantRounds int,
	allToolsFailed, repetition bool,
	tokenIn, tokenOut int,
) *Decision {
	s.mu.Lock()

	// Reconstruct snapshot
	s.snap.CurrentLoop = currentLoop
	s.snap.MaxLoops = maxLoops
	s.snap.AutoCont = autoCont
	s.snap.ToolCallsTotal = toolCalls
	s.snap.ToolErrors = toolErrors
	s.snap.FilesTouched = filesTouched
	s.snap.NudgeCount = nudgeCount
	s.snap.StagnantRounds = stagnantRounds
	s.snap.AllToolsFailed = allToolsFailed
	s.snap.Repetition = repetition
	s.snap.TokenIn = tokenIn
	s.snap.TokenOut = tokenOut
	s.snap.Duration = time.Since(s.start)

	snapCopy := s.snap
	enabled := s.enabled
	s.mu.Unlock()

	if !enabled {
		return nil
	}

	// Evaluate rules outside the lock
	decision := s.evaluate(&snapCopy)
	if decision == nil || decision.Action == ActionContinue {
		return nil
	}

	// Record non-continue decision
	s.mu.Lock()
	s.decision = decision
	s.mu.Unlock()

	// Emit trace event
	if s.tracer != nil {
		s.tracer.EventWithTool(EventStagnation, StageSupervise, "",
			string(decision.Action), decision.Reason, currentLoop)
	}

	return decision
}

// evaluate runs all rules in order and returns the first non-nil decision.
func (s *Supervisor) evaluate(snap *Snapshot) *Decision {
	for _, r := range s.rules {
		d := r.Evaluate(snap)
		if d != nil {
			if s.tracer != nil {
				s.tracer.Event(EventStagnation, StageSupervise, "",
					fmt.Sprintf("rule=%s action=%s reason=%s", r.ID(), d.Action, d.Reason))
			}
			return d
		}
	}
	return nil
}

// OnStreamInterrupt notifies the supervisor of a stream interruption (repetition/length).
func (s *Supervisor) OnStreamInterrupt(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.StreamInterrupted = true
	if s.tracer != nil {
		s.tracer.Event(EventStreamInterrupt, StageSupervise, "", reason)
	}
}

// OnToolCall records a tool call name for the current round (used by repetition rules).
func (s *Supervisor) OnToolCall(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.ToolNames = append(s.snap.ToolNames, name)
}

// --- Convenience: build a Snapshot from LoopState + counters ---

// BuildSnapshot constructs a Snapshot from the live state. Called by the agent loop.
func BuildSnapshot(
	loopState *agentTools.LoopState,
	currentLoop, maxLoops, autoCont int,
	toolCalls, toolErrors, nudgeCount int,
	allToolsFailed, repetition bool,
	tokenIn, tokenOut int,
	startTime time.Time,
) Snapshot {
	return Snapshot{
		CurrentLoop:    currentLoop,
		MaxLoops:       maxLoops,
		AutoCont:       autoCont,
		ToolCallsTotal: toolCalls,
		ToolErrors:     toolErrors,
		FilesTouched:   len(loopState.GetFilesTouched()),
		NudgeCount:     nudgeCount,
		StagnantRounds: loopState.GetStagnantRounds(),
		AllToolsFailed: allToolsFailed,
		Repetition:     repetition,
		TokenIn:        tokenIn,
		TokenOut:       tokenOut,
		Duration:       time.Since(startTime),
	}
}
