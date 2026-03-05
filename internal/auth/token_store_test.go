package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveLoadDeleteToken(t *testing.T) {
	// Override configDir to use a temp directory
	tmpDir := t.TempDir()
	origConfigDir := configDir
	configDirFn = func() string { return tmpDir }
	defer func() { configDirFn = origConfigDir }()

	// Initially no token
	st, err := LoadToken()
	require.NoError(t, err)
	assert.Nil(t, st)

	// Save a token
	err = SaveToken("gho_test_token_123")
	require.NoError(t, err)

	// Verify file permissions
	info, err := os.Stat(filepath.Join(tmpDir, "copilot-token.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())

	// Load it back
	st, err = LoadToken()
	require.NoError(t, err)
	require.NotNil(t, st)
	assert.Equal(t, "gho_test_token_123", st.GitHubToken)
	assert.False(t, st.ObtainedAt.IsZero())

	// Delete
	err = DeleteToken()
	require.NoError(t, err)

	// Confirm deleted
	st, err = LoadToken()
	require.NoError(t, err)
	assert.Nil(t, st)

	// Double delete is safe
	err = DeleteToken()
	require.NoError(t, err)
}
