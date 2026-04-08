package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/modernize"
	webmod "cobol-ingestor/web/modernize"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("loading config", zap.Error(err))
	}

	// Create provider state with deferred init for Copilot auth flow
	ps := modernize.NewProviderState(cfg, logger)
	if ps.TryInit() {
		logger.Info("LLM provider ready at startup")
	} else if cfg.LLM.Provider != "copilot" {
		logger.Fatal("LLM provider init failed (API key required for non-copilot providers)")
	} else {
		logger.Info("copilot provider deferred, browser auth required")
	}

	// Resolve chat model
	chatModel := cfg.Modernize.ChatModel
	if chatModel == "" {
		chatModel = cfg.Claude.OpusModel
	}

	chatMaxTokens := cfg.Modernize.ChatMaxTokens
	if chatMaxTokens == 0 {
		chatMaxTokens = 16384
	}

	// Create MCP client
	ctx := context.Background()
	var mcpClient *modernize.MCPClient

	switch cfg.Modernize.MCPTransport {
	case "http":
		if cfg.Modernize.MCPServerURL == "" {
			logger.Fatal("MCP_SERVER_URL required for http transport")
		}
		mcpClient, err = modernize.NewMCPClientHTTP(ctx, cfg.Modernize.MCPServerURL)
	default: // "command"
		serverBin := cfg.Modernize.MCPServerBin
		if serverBin == "" {
			serverBin = "./bin/cobol-graph-mcp"
		}
		mcpClient, err = modernize.NewMCPClient(ctx, serverBin, os.Environ())
	}
	if err != nil {
		logger.Fatal("creating MCP client", zap.Error(err))
	}
	defer mcpClient.Close()

	logger.Info("MCP client connected",
		zap.String("transport", cfg.Modernize.MCPTransport),
		zap.String("model", chatModel))

	// Create session store
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		logger.Fatal("creating data dir", zap.Error(err))
	}
	sessionsDB := filepath.Join(cfg.DataDir, "sessions.db")
	sessionStore, err := modernize.NewSQLiteSessionStore(sessionsDB, 50)
	if err != nil {
		logger.Fatal("creating session store", zap.Error(err))
	}
	defer sessionStore.Close()
	logger.Info("session store ready", zap.String("path", sessionsDB))

	// Set up Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Serve static files from embedded FS
	r.GET("/", func(c *gin.Context) {
		data, _ := webmod.StaticFS.ReadFile("index.html")
		c.Data(http.StatusOK, "text/html; charset=utf-8", data)
	})
	r.GET("/app.js", func(c *gin.Context) {
		data, _ := webmod.StaticFS.ReadFile("app.js")
		c.Data(http.StatusOK, "application/javascript; charset=utf-8", data)
	})

	// Set default model on ProviderState so chat works before model selection
	ps.SetModel(chatModel, chatMaxTokens)

	// Auth endpoints
	r.GET("/api/auth/status", modernize.AuthStatusHandler(ps))
	r.POST("/api/auth/device-code", modernize.DeviceCodeHandler(ps))
	r.POST("/api/auth/logout", modernize.LogoutHandler(ps))

	// Model endpoints
	r.GET("/api/models", modernize.ModelsHandler(ps))
	r.POST("/api/models/select", modernize.SelectModelHandler(ps))

	// Session endpoints
	r.GET("/api/sessions", modernize.ListSessionsHandler(sessionStore))
	r.GET("/api/sessions/:id", modernize.GetSessionHandler(sessionStore))
	r.POST("/api/sessions", modernize.CreateSessionHandler(sessionStore))
	r.PATCH("/api/sessions/:id", modernize.UpdateSessionHandler(sessionStore))
	r.DELETE("/api/sessions/:id", modernize.DeleteSessionHandler(sessionStore))

	// API endpoints
	r.POST("/api/chat", modernize.ChatHandler(ps, mcpClient, chatModel, chatMaxTokens, sessionStore))
	r.POST("/api/swarm", modernize.SwarmHandler(ps, mcpClient, chatModel, chatMaxTokens, sessionStore))
	r.POST("/api/gap-analysis", modernize.GapAnalysisHandler(ps, mcpClient, chatModel, chatMaxTokens, sessionStore, logger))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":        "ok",
			"provider":      cfg.LLM.Provider,
			"authenticated": ps.IsReady(),
			"model":         chatModel,
		})
	})

	port := cfg.Modernize.Port
	if port == "" {
		port = "8081"
	}

	addr := fmt.Sprintf(":%s", port)
	logger.Info("starting modernize chat server", zap.String("addr", addr))
	if err := r.Run(addr); err != nil {
		logger.Fatal("server error", zap.Error(err))
	}
}
