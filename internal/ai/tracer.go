package ai

import (
	"fmt"
	"sync"
	"time"
)

// --- Trace Types ---
// The tracing system collects a full chain of events from user request to final
// response. Events capture: loop iterations, LLM calls, tool executions, nudges,
// errors, and loop lifecycle. In Phase 1 this is an in-memory ring buffer;
// Phase 6 adds SQLite persistence via TraceSink.

// EventType categorizes a trace event.
type EventType string

const (
	EventLoopStart       EventType = "loop_start"
	EventLoopEnd         EventType = "loop_end"
	EventLLMCall         EventType = "llm_call"
	EventToolCall        EventType = "tool_call"
	EventToolResult      EventType = "tool_result"
	EventNudge           EventType = "nudge"
	EventRepetition      EventType = "repetition_detected"
	EventStagnation      EventType = "stagnation_detected"
	EventLoopExhausted   EventType = "loop_exhausted"
	EventLoopAutoCont    EventType = "loop_auto_continue"
	EventStreamInterrupt EventType = "stream_interrupt"
	EventCircuitBreaker  EventType = "circuit_breaker"

	// DAG Pipeline events (Phase 4+)
	EventDAGStart    EventType = "dag_start"
	EventDAGDone     EventType = "dag_done"
	EventNodeStart   EventType = "node_start"
	EventNodeDone    EventType = "node_done"
	EventNodeFailed  EventType = "node_failed"
	EventNodeSkipped EventType = "node_skipped"
)

// Stage indicates which phase of the pipeline an event belongs to.
// Empty for events that aren't stage-specific.
type Stage string

const (
	StageUnderstand Stage = "understand"
	StageRoute      Stage = "route"
	StageExecute    Stage = "execute"
	StageSupervise  Stage = "supervise"
	StageDeliver    Stage = "deliver"
)

// TraceEvent is a single immutable record within a Trace.
type TraceEvent struct {
	ID        string    `json:"id"`        // unique event id (evt_N)
	Type      EventType `json:"type"`      // event category
	Stage     Stage     `json:"stage"`     // pipeline phase (may be empty)
	AgentID   string    `json:"agent_id"`  // which agent (empty for top-level)
	Message   string    `json:"message"`   // human-readable description
	ToolName  string    `json:"tool_name"` // set for tool_call / tool_result
	Loop      int       `json:"loop"`      // loop iteration (0 = before loop)
	TokenIn   int       `json:"token_in"`  // LLM input tokens for this event
	TokenOut  int       `json:"token_out"` // LLM output tokens for this event
	Timestamp time.Time `json:"timestamp"` // event creation time
}

// Trace is the full chain of events for one ChatStream request.
type Trace struct {
	ID             string       `json:"id"`              // trace id (tr_N)
	ConversationID string       `json:"conversation_id"` // ties to memory store
	TotalLoops     int          `json:"total_loops"`     // actual loop count
	TotalTools     int          `json:"total_tools"`     // total tool calls
	TotalErrors    int          `json:"total_errors"`    // total tool errors
	TokenIn        int          `json:"token_in"`        // cumulative LLM input tokens
	TokenOut       int          `json:"token_out"`       // cumulative LLM output tokens
	Duration       int64        `json:"duration_ms"`     // total wall time in ms
	Events         []TraceEvent `json:"events"`          // ordered event list
	StartTime      time.Time    `json:"start_time"`
	EndTime        time.Time    `json:"end_time,omitempty"`
}

// TraceSink persists a completed Trace. Implementations must be thread-safe.
type TraceSink interface {
	SaveTrace(t *Trace) error
}

// NoopTraceSink discards traces. Used when tracing is disabled.
type NoopTraceSink struct{}

func (NoopTraceSink) SaveTrace(*Trace) error { return nil }

// MemoryTraceSink keeps traces in memory (useful for tests / Phase 1).
type MemoryTraceSink struct {
	mu     sync.RWMutex
	Traces []*Trace
}

func (s *MemoryTraceSink) SaveTrace(t *Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Traces = append(s.Traces, t)
	return nil
}

func (s *MemoryTraceSink) Last() *Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Traces) == 0 {
		return nil
	}
	return s.Traces[len(s.Traces)-1]
}

// Tracer is the thread-safe event collector bound to a single request lifecycle.
type Tracer interface {
	// Event records a new trace event.
	Event(evtType EventType, stage Stage, agentID, message string)
	// EventWithTool records a tool-related event.
	EventWithTool(evtType EventType, stage Stage, agentID, toolName, message string, loop int)
	// EventWithTokens records an LLM call event.
	EventWithTokens(stage Stage, agentID, message string, tokenIn, tokenOut int)
	// Finish completes the trace, sets EndTime & Duration, and persists via sink.
	Finish(totalLoops, totalTools, totalErrors int)
	// GetTrace returns a copy of the current trace.
	GetTrace() *Trace
}

type tracer struct {
	mu       sync.Mutex
	trace    *Trace
	sink     TraceSink
	eventSeq int
}

// NewTracer creates a tracer bound to a conversation ID and sink.
func NewTracer(convID string, sink TraceSink) Tracer {
	if sink == nil {
		sink = NoopTraceSink{}
	}
	return &tracer{
		trace: &Trace{
			ID:             fmt.Sprintf("tr_%d", time.Now().UnixNano()),
			ConversationID: convID,
			Events:         make([]TraceEvent, 0, 32),
			StartTime:      time.Now(),
		},
		sink: sink,
	}
}

func (t *tracer) Event(evtType EventType, stage Stage, agentID, message string) {
	t.EventWithTool(evtType, stage, agentID, "", message, 0)
}

func (t *tracer) EventWithTool(evtType EventType, stage Stage, agentID, toolName, message string, loop int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventSeq++
	evt := TraceEvent{
		ID:        fmt.Sprintf("evt_%d", t.eventSeq),
		Type:      evtType,
		Stage:     stage,
		AgentID:   agentID,
		Message:   message,
		ToolName:  toolName,
		Loop:      loop,
		Timestamp: time.Now(),
	}
	t.trace.Events = append(t.trace.Events, evt)
}

func (t *tracer) EventWithTokens(stage Stage, agentID, message string, tokenIn, tokenOut int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.eventSeq++
	evt := TraceEvent{
		ID:        fmt.Sprintf("evt_%d", t.eventSeq),
		Type:      EventLLMCall,
		Stage:     stage,
		AgentID:   agentID,
		Message:   message,
		TokenIn:   tokenIn,
		TokenOut:  tokenOut,
		Timestamp: time.Now(),
	}
	t.trace.Events = append(t.trace.Events, evt)
	t.trace.TokenIn += tokenIn
	t.trace.TokenOut += tokenOut
}

func (t *tracer) Finish(totalLoops, totalTools, totalErrors int) {
	t.mu.Lock()
	t.trace.TotalLoops = totalLoops
	t.trace.TotalTools = totalTools
	t.trace.TotalErrors = totalErrors
	t.trace.EndTime = time.Now()
	t.trace.Duration = t.trace.EndTime.Sub(t.trace.StartTime).Milliseconds()
	t.mu.Unlock()
	t.sink.SaveTrace(t.trace)
}

func (t *tracer) GetTrace() *Trace {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Return a shallow copy to prevent external mutation
	cp := *t.trace
	cp.Events = make([]TraceEvent, len(t.trace.Events))
	copy(cp.Events, t.trace.Events)
	return &cp
}
