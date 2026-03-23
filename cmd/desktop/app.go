package main

import (
	"context"
	"os"
	"path/filepath"

	"cobol-ingestor/internal/config"
	mcpkg "cobol-ingestor/internal/mcp"
	"cobol-ingestor/internal/modernize"
	n4j "cobol-ingestor/internal/neo4j"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

// App is the main Wails application struct. All exported methods are
// available as bindings in the frontend.
type App struct {
	ctx    context.Context
	logger *zap.Logger
	cfg    *config.Config

	// Services exposed to frontend via Wails bindings
	Neo4jService    *Neo4jService
	IngestService   *IngestService
	ChatService     *ChatService
	ConfigService   *ConfigService
	QueryStore      *QueryStoreService
	StrategyService *StrategyService
	BrowserService  *BrowserService
	ExportService   *ExportService
}

func NewApp(logger *zap.Logger) *App {
	a := &App{logger: logger}
	a.Neo4jService = &Neo4jService{app: a}
	a.IngestService = &IngestService{app: a}
	a.ChatService = &ChatService{app: a}
	a.ConfigService = &ConfigService{app: a}
	a.QueryStore = &QueryStoreService{app: a}
	a.StrategyService = &StrategyService{app: a}
	a.BrowserService = &BrowserService{app: a}
	a.ExportService = &ExportService{app: a}
	return a
}

// startup is called when the Wails app starts. ctx is used for runtime event
// emission throughout the application lifetime.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Load .env from system config dir before config.Load() picks up env vars
	_ = godotenv.Load(a.ConfigService.envPath())

	cfg, err := config.Load()
	if err != nil {
		a.logger.Error("loading config", zap.Error(err))
		cfg = &config.Config{} // fall back to empty config; user configures via Settings
	}
	a.cfg = cfg

	// Ensure data dir exists
	if a.cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		a.cfg.DataDir = filepath.Join(home, ".cobol-graph")
	}
	_ = os.MkdirAll(a.cfg.DataDir, 0700)

	// Try connecting to Neo4j with existing config (non-fatal)
	if a.cfg.Neo4j.URI != "" {
		if err := a.Neo4jService.tryConnect(ctx); err != nil {
			a.logger.Info("neo4j not connected at startup (configure via Settings)", zap.Error(err))
		}
	}

	// Initialize provider state for chat (deferred if copilot)
	a.ChatService.initProvider()

	// Initialize query store
	queriesDB := filepath.Join(a.cfg.DataDir, "queries.db")
	if err := a.QueryStore.open(queriesDB); err != nil {
		a.logger.Error("query store init", zap.Error(err))
	}

	// Initialize session store
	sessionsDB := filepath.Join(a.cfg.DataDir, "sessions.db")
	store, err := modernize.NewSQLiteSessionStore(sessionsDB, 50)
	if err != nil {
		a.logger.Error("session store init", zap.Error(err))
	} else {
		a.ChatService.sessionStore = store
	}
}

// shutdown is called when the Wails app is closing.
func (a *App) shutdown(ctx context.Context) {
	if a.ChatService.mcpClient != nil {
		a.ChatService.mcpClient.Close()
	}
	if a.ChatService.sessionStore != nil {
		a.ChatService.sessionStore.Close()
	}
	a.QueryStore.Close()
	a.Neo4jService.disconnect(ctx)
}

// initMCP creates an in-process MCP client connected to the Neo4j reader.
// Called after Neo4j connects successfully.
func (a *App) initMCP() error {
	if a.Neo4jService.client == nil {
		return nil
	}
	var writer *n4j.BatchWriter
	writer = n4j.NewBatchWriter(a.Neo4jService.client, 500, "default", a.logger)

	server := mcpkg.NewServer(a.Neo4jService.client, writer)
	mcpClient, err := modernize.NewMCPClientInProcess(a.ctx, server)
	if err != nil {
		return err
	}
	// Close previous client if any
	if a.ChatService.mcpClient != nil {
		a.ChatService.mcpClient.Close()
	}
	a.ChatService.mcpClient = mcpClient
	a.logger.Info("in-process MCP client connected")
	return nil
}
