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
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/extdb"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/internal/modernize"
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
	_ = ingestCmd.MarkFlagRequired("dir")
	rootCmd.AddCommand(ingestCmd)

	authCmd.AddCommand(authLoginCmd, authLogoutCmd, authStatusCmd, authModelsCmd)
	rootCmd.AddCommand(authCmd)

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
	ctx := context.Background()

	logger.Info("starting ingestion",
		zap.String("dir", cfg.Ingest.RootDir),
		zap.Int("max_workers", cfg.Ingest.MaxWorkers),
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

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, logger)
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
