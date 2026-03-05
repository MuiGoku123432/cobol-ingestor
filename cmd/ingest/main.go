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

var dir string

func init() {
	ingestCmd.Flags().StringVar(&dir, "dir", "", "Root directory of COBOL source files")
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

	logger.Info("starting ingestion",
		zap.String("dir", cfg.Ingest.RootDir),
		zap.Int("max_workers", cfg.Claude.MaxWorkers),
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

	// 2. Filter to changed files via cache
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	var changed []graph.FileInfo
	for _, f := range scanResult.Files {
		if f.Type != graph.FileTypeCOBOL {
			continue // Pass 1 only processes COBOL programs
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
		logger.Info("no changed files to process")
		return nil
	}
	logger.Info("files to process", zap.Int("changed", len(changed)))

	// 3. Chunk files
	var chunks []chunker.Chunk
	for _, f := range changed {
		fileChunks, err := chunker.ChunkFile(f, cfg.Ingest.TokenLimit, logger)
		if err != nil {
			logger.Error("chunking failed", zap.String("file", f.Path), zap.Error(err))
			continue
		}
		chunks = append(chunks, fileChunks...)
	}

	// 4. Connect to Neo4j + run migrations
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

	// 5. Create Claude client
	claudeClient, err := claude.NewClient(cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	// 6. Define process function
	processFn := func(ctx context.Context, chunk chunker.Chunk) (*graph.Pass1Result, error) {
		jsonResp, err := claudeClient.AnalyzeStructural(ctx, chunk.FileName, chunk.Content)
		if err != nil {
			return nil, fmt.Errorf("claude analysis: %w", err)
		}
		return parser.ParsePass1Response(jsonResp, chunk.FileName)
	}

	// 7. Run worker pool
	results := pool.RunPass1(ctx, chunks, processFn, cfg.Claude.MaxWorkers, logger)

	// 8. Write results to Neo4j and update cache
	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, logger)

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
		// Mark as processed only after successful Neo4j write
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

	logger.Info("ingestion complete",
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
