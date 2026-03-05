package main

import (
	"fmt"
	"os"

	"cobol-ingestor/internal/config"

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

	// Override root dir from flag
	cfg.Ingest.RootDir = dir

	logger.Info("starting ingestion",
		zap.String("dir", cfg.Ingest.RootDir),
		zap.Int("max_workers", cfg.Claude.MaxWorkers),
	)

	// TODO: Wire up scanner -> chunker -> worker pool -> neo4j pipeline
	logger.Info("ingestion pipeline not yet implemented")

	return nil
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
