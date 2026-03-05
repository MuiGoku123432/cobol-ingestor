package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestScan(t *testing.T) {
	logger := zap.NewNop()

	dir := t.TempDir()

	// Create test files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROG1.CBL"), []byte("IDENTIFICATION DIVISION."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROG2.cob"), []byte("IDENTIFICATION DIVISION."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COPY1.CPY"), []byte("01 WS-FIELD PIC X."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COPY2.cpb"), []byte("01 WS-FIELD PIC X."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "JOB1.JCL"), []byte("//JOB1 JOB"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("ignore me"), 0644))

	// Hidden directory should be skipped
	hiddenDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(hiddenDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "config.cbl"), []byte("hidden"), 0644))

	result, err := Scan(context.Background(), dir, logger)
	require.NoError(t, err)

	assert.Len(t, result.Files, 5)
	assert.Empty(t, result.Errors)

	types := map[graph.FileType]int{}
	for _, f := range result.Files {
		types[f.Type]++
		assert.NotEmpty(t, f.Hash, "hash should be populated for %s", f.Path)
		assert.Greater(t, f.Size, int64(0))
	}

	assert.Equal(t, 2, types[graph.FileTypeCOBOL])
	assert.Equal(t, 2, types[graph.FileTypeCopybook])
	assert.Equal(t, 1, types[graph.FileTypeJCL])
}

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		name     string
		expected graph.FileType
		ok       bool
	}{
		{"PROG.CBL", graph.FileTypeCOBOL, true},
		{"PROG.cbl", graph.FileTypeCOBOL, true},
		{"PROG.cob", graph.FileTypeCOBOL, true},
		{"COPY.CPY", graph.FileTypeCopybook, true},
		{"COPY.cpb", graph.FileTypeCopybook, true},
		{"JOB.JCL", graph.FileTypeJCL, true},
		{"JOB.jcl", graph.FileTypeJCL, true},
		{"README.md", "", false},
		{"data.txt", "", false},
	}

	for _, tt := range tests {
		ft, ok := classifyFile(tt.name)
		assert.Equal(t, tt.ok, ok, tt.name)
		if ok {
			assert.Equal(t, tt.expected, ft, tt.name)
		}
	}
}
