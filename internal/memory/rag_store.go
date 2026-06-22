package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type RAGChunk struct {
	ID          string `json:"id"`
	ProjectPath string `json:"projectPath"`
	FilePath    string `json:"filePath"`
	ChunkIndex  int    `json:"chunkIndex"`
	Content     string `json:"content"`
	Language    string `json:"language"`
	IndexedAt   string `json:"indexedAt"`
}

func (s *Store) SaveRAGChunks(chunks []RAGChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO rag_chunks (id, project_path, file_path, chunk_index, content, language, indexed_at) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, c := range chunks {
		_, err = stmt.Exec(c.ID, c.ProjectPath, c.FilePath, c.ChunkIndex, c.Content, c.Language, c.IndexedAt)
		if err != nil {
			return fmt.Errorf("save chunk %s: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

func (s *Store) SaveRAGEmbedding(chunkID string, embedding []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, _ := json.Marshal(embedding)
	_, err := s.db.Exec(`INSERT OR REPLACE INTO rag_embeddings (chunk_id, embedding) VALUES (?, ?)`, chunkID, data)
	return err
}

func (s *Store) LoadRAGChunks(projectPath string) ([]RAGChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, project_path, file_path, chunk_index, content, language, indexed_at FROM rag_chunks WHERE project_path = ?`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chunks []RAGChunk
	for rows.Next() {
		var c RAGChunk
		if err := rows.Scan(&c.ID, &c.ProjectPath, &c.FilePath, &c.ChunkIndex, &c.Content, &c.Language, &c.IndexedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, nil
}

func (s *Store) LoadRAGEmbedding(chunkID string) ([]float32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var data []byte
	err := s.db.QueryRow(`SELECT embedding FROM rag_embeddings WHERE chunk_id = ?`, chunkID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var embedding []float32
	json.Unmarshal(data, &embedding)
	return embedding, nil
}

func (s *Store) DeleteRAGChunksForFile(projectPath, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(`DELETE FROM rag_embeddings WHERE chunk_id IN (SELECT id FROM rag_chunks WHERE project_path = ? AND file_path = ?)`, projectPath, filePath)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`DELETE FROM rag_chunks WHERE project_path = ? AND file_path = ?`, projectPath, filePath)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SaveRAGFileMeta(projectPath, filePath string, modTime time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO rag_file_meta (project_path, file_path, mod_time) VALUES (?, ?, ?)`, projectPath, filePath, modTime.Format(time.RFC3339))
	return err
}

func (s *Store) LoadRAGFileMeta(projectPath string) (map[string]time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT file_path, mod_time FROM rag_file_meta WHERE project_path = ?`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[string]time.Time)
	for rows.Next() {
		var fp, mt string
		if err := rows.Scan(&fp, &mt); err != nil {
			return nil, err
		}
		t, _ := time.Parse(time.RFC3339, mt)
		result[fp] = t
	}
	return result, nil
}

func (s *Store) DeleteRAGIndex(projectPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tx.Exec(`DELETE FROM rag_embeddings WHERE chunk_id IN (SELECT id FROM rag_chunks WHERE project_path = ?)`, projectPath)
	tx.Exec(`DELETE FROM rag_chunks WHERE project_path = ?`, projectPath)
	tx.Exec(`DELETE FROM rag_file_meta WHERE project_path = ?`, projectPath)
	return tx.Commit()
}
