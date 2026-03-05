package pool

import (
	"context"
	"sync"

	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// ProcessFunc processes a single chunk and returns a Pass1Result.
type ProcessFunc func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error)

// FileResult holds the outcome of processing a single file.
type FileResult struct {
	FilePath string
	Result   *graph.Pass1Result
	Err      error
}

// RunPass1 processes all chunks through processFn with bounded concurrency.
// No fail-fast: all files are attempted regardless of individual errors.
func RunPass1(ctx context.Context, chunks []chunker.Chunk, processFn ProcessFunc, maxWorkers int, logger *zap.Logger) []FileResult {
	results := make([]FileResult, len(chunks))
	var mu sync.Mutex
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, c chunker.Chunk) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				mu.Lock()
				results[idx] = FileResult{FilePath: c.FileName, Err: ctx.Err()}
				mu.Unlock()
				return
			}

			res, err := processFn(ctx, c)

			mu.Lock()
			results[idx] = FileResult{
				FilePath: c.FileName,
				Result:   res,
				Err:      err,
			}
			mu.Unlock()

			if err != nil {
				logger.Error("failed to process file",
					zap.String("file", c.FileName),
					zap.Error(err),
				)
			} else {
				logger.Info("processed file",
					zap.String("file", c.FileName),
					zap.Int("programs", len(res.Programs)),
					zap.Int("relationships", len(res.Relationships)),
				)
			}
		}(i, chunk)
	}

	wg.Wait()
	return results
}
