package cache

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// New file should be "changed"
	changed, err := c.IsChanged("/path/to/file.cbl", "abc123")
	require.NoError(t, err)
	assert.True(t, changed)

	// Mark as processed
	require.NoError(t, c.MarkProcessed("/path/to/file.cbl", "abc123"))

	// Same hash → not changed
	changed, err = c.IsChanged("/path/to/file.cbl", "abc123")
	require.NoError(t, err)
	assert.False(t, changed)

	// Different hash → changed
	changed, err = c.IsChanged("/path/to/file.cbl", "def456")
	require.NoError(t, err)
	assert.True(t, changed)

	// Update with new hash
	require.NoError(t, c.MarkProcessed("/path/to/file.cbl", "def456"))

	changed, err = c.IsChanged("/path/to/file.cbl", "def456")
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestPassCacheLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test_pass.sqlite")

	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// New file should be changed for both passes
	changed, err := c.IsChangedForPass("/path/to/file.cbl", "abc123", 1)
	require.NoError(t, err)
	assert.True(t, changed)

	changed, err = c.IsChangedForPass("/path/to/file.cbl", "abc123", 2)
	require.NoError(t, err)
	assert.True(t, changed)

	// Mark processed for pass 1
	require.NoError(t, c.MarkProcessedForPass("/path/to/file.cbl", "abc123", 1))

	// Pass 1 → not changed, Pass 2 → still changed
	changed, err = c.IsChangedForPass("/path/to/file.cbl", "abc123", 1)
	require.NoError(t, err)
	assert.False(t, changed)

	changed, err = c.IsChangedForPass("/path/to/file.cbl", "abc123", 2)
	require.NoError(t, err)
	assert.True(t, changed)

	// Mark processed for pass 2
	require.NoError(t, c.MarkProcessedForPass("/path/to/file.cbl", "abc123", 2))

	changed, err = c.IsChangedForPass("/path/to/file.cbl", "abc123", 2)
	require.NoError(t, err)
	assert.False(t, changed)

	// Different hash → changed for both passes
	changed, err = c.IsChangedForPass("/path/to/file.cbl", "newHash", 1)
	require.NoError(t, err)
	assert.True(t, changed)

	changed, err = c.IsChangedForPass("/path/to/file.cbl", "newHash", 2)
	require.NoError(t, err)
	assert.True(t, changed)
}

func TestClassifyCacheLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "classify.sqlite")
	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// Miss: no entry
	hits, misses, err := c.BatchLookupClassification(map[string]string{
		"/tmp/FILE1.txt": "hash1",
	})
	require.NoError(t, err)
	assert.Empty(t, hits)
	assert.Equal(t, []string{"/tmp/FILE1.txt"}, misses)

	// Mark classified
	require.NoError(t, c.BatchMarkClassified([]ClassifyEntry{
		{Path: "/tmp/FILE1.txt", Hash: "hash1", FileType: "COBOL", Classifier: "LLM"},
	}))

	// Hit: same hash
	hits, misses, err = c.BatchLookupClassification(map[string]string{
		"/tmp/FILE1.txt": "hash1",
	})
	require.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Empty(t, misses)
	assert.Equal(t, "COBOL", hits["/tmp/FILE1.txt"].FileType)
	assert.Equal(t, "LLM", hits["/tmp/FILE1.txt"].Classifier)

	// Stale: different hash → miss
	hits, misses, err = c.BatchLookupClassification(map[string]string{
		"/tmp/FILE1.txt": "hash2",
	})
	require.NoError(t, err)
	assert.Empty(t, hits)
	assert.Equal(t, []string{"/tmp/FILE1.txt"}, misses)
}

func TestBatchClassifyCache(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_classify.sqlite")
	c, err := New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// Pre-populate 3 entries
	require.NoError(t, c.BatchMarkClassified([]ClassifyEntry{
		{Path: "/tmp/A.txt", Hash: "hA", FileType: "COBOL", Classifier: "LLM"},
		{Path: "/tmp/B.txt", Hash: "hB", FileType: "JCL", Classifier: "LLM"},
		{Path: "/tmp/C.txt", Hash: "hC", FileType: "COPYBOOK", Classifier: "HEURISTIC"},
	}))

	// Lookup: 3 cached + 2 new
	pathHashes := map[string]string{
		"/tmp/A.txt": "hA",
		"/tmp/B.txt": "hB",
		"/tmp/C.txt": "hC",
		"/tmp/D.txt": "hD",
		"/tmp/E.txt": "hE",
	}
	hits, misses, err := c.BatchLookupClassification(pathHashes)
	require.NoError(t, err)

	assert.Len(t, hits, 3)
	assert.Equal(t, "COBOL", hits["/tmp/A.txt"].FileType)
	assert.Equal(t, "JCL", hits["/tmp/B.txt"].FileType)
	assert.Equal(t, "COPYBOOK", hits["/tmp/C.txt"].FileType)
	assert.Equal(t, "HEURISTIC", hits["/tmp/C.txt"].Classifier)

	assert.Len(t, misses, 2)
	assert.Contains(t, misses, "/tmp/D.txt")
	assert.Contains(t, misses, "/tmp/E.txt")

	// Mark the new ones
	require.NoError(t, c.BatchMarkClassified([]ClassifyEntry{
		{Path: "/tmp/D.txt", Hash: "hD", FileType: "BMS", Classifier: "LLM"},
		{Path: "/tmp/E.txt", Hash: "hE", FileType: "ASM", Classifier: "LLM"},
	}))

	// Verify all 5 are now cached
	hits, misses, err = c.BatchLookupClassification(pathHashes)
	require.NoError(t, err)
	assert.Len(t, hits, 5)
	assert.Empty(t, misses)
	assert.Equal(t, "BMS", hits["/tmp/D.txt"].FileType)
	assert.Equal(t, "ASM", hits["/tmp/E.txt"].FileType)
}
