package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/estimate"
	"cobol-ingestor/internal/extdb"
	"cobol-ingestor/internal/glossary"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	cobolmcp "cobol-ingestor/internal/mcp"
	"cobol-ingestor/internal/modernize"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
	"cobol-ingestor/internal/pipeline"
	"cobol-ingestor/internal/scanner"
	"cobol-ingestor/internal/targetstack"
	"cobol-ingestor/internal/tokencount"

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
	dir              string
	passFlag         int
	codebaseFlag     string
	contentDetect    bool
	allExtensions    bool
	estimateFlag     bool
	preciseFlag      bool
)

// bw flags
var (
	bwDir          string
	bwExtensions   string
	bwMaxWorkers   int
	bwEstimateFlag bool
	bwPreciseFlag  bool
)

var bwCmd = &cobra.Command{
	Use:   "bw",
	Short: "Ingest Businessware files (Java, docs, config) and extract entities into the graph",
	RunE:  runBW,
}

// external-db flags
var (
	extDBMCPCmd    string
	extDBMCPURL    string
	extGraphMCPBin string
	extGraphMCPURL string
	extDBName      string
	extDBType      string

	// Oracle flags
	oracleHost      string
	oraclePort      string
	oracleService   string
	oracleUser      string
	oraclePassword  string
	oracleWallet    string
	oracleTNSAdmin  string
	oracleSQLclPath string
)

var extDBCmd = &cobra.Command{
	Use:   "external-db",
	Short: "Connect to an external database via MCP and map to COBOL DB2 tables",
	RunE:  runExternalDB,
}

// glossary flags
var (
	glossaryFile     string
	glossaryCodebase string
)

var glossaryCmd = &cobra.Command{
	Use:   "glossary",
	Short: "Ingest a company glossary HTML file and store terms in Neo4j",
	RunE:  runGlossary,
}

// target-stack flags
var (
	tsRepos        []string
	tsDirs         []string // local directory paths (skip clone step)
	tsBranch       string
	tsToken        string
	tsProvider     string
	tsPhase        string
	tsShallow      bool
	tsEstimateFlag bool
	tsPreciseFlag  bool
)

var tsCmd = &cobra.Command{
	Use:   "target-stack",
	Short: "Connect GitHub/Azure DevOps repos as the modern target stack and analyze business logic gaps",
	RunE:  runTargetStack,
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage GitHub Copilot authentication",
}

// cache flags
var (
	cacheCodebase           string
	cacheDBPath             string
	cacheKeepClassifications bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the ingestion cache (SQLite)",
}

var cacheClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear ingestion cache tables so the next run re-processes every file",
	Long: `Clears file_cache, pass_cache, and chunk_cache from the SQLite cache DB,
forcing the next ingestion to re-run Pass 1 / Pass 2 / chunked LLM calls from scratch.

Useful when the Neo4j database has been reset but the cache still says every file is
already processed. Use --keep-classifications to preserve the classify_cache table so
the file-type classifier stays a cache hit (recommended — classification is the most
expensive cached step that doesn't depend on Neo4j state).`,
	RunE: runCacheClear,
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

var authModelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List available Copilot models and show opus/sonnet selection",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		cfg.LLM.Provider = "copilot"

		// Resolve token
		if cfg.LLM.CopilotGitHubToken == "" {
			if st, loadErr := auth.LoadToken(); loadErr == nil && st != nil {
				cfg.LLM.CopilotGitHubToken = st.GitHubToken
			}
		}
		if cfg.LLM.CopilotGitHubToken == "" {
			fmt.Println("No Copilot token found. Run 'cobol-graph auth login' first.")
			return nil
		}

		provider, err := llm.NewCopilotProvider(cfg)
		if err != nil {
			return fmt.Errorf("creating copilot provider: %w", err)
		}
		defer provider.Close()

		ctx := cmd.Context()
		models, err := provider.GetCopilotProvider().GetModels(ctx)
		if err != nil {
			return fmt.Errorf("fetching models: %w", err)
		}

		resolved, _ := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel)

		fmt.Printf("Available Copilot models (%d total):\n\n", len(models))
		for _, m := range models {
			marker := "  "
			if resolved != nil {
				if m.ID == resolved.OpusModel {
					marker = "→ "
				} else if m.ID == resolved.SonnetModel {
					marker = "→ "
				}
			}
			fmt.Printf("%s%-40s %s\n", marker, m.ID, m.Name)
		}

		fmt.Println()
		if resolved != nil {
			fmt.Printf("Selected opus:   %s\n", resolved.OpusModel)
			fmt.Printf("Selected sonnet: %s\n", resolved.SonnetModel)
		}
		return nil
	},
}

func init() {
	ingestCmd.Flags().StringVar(&dir, "dir", "", "Root directory of COBOL source files")
	ingestCmd.Flags().IntVar(&passFlag, "pass", 0, "Which pass to run: 0=all, 1=Pass 1, 2=Pass 2, 3=Pass 3")
	ingestCmd.Flags().StringVar(&codebaseFlag, "codebase", "default", "Codebase identifier for multi-codebase support")
	ingestCmd.Flags().BoolVar(&contentDetect, "content-detect", false, "Enable content-based detection of COBOL/copybook/JCL in .txt files")
	ingestCmd.Flags().BoolVar(&allExtensions, "all-extensions", false, "Ingest every text-like file regardless of extension; unrecognized types → CUSTOM (use with --codebase for mixed-language legacy codebases)")
	ingestCmd.Flags().BoolVar(&estimateFlag, "estimate", false, "Estimate LLM token cost without making any API calls")
	ingestCmd.Flags().BoolVar(&preciseFlag, "precise", false, "Use BPE tokenizer for more accurate token counts (slower, requires internet on first use)")
	_ = ingestCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(ingestCmd)

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd, authModelsCmd)
	rootCmd.AddCommand(authCmd)

	cacheClearCmd.Flags().StringVar(&cacheCodebase, "codebase", "default", "Codebase identifier — resolves to cache-<codebase>.sqlite (or the default cache DB when 'default')")
	cacheClearCmd.Flags().StringVar(&cacheDBPath, "db", "", "Explicit cache DB path (overrides --codebase and config)")
	cacheClearCmd.Flags().BoolVar(&cacheKeepClassifications, "keep-classifications", false, "Preserve classify_cache so the LLM classifier pass stays a cache hit")
	cacheCmd.AddCommand(cacheClearCmd)
	rootCmd.AddCommand(cacheCmd)

	bwCmd.Flags().StringVar(&bwDir, "dir", "", "Root directory of Businessware files")
	bwCmd.Flags().StringVar(&bwExtensions, "extensions", "", "Comma-separated file extensions (default: .java,.md,.bw,.txt,.xml)")
	bwCmd.Flags().IntVar(&bwMaxWorkers, "max-workers", 0, "Max concurrent analysis workers (default: 5)")
	bwCmd.Flags().BoolVar(&bwEstimateFlag, "estimate", false, "Estimate LLM token cost without making any API calls")
	bwCmd.Flags().BoolVar(&bwPreciseFlag, "precise", false, "Use BPE tokenizer for more accurate token counts (slower, requires internet on first use)")
	_ = bwCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(bwCmd)

	extDBCmd.Flags().StringVar(&extDBMCPCmd, "db-mcp-cmd", "", "Shell command to start external DB MCP server (e.g. 'npx -y @modelcontextprotocol/server-postgres postgres://...')")
	extDBCmd.Flags().StringVar(&extDBMCPURL, "db-mcp-url", "", "HTTP endpoint for external DB MCP server")
	extDBCmd.Flags().StringVar(&extGraphMCPBin, "graph-mcp-bin", "", "Path to cobol-graph-mcp binary (default: config EXTDB_GRAPH_MCP_BIN)")
	extDBCmd.Flags().StringVar(&extGraphMCPURL, "graph-mcp-url", "", "HTTP endpoint for COBOL graph MCP server")
	extDBCmd.Flags().StringVar(&extDBName, "db-name", "", "Human-readable database name")
	extDBCmd.Flags().StringVar(&extDBType, "db-type", "", "Database type (oracle, postgres, mysql, etc.)")

	// Oracle SQLcl auto-launch flags
	extDBCmd.Flags().StringVar(&oracleHost, "oracle-host", "localhost", "Oracle database host")
	extDBCmd.Flags().StringVar(&oraclePort, "oracle-port", "1521", "Oracle database port")
	extDBCmd.Flags().StringVar(&oracleService, "oracle-service", "", "Oracle service name or SID (required for Oracle auto-launch)")
	extDBCmd.Flags().StringVar(&oracleUser, "oracle-user", "", "Oracle username (empty = wallet or /nolog)")
	extDBCmd.Flags().StringVar(&oraclePassword, "oracle-password", "", "Oracle password")
	extDBCmd.Flags().StringVar(&oracleWallet, "oracle-wallet", "", "Oracle wallet directory path")
	extDBCmd.Flags().StringVar(&oracleTNSAdmin, "oracle-tns-admin", "", "TNS_ADMIN directory (defaults to wallet path)")
	extDBCmd.Flags().StringVar(&oracleSQLclPath, "oracle-sqlcl-path", "", "Explicit path to SQLcl binary")

	rootCmd.AddCommand(extDBCmd)

	tsCmd.Flags().StringArrayVar(&tsRepos, "repo", nil, "Repository URL(s) — repeatable for multiple repos")
	tsCmd.Flags().StringArrayVar(&tsDirs, "dir", nil, "Local directory path(s) — repeatable; skips clone step")
	tsCmd.Flags().StringVar(&tsBranch, "branch", "main", "Branch to analyze")
	tsCmd.Flags().StringVar(&tsToken, "token", "", "PAT for GitHub or Azure DevOps (also via TS_TOKEN env)")
	tsCmd.Flags().StringVar(&tsProvider, "provider", "", "Git provider: github, azure_devops, generic (auto-detected if empty)")
	tsCmd.Flags().StringVar(&tsPhase, "phase", "all", "Phase to run: scan, analyze, gap, requirements, all")
	tsCmd.Flags().BoolVar(&tsShallow, "shallow", true, "Use shallow clone (depth=1)")
	tsCmd.Flags().BoolVar(&tsEstimateFlag, "estimate", false, "Estimate LLM token cost without making any API calls (clones repo but skips LLM/Neo4j)")
	tsCmd.Flags().BoolVar(&tsPreciseFlag, "precise", false, "Use BPE tokenizer for more accurate token counts (slower, requires internet on first use)")
	tsCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if len(tsRepos) == 0 && len(tsDirs) == 0 {
			return fmt.Errorf("at least one --repo or --dir flag is required")
		}
		return nil
	}
	rootCmd.AddCommand(tsCmd)

	glossaryCmd.Flags().StringVar(&glossaryFile, "file", "", "Path to the glossary HTML file")
	glossaryCmd.Flags().StringVar(&glossaryCodebase, "codebase", "global", "Codebase scope for the glossary — 'global' makes terms available across all codebases (default: global)")
	_ = glossaryCmd.MarkFlagRequired("file")
	rootCmd.AddCommand(glossaryCmd)
}

func runIngest(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	cfg.Ingest.RootDir = dir
	cfg.Ingest.Codebase = codebaseFlag
	if codebaseFlag != "default" {
		cfg.Ingest.CacheDB = fmt.Sprintf("cache-%s.sqlite", codebaseFlag)
	}
	ctx := context.Background()

	logger.Info("starting ingestion",
		zap.String("dir", cfg.Ingest.RootDir),
		zap.String("codebase", cfg.Ingest.Codebase),
		zap.Int("max_workers", cfg.Ingest.MaxWorkers),
		zap.Int("pass", passFlag),
	)

	// Scan filesystem
	detect := contentDetect || cfg.Ingest.ContentDetect
	if allExtensions && codebaseFlag == "default" {
		logger.Warn("--all-extensions is intended for custom codebases; consider using --codebase=<name> to avoid mixing with the default COBOL graph")
	}
	scanResult, err := scanner.Scan(ctx, cfg.Ingest.RootDir, logger, scanner.ScanOptions{
		ContentDetect: detect,
		AllExtensions: allExtensions,
	})
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

	// --estimate: print cost breakdown and exit without touching Neo4j or LLM
	if estimateFlag {
		est := estimate.New(cfg, scanResult, fileCache, logger)
		if preciseFlag {
			bpe, err := tokencount.NewBPECounter()
			if err != nil {
				return fmt.Errorf("--precise: failed to initialize BPE tokenizer: %w", err)
			}
			est.TokenCounter = bpe
		}
		result := est.Run()
		estimate.PrintTable(os.Stdout, result)
		return nil
	}
	if preciseFlag && !estimateFlag {
		return fmt.Errorf("--precise requires --estimate")
	}

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

	// Auto-discover Claude model IDs from Copilot's model catalogue
	if cfg.LLM.Provider == "copilot" {
		resolved, err := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel)
		if err != nil {
			logger.Warn("copilot model discovery failed, using config defaults", zap.Error(err))
		} else {
			cfg.Claude.OpusModel = resolved.OpusModel
			cfg.Claude.SonnetModel = resolved.SonnetModel
			logger.Info("resolved copilot models",
				zap.String("opus", resolved.OpusModel),
				zap.String("sonnet", resolved.SonnetModel),
				zap.Int("claude_models_found", len(resolved.AllModels)),
			)
		}
	}

	// Classify pending files (content-detect .txt files or --all-extensions unknowns)
	if (detect || allExtensions) && len(scanResult.Snippets) > 0 {
		classifyOpts := scanner.ClassifyOptions{MultiLang: allExtensions}
		if err := scanner.ClassifyPendingFiles(ctx, scanResult, provider, cfg.Claude.SonnetModel, logger, fileCache, classifyOpts); err != nil {
			logger.Warn("content classification had errors", zap.Error(err))
		}
	}

	claudeClient, err := claude.NewClient(provider, cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, cfg.Ingest.Codebase, logger)

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

func runBW(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override config with flags
	cfg.BW.Dir = bwDir
	if bwExtensions != "" {
		cfg.BW.Extensions = bwExtensions
	}
	if bwMaxWorkers > 0 {
		cfg.BW.MaxWorkers = bwMaxWorkers
	}
	if cfg.BW.MaxWorkers <= 0 {
		cfg.BW.MaxWorkers = 5
	}
	if cfg.BW.MaxTokens <= 0 {
		cfg.BW.MaxTokens = 16000
	}
	if cfg.BW.TokenLimit <= 0 {
		cfg.BW.TokenLimit = 30000
	}

	ctx := context.Background()

	// Parse extensions
	var extensions []string
	for _, ext := range strings.Split(cfg.BW.Extensions, ",") {
		ext = strings.TrimSpace(ext)
		if ext != "" {
			extensions = append(extensions, ext)
		}
	}

	logger.Info("starting BW ingestion",
		zap.String("dir", cfg.BW.Dir),
		zap.Strings("extensions", extensions),
		zap.Int("max_workers", cfg.BW.MaxWorkers),
	)

	// Scan BW files
	bwScanResult, err := scanner.ScanBW(ctx, cfg.BW.Dir, extensions, logger, cfg.BW.MaxJARDepth, cfg.BW.MaxWorkers)
	if err != nil {
		return fmt.Errorf("scanning BW files: %w", err)
	}
	scanResult := bwScanResult.ScanResult
	if len(scanResult.Files) == 0 {
		fmt.Println("No Businessware files found.")
		return nil
	}

	if bwEstimateFlag {
		// Open cache early (read-only, nil on error is fine — estimator handles nil).
		var fileCache *cache.Cache
		if c, cacheErr := cache.New(cfg.Ingest.CacheDB); cacheErr == nil {
			fileCache = c
			defer fileCache.Close()
		}
		est := estimate.NewBW(cfg, bwScanResult, fileCache, logger)
		if bwPreciseFlag {
			bpe, err := tokencount.NewBPECounter()
			if err != nil {
				return fmt.Errorf("--precise: failed to initialize BPE tokenizer: %w", err)
			}
			est.TokenCounter = bpe
		}
		result := est.Run()
		estimate.PrintBWTable(os.Stdout, result)
		return nil
	}
	if bwPreciseFlag && !bwEstimateFlag {
		return fmt.Errorf("--precise requires --estimate")
	}

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

	// Open cache (use pass 99 to avoid collision with main pipeline)
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// Resolve Copilot token if needed
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, loadErr := auth.LoadToken(); loadErr == nil && st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
		}
		if cfg.LLM.CopilotGitHubToken == "" {
			logger.Info("no copilot token found, starting device flow")
			token, devErr := auth.RunDeviceFlow(ctx)
			if devErr != nil {
				return fmt.Errorf("copilot device flow: %w", devErr)
			}
			_ = auth.SaveToken(token)
			cfg.LLM.CopilotGitHubToken = token
		}
	}

	// Create LLM provider + Claude client
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	if cfg.LLM.Provider == "copilot" {
		resolved, resolveErr := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel)
		if resolveErr != nil {
			logger.Warn("copilot model discovery failed", zap.Error(resolveErr))
		} else {
			cfg.Claude.OpusModel = resolved.OpusModel
			cfg.Claude.SonnetModel = resolved.SonnetModel
		}
	}

	claudeClient, err := claude.NewClient(provider, cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)

	// Query enriched COBOL graph context for BW prompts
	bwGraphCtx, bwCtxErr := neo4jClient.QueryBWGraphContext(ctx)
	if bwCtxErr != nil {
		logger.Warn("failed to query BW graph context, falling back to flat IDs", zap.Error(bwCtxErr))
		// Fallback: use flat program IDs
		existingPrograms := queryProgramIDs(ctx, neo4jClient, logger)
		bwGraphCtx = &n4j.BWGraphContext{RemainingPrograms: strings.Split(existingPrograms, ", ")}
	}
	interfaceProgs, domains, fileIOProgs, remaining := n4j.FormatBWGraphContext(bwGraphCtx, 8000)
	bwPromptCtx := claude.BWPromptContext{
		InterfacePrograms: interfaceProgs,
		BusinessDomains:   domains,
		FileIOPrograms:    fileIOProgs,
		RemainingPrograms: remaining,
	}
	bwPromptOverhead := computeBWPromptOverheadTiered(interfaceProgs, domains, fileIOProgs, remaining)

	// Process files with worker pool
	const bwPassNumber = 99

	// Batch cache check (change 5: single SQLite query instead of N)
	pathHashes := make(map[string]string, len(scanResult.Files))
	fileByPath := make(map[string]graph.FileInfo, len(scanResult.Files))
	for _, f := range scanResult.Files {
		pathHashes[f.Path] = f.Hash
		fileByPath[f.Path] = f
	}

	changedPaths, batchCacheErr := fileCache.BatchIsChangedForPass(pathHashes, bwPassNumber)
	if batchCacheErr != nil {
		logger.Warn("batch cache check failed, processing all files", zap.Error(batchCacheErr))
		changedPaths = make([]string, 0, len(scanResult.Files))
		for _, f := range scanResult.Files {
			changedPaths = append(changedPaths, f.Path)
		}
	}
	changedSet := make(map[string]bool, len(changedPaths))
	for _, p := range changedPaths {
		changedSet[p] = true
	}
	skipped := len(scanResult.Files) - len(changedPaths)

	// Build chunk work items — each chunk is an independent work item (change 2)
	type bwChunkItem struct {
		file        graph.FileInfo
		chunk       chunker.BWChunk
		fileType    string
		totalChunks int
	}

	var chunkItems []bwChunkItem
	for _, f := range scanResult.Files {
		if !changedSet[f.Path] {
			continue
		}

		var content []byte
		var readErr error
		if jarContent, ok := bwScanResult.JARContents[f.Path]; ok {
			content = jarContent
		} else {
			content, readErr = os.ReadFile(f.Path)
		}
		if readErr != nil {
			logger.Error("failed to read file", zap.String("file", f.Path), zap.Error(readErr))
			continue
		}

		chunks, chunkErr := chunker.ChunkBWFile(f.Path, content, cfg.BW.TokenLimit, bwPromptOverhead)
		if chunkErr != nil {
			logger.Error("failed to chunk file", zap.String("file", f.Path), zap.Error(chunkErr))
			continue
		}

		fileType := classifyBWExtension(f.Path)
		for _, chunk := range chunks {
			chunkItems = append(chunkItems, bwChunkItem{
				file:        f,
				chunk:       chunk,
				fileType:    fileType,
				totalChunks: len(chunks),
			})
		}
	}

	logger.Info("BW work plan",
		zap.Int("chunk_items", len(chunkItems)),
		zap.Int("skipped_cached", skipped),
		zap.Int("total_files", len(scanResult.Files)),
	)

	// Process chunks with bounded concurrency (change 2: each chunk is independent)
	sem := make(chan struct{}, cfg.BW.MaxWorkers)
	type bwChunkResult struct {
		filePath string
		fileType string
		file     graph.FileInfo
		index    int
		total    int
		entities []graph.BWEntity
		rels     []graph.BWRelationship
		refs     []graph.BWCobolReference
		summary  string
		err      error
	}
	resultCh := make(chan bwChunkResult, len(chunkItems))

	var bwChunksDone atomic.Int64
	totalChunkItems := len(chunkItems)

	for _, item := range chunkItems {
		sem <- struct{}{}
		go func(ci bwChunkItem) {
			defer func() { <-sem }()

			logger.Debug("BW chunk start",
				zap.String("file", ci.file.Path),
				zap.Int("chunk", ci.chunk.Index),
				zap.Int("of", ci.totalChunks),
			)

			resp, analyzeErr := claudeClient.AnalyzeBW(ctx, ci.file.Path, ci.fileType, ci.chunk.Content, bwPromptCtx, cfg.BW.MaxTokens)
			done := int(bwChunksDone.Add(1))
			if done%25 == 0 {
				logger.Info("BW progress", zap.Int("chunks_done", done), zap.Int("total", totalChunkItems))
			}
			if analyzeErr != nil {
				logger.Error("BW chunk analyze error",
					zap.String("file", ci.file.Path),
					zap.Int("chunk", ci.chunk.Index),
					zap.Error(analyzeErr),
				)
				resultCh <- bwChunkResult{filePath: ci.file.Path, file: ci.file, index: ci.chunk.Index, total: ci.totalChunks, err: fmt.Errorf("analyzing %s chunk %d: %w", ci.file.Path, ci.chunk.Index, analyzeErr)}
				return
			}

			parsed, parseErr := parser.ParseBWResponse(resp, ci.file.Path)
			if parseErr != nil {
				resultCh <- bwChunkResult{filePath: ci.file.Path, file: ci.file, index: ci.chunk.Index, total: ci.totalChunks, err: fmt.Errorf("parsing %s chunk %d: %w", ci.file.Path, ci.chunk.Index, parseErr)}
				return
			}

			resultCh <- bwChunkResult{
				filePath: ci.file.Path,
				fileType: ci.fileType,
				file:     ci.file,
				index:    ci.chunk.Index,
				total:    ci.totalChunks,
				entities: parsed.Entities,
				rels:     parsed.Relationships,
				refs:     parsed.CobolReferences,
				summary:  parsed.File.Summary,
			}
		}(item)
	}

	// Collect chunk results and merge per file
	fileChunks := make(map[string][]bwChunkResult)
	fileErrors := make(map[string]bool)
	for range len(chunkItems) {
		res := <-resultCh
		if res.err != nil {
			logger.Error("BW chunk processing failed", zap.String("file", res.filePath), zap.Error(res.err))
			fileErrors[res.filePath] = true
		}
		fileChunks[res.filePath] = append(fileChunks[res.filePath], res)
	}

	// Async Neo4j writer goroutine (change 4)
	type mergedBWResult struct {
		file   graph.FileInfo
		result *graph.BWResult
	}
	writeCh := make(chan mergedBWResult, len(fileChunks))
	var processedCount atomic.Int64
	var errorsCount atomic.Int64
	var totalEntitiesCount atomic.Int64
	var totalRefsCount atomic.Int64

	var writeWg sync.WaitGroup
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		for mr := range writeCh {
			if writeErr := writer.WriteBWResult(ctx, mr.result); writeErr != nil {
				logger.Error("failed to write BW result", zap.String("file", mr.file.Path), zap.Error(writeErr))
				errorsCount.Add(1)
				continue
			}
			if cacheErr := fileCache.MarkProcessedForPass(mr.file.Path, mr.file.Hash, bwPassNumber); cacheErr != nil {
				logger.Warn("failed to update cache", zap.String("file", mr.file.Path), zap.Error(cacheErr))
			}
			processedCount.Add(1)
			totalEntitiesCount.Add(int64(len(mr.result.Entities)))
			totalRefsCount.Add(int64(len(mr.result.CobolReferences)))
		}
	}()

	// Merge chunks per file and send to writer
	for filePath, chunks := range fileChunks {
		if fileErrors[filePath] {
			errorsCount.Add(1)
			continue
		}

		// Sort by chunk index for deterministic merge
		sort.Slice(chunks, func(i, j int) bool { return chunks[i].index < chunks[j].index })

		var allEntities []graph.BWEntity
		var allRels []graph.BWRelationship
		var allRefs []graph.BWCobolReference
		var summary string
		var fileInfo graph.FileInfo
		var fileType string

		for _, c := range chunks {
			allEntities = append(allEntities, c.entities...)
			allRels = append(allRels, c.rels...)
			allRefs = append(allRefs, c.refs...)
			if summary == "" {
				summary = c.summary
			}
			fileInfo = c.file
			fileType = c.fileType
		}

		writeCh <- mergedBWResult{
			file: fileInfo,
			result: &graph.BWResult{
				File: graph.BWFile{
					Path:     filePath,
					FileType: fileType,
					Summary:  summary,
				},
				Entities:        allEntities,
				Relationships:   allRels,
				CobolReferences: allRefs,
			},
		}
	}
	close(writeCh)
	writeWg.Wait()

	processed := int(processedCount.Load())
	errors := int(errorsCount.Load())
	totalEntities := int(totalEntitiesCount.Load())
	totalRefs := int(totalRefsCount.Load())

	fmt.Printf("\nBusinessware Pass 1 Complete\n")
	fmt.Printf("  Files processed: %d\n", processed)
	fmt.Printf("  Files skipped:   %d (cached)\n", skipped)
	fmt.Printf("  Errors:          %d\n", errors)
	fmt.Printf("  Entities:        %d extracted\n", totalEntities)
	fmt.Printf("  COBOL refs:      %d cross-links\n", totalRefs)

	// BW Pass 2: Cross-file synthesis
	if cfg.BW.EnablePass2 {
		pass2Refs, pass2Rels := runBWPass2(ctx, cfg, neo4jClient, claudeClient, writer, logger)
		fmt.Printf("\nBW Pass 2 (Cross-file Synthesis):\n")
		fmt.Printf("  New COBOL refs:      %d\n", pass2Refs)
		fmt.Printf("  Cross-file rels:     %d\n", pass2Rels)
	}

	// BW Pass 3: Validation & repair
	if cfg.BW.EnablePass3 {
		pass3Repairs := runBWPass3(ctx, cfg, neo4jClient, claudeClient, writer, logger)
		fmt.Printf("\nBW Pass 3 (Validation & Repair):\n")
		fmt.Printf("  Repairs applied:     %d\n", pass3Repairs)
	}

	fmt.Printf("\nResults persisted to Neo4j. Query with:\n")
	fmt.Printf("  MATCH (f:BWFile)-[:BW_CONTAINS]->(e:BWEntity) RETURN f.path, e.name, e.entityType LIMIT 20\n")

	return nil
}

// runBWPass2 performs cross-file synthesis on BW entities using Sonnet.
func runBWPass2(ctx context.Context, cfg *config.Config, neo4jClient *n4j.Client, claudeClient *claude.Client, writer *n4j.BatchWriter, logger *zap.Logger) (int, int) {
	logger.Info("BW Pass 2: starting cross-file synthesis")

	entities, err := neo4jClient.QueryBWEntitiesForSynthesis(ctx)
	if err != nil {
		logger.Warn("BW Pass 2: failed to query entities", zap.Error(err))
		return 0, 0
	}

	if len(entities) == 0 {
		logger.Info("BW Pass 2: no entities to synthesize")
		return 0, 0
	}

	// Group entities into batches by directory prefix
	batchSize := cfg.BW.Pass2BatchSize
	if batchSize <= 0 {
		batchSize = 30
	}

	// Get focused COBOL context
	cobolCtx, _ := neo4jClient.QueryCandidateProgramsForBWRepair(ctx)

	totalRefs := 0
	totalRels := 0

	for i := 0; i < len(entities); i += batchSize {
		end := i + batchSize
		if end > len(entities) {
			end = len(entities)
		}
		batch := entities[i:end]

		// Format entity summaries
		var entitiesCtx string
		for _, e := range batch {
			entitiesCtx += fmt.Sprintf("- **%s** (%s) [%s]\n  %s\n  MergeID: %s\n\n",
				e.Name, e.EntityType, e.SourceFile, e.Description, e.MergeID)
		}

		maxTokens := cfg.BW.Pass2MaxTokens
		if maxTokens <= 0 {
			maxTokens = 4000
		}

		jsonResp, err := claudeClient.AnalyzeBWSynthesis(ctx, entitiesCtx, cobolCtx, maxTokens)
		if err != nil {
			logger.Warn("BW Pass 2: synthesis batch failed", zap.Error(err))
			continue
		}

		parsed, err := parser.ParseBWPass2Response(jsonResp)
		if err != nil {
			logger.Warn("BW Pass 2: parse failed", zap.Error(err))
			continue
		}

		// Build write rows for new refs
		var newRefRows []map[string]any
		for _, ref := range parsed.NewCobolRefs {
			newRefRows = append(newRefRows, map[string]any{
				"mergeId":    ref.EntityMergeID,
				"target":     ref.TargetName,
				"targetType": ref.TargetType,
				"refType":    ref.ReferenceType,
				"desc":       ref.Description,
				"confidence": ref.Confidence,
			})
		}

		// Build write rows for cross-file relationships
		var crossRelRows []map[string]any
		for _, rel := range parsed.CrossFileRels {
			crossRelRows = append(crossRelRows, map[string]any{
				"fromMergeId":  rel.FromMergeID,
				"toMergeId":    rel.ToMergeID,
				"relationType": rel.RelationType,
				"description":  rel.Description,
			})
		}

		// Build cluster nodes
		var clusterRows []map[string]any
		for _, c := range parsed.Clusters {
			clusterRows = append(clusterRows, map[string]any{
				"name":        c.Name,
				"description": c.Description,
			})
		}

		if err := writer.WriteBWPass2Result(ctx, newRefRows, crossRelRows, clusterRows); err != nil {
			logger.Warn("BW Pass 2: write failed", zap.Error(err))
		}

		totalRefs += len(newRefRows)
		totalRels += len(crossRelRows)
	}

	logger.Info("BW Pass 2 complete",
		zap.Int("newRefs", totalRefs),
		zap.Int("crossRels", totalRels),
	)

	return totalRefs, totalRels
}

// runBWPass3 performs validation and repair on BW entities.
func runBWPass3(ctx context.Context, cfg *config.Config, neo4jClient *n4j.Client, claudeClient *claude.Client, writer *n4j.BatchWriter, logger *zap.Logger) int {
	logger.Info("BW Pass 3: starting validation & repair")

	totalRepairs := 0

	// Check 1: Unlinked services
	unlinked, err := neo4jClient.QueryUnlinkedBWServices(ctx)
	if err != nil {
		logger.Warn("BW Pass 3: failed to query unlinked services", zap.Error(err))
	} else if len(unlinked) > 0 {
		logger.Info("BW Pass 3: found unlinked services", zap.Int("count", len(unlinked)))

		var entitiesCtx string
		for _, s := range unlinked {
			entitiesCtx += fmt.Sprintf("- **%s** (%s) [%s]\n  %s\n  MergeID: %s\n\n",
				s.Name, s.EntityType, s.SourceFile, s.Description, s.MergeID)
		}

		candidates, _ := neo4jClient.QueryCandidateProgramsForBWRepair(ctx)

		maxTokens := cfg.BW.Pass3MaxTokens
		if maxTokens <= 0 {
			maxTokens = 4000
		}

		jsonResp, err := claudeClient.AnalyzeBWRepair(ctx, "unlinked_services", entitiesCtx, candidates, maxTokens)
		if err != nil {
			logger.Warn("BW Pass 3: unlinked services repair failed", zap.Error(err))
		} else {
			repairs, parseErr := parser.ParseBWPass3Response(jsonResp)
			if parseErr != nil {
				logger.Warn("BW Pass 3: unlinked services parse failed", zap.Error(parseErr))
			} else if len(repairs) > 0 {
				var repairRows []map[string]any
				for _, r := range repairs {
					repairRows = append(repairRows, map[string]any{
						"mergeId":    r.EntityMergeID,
						"target":     r.TargetName,
						"targetType": r.TargetType,
						"refType":    r.ReferenceType,
						"desc":       r.Description,
						"confidence": r.Confidence,
					})
				}
				if err := writer.WriteBWRepairResult(ctx, repairRows); err != nil {
					logger.Warn("BW Pass 3: unlinked services write failed", zap.Error(err))
				}
				totalRepairs += len(repairs)
			}
		}
	}

	// Check 2: Fuzzy name matches
	fuzzyMatches, err := neo4jClient.QueryBWFuzzyMatches(ctx)
	if err != nil {
		logger.Warn("BW Pass 3: failed to query fuzzy matches", zap.Error(err))
	} else if len(fuzzyMatches) > 0 {
		logger.Info("BW Pass 3: found fuzzy match candidates", zap.Int("count", len(fuzzyMatches)))

		var pairsCtx string
		for _, m := range fuzzyMatches {
			pairsCtx += fmt.Sprintf("- BWEntity: %s (mergeId: %s) ↔ Program: %s (reason: %s)\n",
				m.EntityName, m.EntityMergeID, m.ProgramID, m.MatchReason)
		}

		maxTokens := cfg.BW.Pass3MaxTokens
		if maxTokens <= 0 {
			maxTokens = 4000
		}

		jsonResp, err := claudeClient.AnalyzeBWRepair(ctx, "fuzzy_matches", pairsCtx, "", maxTokens)
		if err != nil {
			logger.Warn("BW Pass 3: fuzzy match repair failed", zap.Error(err))
		} else {
			repairs, parseErr := parser.ParseBWPass3Response(jsonResp)
			if parseErr != nil {
				logger.Warn("BW Pass 3: fuzzy match parse failed", zap.Error(parseErr))
			} else if len(repairs) > 0 {
				var repairRows []map[string]any
				for _, r := range repairs {
					repairRows = append(repairRows, map[string]any{
						"mergeId":    r.EntityMergeID,
						"target":     r.TargetName,
						"targetType": r.TargetType,
						"refType":    r.ReferenceType,
						"desc":       r.Description,
						"confidence": r.Confidence,
					})
				}
				if err := writer.WriteBWRepairResult(ctx, repairRows); err != nil {
					logger.Warn("BW Pass 3: fuzzy match write failed", zap.Error(err))
				}
				totalRepairs += len(repairs)
			}
		}
	}

	logger.Info("BW Pass 3 complete", zap.Int("totalRepairs", totalRepairs))
	return totalRepairs
}

// queryProgramIDs fetches all existing COBOL program IDs from Neo4j for BW prompt context.
// The result is capped to avoid unbounded token growth in the prompt.
func queryProgramIDs(ctx context.Context, client *n4j.Client, logger *zap.Logger) string {
	ids, err := client.QueryAllProgramIDs(ctx)
	if err != nil {
		logger.Warn("failed to query program IDs", zap.Error(err))
		return "(none found)"
	}
	if len(ids) == 0 {
		return "(none found)"
	}
	return capProgramIDs(ids, 4000)
}

// capProgramIDs joins IDs until the token estimate reaches maxTokens,
// then appends a summary of remaining IDs to prevent unbounded prompt growth.
func capProgramIDs(ids []string, maxTokens int) string {
	var sb strings.Builder
	tokens := 0
	for i, id := range ids {
		entry := id
		if i > 0 {
			entry = ", " + id
		}
		entryTokens := chunker.EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			remaining := len(ids) - i
			fmt.Fprintf(&sb, " ... and %d more programs", remaining)
			break
		}
		sb.WriteString(entry)
		tokens += entryTokens
	}
	return sb.String()
}

// computeBWPromptOverhead returns the estimated token overhead for BW prompts
// (system message + template + separator + prefill + margin + existingPrograms).
func computeBWPromptOverhead(existingPrograms string) int {
	return 2100 + chunker.EstimateTokens(existingPrograms)
}

// computeBWPromptOverheadTiered returns the estimated token overhead for BW prompts
// using tiered context (integration patterns section adds ~500 tokens over the flat version).
func computeBWPromptOverheadTiered(interfaceProgs, domains, fileIOProgs, remaining string) int {
	return 2600 + chunker.EstimateTokens(interfaceProgs) +
		chunker.EstimateTokens(domains) +
		chunker.EstimateTokens(fileIOProgs) +
		chunker.EstimateTokens(remaining)
}

// classifyBWExtension returns a human-readable file type from a path's extension.
func classifyBWExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".java":
		return "Java"
	case ".class":
		return "Java Bytecode"
	case ".md":
		return "Markdown"
	case ".xml":
		return "XML"
	case ".bw":
		return "Businessware"
	case ".txt":
		return "Text"
	case ".properties":
		return "Properties"
	case ".json":
		return "JSON"
	case ".yml", ".yaml":
		return "YAML"
	case ".mf":
		return "Manifest"
	case ".vsdx":
		return "Visio Diagram"
	case ".drawio":
		return "DrawIO Diagram"
	case ".svg":
		return "SVG Diagram"
	case ".puml", ".plantuml":
		return "PlantUML Diagram"
	default:
		return "Unknown"
	}
}

func runExternalDB(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override config with flags
	if extDBMCPCmd != "" {
		cfg.ExternalDB.DBMCPCommand = extDBMCPCmd
	}
	if extDBMCPURL != "" {
		cfg.ExternalDB.DBMCPServerURL = extDBMCPURL
	}
	if extGraphMCPBin != "" {
		cfg.ExternalDB.GraphMCPBin = extGraphMCPBin
	}
	if extGraphMCPURL != "" {
		cfg.ExternalDB.GraphMCPURL = extGraphMCPURL
	}
	if extDBName != "" {
		cfg.ExternalDB.DatabaseName = extDBName
	}
	if extDBType != "" {
		cfg.ExternalDB.DatabaseType = extDBType
	}

	// Apply Oracle flag overrides
	applyOracleFlags(cfg)

	// Validate required fields
	if cfg.ExternalDB.DBMCPCommand == "" && cfg.ExternalDB.DBMCPServerURL == "" && !hasOracleConfig(cfg) {
		return fmt.Errorf("specify --db-mcp-cmd, --db-mcp-url, or Oracle connection details (--oracle-service)")
	}
	if cfg.ExternalDB.DatabaseName == "" {
		return fmt.Errorf("--db-name (or EXTDB_DATABASE_NAME) is required")
	}
	if cfg.ExternalDB.DatabaseType == "" {
		return fmt.Errorf("--db-type (or EXTDB_DATABASE_TYPE) is required")
	}

	ctx := context.Background()

	// Resolve Copilot token if needed
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, err := auth.LoadToken(); err == nil && st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
		}
		if cfg.LLM.CopilotGitHubToken == "" {
			logger.Info("no copilot token found, starting device flow")
			token, err := auth.RunDeviceFlow(ctx)
			if err != nil {
				return fmt.Errorf("copilot device flow: %w", err)
			}
			_ = auth.SaveToken(token)
			cfg.LLM.CopilotGitHubToken = token
		}
	}

	// Create LLM provider
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	// Resolve model for chat
	chatProvider, ok := provider.(llm.ChatProvider)
	if !ok {
		return fmt.Errorf("LLM provider %q does not support chat completions", cfg.LLM.Provider)
	}

	model := cfg.Claude.OpusModel
	if cfg.LLM.Provider == "copilot" {
		resolved, err := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel)
		if err != nil {
			logger.Warn("copilot model discovery failed", zap.Error(err))
		} else {
			model = resolved.OpusModel
		}
	}

	logger.Info("starting external DB analysis",
		zap.String("db_name", cfg.ExternalDB.DatabaseName),
		zap.String("db_type", cfg.ExternalDB.DatabaseType),
		zap.String("model", model),
	)

	// Create MCP client for external DB
	var dbClient *modernize.MCPClient
	switch {
	case cfg.ExternalDB.DBMCPServerURL != "":
		dbClient, err = modernize.NewMCPClientHTTP(ctx, cfg.ExternalDB.DBMCPServerURL)
	case cfg.ExternalDB.DBMCPCommand != "":
		parts := strings.Fields(cfg.ExternalDB.DBMCPCommand)
		if len(parts) == 0 {
			return fmt.Errorf("empty db-mcp-cmd")
		}
		if len(parts) > 1 {
			dbClient, err = modernize.NewMCPClientCommand(ctx, exec.Command("sh", "-c", cfg.ExternalDB.DBMCPCommand), nil)
		} else {
			dbClient, err = modernize.NewMCPClient(ctx, parts[0], nil)
		}
	case hasOracleConfig(cfg):
		dbClient, err = extdb.NewOracleMCPClient(ctx, buildOracleConfig(cfg), logger)
	default:
		return fmt.Errorf("no external DB connection configured")
	}
	if err != nil {
		return fmt.Errorf("connecting to external DB MCP: %w", err)
	}
	defer dbClient.Close()

	// Create MCP client for COBOL graph
	var graphClient *modernize.MCPClient
	if cfg.ExternalDB.GraphMCPURL != "" {
		graphClient, err = modernize.NewMCPClientHTTP(ctx, cfg.ExternalDB.GraphMCPURL)
	} else {
		graphClient, err = modernize.NewMCPClient(ctx, cfg.ExternalDB.GraphMCPBin, nil)
	}
	if err != nil {
		return fmt.Errorf("connecting to COBOL graph MCP: %w", err)
	}
	defer graphClient.Close()

	// Create bridge and analyzer
	bridge := extdb.NewMCPBridge(graphClient, dbClient)
	analyzer := extdb.NewAnalyzer(
		bridge,
		chatProvider,
		model,
		cfg.ExternalDB.MaxIterations,
		cfg.ExternalDB.MaxTokens,
		cfg.ExternalDB.DatabaseName,
		cfg.ExternalDB.DatabaseType,
		logger,
	)

	// Run analysis
	result, err := analyzer.Run(ctx)
	if err != nil {
		return fmt.Errorf("external DB analysis: %w", err)
	}

	logger.Info("analysis complete",
		zap.Int("tables", len(result.Tables)),
		zap.Int("mappings", len(result.Mappings)),
		zap.Int("gaps", len(result.Gaps)),
		zap.Int("flows", len(result.Flows)),
	)

	// Write to Neo4j
	neo4jClient, err := n4j.NewClient(ctx, cfg.Neo4j, logger)
	if err != nil {
		return fmt.Errorf("connecting to neo4j: %w", err)
	}
	defer neo4jClient.Close(ctx)

	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)
	if err := writer.WriteExternalDBResult(ctx, result); err != nil {
		return fmt.Errorf("writing results to neo4j: %w", err)
	}

	fmt.Printf("\nExternal DB Gap Analysis Complete\n")
	fmt.Printf("  Database:  %s (%s)\n", result.Database.Name, result.Database.DatabaseType)
	fmt.Printf("  Tables:    %d discovered\n", len(result.Tables))
	fmt.Printf("  Mappings:  %d DB2→external mappings\n", len(result.Mappings))
	fmt.Printf("  Gaps:      %d identified\n", len(result.Gaps))
	fmt.Printf("  Flows:     %d data flow paths\n", len(result.Flows))
	fmt.Printf("\nResults persisted to Neo4j. Query with MCP tools: list_external_db_tables, get_gap_analysis\n")

	return nil
}

// hasOracleConfig returns true if Oracle auto-launch is configured.
func hasOracleConfig(cfg *config.Config) bool {
	return strings.EqualFold(cfg.ExternalDB.DatabaseType, "oracle") && cfg.ExternalDB.OracleService != ""
}

// applyOracleFlags overrides config Oracle fields with non-default flag values.
func applyOracleFlags(cfg *config.Config) {
	if oracleHost != "localhost" || cfg.ExternalDB.OracleHost == "" {
		if oracleHost != "" {
			cfg.ExternalDB.OracleHost = oracleHost
		}
	}
	if oraclePort != "1521" || cfg.ExternalDB.OraclePort == "" {
		if oraclePort != "" {
			cfg.ExternalDB.OraclePort = oraclePort
		}
	}
	if oracleService != "" {
		cfg.ExternalDB.OracleService = oracleService
	}
	if oracleUser != "" {
		cfg.ExternalDB.OracleUser = oracleUser
	}
	if oraclePassword != "" {
		cfg.ExternalDB.OraclePassword = oraclePassword
	}
	if oracleWallet != "" {
		cfg.ExternalDB.OracleWalletPath = oracleWallet
	}
	if oracleTNSAdmin != "" {
		cfg.ExternalDB.OracleTNSAdmin = oracleTNSAdmin
	}
	if oracleSQLclPath != "" {
		cfg.ExternalDB.OracleSQLclPath = oracleSQLclPath
	}
}

// buildOracleConfig maps ExternalDBConfig fields to an extdb.OracleConfig.
func buildOracleConfig(cfg *config.Config) extdb.OracleConfig {
	return extdb.OracleConfig{
		SQLclPath:  cfg.ExternalDB.OracleSQLclPath,
		Host:       cfg.ExternalDB.OracleHost,
		Port:       cfg.ExternalDB.OraclePort,
		Service:    cfg.ExternalDB.OracleService,
		User:       cfg.ExternalDB.OracleUser,
		Password:   cfg.ExternalDB.OraclePassword,
		WalletPath: cfg.ExternalDB.OracleWalletPath,
		TNSAdmin:   cfg.ExternalDB.OracleTNSAdmin,
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runCacheClear(cmd *cobra.Command, args []string) error {
	dbPath := cacheDBPath
	if dbPath == "" {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		dbPath = cfg.Ingest.CacheDB
		if cacheCodebase != "default" {
			dbPath = fmt.Sprintf("cache-%s.sqlite", cacheCodebase)
		}
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Cache DB %s does not exist — nothing to clear.\n", dbPath)
		return nil
	}

	c, err := cache.New(dbPath)
	if err != nil {
		return fmt.Errorf("opening cache %s: %w", dbPath, err)
	}
	defer c.Close()

	deleted, err := c.Clear(cacheKeepClassifications)
	if err != nil {
		return fmt.Errorf("clearing cache: %w", err)
	}

	fmt.Printf("Cleared cache at %s\n", dbPath)
	tables := []string{"file_cache", "pass_cache", "chunk_cache", "classify_cache"}
	for _, t := range tables {
		if n, ok := deleted[t]; ok {
			fmt.Printf("  %-16s %d rows deleted\n", t, n)
		} else {
			fmt.Printf("  %-16s preserved\n", t)
		}
	}
	return nil
}

// runTargetStack runs the full target stack pipeline: clone → scan → analyze → gap → requirements.
func runTargetStack(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Resolve PAT: flag > env > config
	token := tsToken
	if token == "" {
		token = cfg.TargetStack.Token
	}
	// Resolve clone base dir
	cloneDir := cfg.TargetStack.CloneDir
	if cloneDir == "" {
		cloneDir = filepath.Join(cfg.DataDir, "target-repos")
	}

	if tsEstimateFlag {
		// Parse extensions for scanning.
		var estExts []string
		for _, ext := range strings.Split(cfg.TargetStack.Extensions, ",") {
			ext = strings.TrimSpace(ext)
			if ext != "" {
				estExts = append(estExts, ext)
			}
		}
		// Open cache (nil on error is fine — estimator treats nil as "all changed").
		var estCache *cache.Cache
		if c, cacheErr := cache.New(cfg.Ingest.CacheDB); cacheErr == nil {
			estCache = c
			defer estCache.Close()
		}
		var scanResults []*targetstack.ScanResult
		for _, repoURL := range tsRepos {
			providerName := tsProvider
			if providerName == "" {
				providerName = targetstack.DetectProvider(repoURL)
			}
			repoCfg := targetstack.RepoConfig{
				URL:      repoURL,
				Branch:   tsBranch,
				Provider: providerName,
				Token:    token,
			}
			cloneResult, cloneErr := targetstack.CloneOrPull(repoCfg, cloneDir, tsShallow, logger)
			if cloneErr != nil {
				return fmt.Errorf("cloning %s for estimate: %w", repoURL, cloneErr)
			}
			scanResult, scanErr := targetstack.Scan(cloneResult.LocalPath, estExts, logger)
			if scanErr != nil {
				return fmt.Errorf("scanning %s for estimate: %w", repoURL, scanErr)
			}
			scanResult.RepoURL = repoURL
			scanResults = append(scanResults, scanResult)
		}
		for _, dirPath := range tsDirs {
			cloneResult, dirErr := targetstack.LocalDirResult(dirPath, logger)
			if dirErr != nil {
				return fmt.Errorf("resolving dir %s for estimate: %w", dirPath, dirErr)
			}
			scanResult, scanErr := targetstack.Scan(cloneResult.LocalPath, estExts, logger)
			if scanErr != nil {
				return fmt.Errorf("scanning %s for estimate: %w", dirPath, scanErr)
			}
			scanResult.RepoURL = cloneResult.Config.URL
			scanResults = append(scanResults, scanResult)
		}
		est := estimate.NewTS(cfg, scanResults, estCache, logger)
		if tsPreciseFlag {
			bpe, err := tokencount.NewBPECounter()
			if err != nil {
				return fmt.Errorf("--precise: failed to initialize BPE tokenizer: %w", err)
			}
			est.TokenCounter = bpe
		}
		result := est.Run()
		estimate.PrintTSTable(os.Stdout, result)
		return nil
	}
	if tsPreciseFlag && !tsEstimateFlag {
		return fmt.Errorf("--precise requires --estimate")
	}

	// Resolve LLM provider
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, loadErr := auth.LoadToken(); loadErr == nil && st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
		}
	}

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	chatProvider, ok := provider.(llm.ChatProvider)
	if !ok {
		return fmt.Errorf("LLM provider %s does not support chat completions", cfg.LLM.Provider)
	}

	model := cfg.Claude.SonnetModel

	// Connect to Neo4j
	neo4jClient, err := n4j.NewClient(ctx, cfg.Neo4j, logger)
	if err != nil {
		return fmt.Errorf("connecting to neo4j: %w", err)
	}
	defer neo4jClient.Close(ctx)

	if err := neo4jClient.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("neo4j connectivity: %w", err)
	}

	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)

	// Open cache
	fileCache, err := targetstack.NewCache(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// Create analyzer
	analyzer, err := targetstack.NewAnalyzer(
		chatProvider, model,
		cfg.TargetStack.MaxTokens,
		cfg.TargetStack.TokenLimit,
		cfg.TargetStack.Pass2Batch,
		cfg.TargetStack.Pass2MaxTokens,
		logger,
	)
	if err != nil {
		return fmt.Errorf("creating analyzer: %w", err)
	}

	// Parse extensions
	var extensions []string
	for _, ext := range strings.Split(cfg.TargetStack.Extensions, ",") {
		ext = strings.TrimSpace(ext)
		if ext != "" {
			extensions = append(extensions, ext)
		}
	}

	// Process each repo
	for _, repoURL := range tsRepos {
		providerName := tsProvider
		if providerName == "" {
			providerName = targetstack.DetectProvider(repoURL)
		}

		repoCfg := targetstack.RepoConfig{
			URL:      repoURL,
			Branch:   tsBranch,
			Provider: providerName,
			Token:    token,
		}

		logger.Info("processing repository", zap.String("url", repoURL), zap.String("phase", tsPhase))

		// Phase 1: Clone/Pull
		if tsPhase == "scan" || tsPhase == "analyze" || tsPhase == "all" {
			cloneResult, cloneErr := targetstack.CloneOrPull(repoCfg, cloneDir, tsShallow, logger)
			if cloneErr != nil {
				return fmt.Errorf("cloning %s: %w", repoURL, cloneErr)
			}

			// Check if HEAD changed
			prevSHA := targetstack.HeadSHAFromNeo4j(ctx, neo4jClient, repoURL)
			if prevSHA == cloneResult.HeadSHA && tsPhase != "all" {
				logger.Info("HEAD SHA unchanged, skipping analysis", zap.String("sha", cloneResult.HeadSHA))
				continue
			}

			// Phase 2: Scan
			scanResult, scanErr := targetstack.Scan(cloneResult.LocalPath, extensions, logger)
			if scanErr != nil {
				return fmt.Errorf("scanning %s: %w", repoURL, scanErr)
			}
			scanResult.RepoURL = repoURL
			scanResult.HeadSHA = cloneResult.HeadSHA

			logger.Info("scan complete",
				zap.Int("files", len(scanResult.Files)),
				zap.Any("by_lang", scanResult.ByLang),
			)

			if tsPhase == "scan" {
				fmt.Printf("Scan complete: %d files discovered in %s\n", len(scanResult.Files), repoURL)
				for lang, count := range scanResult.ByLang {
					fmt.Printf("  %s: %d files\n", lang, count)
				}
				continue
			}

			// Phase 3: Extract + Synthesize
			extractResults, analyzeErr := analyzer.AnalyzeFiles(ctx, scanResult.Files, repoURL, fileCache, cfg.TargetStack.MaxWorkers)
			if analyzeErr != nil {
				return fmt.Errorf("analyzing %s: %w", repoURL, analyzeErr)
			}

			repoName := targetstack.RepoName(repoURL)
			lang := detectPrimaryLangFromScan(scanResult)

			analysis, synthErr := analyzer.Synthesize(ctx, extractResults, repoName, lang, "")
			if synthErr != nil {
				return fmt.Errorf("synthesis %s: %w", repoURL, synthErr)
			}

			repo := targetstack.RepoFromCloneResult(cloneResult, scanResult, analysis, cloneDir)

			// Phase 4: Write to Neo4j
			if err := targetstack.WriteResult(ctx, writer, repo, analysis); err != nil {
				return fmt.Errorf("writing %s to neo4j: %w", repoURL, err)
			}

			logger.Info("repo analysis complete",
				zap.String("repo", repoURL),
				zap.Int("services", len(analysis.Services)),
				zap.Int("rules", len(analysis.Rules)),
				zap.Int("endpoints", len(analysis.Endpoints)),
			)
		}
	}

	// Process each local directory (--dir flag)
	for _, dirPath := range tsDirs {
		cloneResult, dirErr := targetstack.LocalDirResult(dirPath, logger)
		if dirErr != nil {
			return fmt.Errorf("resolving dir %s: %w", dirPath, dirErr)
		}

		fileURL := cloneResult.Config.URL
		logger.Info("processing local directory", zap.String("url", fileURL), zap.String("phase", tsPhase))

		if tsPhase == "scan" || tsPhase == "analyze" || tsPhase == "all" {
			prevSHA := targetstack.HeadSHAFromNeo4j(ctx, neo4jClient, fileURL)
			if prevSHA == cloneResult.HeadSHA && cloneResult.HeadSHA != "" && tsPhase != "all" {
				logger.Info("HEAD SHA unchanged, skipping analysis", zap.String("sha", cloneResult.HeadSHA))
				continue
			}

			scanResult, scanErr := targetstack.Scan(cloneResult.LocalPath, extensions, logger)
			if scanErr != nil {
				return fmt.Errorf("scanning %s: %w", dirPath, scanErr)
			}
			scanResult.RepoURL = fileURL
			scanResult.HeadSHA = cloneResult.HeadSHA

			logger.Info("scan complete",
				zap.Int("files", len(scanResult.Files)),
				zap.Any("by_lang", scanResult.ByLang),
			)

			if tsPhase == "scan" {
				fmt.Printf("Scan complete: %d files discovered in %s\n", len(scanResult.Files), dirPath)
				for lang, count := range scanResult.ByLang {
					fmt.Printf("  %s: %d files\n", lang, count)
				}
				continue
			}

			extractResults, analyzeErr := analyzer.AnalyzeFiles(ctx, scanResult.Files, fileURL, fileCache, cfg.TargetStack.MaxWorkers)
			if analyzeErr != nil {
				return fmt.Errorf("analyzing %s: %w", dirPath, analyzeErr)
			}

			repoName := targetstack.RepoName(fileURL)
			lang := detectPrimaryLangFromScan(scanResult)

			analysis, synthErr := analyzer.Synthesize(ctx, extractResults, repoName, lang, "")
			if synthErr != nil {
				return fmt.Errorf("synthesis %s: %w", dirPath, synthErr)
			}

			repo := targetstack.RepoFromCloneResult(cloneResult, scanResult, analysis, cloneDir)

			if err := targetstack.WriteResult(ctx, writer, repo, analysis); err != nil {
				return fmt.Errorf("writing %s to neo4j: %w", dirPath, err)
			}

			logger.Info("dir analysis complete",
				zap.String("repo", fileURL),
				zap.Int("services", len(analysis.Services)),
				zap.Int("rules", len(analysis.Rules)),
				zap.Int("endpoints", len(analysis.Endpoints)),
			)
		}
	}

	// Phase 5: Gap Analysis
	if tsPhase == "gap" || tsPhase == "all" {
		logger.Info("starting gap analysis swarm")

		mcpServer := cobolmcp.NewServer(neo4jClient, nil, nil)
		cobolMCPClient, mcpErr := modernize.NewMCPClientInProcess(ctx, mcpServer)
		if mcpErr != nil {
			return fmt.Errorf("creating COBOL MCP client: %w", mcpErr)
		}

		bridge := targetstack.NewGapBridge(cobolMCPClient, cobolMCPClient)

		swarmResult, gapErr := targetstack.RunGapSwarm(ctx, bridge, chatProvider, model, cfg.TargetStack.MaxTokens, logger)
		if gapErr != nil {
			return fmt.Errorf("gap analysis: %w", gapErr)
		}

		if len(swarmResult.Gaps) > 0 {
			if err := targetstack.WriteGaps(ctx, writer, swarmResult.Gaps); err != nil {
				return fmt.Errorf("writing gaps: %w", err)
			}
		}

		if len(swarmResult.Requirements) > 0 {
			if err := targetstack.WriteRequirements(ctx, writer, swarmResult.Requirements); err != nil {
				return fmt.Errorf("writing requirements: %w", err)
			}
		}

		logger.Info("gap analysis complete",
			zap.Int("gaps", len(swarmResult.Gaps)),
			zap.Int("requirements", len(swarmResult.Requirements)),
		)
	}

	// Phase 6: Requirements (standalone)
	if tsPhase == "requirements" {
		logger.Info("generating business requirements from existing gaps")

		mcpServer := cobolmcp.NewServer(neo4jClient, nil, nil)
		cobolMCPClient, mcpErr := modernize.NewMCPClientInProcess(ctx, mcpServer)
		if mcpErr != nil {
			return fmt.Errorf("creating MCP client: %w", mcpErr)
		}

		bridge := targetstack.NewGapBridge(cobolMCPClient, cobolMCPClient)

		reqs, reqErr := targetstack.GenerateRequirements(ctx, bridge, chatProvider, model,
			cfg.TargetStack.MaxTokens, cfg.TargetStack.GapMaxIter, logger)
		if reqErr != nil {
			return fmt.Errorf("generating requirements: %w", reqErr)
		}

		if len(reqs) > 0 {
			if err := targetstack.WriteRequirements(ctx, writer, reqs); err != nil {
				return fmt.Errorf("writing requirements: %w", err)
			}
		}

		logger.Info("requirements generation complete", zap.Int("requirements", len(reqs)))
	}

	return nil
}

func runGlossary(cmd *cobra.Command, args []string) error {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	htmlBytes, err := os.ReadFile(glossaryFile)
	if err != nil {
		return fmt.Errorf("reading glossary file: %w", err)
	}

	ctx := context.Background()

	// Build LLM provider
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	claudeClient, err := claude.NewClient(provider, cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating Claude client: %w", err)
	}

	logger.Info("extracting glossary terms",
		zap.String("file", glossaryFile),
		zap.String("codebase", glossaryCodebase),
	)

	terms, err := glossary.Extract(ctx, glossaryFile, htmlBytes, glossaryCodebase, cfg, claudeClient)
	if err != nil {
		return fmt.Errorf("extracting glossary: %w", err)
	}

	if len(terms) == 0 {
		logger.Info("no glossary terms extracted — nothing to write")
		return nil
	}

	logger.Info("terms extracted", zap.Int("count", len(terms)))

	// Connect to Neo4j and run migrations
	neo4jClient, err := n4j.NewClient(ctx, cfg.Neo4j, logger)
	if err != nil {
		return fmt.Errorf("connecting to Neo4j: %w", err)
	}
	defer neo4jClient.Close(ctx)

	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, glossaryCodebase, logger)

	if err := writer.WriteGlossaryTerms(ctx, glossaryCodebase, glossaryFile, terms); err != nil {
		return fmt.Errorf("writing glossary terms: %w", err)
	}

	logger.Info("glossary ingestion complete",
		zap.Int("terms", len(terms)),
		zap.String("codebase", glossaryCodebase),
	)
	return nil
}

func detectPrimaryLangFromScan(scan *targetstack.ScanResult) string {
	if scan == nil {
		return ""
	}
	best := ""
	bestCount := 0
	for lang, count := range scan.ByLang {
		if count > bestCount && lang != "XML" && lang != "YAML" && lang != "JSON" {
			bestCount = count
			best = lang
		}
	}
	return best
}
