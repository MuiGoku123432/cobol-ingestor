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
// Results are streamed to the returned channel as each chunk completes.
// The channel is closed when all chunks have been processed.
func RunPass1(ctx context.Context, chunks []chunker.Chunk, processFn ProcessFunc, maxWorkers int, logger *zap.Logger) <-chan FileResult {
	results := make(chan FileResult, maxWorkers)
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c chunker.Chunk) {
			defer wg.Done()

			sem <- struct{}{}

			if ctx.Err() != nil {
				<-sem
				results <- FileResult{FilePath: c.FileName, Err: ctx.Err()}
				return
			}

			res, err := processFn(ctx, c)
			// Release the worker slot before sending to the channel so
			// a slow consumer (Neo4j writes) cannot stall LLM workers.
			<-sem

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

			results <- FileResult{
				FilePath: c.FileName,
				Result:   res,
				Err:      err,
			}
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// Pass2ProcessFunc processes a single chunk and returns a Pass2Result.
type Pass2ProcessFunc func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass2Result, error)

// Pass2FileResult holds the outcome of processing a single file through Pass 2.
type Pass2FileResult struct {
	FilePath string
	Result   *graph.Pass2Result
	Err      error
}

// Pass2ChunkResult holds the outcome of processing a single chunk (before merging).
type Pass2ChunkResult struct {
	FileName string
	Chunk    chunker.Chunk
	Result   *graph.Pass2Result
	Err      error
}

// RunPass2 processes all chunks through processFn with bounded concurrency.
// Results are streamed to the returned channel as each chunk completes.
// The channel is closed when all chunks have been processed.
// Callers are responsible for grouping and merging multi-chunk results by FileName.
func RunPass2(ctx context.Context, chunks []chunker.Chunk, processFn Pass2ProcessFunc, maxWorkers int, logger *zap.Logger) <-chan Pass2ChunkResult {
	results := make(chan Pass2ChunkResult, maxWorkers)
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for _, chunk := range chunks {
		wg.Add(1)
		go func(c chunker.Chunk) {
			defer wg.Done()

			sem <- struct{}{}

			if ctx.Err() != nil {
				<-sem
				results <- Pass2ChunkResult{FileName: c.FileName, Chunk: c, Err: ctx.Err()}
				return
			}

			res, err := processFn(ctx, c)
			// Release the worker slot before sending to the channel so
			// a slow consumer (Neo4j writes) cannot stall LLM workers.
			<-sem

			if err != nil {
				logger.Error("pass2: failed to process chunk",
					zap.String("file", c.FileName),
					zap.Int("chunk", c.Index),
					zap.Error(err),
				)
			} else {
				logger.Info("pass2: processed chunk",
					zap.String("file", c.FileName),
					zap.Int("chunk", c.Index+1),
					zap.Int("total", c.Total),
				)
			}

			results <- Pass2ChunkResult{
				FileName: c.FileName,
				Chunk:    c,
				Result:   res,
				Err:      err,
			}
		}(chunk)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}
