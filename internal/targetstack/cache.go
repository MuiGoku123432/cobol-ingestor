package targetstack

import "cobol-ingestor/internal/cache"

// Cache pass numbers for target stack pipeline.
const (
	PassExtract   = 100 // per-file LLM extraction
	PassSynthesis = 101 // cross-file synthesis
)

// NewCache opens (or creates) the SQLite cache at the given path.
// Uses the shared cache.Cache implementation with pass-aware storage.
func NewCache(dbPath string) (*cache.Cache, error) {
	return cache.New(dbPath)
}
