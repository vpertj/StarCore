package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// PersistedHistory is the on-disk format for file history.
type PersistedHistory struct {
	Changes []FileChange `json:"changes"`
	Pos     int          `json:"pos"`
}

// FileHistory tracks file changes within a session for undo/redo.
type FileHistory struct {
	mu         sync.Mutex
	changes    []FileChange
	pos        int // current position in history
	savePath   string
	maxChanges int // maximum history entries to keep
}

// NewFileHistory creates a new file history tracker.
func NewFileHistory() *FileHistory {
	return &FileHistory{
		changes:    make([]FileChange, 0),
		pos:        -1,
		maxChanges: 50,
	}
}

// SetSavePath sets the file path for persisting history to disk.
func (h *FileHistory) SetSavePath(dir string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.savePath = filepath.Join(dir, "file_history.json")
}

// Save persists the current history to disk.
func (h *FileHistory) Save() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.savePath == "" {
		return
	}

	data := PersistedHistory{
		Changes: h.changes,
		Pos:     h.pos,
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(h.savePath), 0755)
	os.WriteFile(h.savePath, jsonData, 0644)
}

// Load restores history from disk. Returns true if history was loaded.
func (h *FileHistory) Load() bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.savePath == "" {
		return false
	}

	data, err := os.ReadFile(h.savePath)
	if err != nil {
		return false
	}

	var persisted PersistedHistory
	if err := json.Unmarshal(data, &persisted); err != nil {
		return false
	}

	h.changes = persisted.Changes
	h.pos = persisted.Pos
	return true
}

// Record records a file change and auto-saves.
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
	// Trim oldest entries if exceeding max
	if len(h.changes) > h.maxChanges {
		h.changes = h.changes[len(h.changes)-h.maxChanges:]
	}
	h.pos = len(h.changes) - 1

	// Auto-save on each change
	h.saveUnlocked()
}

// saveUnlocked saves without acquiring the lock (caller must hold mu).
func (h *FileHistory) saveUnlocked() {
	if h.savePath == "" {
		return
	}
	data := PersistedHistory{Changes: h.changes, Pos: h.pos}
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	os.MkdirAll(filepath.Dir(h.savePath), 0755)
	os.WriteFile(h.savePath, jsonData, 0644)
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
