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
