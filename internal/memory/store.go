package memory

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
	mu sync.RWMutex
}

// DB returns the underlying database handle (for sharing with trace storage, etc.)
func (s *Store) DB() *sql.DB {
	return s.db
}

func NewStore(dataDir string) (*Store, error) {
	if dataDir == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("get user config dir: %w", err)
		}
		dataDir = filepath.Join(configDir, "starcore", "data")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "starcore.db")
	dsn := dbPath + "?_journal_mode=WAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	s := &Store{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) initSchema() error {
	ddl := `
CREATE TABLE IF NOT EXISTS conversations (
    id TEXT PRIMARY KEY,
    project_path TEXT NOT NULL,
    agent_id TEXT DEFAULT '',
    model TEXT DEFAULT '',
    provider_id TEXT DEFAULT '',
    title TEXT DEFAULT '',
    summary TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    message_count INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS messages (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL REFERENCES conversations(id),
    seq INTEGER NOT NULL,
    role TEXT NOT NULL,
    content TEXT DEFAULT '',
    thinking TEXT DEFAULT '',
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS knowledge (
    id TEXT PRIMARY KEY,
    project_path TEXT NOT NULL,
    category TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT DEFAULT '',
    source TEXT DEFAULT 'user',
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS preferences (
    key TEXT PRIMARY KEY,
    value TEXT DEFAULT '',
    scope TEXT DEFAULT 'global'
);

CREATE TABLE IF NOT EXISTS token_usage (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    provider_id TEXT DEFAULT '',
    model TEXT DEFAULT '',
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost REAL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_knowledge_project ON knowledge(project_path);
CREATE INDEX IF NOT EXISTS idx_token_usage_conv ON token_usage(conversation_id);

CREATE TABLE IF NOT EXISTS rag_chunks (
    id TEXT PRIMARY KEY,
    project_path TEXT NOT NULL,
    file_path TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    language TEXT DEFAULT '',
    indexed_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS rag_embeddings (
    chunk_id TEXT PRIMARY KEY REFERENCES rag_chunks(id),
    embedding BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS rag_file_meta (
    project_path TEXT NOT NULL,
    file_path TEXT NOT NULL,
    mod_time TEXT NOT NULL,
    PRIMARY KEY (project_path, file_path)
);

CREATE INDEX IF NOT EXISTS idx_rag_chunks_project ON rag_chunks(project_path);
CREATE INDEX IF NOT EXISTS idx_rag_chunks_file ON rag_chunks(file_path);
`
	_, err := s.db.Exec(ddl)
	if err != nil {
		return err
	}
	s.db.Exec(`ALTER TABLE messages ADD COLUMN metadata TEXT DEFAULT ''`)
	return nil
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

// DeleteConversationMessages deletes all messages for a given conversation.
func (s *Store) DeleteConversationMessages(convID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM messages WHERE conversation_id = ?`, convID)
	if err != nil {
		return fmt.Errorf("delete conversation messages: %w", err)
	}
	return nil
}
