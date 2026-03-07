package modernize

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteSessionStore implements SessionStore using SQLite.
type SQLiteSessionStore struct {
	db          *sql.DB
	mu          sync.Mutex
	maxSessions int
}

// NewSQLiteSessionStore opens (or creates) a SQLite database at dbPath for session storage.
func NewSQLiteSessionStore(dbPath string, maxSessions int) (*SQLiteSessionStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening sessions db: %w", err)
	}

	// Enable WAL mode and foreign keys
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting pragma: %w", err)
		}
	}

	// Create schema
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id         TEXT PRIMARY KEY,
			title      TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating sessions table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS session_messages (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
			role       TEXT NOT NULL,
			content    TEXT NOT NULL,
			seq        INTEGER NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating session_messages table: %w", err)
	}

	if _, err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_messages_session ON session_messages(session_id, seq)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating index: %w", err)
	}

	if maxSessions <= 0 {
		maxSessions = 50
	}

	return &SQLiteSessionStore{db: db, maxSessions: maxSessions}, nil
}

func (s *SQLiteSessionStore) List() ([]SessionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query("SELECT id, title, updated_at FROM sessions ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}
	defer rows.Close()

	var sessions []SessionSummary
	for rows.Next() {
		var ss SessionSummary
		if err := rows.Scan(&ss.ID, &ss.Title, &ss.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning session: %w", err)
		}
		sessions = append(sessions, ss)
	}
	if sessions == nil {
		sessions = []SessionSummary{}
	}
	return sessions, rows.Err()
}

func (s *SQLiteSessionStore) Get(id string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &Session{ID: id}
	err := s.db.QueryRow(
		"SELECT title, created_at, updated_at FROM sessions WHERE id = ?", id,
	).Scan(&session.Title, &session.CreatedAt, &session.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	rows, err := s.db.Query(
		"SELECT role, content FROM session_messages WHERE session_id = ? ORDER BY seq", id,
	)
	if err != nil {
		return nil, fmt.Errorf("querying messages: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m chatInputMessage
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, fmt.Errorf("scanning message: %w", err)
		}
		session.Messages = append(session.Messages, m)
	}
	if session.Messages == nil {
		session.Messages = []chatInputMessage{}
	}
	return session, rows.Err()
}

func (s *SQLiteSessionStore) Create(title string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	session := &Session{
		ID:        uuid.New().String(),
		Title:     title,
		Messages:  []chatInputMessage{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(
		"INSERT INTO sessions (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		session.ID, session.Title, session.CreatedAt, session.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("creating session: %w", err)
	}

	s.pruneOldest()
	return session, nil
}

func (s *SQLiteSessionStore) Update(id string, messages []chatInputMessage, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()

	if title != "" {
		if _, err := tx.Exec("UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?", title, now, id); err != nil {
			return fmt.Errorf("updating session title: %w", err)
		}
	} else {
		if _, err := tx.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", now, id); err != nil {
			return fmt.Errorf("updating session timestamp: %w", err)
		}
	}

	if messages != nil {
		if _, err := tx.Exec("DELETE FROM session_messages WHERE session_id = ?", id); err != nil {
			return fmt.Errorf("deleting old messages: %w", err)
		}
		for i, m := range messages {
			if _, err := tx.Exec(
				"INSERT INTO session_messages (session_id, role, content, seq) VALUES (?, ?, ?, ?)",
				id, m.Role, m.Content, i,
			); err != nil {
				return fmt.Errorf("inserting message %d: %w", i, err)
			}
		}
	}

	return tx.Commit()
}

func (s *SQLiteSessionStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

func (s *SQLiteSessionStore) Close() error {
	return s.db.Close()
}

// pruneOldest removes the oldest sessions beyond maxSessions. Must be called with mu held.
func (s *SQLiteSessionStore) pruneOldest() {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count); err != nil || count <= s.maxSessions {
		return
	}
	s.db.Exec(`
		DELETE FROM sessions WHERE id IN (
			SELECT id FROM sessions ORDER BY updated_at ASC LIMIT ?
		)
	`, count-s.maxSessions)
}
