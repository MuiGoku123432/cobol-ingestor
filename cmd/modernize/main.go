package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

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

	// Auth endpoints
	r.GET("/api/auth/status", modernize.AuthStatusHandler(ps))
	r.POST("/api/auth/device-code", modernize.DeviceCodeHandler(ps))
	r.POST("/api/auth/logout", modernize.LogoutHandler(ps))

	// API endpoints
	r.POST("/api/chat", modernize.ChatHandler(ps, mcpClient, chatModel, chatMaxTokens))

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
