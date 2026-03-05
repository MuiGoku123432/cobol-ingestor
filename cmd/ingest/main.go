package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
	"cobol-ingestor/internal/pool"
	"cobol-ingestor/internal/scanner"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var rootCmd = &cobra.Command{
	Use:   "cobol-graph",
	Short: "COBOL Graph Ingestor — parse COBOL codebases into Neo4j",
}

var ingestCmd = &cobra.Command{
	Use:   "ingest",
	Short: "Ingest COBOL source files and build the graph",
	RunE:  runIngest,
}

var (
	dir      string
	passFlag int
)

func init() {
	ingestCmd.Flags().StringVar(&dir, "dir", "", "Root directory of COBOL source files")
	ingestCmd.Flags().IntVar(&passFlag, "pass", 0, "Which pass to run: 0=both, 1=Pass 1 only, 2=Pass 2 only")
	_ = ingestCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(ingestCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cfg.Ingest.RootDir = dir
	ctx := context.Background()

	runPass1 := passFlag == 0 || passFlag == 1
	runPass2 := passFlag == 0 || passFlag == 2

	logger.Info("starting ingestion",
		zap.String("dir", cfg.Ingest.RootDir),
		zap.Int("max_workers", cfg.Claude.MaxWorkers),
		zap.Bool("pass1", runPass1),
		zap.Bool("pass2", runPass2),
	)

	// 1. Scan filesystem
	scanResult, err := scanner.Scan(ctx, cfg.Ingest.RootDir, logger)
	if err != nil {
		return fmt.Errorf("scanning: %w", err)
	}

	cobolCount := 0
	copyCount := 0
	for _, f := range scanResult.Files {
		switch f.Type {
		case graph.FileTypeCOBOL:
			cobolCount++
		case graph.FileTypeCopybook:
			copyCount++
		}
	}
	logger.Info("scan results",
		zap.Int("total_files", len(scanResult.Files)),
		zap.Int("cobol", cobolCount),
		zap.Int("copybooks", copyCount),
	)

	// 2. Open cache
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// 3. Connect to Neo4j + run migrations
	neo4jClient, err := n4j.NewClient(ctx, cfg.Neo4j, logger)
	if err != nil {
		return fmt.Errorf("connecting to neo4j: %w", err)
	}
	defer neo4jClient.Close(ctx)

	if err := neo4jClient.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("neo4j connectivity check: %w", err)
	}

	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	// 4. Create LLM provider + Claude client
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	claudeClient, err := claude.NewClient(provider, cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, logger)

	// ========== PASS 1 ==========
	if runPass1 {
		if err := executePass1(ctx, cfg, scanResult, fileCache, claudeClient, writer, logger); err != nil {
			return fmt.Errorf("pass 1: %w", err)
		}
	}

	// ========== PASS 2 ==========
	if runPass2 {
		if err := executePass2(ctx, cfg, scanResult, fileCache, claudeClient, neo4jClient, writer, logger); err != nil {
			return fmt.Errorf("pass 2: %w", err)
		}
	}

	return nil
}

func executePass1(ctx context.Context, cfg *config.Config, scanResult *scanner.ScanResult, fileCache *cache.Cache, claudeClient *claude.Client, writer *n4j.BatchWriter, logger *zap.Logger) error {
	var changed []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		isChanged, err := fileCache.IsChanged(f.Path, f.Hash)
		if err != nil {
			return fmt.Errorf("checking cache: %w", err)
		}
		if isChanged {
			changed = append(changed, f)
		}
	}

	if len(changed) == 0 {
		logger.Info("pass 1: no changed files to process")
		return nil
	}
	logger.Info("pass 1: files to process", zap.Int("changed", len(changed)))

	// Chunk files
	var chunks []chunker.Chunk
	for _, f := range changed {
		fileChunks, err := chunker.ChunkFile(f, cfg.Ingest.TokenLimit, logger)
		if err != nil {
			logger.Error("chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		chunks = append(chunks, fileChunks...)
	}

	// Define process function
	processFn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error) {
		jsonResp, err := claudeClient.AnalyzeStructural(ctx, chunk.FileName, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("claude analysis: %w", err)
		}
		return parser.ParsePass1Response(jsonResp, chunk.FileName)
	}

	// Run worker pool
	results := pool.RunPass1(ctx, chunks, processFn, cfg.Claude.MaxWorkers, logger)

	// Write results to Neo4j and update cache
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
			continue
		}
		if err := writer.WritePass1Result(ctx, r.Result); err != nil {
			logger.Error("failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
			errorCount++
			continue
		}
		for _, f := range changed {
			if f.Path == r.FilePath {
				if err := fileCache.MarkProcessed(f.Path, f.Hash); err != nil {
					logger.Error("failed to update cache", zap.String("file", f.Path), zap.Error(err))
				}
				break
			}
		}
		successCount++
	}

	logger.Info("pass 1 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("total", len(results)),
	)

	return nil
}

func executePass2(ctx context.Context, cfg *config.Config, scanResult *scanner.ScanResult, fileCache *cache.Cache, claudeClient *claude.Client, neo4jClient *n4j.Client, writer *n4j.BatchWriter, logger *zap.Logger) error {
	// Build copybook index from scan results
	copybookIndex := chunker.BuildCopybookIndex(scanResult.Files)
	logger.Info("pass 2: built copybook index", zap.Int("copybooks", len(copybookIndex)))

	// Filter COBOL files for Pass 2 via cache
	var changed []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue
		}
		isChanged, err := fileCache.IsChangedForPass(f.Path, f.Hash, 2)
		if err != nil {
			return fmt.Errorf("checking pass 2 cache: %w", err)
		}
		if isChanged {
			changed = append(changed, f)
		}
	}

	if len(changed) == 0 {
		logger.Info("pass 2: no changed files to process")
		return nil
	}
	logger.Info("pass 2: files to process", zap.Int("changed", len(changed)))

	// Chunk files with copybook inlining + division splitting
	chunkOpts := chunker.Pass2ChunkOptions{
		TokenLimit:    cfg.Ingest.Pass2TokenLimit,
		OverlapLines:  cfg.Ingest.OverlapLines,
		CopybookIndex: copybookIndex,
	}

	var allChunks []chunker.Chunk
	// Map file path to programID (from Pass 1 results in Neo4j)
	fileProgramIDs := make(map[string]string)

	for _, f := range changed {
		fileChunks, err := chunker.ChunkFilePass2(f, chunkOpts, logger)
		if err != nil {
			logger.Error("pass 2: chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		allChunks = append(allChunks, fileChunks...)
	}

	// Query programID for each file from Neo4j
	for _, f := range changed {
		session := neo4jClient.NewSession(ctx)
		result, err := session.Run(ctx,
			"MATCH (p:Program {filePath: $path}) RETURN p.programId AS pid LIMIT 1",
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

	// Build context preambles per program
	contextPreambles := make(map[string]string)
	for filePath, programID := range fileProgramIDs {
		pc, err := neo4jClient.QueryProgramContext(ctx, programID)
		if err != nil {
			logger.Warn("pass 2: failed to query context",
				zap.String("program", programID),
				zap.Error(err),
			)
			continue
		}
		contextPreambles[filePath] = n4j.FormatContextPreamble(pc)
	}

	// Define Pass 2 process function
	pass2Fn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass2Result, error) {
		preamble := contextPreambles[chunk.FileName]
		jsonResp, err := claudeClient.AnalyzeDeep(ctx, chunk, preamble)
		if err != nil {
			return nil, fmt.Errorf("claude deep analysis: %w", err)
		}
		programID := fileProgramIDs[chunk.FileName]
		if programID == "" {
			programID = "UNKNOWN"
		}
		return parser.ParsePass2Response(jsonResp, chunk.FileName, programID)
	}

	// Run Pass 2 worker pool
	pass2Workers := cfg.Ingest.Pass2Workers
	if pass2Workers <= 0 {
		pass2Workers = 3
	}
	results := pool.RunPass2(ctx, allChunks, pass2Fn, pass2Workers, logger)

	// Write results to Neo4j and update cache
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Err != nil {
			errorCount++
			continue
		}
		if err := writer.WritePass2Result(ctx, r.Result); err != nil {
			logger.Error("pass 2: failed to write to neo4j", zap.String("file", r.FilePath), zap.Error(err))
			errorCount++
			continue
		}
		for _, f := range changed {
			if f.Path == r.FilePath {
				if err := fileCache.MarkProcessedForPass(f.Path, f.Hash, 2); err != nil {
					logger.Error("pass 2: failed to update cache", zap.String("file", f.Path), zap.Error(err))
				}
				break
			}
		}
		successCount++
	}

	logger.Info("pass 2 complete",
		zap.Int("success", successCount),
		zap.Int("errors", errorCount),
		zap.Int("total", len(results)),
	)

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
