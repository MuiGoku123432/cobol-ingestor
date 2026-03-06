package main

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/llm"
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

	// Create LLM provider
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		logger.Fatal("creating LLM provider", zap.Error(err))
	}
	defer provider.Close()

	chatProvider, ok := provider.(llm.ChatProvider)
	if !ok {
		logger.Fatal("LLM provider does not support ChatProvider interface",
			zap.String("provider", provider.Name()))
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
		// Pass current environment to the subprocess
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
	staticFS, err := fs.Sub(webmod.StaticFS, ".")
	if err != nil {
		logger.Fatal("creating static filesystem", zap.Error(err))
	}

	r.GET("/", func(c *gin.Context) {
		c.FileFromFS("index.html", http.FS(staticFS))
	})
	r.GET("/app.js", func(c *gin.Context) {
		c.FileFromFS("app.js", http.FS(staticFS))
	})

	// API endpoints
	r.POST("/api/chat", modernize.ChatHandler(chatProvider, mcpClient, chatModel, chatMaxTokens))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "provider": provider.Name(), "model": chatModel})
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
