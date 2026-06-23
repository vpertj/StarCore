package tools

import (
	"fmt"
	"strings"
	"sync"
)

// TodoItem represents a single task in the AI's working memory.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`     // pending | in_progress | completed
	ActiveForm string `json:"activeForm"` // gerund form shown when in_progress
}

// ToolCallRecord records a tool call for repetition detection.
type ToolCallRecord struct {
	Name      string
	ArgsHash  string // simplified hash of arguments
	Round     int
}

// LoopState is shared mutable state between the agent loop and tools like todo_write.
// It survives across loop iterations but is reset per conversation.
type LoopState struct {
	mu sync.Mutex

	Todos        []TodoItem
	FilesTouched []string // file paths created or modified this session
	Decisions    []string // key decisions (max 5, oldest evicted)
	LastAction   string   // one-line description of last completed action

	// Goal tracking for anti-drift
	OriginalGoal   string // the user's original request
	GoalKeywords   []string // extracted keywords from the goal
	CurrentStep    string // what the agent is currently working on
	ProgressPct    int    // estimated completion percentage

	// Tool call history for semantic repetition detection
	ToolHistory []ToolCallRecord
	MaxHistory  int

	// Stagnation tracking
	StagnantRounds    int // rounds with no meaningful progress
	ToolCallsThisRound int
	FilesModifiedThisRound int
}

// NewLoopState creates a fresh loop state.
func NewLoopState() *LoopState {
	return &LoopState{
		MaxHistory: 30,
	}
}

// Reset clears all state for a new conversation.
func (s *LoopState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Todos = nil
	s.FilesTouched = nil
	s.Decisions = nil
	s.LastAction = ""
	s.OriginalGoal = ""
	s.GoalKeywords = nil
	s.CurrentStep = ""
	s.ProgressPct = 0
	s.ToolHistory = nil
	s.StagnantRounds = 0
	s.ToolCallsThisRound = 0
	s.FilesModifiedThisRound = 0
}

// SetOriginalGoal stores the user's original request and extracts keywords.
func (s *LoopState) SetOriginalGoal(goal string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OriginalGoal = goal
	s.GoalKeywords = extractKeywords(goal)
}

// GetOriginalGoal returns the stored original goal.
func (s *LoopState) GetOriginalGoal() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.OriginalGoal
}

// SetCurrentStep updates what the agent is currently doing.
func (s *LoopState) SetCurrentStep(step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentStep = step
}

// SetTodos replaces the entire todo list.
func (s *LoopState) SetTodos(todos []TodoItem) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Todos = todos
}

// GetTodos returns a copy of the current todo list.
func (s *LoopState) GetTodos() []TodoItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TodoItem, len(s.Todos))
	copy(out, s.Todos)
	return out
}

// AddFileTouched records a file that was created or modified.
func (s *LoopState) AddFileTouched(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.FilesTouched {
		if p == path {
			return
		}
	}
	s.FilesTouched = append(s.FilesTouched, path)
}

// GetFilesTouched returns a copy of the touched files list.
func (s *LoopState) GetFilesTouched() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.FilesTouched))
	copy(out, s.FilesTouched)
	return out
}

// AddDecision records a key decision, keeping at most 5.
func (s *LoopState) AddDecision(decision string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Decisions = append(s.Decisions, decision)
	if len(s.Decisions) > 5 {
		s.Decisions = s.Decisions[len(s.Decisions)-5:]
	}
}

// SetLastAction records the most recent completed action.
func (s *LoopState) SetLastAction(action string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastAction = action
}

// RecordToolCall records a tool call for repetition detection.
func (s *LoopState) RecordToolCall(name string, args map[string]any, round int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolHistory = append(s.ToolHistory, ToolCallRecord{
		Name:     name,
		ArgsHash: hashToolArgs(name, args),
		Round:    round,
	})
	if len(s.ToolHistory) > s.MaxHistory {
		s.ToolHistory = s.ToolHistory[len(s.ToolHistory)-s.MaxHistory:]
	}
}

// CheckSemanticRepetition detects if the agent is repeating the same approach.
// Returns true if the last N tool calls are semantically similar to the previous N.
func (s *LoopState) CheckSemanticRepetition(windowSize int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.ToolHistory) < windowSize*2 {
		return false
	}

	recent := s.ToolHistory[len(s.ToolHistory)-windowSize:]
	previous := s.ToolHistory[len(s.ToolHistory)-windowSize*2 : len(s.ToolHistory)-windowSize]

	sameCount := 0
	for i := range recent {
		if recent[i].Name == previous[i].Name && recent[i].ArgsHash == previous[i].ArgsHash {
			sameCount++
		}
	}

	return sameCount >= windowSize*80/100
}

// RecordRound records the end of a round and checks for stagnation.
func (s *LoopState) RecordRound(toolCalls int, filesModified int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ToolCallsThisRound = toolCalls
	s.FilesModifiedThisRound = filesModified

	if toolCalls == 0 && filesModified == 0 {
		s.StagnantRounds++
	} else {
		s.StagnantRounds = 0
	}
}

// IsStagnant returns true if the agent hasn't made progress for N rounds.
func (s *LoopState) IsStagnant(threshold int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.StagnantRounds >= threshold
}

// GetStagnantRounds returns the current stagnation count.
func (s *LoopState) GetStagnantRounds() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.StagnantRounds
}

// ProjectStateSummary builds a compact context string for injection into system messages.
func (s *LoopState) ProjectStateSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Todos) == 0 && len(s.FilesTouched) == 0 && s.LastAction == "" && s.OriginalGoal == "" {
		return ""
	}

	var out string
	out = "[Project State]\n"

	if s.OriginalGoal != "" {
		out += "Original goal: " + s.OriginalGoal + "\n"
	}

	if s.CurrentStep != "" {
		out += "Current step: " + s.CurrentStep + "\n"
	}

	if len(s.Todos) > 0 {
		completed := 0
		total := len(s.Todos)
		out += "Tasks:\n"
		for _, t := range s.Todos {
			switch t.Status {
			case "completed":
				completed++
				out += "  ✅ " + t.Content + "\n"
			case "in_progress":
				out += "  🔄 " + t.Content + "\n"
			default:
				out += "  ⏳ " + t.Content + "\n"
			}
		}
		if total > 0 {
			pct := completed * 100 / total
			out += fmt.Sprintf("Progress: %d/%d tasks (%d%%)\n", completed, total, pct)
		}
	}

	if len(s.FilesTouched) > 0 {
		out += "Files modified: "
		for i, f := range s.FilesTouched {
			if i > 0 {
				out += ", "
			}
			out += f
		}
		out += "\n"
	}

	if len(s.Decisions) > 0 {
		out += "Key decisions:\n"
		for _, d := range s.Decisions {
			out += "  • " + d + "\n"
		}
	}

	if s.LastAction != "" {
		out += "Last action: " + s.LastAction + "\n"
	}

	return out
}

// AntiDriftReminder builds a message to re-inject the original goal.
func (s *LoopState) AntiDriftReminder() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.OriginalGoal == "" {
		return ""
	}

	var out string
	out = fmt.Sprintf("[防偏离提醒] 你的原始目标是: %s\n", s.OriginalGoal)

	if s.CurrentStep != "" {
		out += fmt.Sprintf("当前步骤: %s\n", s.CurrentStep)
	}

	if s.StagnantRounds >= 3 {
		out += fmt.Sprintf("⚠️ 已连续 %d 轮无实质进展。请回顾目标，换一种方式尝试。\n", s.StagnantRounds)
	}

	out += "请确保你的每一步操作都直接服务于原始目标。如果偏离了，请纠正。\n"

	return out
}

// extractKeywords extracts simple keywords from text for goal matching.
func extractKeywords(text string) []string {
	// Simple keyword extraction: split by spaces and punctuation, filter short words
	words := strings.FieldsFunc(text, func(r rune) bool {
		return r == ' ' || r == '，' || r == '。' || r == '、' || r == '；' || r == '：' ||
			r == '\u201c' || r == '\u201d' || r == '(' || r == ')' || r == '\n'
	})
	var keywords []string
	seen := make(map[string]bool)
	for _, w := range words {
		w = strings.TrimSpace(w)
		if len(w) >= 2 && !seen[w] {
			keywords = append(keywords, w)
			seen[w] = true
		}
	}
	if len(keywords) > 20 {
		keywords = keywords[:20]
	}
	return keywords
}

// hashToolArgs creates a simplified hash of tool arguments for comparison.
func hashToolArgs(toolName string, args map[string]any) string {
	// For file operations, hash by path only (same file = same approach)
	// For other tools, hash by tool name + key arguments
	switch toolName {
	case "read_file", "write_file", "edit_file", "delete_file", "move_file":
		if path, ok := args["path"].(string); ok {
			return path
		}
		if source, ok := args["source"].(string); ok {
			return source
		}
	case "search_files":
		if query, ok := args["query"].(string); ok {
			return query
		}
	case "glob_files":
		if pattern, ok := args["pattern"].(string); ok {
			return pattern
		}
	case "execute_command":
		if cmd, ok := args["command"].(string); ok {
			return cmd
		}
	}
	// Default: just use tool name
	return toolName
}
