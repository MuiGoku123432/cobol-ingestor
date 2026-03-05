package main

import (
	"fmt"
	"os"

	"cobol-ingestor/internal/config"

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

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// TODO: Register API routes from api/routes.go

	addr := fmt.Sprintf(":%s", cfg.API.Port)
	logger.Info("starting API server", zap.String("addr", addr))

	if err := r.Run(addr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
