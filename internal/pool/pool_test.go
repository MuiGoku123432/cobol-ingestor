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

func TestRunPass2_GroupsAndMerges(t *testing.T) {
	logger := zap.NewNop()

	// Two chunks from the same file
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

	results := RunPass2(context.Background(), chunks, processFn, 2, logger)

	// Should produce 2 results (one per file)
	require.Len(t, results, 2)

	// file1.cbl should have merged results (deduped A->B from 2 chunks)
	assert.Equal(t, "file1.cbl", results[0].FilePath)
	assert.NoError(t, results[0].Err)
	assert.Len(t, results[0].Result.Performs, 1) // deduped

	assert.Equal(t, "file2.cbl", results[1].FilePath)
	assert.NoError(t, results[1].Err)
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

	results := RunPass2(context.Background(), chunks, processFn, 2, logger)

	require.Len(t, results, 2)
	assert.NoError(t, results[0].Err)
	assert.Error(t, results[1].Err)
}
