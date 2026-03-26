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
		assert.Greater(t, f.LineCount, 0, "LineCount should be > 0 for %s", f.Path)
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

func TestClassifyByDoubleExt(t *testing.T) {
	tests := []struct {
		name     string
		expected graph.FileType
		ok       bool
	}{
		{"PROG.CBL.txt", graph.FileTypeCOBOL, true},
		{"PROG.cbl.TXT", graph.FileTypeCOBOL, true},
		{"COPY.CPY.txt", graph.FileTypeCopybook, true},
		{"JOB.JCL.txt", graph.FileTypeJCL, true},
		{"README.txt", "", false},
		{"data.csv.txt", "", false},
		{".txt", "", false},
		{"noext", "", false},
	}

	for _, tt := range tests {
		ft, ok := classifyByDoubleExt(tt.name)
		assert.Equal(t, tt.ok, ok, tt.name)
		if ok {
			assert.Equal(t, tt.expected, ft, tt.name)
		}
	}
}

func TestHashCountAndSnippet(t *testing.T) {
	dir := t.TempDir()

	// File with more lines than maxLines
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, "line content here")
	}
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	hash, lineCount, snippet, err := hashCountAndSnippet(path, 50)
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.Equal(t, 100, lineCount)
	// Smart snippet captures head + middle + tail regions + separator lines;
	// total will be approximately maxLines + 2 separator lines
	assert.Greater(t, len(snippet), 0)
	assert.LessOrEqual(t, len(snippet), 60, "snippet should be close to maxLines")

	// Short file: fewer lines than maxLines
	shortPath := filepath.Join(dir, "short.txt")
	require.NoError(t, os.WriteFile(shortPath, []byte("one\ntwo\nthree\n"), 0644))

	hash2, lineCount2, snippet2, err := hashCountAndSnippet(shortPath, 50)
	require.NoError(t, err)
	assert.NotEmpty(t, hash2)
	assert.Equal(t, 3, lineCount2)
	assert.Len(t, snippet2, 3)
	assert.Equal(t, []string{"one", "two", "three"}, snippet2)

	// Verify hash matches hashAndCountLines
	hashOnly, lcOnly, err := hashAndCountLines(path)
	require.NoError(t, err)
	assert.Equal(t, hashOnly, hash)
	assert.Equal(t, lcOnly, lineCount)
}

func TestScan_ContentDetect(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()

	// Known extensions (should work as before)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROG1.CBL"), []byte("IDENTIFICATION DIVISION."), 0644))

	// Double extension
	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROG2.CBL.txt"), []byte("IDENTIFICATION DIVISION."), 0644))

	// Plain .txt files with COBOL content (should be PENDING)
	cobolTxt := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. TEST1.\n       PROCEDURE DIVISION.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COBOL1.txt"), []byte(cobolTxt), 0644))

	// Plain .txt file with non-mainframe content
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.txt"), []byte("This is just a readme file."), 0644))

	// Non-txt unknown extension (should be skipped even with content detect)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b,c"), 0644))

	result, err := Scan(context.Background(), dir, logger, ScanOptions{ContentDetect: true})
	require.NoError(t, err)

	types := map[graph.FileType]int{}
	for _, f := range result.Files {
		types[f.Type]++
	}

	assert.Equal(t, 2, types[graph.FileTypeCOBOL], "PROG1.CBL + PROG2.CBL.txt (double-ext)")
	assert.Equal(t, 2, types[graph.FileTypePending], "COBOL1.txt and README.txt should be pending")

	// Verify snippets exist for pending files
	assert.Len(t, result.Snippets, 2)

	// Without content detect, .txt files should be skipped
	result2, err := Scan(context.Background(), dir, logger)
	require.NoError(t, err)
	assert.Len(t, result2.Files, 1, "only PROG1.CBL should be found without content detect")
}

func TestScan_ContentDetect_DoubleExt(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "PROG.CBL.txt"), []byte("IDENTIFICATION DIVISION."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COPY.CPY.txt"), []byte("01 FIELD PIC X."), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "JOB.JCL.txt"), []byte("//JOB1 JOB"), 0644))

	result, err := Scan(context.Background(), dir, logger, ScanOptions{ContentDetect: true})
	require.NoError(t, err)

	types := map[graph.FileType]int{}
	for _, f := range result.Files {
		types[f.Type]++
	}

	assert.Equal(t, 1, types[graph.FileTypeCOBOL])
	assert.Equal(t, 1, types[graph.FileTypeCopybook])
	assert.Equal(t, 1, types[graph.FileTypeJCL])
	assert.Equal(t, 0, types[graph.FileTypePending], "double-ext files should be fully classified")
	assert.Empty(t, result.Snippets)
}
