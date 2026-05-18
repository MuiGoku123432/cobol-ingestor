package cache

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChunkPaths(t *testing.T) {
	paths := make([]string, 7)
	for i := range paths {
		paths[i] = fmt.Sprintf("file%d", i)
	}

	chunks := chunkPaths(paths, 3)
	require.Len(t, chunks, 3)
	assert.Len(t, chunks[0], 3)
	assert.Len(t, chunks[1], 3)
	assert.Len(t, chunks[2], 1)

	// Empty input
	assert.Empty(t, chunkPaths(nil, 3))

	// Exact multiple
	chunks = chunkPaths(paths[:6], 3)
	require.Len(t, chunks, 2)
	assert.Len(t, chunks[0], 3)
	assert.Len(t, chunks[1], 3)
}

func TestBatchIsChanged_LargeSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_large.sqlite")
	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	const total = 1000
	const changedCount = 100

	// Insert all 1000 files with known hashes
	for i := 0; i < total; i++ {
		path := fmt.Sprintf("/src/file%04d.java", i)
		hash := fmt.Sprintf("hash%04d", i)
		require.NoError(t, c.MarkProcessed(path, hash))
	}

	// Build pathHashes: change the hash for the first 100 files
	pathHashes := make(map[string]string, total)
	for i := 0; i < total; i++ {
		path := fmt.Sprintf("/src/file%04d.java", i)
		if i < changedCount {
			pathHashes[path] = fmt.Sprintf("newhash%04d", i)
		} else {
			pathHashes[path] = fmt.Sprintf("hash%04d", i)
		}
	}

	changed, err := c.BatchIsChanged(pathHashes)
	require.NoError(t, err)
	assert.Len(t, changed, changedCount)

	// Verify each changed path is one of the first 100
	changedSet := make(map[string]bool, len(changed))
	for _, p := range changed {
		changedSet[p] = true
	}
	for i := 0; i < changedCount; i++ {
		path := fmt.Sprintf("/src/file%04d.java", i)
		assert.True(t, changedSet[path], "expected %s to be changed", path)
	}
}

func TestBatchIsChangedForPass_LargeSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_pass_large.sqlite")
	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	const total = 1000
	const changedCount = 100
	const pass = 2

	for i := 0; i < total; i++ {
		path := fmt.Sprintf("/src/file%04d.cbl", i)
		hash := fmt.Sprintf("hash%04d", i)
		require.NoError(t, c.MarkProcessedForPass(path, hash, pass))
	}

	pathHashes := make(map[string]string, total)
	for i := 0; i < total; i++ {
		path := fmt.Sprintf("/src/file%04d.cbl", i)
		if i < changedCount {
			pathHashes[path] = fmt.Sprintf("newhash%04d", i)
		} else {
			pathHashes[path] = fmt.Sprintf("hash%04d", i)
		}
	}

	changed, err := c.BatchIsChangedForPass(pathHashes, pass)
	require.NoError(t, err)
	assert.Len(t, changed, changedCount)

	changedSet := make(map[string]bool, len(changed))
	for _, p := range changed {
		changedSet[p] = true
	}
	for i := 0; i < changedCount; i++ {
		path := fmt.Sprintf("/src/file%04d.cbl", i)
		assert.True(t, changedSet[path], "expected %s to be changed", path)
	}
}

func TestBatchLookupClassification_LargeSet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_classify_large.sqlite")
	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	const total = 1000
	const staleCount = 100

	// Insert all 1000 classifications
	entries := make([]ClassifyEntry, total)
	for i := 0; i < total; i++ {
		entries[i] = ClassifyEntry{
			Path:       fmt.Sprintf("/src/file%04d.txt", i),
			Hash:       fmt.Sprintf("hash%04d", i),
			FileType:   "JAVA",
			Classifier: "LLM",
			Confidence: 0.95,
		}
	}
	require.NoError(t, c.BatchMarkClassified(entries))

	// Build lookup: make first 100 stale (different hash)
	pathHashes := make(map[string]string, total)
	for i := 0; i < total; i++ {
		path := fmt.Sprintf("/src/file%04d.txt", i)
		if i < staleCount {
			pathHashes[path] = fmt.Sprintf("newhash%04d", i)
		} else {
			pathHashes[path] = fmt.Sprintf("hash%04d", i)
		}
	}

	hits, misses, err := c.BatchLookupClassification(pathHashes)
	require.NoError(t, err)
	assert.Len(t, hits, total-staleCount)
	assert.Len(t, misses, staleCount)

	// Verify misses are the stale ones
	missSet := make(map[string]bool, len(misses))
	for _, p := range misses {
		missSet[p] = true
	}
	for i := 0; i < staleCount; i++ {
		path := fmt.Sprintf("/src/file%04d.txt", i)
		assert.True(t, missSet[path], "expected %s to be a miss", path)
	}
}
