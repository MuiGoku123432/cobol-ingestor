package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/extdb"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/modernize"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
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
	dir            string
	passFlag       int
	codebaseFlag   string
	contentDetect  bool
)

// bw flags
var (
	bwDir        string
	bwExtensions string
	bwMaxWorkers int
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
	_ = ingestCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(ingestCmd)

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd, authModelsCmd)
	rootCmd.AddCommand(authCmd)

	bwCmd.Flags().StringVar(&bwDir, "dir", "", "Root directory of Businessware files")
	bwCmd.Flags().StringVar(&bwExtensions, "extensions", "", "Comma-separated file extensions (default: .java,.md,.bw,.txt,.xml)")
	bwCmd.Flags().IntVar(&bwMaxWorkers, "max-workers", 0, "Max concurrent analysis workers (default: 5)")
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
	scanResult, err := scanner.Scan(ctx, cfg.Ingest.RootDir, logger, scanner.ScanOptions{
		ContentDetect: detect,
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

	// Classify .txt files if content detection is enabled
	if detect && len(scanResult.Snippets) > 0 {
		if err := scanner.ClassifyPendingFiles(ctx, scanResult, provider, cfg.Claude.SonnetModel, logger); err != nil {
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
	bwScanResult, err := scanner.ScanBW(ctx, cfg.BW.Dir, extensions, logger)
	if err != nil {
		return fmt.Errorf("scanning BW files: %w", err)
	}
	scanResult := bwScanResult.ScanResult
	if len(scanResult.Files) == 0 {
		fmt.Println("No Businessware files found.")
		return nil
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

	// Query existing COBOL program IDs for prompt context
	existingPrograms := queryProgramIDs(ctx, neo4jClient, logger)

	// Process files with worker pool
	const bwPassNumber = 99
	type bwWorkItem struct {
		file   graph.FileInfo
		chunks []chunker.BWChunk
	}

	// Build work items, skipping cached files
	var workItems []bwWorkItem
	skipped := 0
	for _, f := range scanResult.Files {
		changed, cacheErr := fileCache.IsChangedForPass(f.Path, f.Hash, bwPassNumber)
		if cacheErr != nil {
			logger.Warn("cache check failed", zap.String("file", f.Path), zap.Error(cacheErr))
			changed = true
		}
		if !changed {
			skipped++
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

		chunks, chunkErr := chunker.ChunkBWFile(f.Path, content, cfg.BW.TokenLimit)
		if chunkErr != nil {
			logger.Error("failed to chunk file", zap.String("file", f.Path), zap.Error(chunkErr))
			continue
		}

		workItems = append(workItems, bwWorkItem{file: f, chunks: chunks})
	}

	logger.Info("BW work plan",
		zap.Int("to_process", len(workItems)),
		zap.Int("skipped_cached", skipped),
		zap.Int("total_files", len(scanResult.Files)),
	)

	// Process with bounded concurrency
	sem := make(chan struct{}, cfg.BW.MaxWorkers)
	type bwResult struct {
		file   graph.FileInfo
		result *graph.BWResult
		err    error
	}
	resultCh := make(chan bwResult, len(workItems))

	for _, item := range workItems {
		sem <- struct{}{}
		go func(wi bwWorkItem) {
			defer func() { <-sem }()

			// For multi-chunk files, concatenate results
			var allEntities []graph.BWEntity
			var allRels []graph.BWRelationship
			var allRefs []graph.BWCobolReference
			var summary string

			fileType := classifyBWExtension(wi.file.Path)

			for _, chunk := range wi.chunks {
				resp, analyzeErr := claudeClient.AnalyzeBW(ctx, wi.file.Path, fileType, chunk.Content, existingPrograms, cfg.BW.MaxTokens)
				if analyzeErr != nil {
					resultCh <- bwResult{file: wi.file, err: fmt.Errorf("analyzing %s chunk %d: %w", wi.file.Path, chunk.Index, analyzeErr)}
					return
				}

				parsed, parseErr := parser.ParseBWResponse(resp, wi.file.Path)
				if parseErr != nil {
					resultCh <- bwResult{file: wi.file, err: fmt.Errorf("parsing %s chunk %d: %w", wi.file.Path, chunk.Index, parseErr)}
					return
				}

				allEntities = append(allEntities, parsed.Entities...)
				allRels = append(allRels, parsed.Relationships...)
				allRefs = append(allRefs, parsed.CobolReferences...)
				if summary == "" {
					summary = parsed.File.Summary
				}
			}

			merged := &graph.BWResult{
				File: graph.BWFile{
					Path:     wi.file.Path,
					FileType: fileType,
					Summary:  summary,
				},
				Entities:        allEntities,
				Relationships:   allRels,
				CobolReferences: allRefs,
			}

			resultCh <- bwResult{file: wi.file, result: merged}
		}(item)
	}

	// Collect results and write to Neo4j
	processed := 0
	errors := 0
	totalEntities := 0
	totalRefs := 0
	for range len(workItems) {
		res := <-resultCh
		if res.err != nil {
			logger.Error("BW processing failed", zap.String("file", res.file.Path), zap.Error(res.err))
			errors++
			continue
		}

		if writeErr := writer.WriteBWResult(ctx, res.result); writeErr != nil {
			logger.Error("failed to write BW result", zap.String("file", res.file.Path), zap.Error(writeErr))
			errors++
			continue
		}

		// Mark as cached
		if cacheErr := fileCache.MarkProcessedForPass(res.file.Path, res.file.Hash, bwPassNumber); cacheErr != nil {
			logger.Warn("failed to update cache", zap.String("file", res.file.Path), zap.Error(cacheErr))
		}

		processed++
		totalEntities += len(res.result.Entities)
		totalRefs += len(res.result.CobolReferences)
	}

	fmt.Printf("\nBusinessware Ingestion Complete\n")
	fmt.Printf("  Files processed: %d\n", processed)
	fmt.Printf("  Files skipped:   %d (cached)\n", skipped)
	fmt.Printf("  Errors:          %d\n", errors)
	fmt.Printf("  Entities:        %d extracted\n", totalEntities)
	fmt.Printf("  COBOL refs:      %d cross-links\n", totalRefs)
	fmt.Printf("\nResults persisted to Neo4j. Query with:\n")
	fmt.Printf("  MATCH (f:BWFile)-[:BW_CONTAINS]->(e:BWEntity) RETURN f.path, e.name, e.entityType LIMIT 20\n")

	return nil
}

// queryProgramIDs fetches all existing COBOL program IDs from Neo4j for BW prompt context.
func queryProgramIDs(ctx context.Context, client *n4j.Client, logger *zap.Logger) string {
	ids, err := client.QueryAllProgramIDs(ctx)
	if err != nil {
		logger.Warn("failed to query program IDs", zap.Error(err))
		return "(none found)"
	}
	if len(ids) == 0 {
		return "(none found)"
	}
	return strings.Join(ids, ", ")
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
