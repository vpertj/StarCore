package tools

import "sync"

// TodoItem represents a single task in the AI's working memory.
type TodoItem struct {
	Content    string `json:"content"`
	Status     string `json:"status"`     // pending | in_progress | completed
	ActiveForm string `json:"activeForm"` // gerund form shown when in_progress
}

// LoopState is shared mutable state between the agent loop and tools like todo_write.
// It survives across loop iterations but is reset per conversation.
type LoopState struct {
	mu sync.Mutex

	Todos        []TodoItem
	FilesTouched []string // file paths created or modified this session
	Decisions    []string // key decisions (max 5, oldest evicted)
	LastAction   string   // one-line description of last completed action
}

// NewLoopState creates a fresh loop state.
func NewLoopState() *LoopState {
	return &LoopState{}
}

// Reset clears all state for a new conversation.
func (s *LoopState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Todos = nil
	s.FilesTouched = nil
	s.Decisions = nil
	s.LastAction = ""
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

// ProjectStateSummary builds a compact context string for injection into system messages.
func (s *LoopState) ProjectStateSummary() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Todos) == 0 && len(s.FilesTouched) == 0 && s.LastAction == "" {
		return ""
	}

	var out string
	out = "[Project State]\n"

	if len(s.Todos) > 0 {
		out += "Tasks:\n"
		for _, t := range s.Todos {
			switch t.Status {
			case "completed":
				out += "  ✅ " + t.Content + "\n"
			case "in_progress":
				out += "  🔄 " + t.Content + "\n"
			default:
				out += "  ⏳ " + t.Content + "\n"
			}
		}
	}

	if len(s.FilesTouched) > 0 {
		out += "Files modified this session: "
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
