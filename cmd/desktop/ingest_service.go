package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/pipeline"
	"cobol-ingestor/internal/scanner"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// IngestService manages COBOL ingestion pipeline runs from the desktop UI.
type IngestService struct {
	app *App

	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc
}

// SelectDirectory opens a native directory picker dialog.
func (s *IngestService) SelectDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(s.app.ctx, runtime.OpenDialogOptions{
		Title: "Select COBOL Source Directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// StartIngestion starts the pipeline in a background goroutine.
// pass=0 runs all passes; pass=1-5 runs a specific pass.
func (s *IngestService) StartIngestion(dir string, pass int) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("ingestion already running")
	}
	s.running = true
	s.mu.Unlock()

	if s.app.Neo4jService.client == nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("not connected to Neo4j")
	}

	ctx, cancel := context.WithCancel(s.app.ctx)
	s.cancelFn = cancel

	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		s.emit("start", map[string]any{"dir": dir, "pass": pass})

		if err := s.runPipeline(ctx, dir, pass); err != nil {
			if ctx.Err() != nil {
				s.emit("cancelled", nil)
			} else {
				s.emit("error", map[string]string{"error": err.Error()})
			}
			return
		}

		s.emit("complete", nil)
	}()

	return nil
}

// CancelIngestion cancels a running ingestion.
func (s *IngestService) CancelIngestion() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return fmt.Errorf("no ingestion running")
	}
	if s.cancelFn != nil {
		s.cancelFn()
	}
	return nil
}

// IsRunning returns whether ingestion is currently active.
func (s *IngestService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

func (s *IngestService) emit(event string, data any) {
	runtime.EventsEmit(s.app.ctx, "ingest:"+event, data)
}

func (s *IngestService) runPipeline(ctx context.Context, dir string, passFlag int) error {
	cfg := s.app.cfg
	cfg.Ingest.RootDir = dir
	logger := s.newEventLogger()

	// Scan filesystem
	s.emit("progress", map[string]any{"phase": "scanning", "message": "Scanning files..."})
	scanResult, err := scanner.Scan(ctx, dir, logger)
	if err != nil {
		return fmt.Errorf("scanning: %w", err)
	}

	cobolCount, copyCount := 0, 0
	for _, f := range scanResult.Files {
		switch f.Type {
		case graph.FileTypeCOBOL:
			cobolCount++
		case graph.FileTypeCopybook:
			copyCount++
		}
	}
	s.emit("progress", map[string]any{
		"phase":   "scanned",
		"message": fmt.Sprintf("Found %d COBOL, %d copybooks", cobolCount, copyCount),
	})

	// Open cache
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// Use existing Neo4j connection
	neo4jClient := s.app.Neo4jService.client

	// Run migrations
	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		logger.Warn("migrations failed (may already be applied)", zap.Error(err))
	}

	// Resolve Copilot token if needed
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, err := auth.LoadToken(); err == nil && st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
		}
	}

	// Create LLM provider
	s.emit("progress", map[string]any{"phase": "init", "message": "Initializing LLM provider..."})
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

	// Auto-discover Claude model IDs from Copilot
	if cfg.LLM.Provider == "copilot" {
		if resolved, err := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel); err == nil {
			cfg.Claude.OpusModel = resolved.OpusModel
			cfg.Claude.SonnetModel = resolved.SonnetModel
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

	s.emit("progress", map[string]any{"phase": "running", "message": "Running pipeline..."})
	return pipe.Run(ctx, scanResult, passFlag)
}

// newEventLogger creates a zap logger that emits progress as Wails events.
func (s *IngestService) newEventLogger() *zap.Logger {
	encoderCfg := zap.NewProductionEncoderConfig()
	core := &wailsZapCore{
		ctx:     s.app.ctx,
		encoder: zapcore.NewJSONEncoder(encoderCfg),
		level:   zapcore.InfoLevel,
	}
	return zap.New(core)
}

// wailsZapCore is a zapcore.Core that emits log entries as Wails events.
type wailsZapCore struct {
	ctx     context.Context
	encoder zapcore.Encoder
	level   zapcore.Level
	fields  []zapcore.Field
}

func (c *wailsZapCore) Enabled(level zapcore.Level) bool {
	return level >= c.level
}

func (c *wailsZapCore) With(fields []zapcore.Field) zapcore.Core {
	return &wailsZapCore{
		ctx:     c.ctx,
		encoder: c.encoder.Clone(),
		level:   c.level,
		fields:  append(append([]zapcore.Field{}, c.fields...), fields...),
	}
}

func (c *wailsZapCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return ce.AddCore(entry, c)
	}
	return ce
}

func (c *wailsZapCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	allFields := append(c.fields, fields...)
	progress := map[string]any{
		"level":   entry.Level.String(),
		"message": entry.Message,
	}
	for _, f := range allFields {
		switch f.Key {
		case "pass", "filesTotal", "filesProcessed", "errors":
			progress[f.Key] = f.Integer
		case "phase", "currentFile", "file":
			progress[f.Key] = f.String
		}
	}
	runtime.EventsEmit(c.ctx, "ingest:progress", progress)
	return nil
}

func (c *wailsZapCore) Sync() error {
	return nil
}
