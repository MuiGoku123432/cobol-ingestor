package main

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SavedQuery represents a stored Cypher query.
type SavedQuery struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Cypher    string `json:"cypher"`
	Category  string `json:"category"`
	Builtin   bool   `json:"builtin"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// QueryStoreService manages saved Cypher queries in SQLite.
type QueryStoreService struct {
	app *App
	db  *sql.DB
	mu  sync.Mutex
}

func (s *QueryStoreService) open(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("opening queries db: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return fmt.Errorf("setting pragma: %w", err)
		}
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS saved_queries (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			cypher     TEXT NOT NULL,
			category   TEXT DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`); err != nil {
		db.Close()
		return fmt.Errorf("creating saved_queries table: %w", err)
	}

	s.db = db
	return nil
}

// Close closes the underlying database.
func (s *QueryStoreService) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

// ListQueries returns all saved queries plus builtins.
func (s *QueryStoreService) ListQueries() ([]SavedQuery, error) {
	builtins := s.GetBuiltinQueries()

	if s.db == nil {
		return builtins, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`SELECT id, name, cypher, category, created_at, updated_at FROM saved_queries ORDER BY updated_at DESC`)
	if err != nil {
		return builtins, fmt.Errorf("listing queries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var q SavedQuery
		if err := rows.Scan(&q.ID, &q.Name, &q.Cypher, &q.Category, &q.CreatedAt, &q.UpdatedAt); err != nil {
			continue
		}
		builtins = append(builtins, q)
	}
	return builtins, nil
}

// SaveQuery creates or updates a saved query.
func (s *QueryStoreService) SaveQuery(name, cypher, category string) (*SavedQuery, error) {
	if s.db == nil {
		return nil, fmt.Errorf("query store not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	q := &SavedQuery{
		ID:        uuid.New().String(),
		Name:      name,
		Cypher:    cypher,
		Category:  category,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := s.db.Exec(
		`INSERT INTO saved_queries (id, name, cypher, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		q.ID, q.Name, q.Cypher, q.Category, q.CreatedAt, q.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("saving query: %w", err)
	}
	return q, nil
}

// DeleteQuery removes a saved query by ID.
func (s *QueryStoreService) DeleteQuery(id string) error {
	if s.db == nil {
		return fmt.Errorf("query store not initialized")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM saved_queries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting query: %w", err)
	}
	return nil
}

// GetBuiltinQueries returns the set of predefined useful queries.
func (s *QueryStoreService) GetBuiltinQueries() []SavedQuery {
	return []SavedQuery{
		{
			ID:       "builtin-calls",
			Name:     "All CALLS relationships",
			Cypher:   "MATCH (p:Program)-[r:CALLS]->(q:Program) RETURN p.programId, q.programId LIMIT 100",
			Category: "Relationships",
			Builtin:  true,
		},
		{
			ID:       "builtin-dead-paragraphs",
			Name:     "Dead paragraphs",
			Cypher:   "MATCH (p:Paragraph) WHERE p.deadCode = true RETURN p.name, p.programId",
			Category: "Analysis",
			Builtin:  true,
		},
		{
			ID:       "builtin-risk",
			Name:     "Programs by risk",
			Cypher:   "MATCH (p:Program) WHERE p.riskScore > 0 RETURN p.programId, p.riskScore ORDER BY p.riskScore DESC",
			Category: "Analysis",
			Builtin:  true,
		},
		{
			ID:       "builtin-copybook-usage",
			Name:     "Copybook usage",
			Cypher:   "MATCH (c:Copybook)<-[:INCLUDES]-(p:Program) RETURN c.name, count(p) AS usage ORDER BY usage DESC",
			Category: "Relationships",
			Builtin:  true,
		},
	}
}
