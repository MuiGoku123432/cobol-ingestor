package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/pipeline"
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

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage GitHub Copilot authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with GitHub Copilot via device flow",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := auth.RunDeviceFlow(cmd.Context())
		if err != nil {
			return fmt.Errorf("device flow: %w", err)
		}
		if err := auth.SaveToken(token); err != nil {
			return fmt.Errorf("saving token: %w", err)
		}
		fmt.Println("Authentication successful! Token saved to", auth.TokenFilePath())
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove cached Copilot token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := auth.DeleteToken(); err != nil {
			return fmt.Errorf("deleting token: %w", err)
		}
		fmt.Println("Cached token removed.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show whether a cached Copilot token exists",
	RunE: func(cmd *cobra.Command, args []string) error {
		st, err := auth.LoadToken()
		if err != nil {
			return fmt.Errorf("reading token: %w", err)
		}
		if st == nil {
			fmt.Println("No cached token found. Run 'cobol-graph auth login' to authenticate.")
			return nil
		}
		fmt.Printf("Cached token found (obtained %s)\n", st.ObtainedAt.Format("2006-01-02 15:04:05"))
		fmt.Println("File:", auth.TokenFilePath())
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVar(&dir, "dir", "", "Root directory of COBOL source files")
	ingestCmd.Flags().IntVar(&passFlag, "pass", 0, "Which pass to run: 0=all, 1=Pass 1, 2=Pass 2, 3=Pass 3")
	_ = ingestCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(ingestCmd)

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd)
	rootCmd.AddCommand(authCmd)
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
		zap.Int("pass", passFlag),
	)

	// Scan filesystem
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

	// Open cache
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// Connect to Neo4j + run migrations
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

	// Resolve Copilot token: env var → cached file → interactive device flow
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, err := auth.LoadToken(); err != nil {
			logger.Warn("failed to load cached copilot token", zap.Error(err))
		} else if st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
			logger.Info("using cached copilot token", zap.Time("obtained_at", st.ObtainedAt))
		}

		if cfg.LLM.CopilotGitHubToken == "" {
			logger.Info("no copilot token found, starting device flow")
			token, err := auth.RunDeviceFlow(ctx)
			if err != nil {
				return fmt.Errorf("copilot device flow: %w", err)
			}
			if err := auth.SaveToken(token); err != nil {
				logger.Warn("failed to save copilot token", zap.Error(err))
			}
			cfg.LLM.CopilotGitHubToken = token
		}
	}

	// Create LLM provider + Claude client
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

	pipe := &pipeline.Pipeline{
		Config:      cfg,
		Claude:      claudeClient,
		Neo4jClient: neo4jClient,
		Writer:      writer,
		Cache:       fileCache,
		Logger:      logger,
	}

	return pipe.Run(ctx, scanResult, passFlag)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
