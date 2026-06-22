package memory

import (
	"encoding/json"

	"os"
	"path/filepath"
	"time"
)

type SessionState struct {
	ActiveConvID   string    `json:"activeConvId"`
	ProjectPath    string    `json:"projectPath"`
	AgentID        string    `json:"agentId"`
	Mode           string    `json:"mode"`
	ProviderID     string    `json:"providerId"`
	Model          string    `json:"model"`
	LastMessageAt  string    `json:"lastMessageAt"`
	UnsavedContent string    `json:"unsavedContent,omitempty"`
	Crashed        bool      `json:"crashed"`
	SavedAt        time.Time `json:"savedAt"`
}

func (s *Store) SaveSessionState(state *SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO preferences (key, value, scope) VALUES (?, ?, 'session')`,
		"active_session", toJSONString(state))
	return err
}

func (s *Store) LoadSessionState() (*SessionState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var value string
	err := s.db.QueryRow(`SELECT value FROM preferences WHERE key = ? AND scope = 'session'`, "active_session").Scan(&value)
	if err != nil {
		return nil, err
	}
	var state SessionState
	if err := json.Unmarshal([]byte(value), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) MarkSessionCrashed() error {
	state, err := s.LoadSessionState()
	if err != nil {
		return err
	}
	state.Crashed = true
	state.SavedAt = time.Now()
	return s.SaveSessionState(state)
}

func (s *Store) ClearSessionState() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM preferences WHERE key = ? AND scope = 'session'`, "active_session")
	return err
}

func (s *Store) GetRecentConversations(projectPath string, limit int) ([]Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 5
	}
	cutoff := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	rows, err := s.db.Query(`SELECT id, project_path, agent_id, model, provider_id, title, summary, created_at, updated_at, message_count
		FROM conversations WHERE project_path = ? AND updated_at > ? ORDER BY updated_at DESC LIMIT ?`,
		projectPath, cutoff, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.ProjectPath, &c.AgentID, &c.Model, &c.ProviderID,
			&c.Title, &c.Summary, &c.CreatedAt, &c.UpdatedAt, &c.MessageCount); err != nil {
			return nil, err
		}
		result = append(result, c)
	}
	return result, nil
}

func SaveCrashMarker(dir string) error {
	marker := filepath.Join(dir, ".crash_marker")
	data, _ := json.Marshal(map[string]any{
		"crashedAt": time.Now().Format(time.RFC3339),
		"pid":       os.Getpid(),
	})
	return os.WriteFile(marker, data, 0644)
}

func CheckAndClearCrashMarker(dir string) (bool, string) {
	marker := filepath.Join(dir, ".crash_marker")
	data, err := os.ReadFile(marker)
	if err != nil {
		return false, ""
	}
	os.Remove(marker)
	var m map[string]any
	json.Unmarshal(data, &m)
	t, _ := m["crashedAt"].(string)
	return true, t
}

func toJSONString(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
