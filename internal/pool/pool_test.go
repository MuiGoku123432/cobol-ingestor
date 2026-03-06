package pool

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/graph"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// collectPass1 drains a FileResult channel into a slice.
func collectPass1(ch <-chan FileResult) []FileResult {
	var results []FileResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestRunPass1_Success(t *testing.T) {
	logger := zap.NewNop()

	chunks := []chunker.Chunk{
		{FileName: "file1.cbl", Content: "content1"},
		{FileName: "file2.cbl", Content: "content2"},
		{FileName: "file3.cbl", Content: "content3"},
	}

	processFn := func(ctx context.Context, c chunker.Chunk) (*graph.Pass1Result, error) {
		return &graph.Pass1Result{
			SourceFile: c.FileName,
			Programs:   []graph.Program{{ProgramID: "TEST"}},
		}, nil
	}

	results := collectPass1(RunPass1(context.Background(), chunks, processFn, 2, logger))

	require.Len(t, results, 3)
	for _, r := range results {
		assert.NoError(t, r.Err)
		assert.NotNil(t, r.Result)
	}
}

func TestRunPass1_ErrorsCollected(t *testing.T) {
	logger := zap.NewNop()

	chunks := []chunker.Chunk{
		{FileName: "good.cbl"},
		{FileName: "bad.cbl"},
		{FileName: "good2.cbl"},
	}

	processFn := func(ctx context.Context, c chunker.Chunk) (*graph.Pass1Result, error) {
		if c.FileName == "bad.cbl" {
			return nil, fmt.Errorf("simulated failure")
		}
		return &graph.Pass1Result{SourceFile: c.FileName}, nil
	}

	results := collectPass1(RunPass1(context.Background(), chunks, processFn, 2, logger))

	require.Len(t, results, 3)

	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
		} else {
			successCount++
		}
	}
	assert.Equal(t, 2, successCount)
	assert.Equal(t, 1, errorCount)
}

func TestRunPass1_ConcurrencyBounded(t *testing.T) {
	logger := zap.NewNop()
	maxWorkers := 2

	chunks := make([]chunker.Chunk, 10)
	for i := range chunks {
		chunks[i] = chunker.Chunk{FileName: fmt.Sprintf("file%d.cbl", i)}
	}

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32

	processFn := func(ctx context.Context, c chunker.Chunk) (*graph.Pass1Result, error) {
		cur := concurrent.Add(1)
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}
		concurrent.Add(-1)
		return &graph.Pass1Result{SourceFile: c.FileName}, nil
	}

	results := collectPass1(RunPass1(context.Background(), chunks, processFn, maxWorkers, logger))
	require.Len(t, results, 10)

	// Max concurrent should not exceed maxWorkers
	assert.LessOrEqual(t, int(maxConcurrent.Load()), maxWorkers)
}

// collectPass2 drains a Pass2ChunkResult channel into a slice.
func collectPass2(ch <-chan Pass2ChunkResult) []Pass2ChunkResult {
	var results []Pass2ChunkResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

func TestRunPass2_StreamsResults(t *testing.T) {
	logger := zap.NewNop()

	// Two chunks from the same file + one single-chunk file
	chunks := []chunker.Chunk{
		{FileName: "file1.cbl", Content: "chunk1", Index: 0, Total: 2, Pass: 2},
		{FileName: "file1.cbl", Content: "chunk2", Index: 1, Total: 2, Pass: 2},
		{FileName: "file2.cbl", Content: "all", Index: 0, Total: 1, Pass: 2},
	}

	processFn := func(ctx context.Context, c chunker.Chunk) (*graph.Pass2Result, error) {
		return &graph.Pass2Result{
			SourceFile: c.FileName,
			ProgramID:  "TEST",
			Performs: []graph.PerformRelation{
				{FromParagraph: "A", ToParagraph: "B"},
			},
		}, nil
	}

	results := collectPass2(RunPass2(context.Background(), chunks, processFn, 2, logger))

	// Should produce 3 chunk results (one per chunk)
	require.Len(t, results, 3)

	// All should succeed
	for _, r := range results {
		assert.NoError(t, r.Err)
		assert.NotNil(t, r.Result)
	}

	// Verify file1 has 2 results and file2 has 1
	file1Count := 0
	file2Count := 0
	for _, r := range results {
		switch r.FileName {
		case "file1.cbl":
			file1Count++
		case "file2.cbl":
			file2Count++
		}
	}
	assert.Equal(t, 2, file1Count)
	assert.Equal(t, 1, file2Count)
}

func TestRunPass2_ErrorsCollected(t *testing.T) {
	logger := zap.NewNop()

	chunks := []chunker.Chunk{
		{FileName: "good.cbl", Content: "ok", Index: 0, Total: 1, Pass: 2},
		{FileName: "bad.cbl", Content: "fail", Index: 0, Total: 1, Pass: 2},
	}

	processFn := func(ctx context.Context, c chunker.Chunk) (*graph.Pass2Result, error) {
		if c.FileName == "bad.cbl" {
			return nil, fmt.Errorf("simulated failure")
		}
		return &graph.Pass2Result{SourceFile: c.FileName, ProgramID: "TEST"}, nil
	}

	results := collectPass2(RunPass2(context.Background(), chunks, processFn, 2, logger))

	require.Len(t, results, 2)

	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
		} else {
			successCount++
		}
	}
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 1, errorCount)
}
