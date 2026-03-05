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

	results := RunPass1(context.Background(), chunks, processFn, 2, logger)

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

	results := RunPass1(context.Background(), chunks, processFn, 2, logger)

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

	results := RunPass1(context.Background(), chunks, processFn, maxWorkers, logger)
	require.Len(t, results, 10)

	// Max concurrent should not exceed maxWorkers
	assert.LessOrEqual(t, int(maxConcurrent.Load()), maxWorkers)
}
