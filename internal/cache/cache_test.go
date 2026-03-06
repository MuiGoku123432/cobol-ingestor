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
