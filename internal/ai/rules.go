package ai

import (
	"fmt"
	"strings"
	"time"
)

// --- Supervisor Rule Engine ---
// The supervisor is a rule-based observer that monitors the agent loop without
// calling the LLM. It evaluates rules at key checkpoints (end of each round,
// stagnation, errors, etc.) and returns decisions.
//
// Design principles:
//   - No LLM calls — pure rule engine (fast, deterministic, zero cost)
//   - Stateless rules — each rule inspects the current Snapshot independently
//   - Composable — rules are evaluated in priority order, first match wins
//   - Observable — all decisions are traceable via the Tracer

// Decision represents the supervisor's response to a rule match.
type Decision struct {
	Action   RuleAction // what to do
	Reason   string     // human-readable explanation (also emitted to frontend)
	Severity Severity   // how critical this decision is
	Detail   string     // optional extra context (shown in trace/log)
}

// RuleAction enumerates all actions the supervisor can take.
type RuleAction string

const (
	ActionContinue     RuleAction = "continue"      // no intervention, proceed normally
	ActionNudge        RuleAction = "nudge"         // inject a nudge message
	ActionForceStop    RuleAction = "force_stop"    // terminate the loop immediately
	ActionAutoContinue RuleAction = "auto_continue" // add extra loop iterations
	ActionEscalate     RuleAction = "escalate"      // escalate to human (ask_user)
)

// Severity indicates urgency.
type Severity int

const (
	SevInfo Severity = iota
	SevWarn
	SevCritical
)

func (s Severity) String() string {
	switch s {
	case SevInfo:
		return "INFO"
	case SevWarn:
		return "WARN"
	case SevCritical:
		return "CRITICAL"
	}
	return "UNKNOWN"
}

// Snapshot is a point-in-time view of the loop state for rule evaluation.
// Rules read-only inspect this; they never mutate the actual loop/state.
type Snapshot struct {
	// Identity
	ConversationID string
	AgentID        string
	Mode           string // chat | plan | build

	// Loop progress
	CurrentLoop int // 0-based iteration index
	MaxLoops    int // current upper limit (may grow with auto-continue)
	AutoCont    int // how many times auto-continue has fired (max 3)

	// Progress metrics
	ToolCallsTotal int // total successful tool calls
	ToolErrors     int // total failed tool calls
	FilesTouched   int // unique files modified
	NudgeCount     int // total nudges issued
	StagnantRounds int // consecutive rounds without progress

	// Flags (pre-computed by the loop or tracer)
	AllToolsFailed    bool // last round: every tool call failed
	Repetition        bool // semantic repetition detected
	StreamInterrupted bool // stream was interrupted this round
	TokenIn           int  // cumulative input tokens this session
	TokenOut          int  // cumulative output tokens this session

	// Derived
	Duration  time.Duration // wall time since loop start
	ToolNames []string      // tool names called in the last round
}

// Rule inspects a Snapshot and returns a Decision if triggered, or nil if not.
type Rule interface {
	// ID returns a stable identifier for this rule (used in traces/tests).
	ID() string
	// Evaluate returns a non-nil Decision when the rule fires.
	Evaluate(snap *Snapshot) *Decision
}

// RuleFunc adapts a plain function to the Rule interface.
type RuleFunc struct {
	id   string
	Eval func(snap *Snapshot) *Decision
}

func (r RuleFunc) ID() string                        { return r.id }
func (r RuleFunc) Evaluate(snap *Snapshot) *Decision { return r.Eval(snap) }

// --- Built-in Rules ---

// RuleFunc constructs a Rule from a function. Useful for quick ad-hoc rules.
func rule(id string, fn func(snap *Snapshot) *Decision) Rule {
	return RuleFunc{id: id, Eval: fn}
}

// allRules returns the default rule set in evaluation order (first match wins).
func allRules() []Rule {
	return []Rule{
		// Priority 1: Hard limits (prevent runaway loops)
		rule("max_loops_hard_limit", maxLoopsHardLimitRule),
		rule("auto_continue_exhausted", autoContinueExhaustedRule),

		// Priority 2: Total tool failure (circuit breaker pattern)
		rule("all_tools_failed_3x", allToolsFailed3xRule),

		// Priority 3: Stagnation (no progress)
		rule("stagnation_8", stagnation8Rule),
		rule("stagnation_5_with_nudge", stagnation5WithNudgeRule),

		// Priority 4: Repetition loops
		rule("repetition_3x", repetition3xRule),

		// Priority 5: High error rate
		rule("high_error_rate", highErrorRateRule),

		// Priority 6: Token budget protection
		rule("token_budget_warning", tokenBudgetWarningRule),
	}
}

// maxLoopsHardLimitRule fires when the loop exceeds 100 iterations (absolute ceiling).
func maxLoopsHardLimitRule(snap *Snapshot) *Decision {
	if snap.CurrentLoop >= 100 {
		return &Decision{
			Action:   ActionForceStop,
			Reason:   fmt.Sprintf("已达到绝对上限 %d 轮，强制终止以防止无限循环", snap.CurrentLoop),
			Severity: SevCritical,
			Detail:   "Absolute max_loops ceiling reached",
		}
	}
	return nil
}

// autoContinueExhaustedRule fires when auto-continue has been used 3 times but still no progress.
func autoContinueExhaustedRule(snap *Snapshot) *Decision {
	if snap.AutoCont >= 3 && snap.StagnantRounds >= 3 {
		return &Decision{
			Action:   ActionForceStop,
			Reason:   "已自动继续 3 次但仍无进展，判定任务无法自动完成",
			Severity: SevCritical,
			Detail:   fmt.Sprintf("autoCont=%d stagnant=%d", snap.AutoCont, snap.StagnantRounds),
		}
	}
	return nil
}

// allToolsFailed3xRule fires when all tool calls fail for 3 consecutive rounds.
func allToolsFailed3xRule(snap *Snapshot) *Decision {
	if snap.AllToolsFailed && snap.ToolErrors >= 3 {
		return &Decision{
			Action:   ActionEscalate,
			Reason:   fmt.Sprintf("连续多轮全部工具调用失败（%d次错误），可能需要人工介入", snap.ToolErrors),
			Severity: SevCritical,
			Detail:   fmt.Sprintf("toolErrors=%d", snap.ToolErrors),
		}
	}
	return nil
}

// stagnation8Rule fires at 8+ stagnant rounds — force stop.
func stagnation8Rule(snap *Snapshot) *Decision {
	if snap.StagnantRounds >= 8 {
		return &Decision{
			Action:   ActionForceStop,
			Reason:   fmt.Sprintf("连续 %d 轮无实质进展，判定陷入停滞", snap.StagnantRounds),
			Severity: SevWarn,
			Detail:   fmt.Sprintf("stagnantRounds=%d", snap.StagnantRounds),
		}
	}
	return nil
}

// stagnation5WithNudgeRule fires at 5+ stagnant rounds — one final nudge.
func stagnation5WithNudgeRule(snap *Snapshot) *Decision {
	if snap.StagnantRounds >= 5 && snap.NudgeCount < 5 {
		return &Decision{
			Action:   ActionNudge,
			Reason:   fmt.Sprintf("连续 %d 轮无进展，最后一次尝试引导调整策略", snap.StagnantRounds),
			Severity: SevWarn,
			Detail:   fmt.Sprintf("stagnantRounds=%d nudgeCount=%d", snap.StagnantRounds, snap.NudgeCount),
		}
	}
	return nil
}

// repetition3xRule fires when semantic repetition is detected 3+ times.
func repetition3xRule(snap *Snapshot) *Decision {
	if snap.Repetition && snap.StagnantRounds >= 3 {
		return &Decision{
			Action:   ActionNudge,
			Reason:   "检测到重复操作模式（3轮以上相同策略），建议换一种方式",
			Severity: SevWarn,
			Detail:   "repeated_tool_pattern",
		}
	}
	return nil
}

// highErrorRateRule fires when >50% of tool calls failed (min 4 calls).
func highErrorRateRule(snap *Snapshot) *Decision {
	total := snap.ToolCallsTotal + snap.ToolErrors
	if total >= 4 && snap.ToolErrors*2 > total {
		return &Decision{
			Action:   ActionNudge,
			Reason:   fmt.Sprintf("工具调用错误率过高（%d/%d 失败），请检查参数或换一种方式", snap.ToolErrors, total),
			Severity: SevWarn,
			Detail:   fmt.Sprintf("errors=%d total=%d", snap.ToolErrors, total),
		}
	}
	return nil
}

// tokenBudgetWarningRule fires when cumulative tokens exceed 800k — not a stop, just a nudge.
func tokenBudgetWarningRule(snap *Snapshot) *Decision {
	const warnThreshold = 800000
	if snap.TokenIn+snap.TokenOut > warnThreshold && snap.TokenOut > 0 {
		// Only fire once: high token usage is informational
		return &Decision{
			Action:   ActionContinue,
			Reason:   fmt.Sprintf("累计 Token 使用量较高（≈%d），请注意上下文长度", snap.TokenIn+snap.TokenOut),
			Severity: SevInfo,
			Detail:   fmt.Sprintf("tokensIn=%d tokensOut=%d", snap.TokenIn, snap.TokenOut),
		}
	}
	return nil
}

// --- Decision formatting helpers ---

// FrontendMessage converts a Decision into a user-facing message string.
func (d *Decision) FrontendMessage() string {
	switch d.Action {
	case ActionForceStop:
		return fmt.Sprintf("🛑 %s", d.Reason)
	case ActionNudge:
		return fmt.Sprintf("💡 %s", d.Reason)
	case ActionEscalate:
		return fmt.Sprintf("⚠️ %s", d.Reason)
	case ActionAutoContinue:
		return fmt.Sprintf("🔄 %s", d.Reason)
	default:
		return d.Reason
	}
}

// WailsEvent converts a Decision to a (event, data) pair for the frontend.
func (d *Decision) WailsEvent() (string, map[string]any) {
	return "ai:supervisor:decision", map[string]any{
		"action":   string(d.Action),
		"severity": d.Severity.String(),
		"reason":   d.Reason,
		"detail":   strings.TrimSpace(d.Detail),
	}
}
