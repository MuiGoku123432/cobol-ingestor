package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"go.uber.org/zap"
)

//go:embed all:frontend/dist
var assets embed.FS

func newLogger() (*zap.Logger, error) {
	logDir := filepath.Join(os.Getenv("HOME"), ".cobol-graph", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	logPath := filepath.Join(logDir, "desktop.log")

	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout", logPath}
	cfg.ErrorOutputPaths = []string{"stderr", logPath}
	return cfg.Build()
}

func main() {
	logger, _ := newLogger()
	if logger == nil {
		logger, _ = zap.NewProduction()
	}
	defer logger.Sync()

	app := NewApp(logger)

	err := wails.Run(&options.App{
		Title:  "COBOL Graph",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
			app.Neo4jService,
			app.IngestService,
			app.ChatService,
			app.ConfigService,
			app.QueryStore,
			app.StrategyService,
			app.BrowserService,
			app.ExportService,
		},
	})
	if err != nil {
		logger.Fatal("wails run", zap.Error(err))
	}
}
