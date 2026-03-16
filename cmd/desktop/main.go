package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"go.uber.org/zap"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	logger, _ := zap.NewProduction()
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
			app.GraphService,
			app.IngestService,
			app.ChatService,
			app.ConfigService,
		},
	})
	if err != nil {
		logger.Fatal("wails run", zap.Error(err))
	}
}
