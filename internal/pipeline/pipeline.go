package pipeline

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
	"cobol-ingestor/internal/pool"
	"cobol-ingestor/internal/scanner"

	"go.uber.org/zap"
)

// Pipeline orchestrates the multi-pass COBOL analysis workflow.
type Pipeline struct {
	Config      *config.Config
	Claude      *claude.Client
	Neo4jClient *n4j.Client
	Writer      *n4j.BatchWriter
	Cache       *cache.Cache
	Logger      *zap.Logger
}

// Run executes the pipeline for the specified pass (0=all, 1/2/3=individual).
func (p *Pipeline) Run(ctx context.Context, scanResult *scanner.ScanResult, passFlag int) error {
	if passFlag == 0 || passFlag == 1 {
		if err := p.RunPass1(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 1: %w", err)
		}
	}
	if passFlag == 0 || passFlag == 2 {
		if err := p.RunPass2(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 2: %w", err)
		}
	}
	if passFlag == 0 || passFlag == 3 {
		if err := p.RunPass3(ctx); err != nil {
			return fmt.Errorf("pass 3: %w", err)
		}
	}
	return nil
}

// RunPass1 executes Pass 1 structural analysis.
func (p *Pipeline) RunPass1(ctx context.Context, scanResult *scanner.ScanResult) error {
	var changed []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		isChanged, err := p.Cache.IsChanged(f.Path, f.Hash)
		if err != nil {
			return fmt.Errorf("checking cache: %w", err)
		}
		if isChanged {
			changed = append(changed, f)
		}
	}

	if len(changed) == 0 {
		p.Logger.Info("pass 1: no changed files to process")
		return nil
	}
	p.Logger.Info("pass 1: files to process", zap.Int("changed", len(changed)))

	var chunks []chunker.Chunk
	for _, f := range changed {
		fileChunks, err := chunker.ChunkFile(f, p.Config.Ingest.TokenLimit, p.Logger)
		if err != nil {
			p.Logger.Error("chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		chunks = append(chunks, fileChunks...)
	}

	processFn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error) {
		jsonResp, err := p.Claude.AnalyzeStructural(ctx, chunk.FileName, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("claude analysis: %w", err)
		}
		return parser.ParsePass1Response(jsonResp, chunk.FileName)
	}

	results := pool.RunPass1(ctx, chunks, processFn, p.Config.Claude.MaxWorkers, p.Logger)

	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
			continue
		}
		if err := p.Writer.WritePass1Result(ctx, r.Result); err != nil {
			p.Logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
			errorCount++
			continue
		}
		for _, f := range changed {
			if f.Path == r.FilePath {
				if err := p.Cache.MarkProcessed(f.Path, f.Hash); err != nil {
					p.Logger.Error("failed to update cache", zap.String("file", f.Path), zap.Error(err))
				}
				break
			}
		}
		successCount++
	}

	p.Logger.Info("pass 1 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("total", len(results)),
	)

	return nil
}

// RunPass2 executes Pass 2 deep semantic analysis.
func (p *Pipeline) RunPass2(ctx context.Context, scanResult *scanner.ScanResult) error {
	copybookIndex := chunker.BuildCopybookIndex(scanResult.Files)
	p.Logger.Info("pass 2: built copybook index", zap.Int("copybooks", len(copybookIndex)))

	var changed []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		isChanged, err := p.Cache.IsChangedForPass(f.Path, f.Hash, 2)
		if err != nil {
			return fmt.Errorf("checking pass 2 cache: %w", err)
		}
		if isChanged {
			changed = append(changed, f)
		}
	}

	if len(changed) == 0 {
		p.Logger.Info("pass 2: no changed files to process")
		return nil
	}
	p.Logger.Info("pass 2: files to process", zap.Int("changed", len(changed)))

	chunkOpts := chunker.Pass2ChunkOptions{
		TokenLimit:    p.Config.Ingest.Pass2TokenLimit,
		OverlapLines:  p.Config.Ingest.OverlapLines,
		CopybookIndex: copybookIndex,
	}

	var allChunks []chunker.Chunk
	fileProgramIDs := make(map[string]string)

	for _, f := range changed {
		fileChunks, err := chunker.ChunkFilePass2(f, chunkOpts, p.Logger)
		if err != nil {
			p.Logger.Error("pass 2: chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		allChunks = append(allChunks, fileChunks...)
	}

	for _, f := range changed {
		session := p.Neo4jClient.NewSession(ctx)
		result, err := session.Run(ctx,
			"MATCH (prog:Program {filePath: $path}) RETURN prog.programId AS pid LIMIT 1",
			map[string]any{"path": f.Path},
		)
		if err == nil && result.Next(ctx) {
			if val, ok := result.Record().Get("pid"); ok {
				if pid, ok := val.(string); ok {
					fileProgramIDs[f.Path] = pid
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

	pass2Fn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass2Result, error) {
		preamble := contextPreambles[chunk.FileName]
		jsonResp, err := p.Claude.AnalyzeDeep(ctx, chunk, preamble)
		if err != nil {
			return nil, fmt.Errorf("claude deep analysis: %w", err)
		}
		programID := fileProgramIDs[chunk.FileName]
		if programID == "" {
			programID = "UNKNOWN"
		}
		return parser.ParsePass2Response(jsonResp, chunk.FileName, programID)
	}

	pass2Workers := p.Config.Ingest.Pass2Workers
	if pass2Workers <= 0 {
		pass2Workers = 3
	}
	results := pool.RunPass2(ctx, allChunks, pass2Fn, pass2Workers, p.Logger)

	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
			continue
		}
		if err := p.Writer.WritePass2Result(ctx, r.Result); err != nil {
			p.Logger.Error("pass 2: failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
			errorCount++
			continue
		}
		for _, f := range changed {
			if f.Path == r.FilePath {
				if err := p.Cache.MarkProcessedForPass(f.Path, f.Hash, 2); err != nil {
					p.Logger.Error("pass 2: failed to update cache", zap.String("file", f.Path), zap.Error(err))
				}
				break
			}
		}
		successCount++
	}

	p.Logger.Info("pass 2 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("total", len(results)),
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

	batchSize := p.Config.Ingest.Pass3BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	offset := 0
	totalBatches := 0
	for {
		slices, err := p.Neo4jClient.QueryCallGraphSlice(ctx, batchSize, offset)
		if err != nil {
			return fmt.Errorf("querying call graph slice: %w", err)
		}
		if len(slices) == 0 {
			break
		}

		graphText := n4j.FormatGraphSlice(slices, orphans, hubs)

		jsonResp, err := p.Claude.AnalyzeCrossCutting(ctx, graphText)
		if err != nil {
			p.Logger.Error("pass 3: Claude analysis failed", zap.Error(err))
			offset += batchSize
			continue
		}

		result, err := parser.ParsePass3Response(jsonResp)
		if err != nil {
			p.Logger.Error("pass 3: parsing failed", zap.Error(err))
			offset += batchSize
			continue
		}

		if err := p.Writer.WritePass3Result(ctx, result); err != nil {
			p.Logger.Error("pass 3: write failed", zap.Error(err))
		}

		totalBatches++
		offset += batchSize

		if len(slices) < batchSize {
			break
		}
	}

	p.Logger.Info("pass 3 complete", zap.Int("batches", totalBatches))
	return nil
}
