package cache

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Cache tracks file hashes in SQLite for incremental processing.
type Cache struct {
	db *sql.DB
	mu sync.Mutex
}

// New opens (or creates) the SQLite cache at dbPath.
func New(dbPath string) (*Cache, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening cache db: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS file_cache (
			file_path    TEXT PRIMARY KEY,
			content_hash TEXT NOT NULL,
			processed_at DATETIME NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating cache table: %w", err)
	}

	// Pass-aware cache table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS pass_cache (
			file_path    TEXT NOT NULL,
			pass         INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			processed_at DATETIME NOT NULL,
			PRIMARY KEY (file_path, pass)
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating pass_cache table: %w", err)
	}

	return &Cache{db: db}, nil
}

// IsChanged returns true if the file has no cache entry or its hash differs.
func (c *Cache) IsChanged(path, hash string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var storedHash string
	err := c.db.QueryRow(
		"SELECT content_hash FROM file_cache WHERE file_path = ?", path,
	).Scan(&storedHash)

	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying cache: %w", err)
	}

	return storedHash != hash, nil
}

// MarkProcessed records (or updates) the file's hash in the cache.
func (c *Cache) MarkProcessed(path, hash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO file_cache (file_path, content_hash, processed_at) VALUES (?, ?, ?)",
		path, hash, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("updating cache: %w", err)
	}
	return nil
}

// IsChangedForPass returns true if the file has no cache entry for the given pass or its hash differs.
func (c *Cache) IsChangedForPass(path, hash string, pass int) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var storedHash string
	err := c.db.QueryRow(
		"SELECT content_hash FROM pass_cache WHERE file_path = ? AND pass = ?", path, pass,
	).Scan(&storedHash)

	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("querying pass cache: %w", err)
	}

	return storedHash != hash, nil
}

// MarkProcessedForPass records (or updates) the file's hash for a specific pass.
func (c *Cache) MarkProcessedForPass(path, hash string, pass int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO pass_cache (file_path, pass, content_hash, processed_at) VALUES (?, ?, ?, ?)",
		path, pass, hash, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("updating pass cache: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}
