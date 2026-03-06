package pipeline

import (
	"context"
	"fmt"
	"os"

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

// Run executes the pipeline for the specified pass (0=all, 1/2/3/4=individual).
func (p *Pipeline) Run(ctx context.Context, scanResult *scanner.ScanResult, passFlag int) error {
	if passFlag == 0 || passFlag == 1 {
		if err := p.RunPass1(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 1: %w", err)
		}
		// JCL analysis runs as part of Pass 1
		if err := p.RunPass1JCL(ctx, scanResult); err != nil {
			return fmt.Errorf("pass 1 JCL: %w", err)
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

	return nil
}

// RunPass1 executes Pass 1 structural analysis.
// Results are streamed to Neo4j as each file completes (or as all chunks for a
// multi-chunk file complete). This ensures partial progress is persisted even
// if the pipeline is interrupted.
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

	processFn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error) {
		jsonResp, err := p.Claude.AnalyzeStructural(ctx, chunk.FileName, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("claude analysis: %w", err)
		}
		return parser.ParsePass1Response(jsonResp, chunk.FileName)
	}

	resultsCh := pool.RunPass1(ctx, chunks, processFn, p.Config.Claude.MaxWorkers, p.Logger)

	// Accumulate multi-chunk results per file, write as soon as all chunks arrive.
	// Single-chunk files (the common case) are written immediately.
	pendingChunks := make(map[string][]*graph.Pass1Result)
	fileHasError := make(map[string]bool)
	successCount := 0
	errorCount := 0

	for r := range resultsCh {
		if r.Err != nil {
			errorCount++
			fileHasError[r.FilePath] = true
			continue
		}

		expected := chunksPerFile[r.FilePath]
		if expected <= 1 {
			// Single-chunk file — write immediately
			if err := p.Writer.WritePass1Result(ctx, r.Result); err != nil {
				p.Logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
				errorCount++
				continue
			}
			if hash, ok := hashByPath[r.FilePath]; ok {
				if err := p.Cache.MarkProcessed(r.FilePath, hash); err != nil {
					p.Logger.Error("failed to update cache", zap.String("file", r.FilePath), zap.Error(err))
				}
			}
			successCount++
		} else {
			// Multi-chunk file — accumulate and write when all chunks arrive
			pendingChunks[r.FilePath] = append(pendingChunks[r.FilePath], r.Result)

			if len(pendingChunks[r.FilePath]) == expected && !fileHasError[r.FilePath] {
				merged := graph.MergePass1Results(pendingChunks[r.FilePath])
				if err := p.Writer.WritePass1Result(ctx, merged); err != nil {
					p.Logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
					errorCount++
				} else {
					if hash, ok := hashByPath[r.FilePath]; ok {
						if err := p.Cache.MarkProcessed(r.FilePath, hash); err != nil {
							p.Logger.Error("failed to update cache", zap.String("file", r.FilePath), zap.Error(err))
						}
					}
					successCount++
				}
				delete(pendingChunks, r.FilePath)
			}
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

	hashByPath := make(map[string]string, len(changed))
	for _, f := range changed {
		hashByPath[f.Path] = f.Hash
	}

	chunkOpts := chunker.Pass2ChunkOptions{
		TokenLimit:    p.Config.Ingest.Pass2TokenLimit,
		OverlapLines:  p.Config.Ingest.OverlapLines,
		CopybookIndex: copybookIndex,
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
	resultsCh := pool.RunPass2(ctx, allChunks, pass2Fn, pass2Workers, p.Logger)

	// Stream results to Neo4j, merging multi-chunk files as they complete
	pendingChunks := make(map[string][]*graph.Pass2Result)
	fileHasError := make(map[string]bool)
	successCount := 0
	errorCount := 0

	for cr := range resultsCh {
		if cr.Err != nil {
			errorCount++
			fileHasError[cr.FileName] = true
			continue
		}

		expected := chunksPerFile[cr.FileName]
		if expected <= 1 {
			// Single-chunk file — write immediately
			if err := p.Writer.WritePass2Result(ctx, cr.Result); err != nil {
				p.Logger.Error("pass 2: failed to write to neo4j", zap.String("file", cr.FileName), zap.Error(err))
				errorCount++
				continue
			}
			if hash, ok := hashByPath[cr.FileName]; ok {
				if err := p.Cache.MarkProcessedForPass(cr.FileName, hash, 2); err != nil {
					p.Logger.Error("pass 2: failed to update cache", zap.String("file", cr.FileName), zap.Error(err))
				}
			}
			successCount++
		} else {
			// Multi-chunk file — accumulate and write when all chunks arrive
			pendingChunks[cr.FileName] = append(pendingChunks[cr.FileName], cr.Result)

			if len(pendingChunks[cr.FileName]) == expected && !fileHasError[cr.FileName] {
				merged := graph.MergePass2Results(pendingChunks[cr.FileName])
				if err := p.Writer.WritePass2Result(ctx, merged); err != nil {
					p.Logger.Error("pass 2: failed to write to neo4j", zap.String("file", cr.FileName), zap.Error(err))
					errorCount++
				} else {
					if hash, ok := hashByPath[cr.FileName]; ok {
						if err := p.Cache.MarkProcessedForPass(cr.FileName, hash, 2); err != nil {
							p.Logger.Error("pass 2: failed to update cache", zap.String("file", cr.FileName), zap.Error(err))
						}
					}
					successCount++
				}
				delete(pendingChunks, cr.FileName)
			}
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

		if err := p.processPass3Batch(ctx, slices, orphans, hubs); err != nil {
			p.Logger.Error("pass 3: batch failed, retrying halves",
				zap.Int("offset", offset), zap.Int("size", len(slices)), zap.Error(err))

			half := len(slices) / 2
			if half > 0 {
				if err := p.processPass3Batch(ctx, slices[:half], orphans, hubs); err != nil {
					p.Logger.Error("pass 3: first half retry failed", zap.Error(err))
					for _, s := range slices[:half] {
						failedPrograms = append(failedPrograms, s.ProgramID)
					}
				} else {
					totalBatches++
				}
				if err := p.processPass3Batch(ctx, slices[half:], orphans, hubs); err != nil {
					p.Logger.Error("pass 3: second half retry failed", zap.Error(err))
					for _, s := range slices[half:] {
						failedPrograms = append(failedPrograms, s.ProgramID)
					}
				} else {
					totalBatches++
				}
			} else {
				// Single program batch still failed
				for _, s := range slices {
					failedPrograms = append(failedPrograms, s.ProgramID)
				}
			}
		} else {
			totalBatches++
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

// RunPass1JCL analyzes JCL files using Sonnet (cheap/fast).
func (p *Pipeline) RunPass1JCL(ctx context.Context, scanResult *scanner.ScanResult) error {
	var jclFiles []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeJCL {
			continue
		}
		isChanged, err := p.Cache.IsChanged(f.Path, f.Hash)
		if err != nil {
			return fmt.Errorf("checking JCL cache: %w", err)
		}
		if isChanged {
			jclFiles = append(jclFiles, f)
		}
	}

	if len(jclFiles) == 0 {
		p.Logger.Info("pass 1 JCL: no changed JCL files to process")
		return nil
	}
	p.Logger.Info("pass 1 JCL: files to process", zap.Int("count", len(jclFiles)))

	successCount := 0
	errorCount := 0

	for _, f := range jclFiles {
		data, err := os.ReadFile(f.Path)
		content := string(data)
		if err != nil {
			p.Logger.Error("pass 1 JCL: failed to read file", zap.String("file", f.Path), zap.Error(err))
			errorCount++
			continue
		}

		jsonResp, err := p.Claude.AnalyzeJCL(ctx, f.Path, content)
		if err != nil {
			p.Logger.Error("pass 1 JCL: claude analysis failed", zap.String("file", f.Path), zap.Error(err))
			errorCount++
			continue
		}

		result, err := parser.ParseJCLResponse(jsonResp, f.Path)
		if err != nil {
			p.Logger.Error("pass 1 JCL: parse failed", zap.String("file", f.Path), zap.Error(err))
			errorCount++
			continue
		}

		if err := p.Writer.WriteJCLResult(ctx, result); err != nil {
			p.Logger.Error("pass 1 JCL: write failed", zap.String("file", f.Path), zap.Error(err))
			errorCount++
			continue
		}

		if err := p.Cache.MarkProcessed(f.Path, f.Hash); err != nil {
			p.Logger.Error("pass 1 JCL: cache update failed", zap.String("file", f.Path), zap.Error(err))
		}
		successCount++
	}

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
	successCount := 0
	errorCount := 0

	for _, pair := range callPairs {
		fieldContext := formatFieldContext(pair)

		jsonResp, err := p.Claude.AnalyzeCrossProgramFlow(ctx, pair.CallerID, pair.CalleeID, fieldContext)
		if err != nil {
			p.Logger.Warn("pass 4: LINKAGE analysis failed",
				zap.String("caller", pair.CallerID),
				zap.String("callee", pair.CalleeID),
				zap.Error(err))
			errorCount++
			continue
		}

		fields, err := parser.ParsePass4Response(jsonResp)
		if err != nil {
			p.Logger.Warn("pass 4: parse failed",
				zap.String("caller", pair.CallerID),
				zap.String("callee", pair.CalleeID),
				zap.Error(err))
			errorCount++
			continue
		}

		if len(fields) > 0 {
			pass4Result.Flows = append(pass4Result.Flows, graph.CrossProgramFlow{
				FromProgram: pair.CallerID,
				ToProgram:   pair.CalleeID,
				Channel:     "LINKAGE",
				Fields:      fields,
			})
		}
		successCount++
	}

	if len(pass4Result.Flows) > 0 {
		if err := p.Writer.WritePass4Result(ctx, pass4Result); err != nil {
			return fmt.Errorf("writing pass 4 results: %w", err)
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

// processPass3Batch runs Claude analysis and writes results for a batch of program slices.
func (p *Pipeline) processPass3Batch(ctx context.Context, slices []n4j.ProgramSlice, orphans, hubs []string) error {
	graphText := n4j.FormatGraphSlice(slices, orphans, hubs)

	jsonResp, err := p.Claude.AnalyzeCrossCutting(ctx, graphText)
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
