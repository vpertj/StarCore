package ai

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// --- Blackboard ---
// A shared state space for inter-agent communication.
//
// In the 5-layer architecture, the Blackboard is the "memory" that connects
// the Understand → Route → Execute pipeline. Multiple agents (or sub-agents)
// can concurrently:
//   - Publish findings    (blackboard.Write)
//   - Read other findings (blackboard.Read)
//   - Watch for updates   (blackboard.Watch)
//   - Query by pattern    (blackboard.Query)
//
// Design principles:
//   - Thread-safe: all operations are goroutine-safe
//   - Scoped: each conversation has its own blackboard (bound to convID)
//   - Ephemeral: cleared when conversation ends (no cross-talk between sessions)
//   - Schema-light: entries are key→value with optional metadata tags

// Entry represents a single piece of information on the blackboard.
type Entry struct {
	Key       string    // unique key (e.g. "file:src/main.go:analysis")
	Value     string    // content
	Author    string    // which agent/sub-agent wrote this
	Tags      []string  // searchable tags (e.g. ["analysis", "auth", "security"])
	Timestamp time.Time // when written
	Priority  int       // higher = more important (used for eviction)
	Version   int       // incremented on overwrite
}

// Blackboard is the shared state for one conversation.
type Blackboard struct {
	mu       sync.RWMutex
	convID   string
	entries  map[string]Entry
	watchers []chan Entry // channels notified on new/updated entries
}

// NewBlackboard creates a blackboard for a conversation.
func NewBlackboard(convID string) *Blackboard {
	return &Blackboard{
		convID:  convID,
		entries: make(map[string]Entry),
	}
}

// Write stores or updates an entry.
func (b *Blackboard) Write(key, value, author string, tags []string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	existing, exists := b.entries[key]
	version := 1
	if exists {
		version = existing.Version + 1
	}

	entry := Entry{
		Key:       key,
		Value:     value,
		Author:    author,
		Tags:      tags,
		Timestamp: time.Now(),
		Version:   version,
	}
	b.entries[key] = entry

	// Notify watchers (non-blocking)
	for _, ch := range b.watchers {
		select {
		case ch <- entry:
		default:
		}
	}
}

// Read retrieves an entry by key.
func (b *Blackboard) Read(key string) (Entry, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	e, ok := b.entries[key]
	return e, ok
}

// Query returns entries matching ALL given tags.
func (b *Blackboard) Query(tags []string) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(tags) == 0 {
		// Return all entries
		result := make([]Entry, 0, len(b.entries))
		for _, e := range b.entries {
			result = append(result, e)
		}
		return result
	}

	var result []Entry
	for _, e := range b.entries {
		if containsAllTags(e.Tags, tags) {
			result = append(result, e)
		}
	}
	return result
}

// QueryPrefix returns entries whose key starts with the given prefix.
func (b *Blackboard) QueryPrefix(prefix string) []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var result []Entry
	for k, e := range b.entries {
		if strings.HasPrefix(k, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// ReadString is a convenience method that returns the value directly.
func (b *Blackboard) ReadString(key string) string {
	e, ok := b.Read(key)
	if !ok {
		return ""
	}
	return e.Value
}

// Has checks if a key exists.
func (b *Blackboard) Has(key string) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.entries[key]
	return ok
}

// Delete removes an entry by key.
func (b *Blackboard) Delete(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
}

// Keys returns all entry keys.
func (b *Blackboard) Keys() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	keys := make([]string, 0, len(b.entries))
	for k := range b.entries {
		keys = append(keys, k)
	}
	return keys
}

// Size returns the number of entries.
func (b *Blackboard) Size() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.entries)
}

// Snapshot returns a copy of all entries (for serialization/tracing).
func (b *Blackboard) Snapshot() map[string]Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	snap := make(map[string]Entry, len(b.entries))
	for k, v := range b.entries {
		snap[k] = v
	}
	return snap
}

// Watch registers a channel to receive notifications on new/updated entries.
func (b *Blackboard) Watch(ch chan Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.watchers = append(b.watchers, ch)
}

// Unwatch removes a watcher channel.
func (b *Blackboard) Unwatch(ch chan Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, w := range b.watchers {
		if w == ch {
			b.watchers = append(b.watchers[:i], b.watchers[i+1:]...)
			return
		}
	}
}

// Clear removes all entries.
func (b *Blackboard) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = make(map[string]Entry)
}

// Summary returns a compact text summary of all entries (for injection into prompts).
func (b *Blackboard) Summary() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.entries) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("=== 共享黑板 ===\n")
	for k, e := range b.entries {
		sb.WriteString(fmt.Sprintf("[%s] %s: ", e.Author, k))
		// Truncate long values
		val := e.Value
		if len(val) > 200 {
			val = val[:200] + "..."
		}
		// Indent multi-line
		val = strings.ReplaceAll(val, "\n", "\n    ")
		sb.WriteString(val)
		sb.WriteString("\n")
	}
	return sb.String()
}

// --- Conversation-level registry ---
// Manages blackboards per conversation.

var (
	blackboardMu    sync.RWMutex
	blackboards     = make(map[string]*Blackboard)
	blackboardWatch chan Entry // global watcher for tracing
)

// GetBlackboard returns (or creates) the blackboard for a conversation.
func GetBlackboard(convID string) *Blackboard {
	blackboardMu.Lock()
	defer blackboardMu.Unlock()

	if bb, ok := blackboards[convID]; ok {
		return bb
	}

	bb := NewBlackboard(convID)
	blackboards[convID] = bb
	return bb
}

// DeleteBlackboard removes a conversation's blackboard.
func DeleteBlackboard(convID string) {
	blackboardMu.Lock()
	defer blackboardMu.Unlock()
	delete(blackboards, convID)
}

// SetGlobalWatcher sets a channel to receive all blackboard writes (for tracing).
func SetGlobalWatcher(ch chan Entry) {
	blackboardMu.Lock()
	defer blackboardMu.Unlock()
	blackboardWatch = ch
}

// --- Helper ---

func containsAllTags(have, need []string) bool {
	for _, n := range need {
		found := false
		for _, h := range have {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
