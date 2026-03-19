package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cobol-ingestor/api"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/llm"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/pipeline"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// @title COBOL Graph API
// @version 1.0
// @description REST API for querying COBOL code relationships stored in Neo4j
// @host localhost:8080
// @BasePath /api/v1

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("loading config", zap.Error(err))
	}

	if err := cfg.Validate(); err != nil {
		logger.Warn("config validation", zap.Error(err))
	}

	ctx := context.Background()

	// Connect to Neo4j
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

	// Set up pipeline for async jobs (optional — may fail if no API key)
	var pipe *pipeline.Pipeline
	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)
	provider, providerErr := llm.NewProvider(cfg)
	if providerErr == nil {
		claudeClient, claudeErr := claude.NewClient(provider, cfg.Claude, logger)
		if claudeErr == nil {
			fileCache, cacheErr := cache.New(cfg.Ingest.CacheDB)
			if cacheErr == nil {
				pipe = &pipeline.Pipeline{
					Config:      cfg,
					Claude:      claudeClient,
					Neo4jClient: neo4jClient,
					Writer:      writer,
					Cache:       fileCache,
					Logger:      logger,
				}
			}
		}
	}
	if pipe == nil {
		logger.Warn("pipeline not available — async jobs disabled")
	}

	r := gin.Default()
	r.Use(cors.Default())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api.RegisterRoutes(r, neo4jClient, writer, pipe, logger)

	addr := fmt.Sprintf(":%s", cfg.API.Port)
	logger.Info("starting API server", zap.String("addr", addr))

	if err := r.Run(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
