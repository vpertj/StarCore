package memory

import "fmt"

type Conversation struct {
	ID           string `json:"id"`
	ProjectPath  string `json:"projectPath"`
	AgentID      string `json:"agentId"`
	Model        string `json:"model"`
	ProviderID   string `json:"providerId"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	MessageCount int    `json:"messageCount"`
}

type Message struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	Seq            int    `json:"seq"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Thinking       string `json:"thinking,omitempty"`
	TokensIn       int    `json:"tokensIn"`
	TokensOut      int    `json:"tokensOut"`
	CreatedAt      string `json:"createdAt"`
	Metadata       string `json:"metadata,omitempty"`
}

func (s *Store) SaveConversation(conv *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO conversations
		(id, project_path, agent_id, model, provider_id, title, summary, created_at, updated_at, message_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		conv.ID, conv.ProjectPath, conv.AgentID, conv.Model, conv.ProviderID,
		conv.Title, conv.Summary, conv.CreatedAt, conv.UpdatedAt, conv.MessageCount)
	return err
}

func (s *Store) GetConversations(projectPath string, limit int, offset int) ([]Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`SELECT id, project_path, agent_id, model, provider_id, title, summary, created_at, updated_at, message_count
		FROM conversations WHERE project_path = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?`,
		projectPath, limit, offset)
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
	return result, rows.Err()
}

func (s *Store) GetConversation(id string) (*Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var c Conversation
	err := s.db.QueryRow(`SELECT id, project_path, agent_id, model, provider_id, title, summary, created_at, updated_at, message_count
		FROM conversations WHERE id = ?`, id).Scan(
		&c.ID, &c.ProjectPath, &c.AgentID, &c.Model, &c.ProviderID,
		&c.Title, &c.Summary, &c.CreatedAt, &c.UpdatedAt, &c.MessageCount)
	if err != nil {
		return nil, fmt.Errorf("get conversation %s: %w", id, err)
	}
	return &c, nil
}

func (s *Store) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM messages WHERE conversation_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM conversations WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DeleteMessage(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var convID string
	if err := tx.QueryRow("SELECT conversation_id FROM messages WHERE id = ?", id).Scan(&convID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM messages WHERE id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE conversations SET message_count = message_count - 1, updated_at = ? WHERE id = ?", now(), convID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveMessage(msg *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`INSERT OR REPLACE INTO messages
		(id, conversation_id, seq, role, content, thinking, tokens_in, tokens_out, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.ConversationID, msg.Seq, msg.Role, msg.Content,
		msg.Thinking, msg.TokensIn, msg.TokensOut, msg.CreatedAt, msg.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?), updated_at = ? WHERE id = ?`,
		msg.ConversationID, now(), msg.ConversationID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetMessages(conversationID string) ([]Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, conversation_id, seq, role, content, thinking, tokens_in, tokens_out, created_at, COALESCE(metadata, '')
		FROM messages WHERE conversation_id = ? ORDER BY seq ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.Role, &m.Content,
			&m.Thinking, &m.TokensIn, &m.TokensOut, &m.CreatedAt, &m.Metadata); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

func (s *Store) UpdateConversationSummary(id string, summary string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`UPDATE conversations SET summary = ?, updated_at = ? WHERE id = ?`, summary, now(), id)
	return err
}
