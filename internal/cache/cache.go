package cache

import (
	"database/sql"
	"fmt"
	"strings"
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

	// Classification cache table
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS classify_cache (
			file_path      TEXT PRIMARY KEY,
			content_hash   TEXT NOT NULL,
			file_type      TEXT NOT NULL,
			classifier     TEXT NOT NULL,
			classified_at  DATETIME NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating classify_cache table: %w", err)
	}

	// Per-chunk cache table for incremental multi-chunk file processing
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS chunk_cache (
			file_path    TEXT NOT NULL,
			pass         INTEGER NOT NULL,
			chunk_index  INTEGER NOT NULL,
			chunk_total  INTEGER NOT NULL,
			content_hash TEXT NOT NULL,
			result_json  TEXT NOT NULL,
			processed_at DATETIME NOT NULL,
			PRIMARY KEY (file_path, pass, chunk_index)
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating chunk_cache table: %w", err)
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

// BatchIsChanged returns the file paths whose hash differs from the cache (or are absent).
func (c *Cache) BatchIsChanged(pathHashes map[string]string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(pathHashes) == 0 {
		return nil, nil
	}

	// Build query with placeholders
	paths := make([]string, 0, len(pathHashes))
	for p := range pathHashes {
		paths = append(paths, p)
	}

	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}

	query := "SELECT file_path, content_hash FROM file_cache WHERE file_path IN (" +
		strings.Join(placeholders, ",") + ")"
	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch cache query: %w", err)
	}
	defer rows.Close()

	cached := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("scanning cache row: %w", err)
		}
		cached[path] = hash
	}

	var changed []string
	for _, p := range paths {
		storedHash, ok := cached[p]
		if !ok || storedHash != pathHashes[p] {
			changed = append(changed, p)
		}
	}

	return changed, nil
}

// BatchIsChangedForPass returns file paths whose hash differs from the pass cache (or are absent).
func (c *Cache) BatchIsChangedForPass(pathHashes map[string]string, pass int) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(pathHashes) == 0 {
		return nil, nil
	}

	paths := make([]string, 0, len(pathHashes))
	for p := range pathHashes {
		paths = append(paths, p)
	}

	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}
	args = append(args, pass)

	query := "SELECT file_path, content_hash FROM pass_cache WHERE file_path IN (" +
		strings.Join(placeholders, ",") + ") AND pass = ?"
	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch pass cache query: %w", err)
	}
	defer rows.Close()

	cached := make(map[string]string)
	for rows.Next() {
		var path, hash string
		if err := rows.Scan(&path, &hash); err != nil {
			return nil, fmt.Errorf("scanning pass cache row: %w", err)
		}
		cached[path] = hash
	}

	var changed []string
	for _, p := range paths {
		storedHash, ok := cached[p]
		if !ok || storedHash != pathHashes[p] {
			changed = append(changed, p)
		}
	}

	return changed, nil
}

// ClassifyResult holds a cached classification outcome.
type ClassifyResult struct {
	FileType   string
	Classifier string // "LLM" or "HEURISTIC"
}

// ClassifyEntry is an input to BatchMarkClassified.
type ClassifyEntry struct {
	Path, Hash, FileType, Classifier string
}

// BatchLookupClassification returns cached classifications where the hash still matches.
// hits contains entries whose hash is current; misses lists paths that are absent or stale.
func (c *Cache) BatchLookupClassification(pathHashes map[string]string) (hits map[string]ClassifyResult, misses []string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	hits = make(map[string]ClassifyResult, len(pathHashes))
	if len(pathHashes) == 0 {
		return hits, nil, nil
	}

	paths := make([]string, 0, len(pathHashes))
	for p := range pathHashes {
		paths = append(paths, p)
	}

	placeholders := make([]string, len(paths))
	args := make([]any, len(paths))
	for i, p := range paths {
		placeholders[i] = "?"
		args[i] = p
	}

	query := "SELECT file_path, content_hash, file_type, classifier FROM classify_cache WHERE file_path IN (" +
		strings.Join(placeholders, ",") + ")"
	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("batch classify lookup: %w", err)
	}
	defer rows.Close()

	cached := make(map[string]struct {
		hash, fileType, classifier string
	})
	for rows.Next() {
		var path, hash, ft, cls string
		if err := rows.Scan(&path, &hash, &ft, &cls); err != nil {
			return nil, nil, fmt.Errorf("scanning classify row: %w", err)
		}
		cached[path] = struct{ hash, fileType, classifier string }{hash, ft, cls}
	}

	for _, p := range paths {
		entry, ok := cached[p]
		if !ok || entry.hash != pathHashes[p] {
			misses = append(misses, p)
		} else {
			hits[p] = ClassifyResult{FileType: entry.fileType, Classifier: entry.classifier}
		}
	}

	return hits, misses, nil
}

// BatchMarkClassified upserts classification results into the cache.
func (c *Cache) BatchMarkClassified(entries []ClassifyEntry) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(entries) == 0 {
		return nil
	}

	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin classify tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO classify_cache (file_path, content_hash, file_type, classifier, classified_at) VALUES (?, ?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("prepare classify insert: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for _, e := range entries {
		if _, err := stmt.Exec(e.Path, e.Hash, e.FileType, e.Classifier, now); err != nil {
			return fmt.Errorf("inserting classify entry: %w", err)
		}
	}

	return tx.Commit()
}

// ChunkResult holds a cached per-chunk LLM response.
type ChunkResult struct {
	ChunkIndex  int
	ChunkTotal  int
	ContentHash string
	ResultJSON  string
}

// SaveChunkResult caches the raw LLM JSON response for a specific chunk.
func (c *Cache) SaveChunkResult(path string, pass, chunkIndex, chunkTotal int, contentHash, resultJSON string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec(
		"INSERT OR REPLACE INTO chunk_cache (file_path, pass, chunk_index, chunk_total, content_hash, result_json, processed_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		path, pass, chunkIndex, chunkTotal, contentHash, resultJSON, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("saving chunk result: %w", err)
	}
	return nil
}

// LoadChunkResults returns all cached chunk results for a file+pass.
// Returns nil if no cached chunks exist.
func (c *Cache) LoadChunkResults(path string, pass int) ([]ChunkResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	rows, err := c.db.Query(
		"SELECT chunk_index, chunk_total, content_hash, result_json FROM chunk_cache WHERE file_path = ? AND pass = ? ORDER BY chunk_index",
		path, pass,
	)
	if err != nil {
		return nil, fmt.Errorf("loading chunk results: %w", err)
	}
	defer rows.Close()

	var results []ChunkResult
	for rows.Next() {
		var cr ChunkResult
		if err := rows.Scan(&cr.ChunkIndex, &cr.ChunkTotal, &cr.ContentHash, &cr.ResultJSON); err != nil {
			return nil, fmt.Errorf("scanning chunk result: %w", err)
		}
		results = append(results, cr)
	}
	return results, nil
}

// ClearChunkResults removes all cached chunk results for a file+pass.
func (c *Cache) ClearChunkResults(path string, pass int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec("DELETE FROM chunk_cache WHERE file_path = ? AND pass = ?", path, pass)
	if err != nil {
		return fmt.Errorf("clearing chunk results: %w", err)
	}
	return nil
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}
