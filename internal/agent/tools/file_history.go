package tools

import (
	"os"
	"sync"
	"time"
)

// FileChange represents a single file change for undo/redo.
type FileChange struct {
	FilePath    string    `json:"filePath"`
	OldContent  string    `json:"oldContent"`
	NewContent  string    `json:"newContent"`
	Operation   string    `json:"operation"` // "write", "edit"
	Timestamp   time.Time `json:"timestamp"`
	Description string    `json:"description"`
}

// FileHistory tracks file changes within a session for undo/redo.
type FileHistory struct {
	mu      sync.Mutex
	changes []FileChange
	pos     int // current position in history
}

// NewFileHistory creates a new file history tracker.
func NewFileHistory() *FileHistory {
	return &FileHistory{
		changes: make([]FileChange, 0),
		pos:     -1,
	}
}

// Record records a file change.
func (h *FileHistory) Record(filePath, oldContent, newContent, operation, description string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If we're in the middle of history (after undo), truncate forward
	if h.pos < len(h.changes)-1 {
		h.changes = h.changes[:h.pos+1]
	}

	h.changes = append(h.changes, FileChange{
		FilePath:    filePath,
		OldContent:  oldContent,
		NewContent:  newContent,
		Operation:   operation,
		Timestamp:   time.Now(),
		Description: description,
	})
	h.pos = len(h.changes) - 1
}

// Undo reverts the last change and returns the change that was undone.
func (h *FileHistory) Undo() (*FileChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pos < 0 {
		return nil, nil // Nothing to undo
	}

	change := &h.changes[h.pos]

	// Write old content back
	if err := os.WriteFile(change.FilePath, []byte(change.OldContent), 0644); err != nil {
		return nil, err
	}

	h.pos--
	return change, nil
}

// Redo re-applies the last undone change.
func (h *FileHistory) Redo() (*FileChange, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pos >= len(h.changes)-1 {
		return nil, nil // Nothing to redo
	}

	h.pos++
	change := &h.changes[h.pos]

	// Write new content
	if err := os.WriteFile(change.FilePath, []byte(change.NewContent), 0644); err != nil {
		return nil, err
	}

	return change, nil
}

// CanUndo returns whether undo is possible.
func (h *FileHistory) CanUndo() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pos >= 0
}

// CanRedo returns whether redo is possible.
func (h *FileHistory) CanRedo() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pos < len(h.changes)-1
}

// GetHistory returns all recorded changes.
func (h *FileHistory) GetHistory() []FileChange {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]FileChange, len(h.changes))
	copy(result, h.changes)
	return result
}

// Clear clears the history.
func (h *FileHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.changes = h.changes[:0]
	h.pos = -1
}
