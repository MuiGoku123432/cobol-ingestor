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

// Pass2ProcessFunc processes a single chunk and returns a Pass2Result.
type Pass2ProcessFunc func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass2Result, error)

// Pass2FileResult holds the outcome of processing a single file through Pass 2.
type Pass2FileResult struct {
	FilePath string
	Result   *graph.Pass2Result
	Err      error
}

// RunPass2 processes all chunks through processFn with bounded concurrency.
// Groups chunk results by FileName and merges multi-chunk results per file.
func RunPass2(ctx context.Context, chunks []chunker.Chunk, processFn Pass2ProcessFunc, maxWorkers int, logger *zap.Logger) []Pass2FileResult {
	type chunkResult struct {
		fileName string
		result   *graph.Pass2Result
		err      error
	}

	chunkResults := make([]chunkResult, len(chunks))
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
				chunkResults[idx] = chunkResult{fileName: c.FileName, err: ctx.Err()}
				mu.Unlock()
				return
			}

			res, err := processFn(ctx, c)

			mu.Lock()
			chunkResults[idx] = chunkResult{
				fileName: c.FileName,
				result:   res,
				err:      err,
			}
			mu.Unlock()

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
		}(i, chunk)
	}

	wg.Wait()

	// Group by file name
	fileChunks := make(map[string][]*graph.Pass2Result)
	fileErrors := make(map[string]error)

	for _, cr := range chunkResults {
		if cr.err != nil {
			fileErrors[cr.fileName] = cr.err
			continue
		}
		if cr.result != nil {
			fileChunks[cr.fileName] = append(fileChunks[cr.fileName], cr.result)
		}
	}

	// Merge and build results
	var results []Pass2FileResult

	// Collect unique file names preserving order
	seen := make(map[string]bool)
	var fileOrder []string
	for _, c := range chunks {
		if !seen[c.FileName] {
			seen[c.FileName] = true
			fileOrder = append(fileOrder, c.FileName)
		}
	}

	for _, fileName := range fileOrder {
		if err, hasErr := fileErrors[fileName]; hasErr {
			results = append(results, Pass2FileResult{
				FilePath: fileName,
				Err:      err,
			})
			continue
		}
		parts := fileChunks[fileName]
		if len(parts) == 0 {
			continue
		}
		merged := graph.MergePass2Results(parts)
		results = append(results, Pass2FileResult{
			FilePath: fileName,
			Result:   merged,
		})
	}

	return results
}
