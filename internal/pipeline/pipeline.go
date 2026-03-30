package pipeline

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
	"cobol-ingestor/internal/pool"
	"cobol-ingestor/internal/scanner"
	"cobol-ingestor/internal/static"
	"cobol-ingestor/internal/validator"

	"go.uber.org/zap"
)

// PipelineStats aggregates metrics across all pipeline passes.
type PipelineStats struct {
	Pass1Processed   atomic.Int64
	Pass1Skipped     atomic.Int64
	Pass1Failed      atomic.Int64
	Pass2Processed   atomic.Int64
	Pass2Skipped     atomic.Int64
	Pass2Failed      atomic.Int64
	Pass3Batches     atomic.Int64
	Pass3Failed      atomic.Int64
	ChunksTotal      atomic.Int64
	ChunksCached     atomic.Int64
	ChunksFailed     atomic.Int64
	TruncationRetries atomic.Int64
	PartialRecoveries atomic.Int64
	LLMCalls         atomic.Int64
}

// multiRepairItem holds a program ID and the set of repair types it requires.
// Used by repairCombined to batch multiple gap types into a single LLM call.
type multiRepairItem struct {
	pid   string
	types []string
}

// Pipeline orchestrates the multi-pass COBOL analysis workflow.
type Pipeline struct {
	Config      *config.Config
	Claude      *claude.Client
	Neo4jClient *n4j.Client
	Writer      *n4j.BatchWriter
	Cache       *cache.Cache
	Logger      *zap.Logger
	Stats       PipelineStats
	Calibrator  *chunker.TokenCalibrator
	Validator   *validator.Validator
}

// Run executes the pipeline for the specified pass (0=all, 1/2/3/4=individual).
func (p *Pipeline) Run(ctx context.Context, scanResult *scanner.ScanResult, passFlag int) error {
	// Apply config to package-level vars
	if p.Config.Ingest.TokenEstimationRatio > 0 {
		chunker.TokenEstimationRatio = p.Config.Ingest.TokenEstimationRatio
	}
	chunker.StripSequenceColumns = p.Config.Ingest.StripSequenceColumns
	parser.SetLogger(p.Logger)

	// Initialize token calibrator
	if p.Calibrator == nil {
		p.Calibrator = chunker.NewTokenCalibrator(200, p.Logger)
	}
	p.Claude.SetCalibrator(p.Calibrator)

	// Initialize validator
	if p.Validator == nil {
		p.Validator = validator.New(p.Logger)
	}

	if passFlag == 0 || passFlag == 1 {
		if err := p.RunPass1(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 1: %w", err)
		}
		// JCL analysis runs as part of Pass 1
		if err := p.RunPass1JCL(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 1 JCL: %w", err)
		}
	}
	// Static CHILD_OF extraction: deterministic data hierarchy from level numbers
	if passFlag == 0 || passFlag == 1 {
		p.runStaticDataHierarchy(ctx, scanResult)
	}

	// Mark external programs early so Pass 3 can exclude them
	if passFlag == 0 || passFlag == 1 {
		if fixed, err := p.Writer.FixDanglingCalls(ctx); err != nil {
			p.Logger.Warn("early external marking failed", zap.Error(err))
		} else if fixed > 0 {
			p.Logger.Info("marked external programs (pre-Pass 3)", zap.Int("count", fixed))
		}
	}

	if passFlag == 0 || passFlag == 2 {
		if err := p.RunPass2(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 2: %w", err)
		}
		// Dead paragraph detection runs after Pass 2 (uses PERFORMS graph)
		if err := p.Writer.DetectDeadParagraphs(ctx); err != nil {
			p.Logger.Warn("dead paragraph detection failed", zap.Error(err))
		}
	}
	if passFlag == 0 || passFlag == 3 {
		if err := p.RunPass3(ctx); err != nil {
			return fmt.Errorf("pass 3: %w", err)
		}
	}
	if passFlag == 0 || passFlag == 4 {
		if err := p.RunPass4(ctx); err != nil {
			return fmt.Errorf("pass 4: %w", err)
		}
	}

	// Post-processing: link DD cards to File nodes (after both JCL and COBOL analysis)
	if passFlag == 0 {
		if err := p.Writer.LinkDDCardsToFiles(ctx); err != nil {
			p.Logger.Warn("DD card to file linking failed", zap.Error(err))
		}
	}

	if passFlag == 0 || passFlag == 5 {
		if err := p.RunPass5(ctx, scanResult); err != nil {
			p.Logger.Warn("pass 5 validation/repair failed", zap.Error(err))
		}
	}

	// Phase 7: Log pipeline stats summary
	p.Logger.Info("pipeline stats summary",
		zap.Int64("pass1_processed", p.Stats.Pass1Processed.Load()),
		zap.Int64("pass1_skipped", p.Stats.Pass1Skipped.Load()),
		zap.Int64("pass1_failed", p.Stats.Pass1Failed.Load()),
		zap.Int64("pass2_processed", p.Stats.Pass2Processed.Load()),
		zap.Int64("pass2_skipped", p.Stats.Pass2Skipped.Load()),
		zap.Int64("pass2_failed", p.Stats.Pass2Failed.Load()),
		zap.Int64("pass3_batches", p.Stats.Pass3Batches.Load()),
		zap.Int64("pass3_failed", p.Stats.Pass3Failed.Load()),
		zap.Int64("chunks_total", p.Stats.ChunksTotal.Load()),
		zap.Int64("chunks_cached", p.Stats.ChunksCached.Load()),
		zap.Int64("chunks_failed", p.Stats.ChunksFailed.Load()),
		zap.Int64("truncation_retries", p.Stats.TruncationRetries.Load()),
		zap.Int64("partial_recoveries", p.Stats.PartialRecoveries.Load()),
		zap.Int64("llm_calls", p.Stats.LLMCalls.Load()),
	)

	return nil
}

// RunPass1 executes Pass 1 structural analysis.
// Results are streamed to Neo4j as each file completes (or as all chunks for a
// multi-chunk file complete). This ensures partial progress is persisted even
// if the pipeline is interrupted.
func (p *Pipeline) RunPass1(ctx context.Context, scanResult *scanner.ScanResult) error {
	// Collect all COBOL files and batch-check cache
	cobolFiles := make(map[string]graph.FileInfo)
	pathHashes := make(map[string]string)
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		cobolFiles[f.Path] = f
		pathHashes[f.Path] = f.Hash
	}

	changedPaths, err := p.Cache.BatchIsChanged(pathHashes)
	if err != nil {
		return fmt.Errorf("batch checking cache: %w", err)
	}

	changed := make([]graph.FileInfo, 0, len(changedPaths))
	for _, path := range changedPaths {
		changed = append(changed, cobolFiles[path])
	}

	if len(changed) == 0 {
		p.Logger.Info("pass 1: no changed files to process")
		return nil
	}
	p.Logger.Info("pass 1: files to process", zap.Int("changed", len(changed)))

	// Build a hash lookup for marking cache after write
	hashByPath := make(map[string]string, len(changed))
	for _, f := range changed {
		hashByPath[f.Path] = f.Hash
	}

	// Pre-compute how many chunks each file produces
	chunksPerFile := make(map[string]int)
	var chunks []chunker.Chunk
	for _, f := range changed {
		fileChunks, err := chunker.ChunkFile(f, p.Config.Ingest.TokenLimit, p.Logger)
		if err != nil {
			p.Logger.Error("chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		chunksPerFile[f.Path] = len(fileChunks)
		chunks = append(chunks, fileChunks...)
	}

	// Phase 4: Build content hashes for per-chunk caching
	chunkHashes := make(map[string][]string) // filePath → []hash
	for _, chunk := range chunks {
		h := sha256.Sum256([]byte(chunk.Content))
		hash := hex.EncodeToString(h[:])
		chunkHashes[chunk.FileName] = append(chunkHashes[chunk.FileName], hash)
	}

	// Phase 4: Check for cached chunk results and filter out already-cached chunks
	var uncachedChunks []chunker.Chunk
	cachedResults := make(map[string]map[int]string) // filePath → chunkIndex → resultJSON

	for _, chunk := range chunks {
		p.Stats.ChunksTotal.Add(1)
		if chunk.Total > 1 {
			// Only use per-chunk cache for multi-chunk files
			cached, err := p.Cache.LoadChunkResults(chunk.FileName, 1)
			if err == nil && len(cached) > 0 {
				hashes := chunkHashes[chunk.FileName]
				// Check if this specific chunk is cached with matching hash
				for _, cr := range cached {
					if cr.ChunkIndex == chunk.Index && chunk.Index < len(hashes) && cr.ContentHash == hashes[chunk.Index] {
						if cachedResults[chunk.FileName] == nil {
							cachedResults[chunk.FileName] = make(map[int]string)
						}
						cachedResults[chunk.FileName][chunk.Index] = cr.ResultJSON
						p.Stats.ChunksCached.Add(1)
					}
				}
			}
		}
		if _, ok := cachedResults[chunk.FileName][chunk.Index]; !ok {
			uncachedChunks = append(uncachedChunks, chunk)
		}
	}

	if len(cachedResults) > 0 {
		p.Logger.Info("pass 1: using cached chunk results",
			zap.Int("cached_chunks", int(p.Stats.ChunksCached.Load())),
			zap.Int("uncached_chunks", len(uncachedChunks)),
		)
	}

	processFn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error) {
		p.Stats.LLMCalls.Add(1)
		jsonResp, err := p.Claude.AnalyzeStructural(ctx, chunk.FileName, chunk.Content)
		if err != nil {
			if errors.Is(err, claude.ErrResponseTruncated) {
				p.Stats.TruncationRetries.Add(1)
			}
			// Even on truncation error, jsonResp may contain partial content
			if jsonResp != "" {
				result, parseErr := parser.ParsePass1Response(jsonResp, chunk.FileName)
				if parseErr == nil && result.Partial {
					p.Stats.PartialRecoveries.Add(1)
					// Cache the partial result for multi-chunk files
					if chunk.Total > 1 {
						hashes := chunkHashes[chunk.FileName]
						if chunk.Index < len(hashes) {
							_ = p.Cache.SaveChunkResult(chunk.FileName, 1, chunk.Index, chunk.Total, hashes[chunk.Index], jsonResp)
						}
					}
					return result, nil
				}
			}
			return nil, fmt.Errorf("claude analysis: %w", err)
		}
		// Cache successful chunk result for multi-chunk files
		if chunk.Total > 1 {
			hashes := chunkHashes[chunk.FileName]
			if chunk.Index < len(hashes) {
				_ = p.Cache.SaveChunkResult(chunk.FileName, 1, chunk.Index, chunk.Total, hashes[chunk.Index], jsonResp)
			}
		}
		return parser.ParsePass1Response(jsonResp, chunk.FileName)
	}

	resultsCh := pool.RunPass1(ctx, uncachedChunks, processFn, p.Config.Ingest.WorkersForPass(1), p.Logger)

	// Accumulate multi-chunk results per file, write as soon as all chunks arrive.
	// Single-chunk files (the common case) are written immediately.
	pendingChunks := make(map[string][]*graph.Pass1Result)
	fileHasError := make(map[string]bool)
	successCount := 0
	errorCount := 0

	// Phase 4: Pre-populate pendingChunks with cached results
	for filePath, cachedChunks := range cachedResults {
		for chunkIdx, resultJSON := range cachedChunks {
			result, err := parser.ParsePass1Response(resultJSON, filePath)
			if err != nil {
				p.Logger.Warn("pass 1: failed to parse cached chunk", zap.String("file", filePath), zap.Int("chunk", chunkIdx), zap.Error(err))
				fileHasError[filePath] = true
				continue
			}
			pendingChunks[filePath] = append(pendingChunks[filePath], result)
		}
	}

	for r := range resultsCh {
		if r.Err != nil {
			errorCount++
			p.Stats.ChunksFailed.Add(1)
			fileHasError[r.FilePath] = true
			continue
		}

		expected := chunksPerFile[r.FilePath]
		if expected <= 1 {
			// Single-chunk file — write immediately
			if err := p.Writer.WritePass1Result(ctx, r.Result); err != nil {
				p.Logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
				errorCount++
				p.Stats.Pass1Failed.Add(1)
				continue
			}
			if hash, ok := hashByPath[r.FilePath]; ok {
				if err := p.Cache.MarkProcessed(r.FilePath, hash); err != nil {
					p.Logger.Error("failed to update cache", zap.String("file", r.FilePath), zap.Error(err))
				}
			}
			successCount++
			p.Stats.Pass1Processed.Add(1)
		} else {
			// Multi-chunk file — accumulate and write when all chunks arrive
			pendingChunks[r.FilePath] = append(pendingChunks[r.FilePath], r.Result)

			if len(pendingChunks[r.FilePath]) == expected && !fileHasError[r.FilePath] {
				merged := graph.MergePass1Results(pendingChunks[r.FilePath])
				if err := p.Writer.WritePass1Result(ctx, merged); err != nil {
					p.Logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
					errorCount++
					p.Stats.Pass1Failed.Add(1)
				} else {
					if hash, ok := hashByPath[r.FilePath]; ok {
						if err := p.Cache.MarkProcessed(r.FilePath, hash); err != nil {
							p.Logger.Error("failed to update cache", zap.String("file", r.FilePath), zap.Error(err))
						}
					}
					successCount++
					p.Stats.Pass1Processed.Add(1)
				}
				delete(pendingChunks, r.FilePath)
			}
		}
	}

	// Check for multi-chunk files where all cached chunks were ready (no LLM calls needed)
	for filePath, results := range pendingChunks {
		if fileHasError[filePath] {
			continue
		}
		expected := chunksPerFile[filePath]
		if len(results) == expected {
			merged := graph.MergePass1Results(results)
			if err := p.Writer.WritePass1Result(ctx, merged); err != nil {
				p.Logger.Error("failed to write cached results to neo4j", zap.String("file", filePath), zap.Error(err))
				errorCount++
				p.Stats.Pass1Failed.Add(1)
			} else {
				if hash, ok := hashByPath[filePath]; ok {
					if err := p.Cache.MarkProcessed(filePath, hash); err != nil {
						p.Logger.Error("failed to update cache", zap.String("file", filePath), zap.Error(err))
					}
				}
				successCount++
				p.Stats.Pass1Processed.Add(1)
			}
		}
	}

	// Write partial results for multi-chunk files where some chunks failed
	for filePath, results := range pendingChunks {
		if !fileHasError[filePath] {
			continue // already handled above
		}
		expected := chunksPerFile[filePath]
		if len(results) == 0 {
			continue // no successful chunks at all
		}
		p.Logger.Warn("pass 1: writing partial results for multi-chunk file",
			zap.String("file", filePath),
			zap.Int("successful_chunks", len(results)),
			zap.Int("expected_chunks", expected),
		)
		merged := graph.MergePass1Results(results)
		merged.Partial = true
		if err := p.Writer.WritePass1Result(ctx, merged); err != nil {
			p.Logger.Error("pass 1: failed to write partial results", zap.String("file", filePath), zap.Error(err))
			p.Stats.Pass1Failed.Add(1)
		} else {
			if hash, ok := hashByPath[filePath]; ok {
				if err := p.Cache.MarkPartiallyProcessed(filePath, hash); err != nil {
					p.Logger.Error("pass 1: failed to mark partial cache", zap.String("file", filePath), zap.Error(err))
				}
			}
			p.Stats.PartialRecoveries.Add(1)
		}
	}

	p.Logger.Info("pass 1 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
	)

	return nil
}

// RunPass2 executes Pass 2 deep semantic analysis.
// Results are streamed to Neo4j as each file's chunks complete.
func (p *Pipeline) RunPass2(ctx context.Context, scanResult *scanner.ScanResult) error {
	copybookIndex := chunker.BuildCopybookIndex(scanResult.Files)
	p.Logger.Info("pass 2: built copybook index", zap.Int("copybooks", len(copybookIndex)))

	// Collect all COBOL files and batch-check pass 2 cache
	cobolFiles := make(map[string]graph.FileInfo)
	pathHashes := make(map[string]string)
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		cobolFiles[f.Path] = f
		pathHashes[f.Path] = f.Hash
	}

	changedPaths, err := p.Cache.BatchIsChangedForPass(pathHashes, 2)
	if err != nil {
		return fmt.Errorf("batch checking pass 2 cache: %w", err)
	}

	changed := make([]graph.FileInfo, 0, len(changedPaths))
	for _, path := range changedPaths {
		changed = append(changed, cobolFiles[path])
	}

	if len(changed) == 0 {
		p.Logger.Info("pass 2: no changed files to process")
		return nil
	}
	p.Logger.Info("pass 2: files to process", zap.Int("changed", len(changed)))

	hashByPath := make(map[string]string, len(changed))
	for _, f := range changed {
		hashByPath[f.Path] = f.Hash
	}

	chunkOpts := chunker.Pass2ChunkOptions{
		TokenLimit:    p.Config.Ingest.Pass2TokenLimit,
		OverlapLines:  p.Config.Ingest.OverlapLines,
		CopybookIndex: copybookIndex,
		CopybookCache: chunker.NewCopybookCache(),
	}

	chunksPerFile := make(map[string]int)
	var allChunks []chunker.Chunk
	fileProgramIDs := make(map[string]string)

	for _, f := range changed {
		fileChunks, err := chunker.ChunkFilePass2(f, chunkOpts, p.Logger)
		if err != nil {
			p.Logger.Error("pass 2: chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		chunksPerFile[f.Path] = len(fileChunks)
		allChunks = append(allChunks, fileChunks...)
	}

	// Batch lookup: resolve file paths to program IDs in a single query
	{
		paths := make([]any, len(changed))
		for i, f := range changed {
			paths[i] = f.Path
		}
		session := p.Neo4jClient.NewSession(ctx)
		result, err := session.Run(ctx,
			"UNWIND $paths AS path MATCH (prog:Program {filePath: path}) RETURN path, prog.programId AS pid",
			map[string]any{"paths": paths},
		)
		if err == nil {
			for result.Next(ctx) {
				record := result.Record()
				pathVal, _ := record.Get("path")
				pidVal, _ := record.Get("pid")
				if path, ok := pathVal.(string); ok {
					if pid, ok := pidVal.(string); ok {
						fileProgramIDs[path] = pid
					}
				}
			}
		}
		session.Close(ctx)
	}

	contextPreambles := make(map[string]string)
	for filePath, programID := range fileProgramIDs {
		pc, err := p.Neo4jClient.QueryProgramContext(ctx, programID)
		if err != nil {
			p.Logger.Warn("pass 2: failed to query context",
				zap.String("program", programID), zap.Error(err))
			continue
		}
		contextPreambles[filePath] = n4j.FormatContextPreamble(pc)
	}

	// Phase 4: Build content hashes for per-chunk caching
	pass2ChunkHashes := make(map[string][]string)
	for _, chunk := range allChunks {
		h := sha256.Sum256([]byte(chunk.Content))
		hash := hex.EncodeToString(h[:])
		pass2ChunkHashes[chunk.FileName] = append(pass2ChunkHashes[chunk.FileName], hash)
	}

	// Phase 4: Check for cached chunk results
	var uncachedPass2Chunks []chunker.Chunk
	cachedPass2Results := make(map[string]map[int]string)

	for _, chunk := range allChunks {
		p.Stats.ChunksTotal.Add(1)
		if chunk.Total > 1 {
			cached, err := p.Cache.LoadChunkResults(chunk.FileName, 2)
			if err == nil && len(cached) > 0 {
				hashes := pass2ChunkHashes[chunk.FileName]
				for _, cr := range cached {
					if cr.ChunkIndex == chunk.Index && chunk.Index < len(hashes) && cr.ContentHash == hashes[chunk.Index] {
						if cachedPass2Results[chunk.FileName] == nil {
							cachedPass2Results[chunk.FileName] = make(map[int]string)
						}
						cachedPass2Results[chunk.FileName][chunk.Index] = cr.ResultJSON
						p.Stats.ChunksCached.Add(1)
					}
				}
			}
		}
		if _, ok := cachedPass2Results[chunk.FileName][chunk.Index]; !ok {
			uncachedPass2Chunks = append(uncachedPass2Chunks, chunk)
		}
	}

	if len(cachedPass2Results) > 0 {
		p.Logger.Info("pass 2: using cached chunk results",
			zap.Int("cached_chunks", len(cachedPass2Results)),
			zap.Int("uncached_chunks", len(uncachedPass2Chunks)),
		)
	}

	pass2Fn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass2Result, error) {
		p.Stats.LLMCalls.Add(1)
		preamble := contextPreambles[chunk.FileName]
		jsonResp, err := p.Claude.AnalyzeDeep(ctx, chunk, preamble)
		if err != nil {
			if errors.Is(err, claude.ErrResponseTruncated) {
				p.Stats.TruncationRetries.Add(1)
			}
			programID := fileProgramIDs[chunk.FileName]
			if programID == "" {
				programID = "UNKNOWN"
			}
			if jsonResp != "" {
				result, parseErr := parser.ParsePass2Response(jsonResp, chunk.FileName, programID)
				if parseErr == nil && result.Partial {
					p.Stats.PartialRecoveries.Add(1)
					if chunk.Total > 1 {
						hashes := pass2ChunkHashes[chunk.FileName]
						if chunk.Index < len(hashes) {
							_ = p.Cache.SaveChunkResult(chunk.FileName, 2, chunk.Index, chunk.Total, hashes[chunk.Index], jsonResp)
						}
					}
					return result, nil
				}
			}
			return nil, fmt.Errorf("claude deep analysis: %w", err)
		}
		programID := fileProgramIDs[chunk.FileName]
		if programID == "" {
			programID = "UNKNOWN"
		}
		// Cache successful chunk result
		if chunk.Total > 1 {
			hashes := pass2ChunkHashes[chunk.FileName]
			if chunk.Index < len(hashes) {
				_ = p.Cache.SaveChunkResult(chunk.FileName, 2, chunk.Index, chunk.Total, hashes[chunk.Index], jsonResp)
			}
		}
		return parser.ParsePass2Response(jsonResp, chunk.FileName, programID)
	}

	resultsCh := pool.RunPass2(ctx, uncachedPass2Chunks, pass2Fn, p.Config.Ingest.WorkersForPass(2), p.Logger)

	// Stream results to Neo4j, merging multi-chunk files as they complete
	pendingPass2Chunks := make(map[string][]*graph.Pass2Result)
	fileHasError := make(map[string]bool)
	successCount := 0
	errorCount := 0

	// Phase 4: Pre-populate with cached results
	for filePath, cachedChunks := range cachedPass2Results {
		programID := fileProgramIDs[filePath]
		if programID == "" {
			programID = "UNKNOWN"
		}
		for chunkIdx, resultJSON := range cachedChunks {
			result, err := parser.ParsePass2Response(resultJSON, filePath, programID)
			if err != nil {
				p.Logger.Warn("pass 2: failed to parse cached chunk", zap.String("file", filePath), zap.Int("chunk", chunkIdx), zap.Error(err))
				fileHasError[filePath] = true
				continue
			}
			pendingPass2Chunks[filePath] = append(pendingPass2Chunks[filePath], result)
		}
	}

	for cr := range resultsCh {
		if cr.Err != nil {
			errorCount++
			p.Stats.ChunksFailed.Add(1)
			fileHasError[cr.FileName] = true
			continue
		}

		expected := chunksPerFile[cr.FileName]
		if expected <= 1 {
			// Single-chunk file — write immediately
			if err := p.Writer.WritePass2Result(ctx, cr.Result); err != nil {
				p.Logger.Error("pass 2: failed to write to neo4j", zap.String("file", cr.FileName), zap.Error(err))
				errorCount++
				p.Stats.Pass2Failed.Add(1)
				continue
			}
			if hash, ok := hashByPath[cr.FileName]; ok {
				if err := p.Cache.MarkProcessedForPass(cr.FileName, hash, 2); err != nil {
					p.Logger.Error("pass 2: failed to update cache", zap.String("file", cr.FileName), zap.Error(err))
				}
			}
			successCount++
			p.Stats.Pass2Processed.Add(1)
		} else {
			// Multi-chunk file — accumulate and write when all chunks arrive
			pendingPass2Chunks[cr.FileName] = append(pendingPass2Chunks[cr.FileName], cr.Result)

			if len(pendingPass2Chunks[cr.FileName]) == expected && !fileHasError[cr.FileName] {
				merged := graph.MergePass2Results(pendingPass2Chunks[cr.FileName])
				if err := p.Writer.WritePass2Result(ctx, merged); err != nil {
					p.Logger.Error("pass 2: failed to write to neo4j", zap.String("file", cr.FileName), zap.Error(err))
					errorCount++
					p.Stats.Pass2Failed.Add(1)
				} else {
					if hash, ok := hashByPath[cr.FileName]; ok {
						if err := p.Cache.MarkProcessedForPass(cr.FileName, hash, 2); err != nil {
							p.Logger.Error("pass 2: failed to update cache", zap.String("file", cr.FileName), zap.Error(err))
						}
					}
					successCount++
					p.Stats.Pass2Processed.Add(1)
				}
				delete(pendingPass2Chunks, cr.FileName)
			}
		}
	}

	// Check for multi-chunk files where all cached chunks were ready
	for filePath, results := range pendingPass2Chunks {
		if fileHasError[filePath] {
			continue
		}
		expected := chunksPerFile[filePath]
		if len(results) == expected {
			merged := graph.MergePass2Results(results)
			if err := p.Writer.WritePass2Result(ctx, merged); err != nil {
				p.Logger.Error("pass 2: failed to write cached results to neo4j", zap.String("file", filePath), zap.Error(err))
				errorCount++
				p.Stats.Pass2Failed.Add(1)
			} else {
				if hash, ok := hashByPath[filePath]; ok {
					if err := p.Cache.MarkProcessedForPass(filePath, hash, 2); err != nil {
						p.Logger.Error("pass 2: failed to update cache", zap.String("file", filePath), zap.Error(err))
					}
				}
				successCount++
				p.Stats.Pass2Processed.Add(1)
			}
		}
	}

	// Write partial results for multi-chunk files where some chunks failed
	for filePath, results := range pendingPass2Chunks {
		if !fileHasError[filePath] {
			continue // already handled above
		}
		expected := chunksPerFile[filePath]
		if len(results) == 0 {
			continue // no successful chunks at all
		}
		p.Logger.Warn("pass 2: writing partial results for multi-chunk file",
			zap.String("file", filePath),
			zap.Int("successful_chunks", len(results)),
			zap.Int("expected_chunks", expected),
		)
		merged := graph.MergePass2Results(results)
		merged.Partial = true
		if err := p.Writer.WritePass2Result(ctx, merged); err != nil {
			p.Logger.Error("pass 2: failed to write partial results", zap.String("file", filePath), zap.Error(err))
			p.Stats.Pass2Failed.Add(1)
		} else {
			if hash, ok := hashByPath[filePath]; ok {
				if err := p.Cache.MarkPartiallyProcessed(filePath, hash); err != nil {
					p.Logger.Error("pass 2: failed to mark partial cache", zap.String("file", filePath), zap.Error(err))
				}
			}
			p.Stats.PartialRecoveries.Add(1)
		}
	}

	p.Logger.Info("pass 2 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
	)

	return nil
}

// RunPass3 executes Pass 3 cross-cutting analysis.
func (p *Pipeline) RunPass3(ctx context.Context) error {
	p.Logger.Info("pass 3: starting cross-cutting analysis")

	// Query orphan and hub programs
	orphans, err := p.Neo4jClient.QueryOrphanPrograms(ctx)
	if err != nil {
		p.Logger.Warn("pass 3: failed to query orphans", zap.Error(err))
	}

	hubs, err := p.Neo4jClient.QueryHubPrograms(ctx, 5)
	if err != nil {
		p.Logger.Warn("pass 3: failed to query hubs", zap.Error(err))
	}

	// Query existing domain names so batches reuse them instead of inventing synonyms
	existingDomains, err := p.Neo4jClient.QueryExistingDomainNames(ctx)
	if err != nil {
		p.Logger.Warn("pass 3: failed to query existing domains", zap.Error(err))
	}

	batchSize := p.Config.Ingest.Pass3BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	offset := 0
	totalBatches := 0
	var failedPrograms []string
	for {
		slices, err := p.Neo4jClient.QueryCallGraphSlice(ctx, batchSize, offset)
		if err != nil {
			return fmt.Errorf("querying call graph slice: %w", err)
		}
		if len(slices) == 0 {
			break
		}

		existingDomainsStr := formatDomainList(existingDomains)

		if err := p.processPass3Batch(ctx, slices, orphans, hubs, existingDomainsStr); err != nil {
			p.Logger.Error("pass 3: batch failed, retrying halves",
				zap.Int("offset", offset), zap.Int("size", len(slices)), zap.Error(err))

			half := len(slices) / 2
			if half > 0 {
				if err := p.processPass3Batch(ctx, slices[:half], orphans, hubs, existingDomainsStr); err != nil {
					p.Logger.Error("pass 3: first half retry failed", zap.Error(err))
					for _, s := range slices[:half] {
						failedPrograms = append(failedPrograms, s.ProgramID)
					}
				} else {
					totalBatches++
					existingDomains = p.refreshDomainNames(ctx, existingDomains)
				}
				existingDomainsStr = formatDomainList(existingDomains)
				if err := p.processPass3Batch(ctx, slices[half:], orphans, hubs, existingDomainsStr); err != nil {
					p.Logger.Error("pass 3: second half retry failed", zap.Error(err))
					for _, s := range slices[half:] {
						failedPrograms = append(failedPrograms, s.ProgramID)
					}
				} else {
					totalBatches++
					existingDomains = p.refreshDomainNames(ctx, existingDomains)
				}
			} else {
				// Single program batch still failed
				for _, s := range slices {
					failedPrograms = append(failedPrograms, s.ProgramID)
				}
			}
		} else {
			totalBatches++
			existingDomains = p.refreshDomainNames(ctx, existingDomains)
		}

		offset += batchSize

		if len(slices) < batchSize {
			break
		}
	}

	if len(failedPrograms) > 0 {
		p.Logger.Warn("pass 3: programs failed analysis",
			zap.Int("count", len(failedPrograms)),
			zap.Strings("programIds", failedPrograms))
	}

	p.Logger.Info("pass 3 complete", zap.Int("batches", totalBatches))
	return nil
}

// formatDomainList formats domain names as a bulleted list for the prompt.
func formatDomainList(domains []string) string {
	if len(domains) == 0 {
		return ""
	}
	var b strings.Builder
	for _, d := range domains {
		b.WriteString("- ")
		b.WriteString(d)
		b.WriteString("\n")
	}
	return b.String()
}

// refreshDomainNames re-queries domain names from Neo4j after a batch write.
func (p *Pipeline) refreshDomainNames(ctx context.Context, fallback []string) []string {
	updated, err := p.Neo4jClient.QueryExistingDomainNames(ctx)
	if err != nil {
		p.Logger.Warn("pass 3: failed to refresh domain names", zap.Error(err))
		return fallback
	}
	return updated
}

// RunPass1JCL analyzes JCL files using Sonnet (cheap/fast).
func (p *Pipeline) RunPass1JCL(ctx context.Context, scanResult *scanner.ScanResult) error {
	jclFileMap := make(map[string]graph.FileInfo)
	jclPathHashes := make(map[string]string)
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeJCL {
			continue
		}
		jclFileMap[f.Path] = f
		jclPathHashes[f.Path] = f.Hash
	}

	changedJCLPaths, err := p.Cache.BatchIsChanged(jclPathHashes)
	if err != nil {
		return fmt.Errorf("batch checking JCL cache: %w", err)
	}

	jclFiles := make([]graph.FileInfo, 0, len(changedJCLPaths))
	for _, path := range changedJCLPaths {
		jclFiles = append(jclFiles, jclFileMap[path])
	}

	if len(jclFiles) == 0 {
		p.Logger.Info("pass 1 JCL: no changed JCL files to process")
		return nil
	}
	p.Logger.Info("pass 1 JCL: files to process", zap.Int("count", len(jclFiles)))

	var (
		successCount int
		errorCount   int
		mu           sync.Mutex
	)

	sem := make(chan struct{}, p.Config.Ingest.WorkersForPass(1))
	var wg sync.WaitGroup

	for _, f := range jclFiles {
		wg.Add(1)
		go func(f graph.FileInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			data, err := os.ReadFile(f.Path)
			if err != nil {
				p.Logger.Error("pass 1 JCL: failed to read file", zap.String("file", f.Path), zap.Error(err))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			jsonResp, err := p.Claude.AnalyzeJCL(ctx, f.Path, string(data))
			if err != nil {
				p.Logger.Error("pass 1 JCL: claude analysis failed", zap.String("file", f.Path), zap.Error(err))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			result, err := parser.ParseJCLResponse(jsonResp, f.Path)
			if err != nil {
				p.Logger.Error("pass 1 JCL: parse failed", zap.String("file", f.Path), zap.Error(err))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			if err := p.Writer.WriteJCLResult(ctx, result); err != nil {
				p.Logger.Error("pass 1 JCL: write failed", zap.String("file", f.Path), zap.Error(err))
				mu.Lock()
				errorCount++
				mu.Unlock()
				return
			}

			if err := p.Cache.MarkProcessed(f.Path, f.Hash); err != nil {
				p.Logger.Error("pass 1 JCL: cache update failed", zap.String("file", f.Path), zap.Error(err))
			}
			mu.Lock()
			successCount++
			mu.Unlock()
		}(f)
	}

	wg.Wait()

	p.Logger.Info("pass 1 JCL complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
	)

	return nil
}

// RunPass4 executes Pass 4 cross-program data flow analysis.
func (p *Pipeline) RunPass4(ctx context.Context) error {
	p.Logger.Info("pass 4: starting cross-program data flow analysis")

	// Step 1: Graph-only shared file flows
	if err := p.Writer.DetectSharedFileFlows(ctx); err != nil {
		p.Logger.Warn("pass 4: shared file flow detection failed", zap.Error(err))
	}

	// Step 2: Graph-only shared DB2 flows
	if err := p.Writer.DetectSharedDB2Flows(ctx); err != nil {
		p.Logger.Warn("pass 4: shared DB2 flow detection failed", zap.Error(err))
	}

	// Step 3: LLM-assisted LINKAGE parameter mapping
	callPairs, err := p.Neo4jClient.QueryCallPairsForPass4(ctx)
	if err != nil {
		p.Logger.Warn("pass 4: failed to query call pairs", zap.Error(err))
		return nil
	}

	if len(callPairs) == 0 {
		p.Logger.Info("pass 4: no call pairs with LINKAGE parameters found")
		return nil
	}

	p.Logger.Info("pass 4: analyzing LINKAGE mappings", zap.Int("callPairs", len(callPairs)))

	pass4Result := &graph.Pass4Result{}
	var (
		successCount int
		errorCount   int
		pass4Mu      sync.Mutex
	)

	pass4Sem := make(chan struct{}, p.Config.Ingest.WorkersForPass(2))
	var pass4Wg sync.WaitGroup

	for _, pair := range callPairs {
		pass4Wg.Add(1)
		go func(pair n4j.CallPairContext) {
			defer pass4Wg.Done()
			pass4Sem <- struct{}{}
			defer func() { <-pass4Sem }()

			if ctx.Err() != nil {
				return
			}

			fieldContext := formatFieldContext(pair)

			jsonResp, err := p.Claude.AnalyzeCrossProgramFlow(ctx, pair.CallerID, pair.CalleeID, fieldContext)
			if err != nil {
				p.Logger.Warn("pass 4: LINKAGE analysis failed",
					zap.String("caller", pair.CallerID),
					zap.String("callee", pair.CalleeID),
					zap.Error(err))
				pass4Mu.Lock()
				errorCount++
				pass4Mu.Unlock()
				return
			}

			fields, err := parser.ParsePass4Response(jsonResp)
			if err != nil {
				p.Logger.Warn("pass 4: parse failed",
					zap.String("caller", pair.CallerID),
					zap.String("callee", pair.CalleeID),
					zap.Error(err))
				pass4Mu.Lock()
				errorCount++
				pass4Mu.Unlock()
				return
			}

			pass4Mu.Lock()
			if len(fields) > 0 {
				pass4Result.Flows = append(pass4Result.Flows, graph.CrossProgramFlow{
					FromProgram: pair.CallerID,
					ToProgram:   pair.CalleeID,
					Channel:     "LINKAGE",
					Fields:      fields,
				})
			}
			successCount++
			pass4Mu.Unlock()
		}(pair)
	}

	pass4Wg.Wait()

	if len(pass4Result.Flows) > 0 {
		if err := p.Writer.WritePass4Result(ctx, pass4Result); err != nil {
			return fmt.Errorf("writing pass 4 results: %w", err)
		}
		if err := p.Writer.WritePass4FieldMappings(ctx, pass4Result); err != nil {
			p.Logger.Warn("pass 4: field mapping write failed", zap.Error(err))
		}
	}

	p.Logger.Info("pass 4 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("linkageFlows", len(pass4Result.Flows)),
	)

	return nil
}

// formatFieldContext formats call pair context for the Pass 4 prompt.
func formatFieldContext(pair n4j.CallPairContext) string {
	result := fmt.Sprintf("Caller fields (%s):\n", pair.CallerID)
	for _, f := range pair.CallerFields {
		result += fmt.Sprintf("  - %s (PIC: %s)\n", f.Name, f.Picture)
	}
	result += fmt.Sprintf("\nCallee LINKAGE parameters (%s):\n", pair.CalleeID)
	for _, f := range pair.CalleeParams {
		result += fmt.Sprintf("  - %s (direction: %s)\n", f.Name, f.Direction)
	}
	return result
}

// RunPass5 executes Pass 5: graph validation and LLM-assisted repair.
func (p *Pipeline) RunPass5(ctx context.Context, scanResult *scanner.ScanResult) error {
	p.Logger.Info("pass 5: starting graph validation and repair")

	// Step 1: Run validation checks
	validationResult, err := p.Writer.RunValidation(ctx)
	if err != nil {
		return fmt.Errorf("running validation: %w", err)
	}

	for _, chk := range validationResult.Checks {
		if chk.Count > 0 {
			p.Logger.Info("pass 5: gap detected",
				zap.String("check", chk.Name),
				zap.Int("count", chk.Count),
				zap.String("severity", chk.Severity),
				zap.String("fixMethod", chk.FixMethod),
			)
		}
	}

	// Steps 2-4: Detect all repair needs and batch where possible
	missingCalls, err := p.Neo4jClient.QueryProgramsMissingCalls(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query missing CALLS", zap.Error(err))
	}
	missingChildOf, err := p.Neo4jClient.QueryProgramsMissingChildOf(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query missing CHILD_OF", zap.Error(err))
	}
	missingMovesTo, err := p.Neo4jClient.QueryProgramsMissingMovesTo(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query missing MOVES_TO", zap.Error(err))
	}

	// Build per-program repair type map
	repairNeeds := make(map[string][]string) // programID → []repairType
	for _, pid := range missingCalls {
		repairNeeds[pid] = append(repairNeeds[pid], "MISSING_CALLS")
	}
	for _, pid := range missingChildOf {
		repairNeeds[pid] = append(repairNeeds[pid], "CHILD_OF")
	}
	for _, pid := range missingMovesTo {
		repairNeeds[pid] = append(repairNeeds[pid], "MOVES_TO")
	}

	// Separate into single-type and multi-type repairs
	type singleRepairItem struct {
		pid        string
		repairType string
	}
	var singleRepairs []singleRepairItem
	var multiRepairs []multiRepairItem
	for pid, types := range repairNeeds {
		if len(types) == 1 {
			singleRepairs = append(singleRepairs, singleRepairItem{pid, types[0]})
		} else {
			multiRepairs = append(multiRepairs, multiRepairItem{pid, types})
		}
	}

	// Process multi-type repairs with combined calls
	if len(multiRepairs) > 0 {
		p.Logger.Info("pass 5: batching multi-type repairs", zap.Int("programs", len(multiRepairs)))
		p.repairCombined(ctx, multiRepairs)
	}

	// Process single-type repairs with existing logic, grouped by type
	singleByType := make(map[string][]string)
	for _, sr := range singleRepairs {
		singleByType[sr.repairType] = append(singleByType[sr.repairType], sr.pid)
	}
	for repairType, pids := range singleByType {
		if len(pids) > 0 {
			p.Logger.Info("pass 5: repairing gaps", zap.String("type", repairType), zap.Int("count", len(pids)))
			p.repairRelationshipGap(ctx, pids, repairType)
		}
	}

	// Step 5: Fix Gap — Dangling CALLS targets (graph-only, creates stub Programs)
	if fixed, err := p.Writer.FixDanglingCalls(ctx); err != nil {
		p.Logger.Warn("pass 5: dangling calls fix failed", zap.Error(err))
	} else if fixed > 0 {
		p.Logger.Info("pass 5: marked external programs", zap.Int("fixed", fixed))
	}

	// Step 5b: Clear scores from external programs (may have been scored before marking)
	if cleared, err := p.Writer.ClearExternalScores(ctx); err != nil {
		p.Logger.Warn("pass 5: clear external scores failed", zap.Error(err))
	} else if cleared > 0 {
		p.Logger.Info("pass 5: cleared scores from external programs", zap.Int("cleared", cleared))
	}

	// Step 5c: Fix false-positive dead code flags
	if fixed, err := p.Writer.FixFalseDeadCode(ctx); err != nil {
		p.Logger.Warn("pass 5: dead code false positive fix failed", zap.Error(err))
	} else if fixed > 0 {
		p.Logger.Info("pass 5: cleared false dead code flags", zap.Int("fixed", fixed))
	}

	// Step 6: Re-run Pass 3 for programs missing riskScore (needs relationships from steps 2-5)
	missingPass3, err := p.Neo4jClient.QueryProgramsMissingPass3(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query missing Pass 3 programs", zap.Error(err))
	} else if len(missingPass3) > 0 {
		p.Logger.Info("pass 5: re-running Pass 3 for missing programs", zap.Int("count", len(missingPass3)))
		if err := p.rerunPass3ForPrograms(ctx, missingPass3); err != nil {
			p.Logger.Warn("pass 5: Pass 3 re-run failed", zap.Error(err))
		}
	}

	// Step 7: Merge duplicate domains (safety net after Pass 3 reruns)
	if merged, err := p.Writer.MergeDuplicateDomains(ctx); err != nil {
		p.Logger.Warn("pass 5: domain merge failed", zap.Error(err))
	} else if merged > 0 {
		p.Logger.Info("pass 5: merged duplicate domains", zap.Int("merged", merged))
	}

	// Step 8: Fix Gap — Unannotated paragraphs
	unannotated, err := p.Neo4jClient.QueryUnannotatedParagraphs(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query unannotated paragraphs", zap.Error(err))
	} else if len(unannotated) > 0 {
		p.Logger.Info("pass 5: repairing paragraph annotations", zap.Int("programs", len(unannotated)))
		p.repairAnnotations(ctx, unannotated)
	}

	// Step 9: Fix Gap — Unlinked DDCards (graph-only)
	if fixed, err := p.Writer.FixUnlinkedDDCards(ctx); err != nil {
		p.Logger.Warn("pass 5: DD card fix failed", zap.Error(err))
	} else if fixed > 0 {
		p.Logger.Info("pass 5: linked DD cards to files", zap.Int("fixed", fixed))
	}

	// Step 10: Re-run validation and log final summary
	finalResult, err := p.Writer.RunValidation(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: final validation failed", zap.Error(err))
	} else {
		for _, chk := range finalResult.Checks {
			if chk.Count > 0 {
				p.Logger.Info("pass 5: remaining gap",
					zap.String("check", chk.Name),
					zap.Int("count", chk.Count),
				)
			}
		}
	}

	p.Logger.Info("pass 5 complete")
	return nil
}

// rerunPass3ForPrograms re-runs Pass 3 analysis for specific programs.
func (p *Pipeline) rerunPass3ForPrograms(ctx context.Context, programIDs []string) error {
	batchSize := p.Config.Ingest.Pass3BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	orphans, err := p.Neo4jClient.QueryOrphanPrograms(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query orphans for Pass 3 rerun", zap.Error(err))
	}

	hubs, err := p.Neo4jClient.QueryHubPrograms(ctx, 5)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query hubs for Pass 3 rerun", zap.Error(err))
	}

	// Feed existing domains so rerun batches don't invent synonyms
	existingDomains, err := p.Neo4jClient.QueryExistingDomainNames(ctx)
	if err != nil {
		p.Logger.Warn("pass 5: failed to query existing domains for Pass 3 rerun", zap.Error(err))
	}

	// Query call graph slices for just the missing programs
	for i := 0; i < len(programIDs); i += batchSize {
		end := i + batchSize
		if end > len(programIDs) {
			end = len(programIDs)
		}
		batch := programIDs[i:end]

		slices, err := p.Neo4jClient.QueryCallGraphSliceForPrograms(ctx, batch)
		if err != nil {
			p.Logger.Warn("pass 5: failed to query slices for Pass 3 rerun", zap.Error(err))
			continue
		}
		if len(slices) == 0 {
			continue
		}

		existingDomainsStr := formatDomainList(existingDomains)
		if err := p.processPass3Batch(ctx, slices, orphans, hubs, existingDomainsStr); err != nil {
			p.Logger.Warn("pass 5: Pass 3 rerun batch failed", zap.Error(err))
		} else {
			existingDomains = p.refreshDomainNames(ctx, existingDomains)
		}
	}

	return nil
}

// pass5Workers returns the worker count for Pass 5 LLM calls.
func (p *Pipeline) pass5Workers() int {
	return p.Config.Ingest.WorkersForPass(5)
}

// repairRelationshipGap sends repair prompts to Opus for a gap type and writes results.
func (p *Pipeline) repairRelationshipGap(ctx context.Context, programIDs []string, repairType string) {
	sem := make(chan struct{}, p.pass5Workers())
	var wg sync.WaitGroup

	for _, pid := range programIDs {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			filePath, err := p.Neo4jClient.GetProgramFilePath(ctx, pid)
			if err != nil || filePath == "" {
				p.Logger.Debug("pass 5: skipping repair (no source file)", zap.String("program", pid))
				return
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				p.Logger.Debug("pass 5: cannot read source file", zap.String("file", filePath), zap.Error(err))
				return
			}

			tokenBudget := p.Config.Ingest.Pass2TokenLimit
			if tokenBudget <= 0 {
				tokenBudget = p.Config.Ingest.TokenLimit
			}
			trimmedSource := truncateForRepair(string(sourceCode), tokenBudget, repairType)

			jsonResp, err := p.Claude.AnalyzeRepair(ctx, repairType, pid, "", trimmedSource, "")
			if err != nil {
				p.Logger.Warn("pass 5: repair LLM call failed",
					zap.String("program", pid),
					zap.String("repairType", repairType),
					zap.Error(err))
				return
			}

			var rels []graph.Relationship
			switch repairType {
			case "CHILD_OF":
				rels, err = parser.ParseRepairChildOf(jsonResp, pid)
			case "MOVES_TO":
				rels, err = parser.ParseRepairMovesTo(jsonResp, pid)
			case "MISSING_CALLS":
				rels, err = parser.ParseRepairCalls(jsonResp, pid)
			}

			if err != nil {
				p.Logger.Warn("pass 5: repair parse failed",
					zap.String("program", pid),
					zap.String("repairType", repairType),
					zap.Error(err))
				return
			}

			// Ensure DataItem nodes exist before writing CHILD_OF relationships
			if repairType == "CHILD_OF" && len(rels) > 0 {
				p.ensureDataItemNodesForRepair(ctx, pid, rels)
			}

			if len(rels) > 0 {
				if err := p.writeRepairRelationships(ctx, rels); err != nil {
					p.Logger.Warn("pass 5: repair write failed",
						zap.String("program", pid),
						zap.String("repairType", repairType),
						zap.Error(err))
				} else {
					p.Logger.Info("pass 5: repaired relationships",
						zap.String("program", pid),
						zap.String("repairType", repairType),
						zap.Int("count", len(rels)),
					)
				}
			}
		}(pid)
	}

	wg.Wait()
}

// repairCombined sends a single combined repair prompt for programs needing multiple repair types.
// On LLM or parse failure it falls back to individual repairRelationshipGap calls per type.
func (p *Pipeline) repairCombined(ctx context.Context, repairs []multiRepairItem) {
	sem := make(chan struct{}, p.pass5Workers())
	var wg sync.WaitGroup

	for _, repair := range repairs {
		wg.Add(1)
		go func(pid string, types []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			filePath, err := p.Neo4jClient.GetProgramFilePath(ctx, pid)
			if err != nil || filePath == "" {
				p.Logger.Debug("pass 5: skipping combined repair (no source file)", zap.String("program", pid))
				return
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				p.Logger.Debug("pass 5: cannot read source file", zap.String("file", filePath), zap.Error(err))
				return
			}

			tokenBudget := p.Config.Ingest.Pass2TokenLimit
			if tokenBudget <= 0 {
				tokenBudget = p.Config.Ingest.TokenLimit
			}
			// Combined repairs always need DATA + PROCEDURE divisions
			trimmedSource := truncateForRepair(string(sourceCode), tokenBudget, "CHILD_OF")

			combinedType := strings.Join(types, "+")
			jsonResp, err := p.Claude.AnalyzeRepair(ctx, combinedType, pid, "", trimmedSource, "")
			if err != nil {
				p.Logger.Warn("pass 5: combined repair LLM call failed",
					zap.String("program", pid),
					zap.String("repairTypes", combinedType),
					zap.Error(err))
				// Fall back to individual repairs
				for _, rt := range types {
					p.repairRelationshipGap(ctx, []string{pid}, rt)
				}
				return
			}

			// Parse each repair type's section from the combined response
			var allRels []graph.Relationship
			parseSuccess := true

			for _, rt := range types {
				var rels []graph.Relationship
				var parseErr error
				switch rt {
				case "CHILD_OF":
					rels, parseErr = parser.ParseRepairChildOf(jsonResp, pid)
				case "MOVES_TO":
					rels, parseErr = parser.ParseRepairMovesTo(jsonResp, pid)
				case "MISSING_CALLS":
					rels, parseErr = parser.ParseRepairCalls(jsonResp, pid)
				}
				if parseErr != nil {
					p.Logger.Warn("pass 5: combined repair parse failed for type",
						zap.String("program", pid),
						zap.String("repairType", rt),
						zap.Error(parseErr))
					parseSuccess = false
					break
				}
				allRels = append(allRels, rels...)
			}

			if !parseSuccess {
				// Fall back to individual repairs
				for _, rt := range types {
					p.repairRelationshipGap(ctx, []string{pid}, rt)
				}
				return
			}

			// Ensure DataItem nodes exist before writing CHILD_OF relationships
			for _, rt := range types {
				if rt == "CHILD_OF" {
					var childOfRels []graph.Relationship
					for _, r := range allRels {
						if r.Type == graph.RelChildOf {
							childOfRels = append(childOfRels, r)
						}
					}
					if len(childOfRels) > 0 {
						p.ensureDataItemNodesForRepair(ctx, pid, childOfRels)
					}
					break
				}
			}

			if len(allRels) > 0 {
				if err := p.writeRepairRelationships(ctx, allRels); err != nil {
					p.Logger.Warn("pass 5: combined repair write failed",
						zap.String("program", pid),
						zap.Error(err))
				} else {
					p.Logger.Info("pass 5: combined repair succeeded",
						zap.String("program", pid),
						zap.String("types", combinedType),
						zap.Int("relationships", len(allRels)),
					)
				}
			}
		}(repair.pid, repair.types)
	}

	wg.Wait()
}

// repairAnnotations sends annotation repair prompts for programs with unannotated paragraphs.
func (p *Pipeline) repairAnnotations(ctx context.Context, unannotated map[string][]string) {
	sem := make(chan struct{}, p.pass5Workers())
	var wg sync.WaitGroup

	for pid, paraNames := range unannotated {
		wg.Add(1)
		go func(pid string, paraNames []string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			filePath, err := p.Neo4jClient.GetProgramFilePath(ctx, pid)
			if err != nil || filePath == "" {
				return
			}

			sourceCode, err := os.ReadFile(filePath)
			if err != nil {
				return
			}

			tokenBudget := p.Config.Ingest.Pass2TokenLimit
			if tokenBudget <= 0 {
				tokenBudget = p.Config.Ingest.TokenLimit
			}
			trimmedSource := truncateForRepair(string(sourceCode), tokenBudget, "ANNOTATIONS")

			missingList := ""
			for i, n := range paraNames {
				if i > 0 {
					missingList += ", "
				}
				missingList += n
			}

			jsonResp, err := p.Claude.AnalyzeRepair(ctx, "ANNOTATIONS", pid, "", trimmedSource, missingList)
			if err != nil {
				p.Logger.Warn("pass 5: annotation repair LLM failed", zap.String("program", pid), zap.Error(err))
				return
			}

			annotations, err := parser.ParseRepairAnnotations(jsonResp)
			if err != nil {
				p.Logger.Warn("pass 5: annotation parse failed", zap.String("program", pid), zap.Error(err))
				return
			}

			if len(annotations) > 0 {
				rows := make([]map[string]any, len(annotations))
				for i, a := range annotations {
					rows[i] = map[string]any{
						"name":        a.Paragraph,
						"description": a.Description,
						"category":    a.Category,
					}
				}
				if err := p.Writer.WriteParagraphAnnotations(ctx, pid, rows); err != nil {
					p.Logger.Warn("pass 5: annotation write failed", zap.String("program", pid), zap.Error(err))
				} else {
					p.Logger.Info("pass 5: annotated paragraphs",
						zap.String("program", pid),
						zap.Int("count", len(annotations)),
					)
				}
			}
		}(pid, paraNames)
	}

	wg.Wait()
}

// truncateForRepair trims source code to fit within a token budget for Pass 5 LLM calls.
// For CHILD_OF and MOVES_TO repairs it keeps DATA + PROCEDURE divisions; for CALLS it keeps
// only PROCEDURE. If the result still exceeds the budget it hard-truncates.
func truncateForRepair(sourceCode string, tokenBudget int, repairType string) string {
	if chunker.EstimateTokens(sourceCode) <= tokenBudget {
		return sourceCode
	}

	divs := chunker.SplitDivisions(sourceCode)

	var trimmed string
	switch {
	case repairType == "CHILD_OF" || repairType == "MOVES_TO" || repairType == "ANNOTATIONS":
		// These need DATA DIVISION context too
		trimmed = divs["DATA"] + "\n" + divs["PROCEDURE"]
	case strings.Contains(repairType, "CHILD_OF") || strings.Contains(repairType, "MOVES_TO"):
		// Combined repair types that include data-flow repair also need DATA DIVISION
		trimmed = divs["DATA"] + "\n" + divs["PROCEDURE"]
	default:
		// MISSING_CALLS only needs PROCEDURE
		trimmed = divs["PROCEDURE"]
	}

	if trimmed == "" {
		trimmed = sourceCode
	}

	if chunker.EstimateTokens(trimmed) <= tokenBudget {
		return trimmed
	}

	// Hard truncation as last resort
	maxChars := tokenBudget * 4
	if maxChars > len(trimmed) {
		return trimmed
	}
	return trimmed[:maxChars]
}

// ensureDataItemNodesForRepair creates minimal DataItem nodes for CHILD_OF repair targets.
func (p *Pipeline) ensureDataItemNodesForRepair(ctx context.Context, programID string, rels []graph.Relationship) {
	seen := make(map[string]bool)
	var nodes []map[string]any
	for _, r := range rels {
		for _, fqn := range []string{r.FromKey, r.ToKey} {
			if seen[fqn] {
				continue
			}
			seen[fqn] = true
			// Parse level and name from FQN: "PROGRAM.LEVEL.NAME"
			parts := strings.SplitN(fqn, ".", 3)
			if len(parts) != 3 {
				continue
			}
			level, _ := strconv.Atoi(parts[1])
			nodes = append(nodes, map[string]any{
				"name":      parts[2],
				"level":     level,
				"programId": programID,
				"fqn":       fqn,
			})
		}
	}
	if len(nodes) > 0 {
		if err := p.Writer.WriteNodes(ctx, "DataItem", "fqn", nodes); err != nil {
			p.Logger.Warn("pass 5: failed to ensure DataItem nodes for CHILD_OF repair", zap.Error(err))
		}
	}
}

// writeRepairRelationships converts graph.Relationship slice to map rows and writes to Neo4j.
func (p *Pipeline) writeRepairRelationships(ctx context.Context, rels []graph.Relationship) error {
	// Group by (relType, fromLabel, fromKey, toLabel, toKey) pattern
	type relGroup struct {
		relType   string
		fromLabel string
		fromKey   string
		toLabel   string
		toKey     string
	}

	groups := make(map[relGroup][]map[string]any)
	for _, r := range rels {
		key := relGroup{
			relType:   string(r.Type),
			fromLabel: r.FromLabel,
			fromKey:   mergeKeyForLabel(r.FromLabel),
			toLabel:   r.ToLabel,
			toKey:     mergeKeyForLabel(r.ToLabel),
		}
		row := map[string]any{
			"fromKey": r.FromKey,
			"toKey":   r.ToKey,
			"props":   r.Properties,
		}
		groups[key] = append(groups[key], row)
	}

	for g, rows := range groups {
		if err := p.Writer.WriteRepairRelationships(ctx, g.relType, g.fromLabel, g.fromKey, g.toLabel, g.toKey, rows); err != nil {
			return err
		}
	}
	return nil
}

// mergeKeyForLabel returns the merge key for a node label (duplicated from writer for pipeline use).
func mergeKeyForLabel(label string) string {
	switch label {
	case "Program":
		return "programId"
	case "Copybook":
		return "name"
	case "DataItem":
		return "fqn"
	case "Paragraph":
		return "mergeId"
	case "Section":
		return "mergeId"
	case "File":
		return "name"
	default:
		return "id"
	}
}

// runStaticDataHierarchy extracts CHILD_OF relationships deterministically from COBOL
// level numbers, eliminating the need for LLM-based CHILD_OF repair in Pass 5.
func (p *Pipeline) runStaticDataHierarchy(ctx context.Context, scanResult *scanner.ScanResult) {
	var totalRels int
	var filesProcessed int

	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}

		data, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}

		content := string(data)
		if chunker.StripSequenceColumns {
			content = chunker.SplitDivisions(content)["DATA"]
		} else {
			content = chunker.SplitDivisions(content)["DATA"]
		}

		if content == "" {
			continue
		}

		// Look up program ID from Neo4j
		session := p.Neo4jClient.NewSession(ctx)
		result, err := session.Run(ctx,
			"MATCH (prog:Program {filePath: $path}) RETURN prog.programId AS pid LIMIT 1",
			map[string]any{"path": f.Path},
		)
		var programID string
		if err == nil && result.Next(ctx) {
			if pid, ok := result.Record().Get("pid"); ok {
				programID, _ = pid.(string)
			}
		}
		session.Close(ctx)

		if programID == "" {
			continue
		}

		rels := static.ExtractDataHierarchy(content, programID)
		if len(rels) == 0 {
			continue
		}

		// Ensure DataItem nodes exist
		p.ensureDataItemNodesForRepair(ctx, programID, rels)

		// Write CHILD_OF relationships
		if err := p.writeRepairRelationships(ctx, rels); err != nil {
			p.Logger.Warn("static CHILD_OF write failed",
				zap.String("program", programID),
				zap.Error(err))
			continue
		}

		totalRels += len(rels)
		filesProcessed++
	}

	if totalRels > 0 {
		p.Logger.Info("static data hierarchy extraction complete",
			zap.Int("files", filesProcessed),
			zap.Int("child_of_relationships", totalRels),
		)
	}
}

// processPass3Batch runs Claude analysis and writes results for a batch of program slices.
func (p *Pipeline) processPass3Batch(ctx context.Context, slices []n4j.ProgramSlice, orphans, hubs []string, existingDomains string) error {
	graphText := n4j.FormatGraphSlice(slices, orphans, hubs)

	jsonResp, err := p.Claude.AnalyzeCrossCutting(ctx, graphText, existingDomains)
	if err != nil {
		return fmt.Errorf("claude analysis: %w", err)
	}

	result, err := parser.ParsePass3Response(jsonResp)
	if err != nil {
		return fmt.Errorf("parsing: %w", err)
	}

	if err := p.Writer.WritePass3Result(ctx, result); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	return nil
}
