package memory

import (
	"fmt"
	"time"
)

type Knowledge struct {
	ID          string `json:"id"`
	ProjectPath string `json:"projectPath"`
	Category    string `json:"category"`
	Key         string `json:"key"`
	Value       string `json:"value"`
	Source      string `json:"source"`
	UpdatedAt   string `json:"updatedAt"`
}

func (s *Store) SaveKnowledge(entry *Knowledge) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`DELETE FROM knowledge WHERE project_path = ? AND category = ? AND key = ?`,
		entry.ProjectPath, entry.Category, entry.Key)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO knowledge (id, project_path, category, key, value, source, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.ProjectPath, entry.Category, entry.Key, entry.Value, entry.Source, entry.UpdatedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) GetKnowledge(projectPath string) ([]Knowledge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, project_path, category, key, value, source, updated_at
		FROM knowledge WHERE project_path = ? ORDER BY updated_at DESC`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Knowledge
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.ProjectPath, &k.Category, &k.Key, &k.Value, &k.Source, &k.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (s *Store) GetKnowledgeByCategory(projectPath string, category string) ([]Knowledge, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, project_path, category, key, value, source, updated_at
		FROM knowledge WHERE project_path = ? AND category = ? ORDER BY updated_at DESC`,
		projectPath, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Knowledge
	for rows.Next() {
		var k Knowledge
		if err := rows.Scan(&k.ID, &k.ProjectPath, &k.Category, &k.Key, &k.Value, &k.Source, &k.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (s *Store) DeleteKnowledge(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.db.Exec("DELETE FROM knowledge WHERE id = ?", id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("knowledge %s not found", id)
	}
	return nil
}

// LearnPreference records a user preference observation.
func (s *Store) LearnPreference(projectPath, key, value, source string) error {
	entry := &Knowledge{
		ID:          fmt.Sprintf("pref_%d", time.Now().UnixNano()),
		ProjectPath: projectPath,
		Category:    "preference",
		Key:         key,
		Value:       value,
		Source:      source,
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	return s.SaveKnowledge(entry)
}

// LearnPattern records an observed coding pattern.
func (s *Store) LearnPattern(projectPath, pattern, description string) error {
	entry := &Knowledge{
		ID:          fmt.Sprintf("pat_%d", time.Now().UnixNano()),
		ProjectPath: projectPath,
		Category:    "pattern",
		Key:         pattern,
		Value:       description,
		Source:      "observed",
		UpdatedAt:   time.Now().Format(time.RFC3339),
	}
	return s.SaveKnowledge(entry)
}

// GetPreferences returns all learned preferences for a project.
func (s *Store) GetPreferences(projectPath string) ([]Knowledge, error) {
	return s.GetKnowledgeByCategory(projectPath, "preference")
}

// GetPatterns returns all observed patterns for a project.
func (s *Store) GetPatterns(projectPath string) ([]Knowledge, error) {
	return s.GetKnowledgeByCategory(projectPath, "pattern")
}
