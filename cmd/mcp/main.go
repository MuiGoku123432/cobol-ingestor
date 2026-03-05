package main

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"cobol-ingestor/internal/config"
	mcpkg "cobol-ingestor/internal/mcp"
	n4j "cobol-ingestor/internal/neo4j"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("loading config", zap.Error(err))
	}

	ctx := context.Background()

	neo4jClient, err := n4j.NewClient(ctx, cfg.Neo4j, logger)
	if err != nil {
		logger.Fatal("connecting to neo4j", zap.Error(err))
	}
	defer neo4jClient.Close(ctx)

	if err := neo4jClient.VerifyConnectivity(ctx); err != nil {
		logger.Fatal("neo4j connectivity check", zap.Error(err))
	}

	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		logger.Fatal("running migrations", zap.Error(err))
	}

	server := mcpkg.NewServer(neo4jClient)

	// If MCP_HTTP_PORT is set, run as Streamable HTTP server.
	// Otherwise, run as stdio transport (for Claude Desktop / Claude Code).
	httpPort := cfg.MCP.HTTPPort
	if httpPort != "" {
		handler := gomcp.NewStreamableHTTPHandler(func(r *http.Request) *gomcp.Server {
			return server
		}, nil)
		addr := fmt.Sprintf(":%s", httpPort)
		logger.Info("starting MCP HTTP server", zap.String("addr", addr))
		if err := http.ListenAndServe(addr, handler); err != nil {
			logger.Fatal("http server", zap.Error(err))
		}
	} else {
		logger.Info("starting MCP server on stdio")
		if err := server.Run(ctx, &gomcp.StdioTransport{}); err != nil {
			logger.Fatal("stdio server", zap.Error(err))
		}
	}
}
