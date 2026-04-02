package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/cache"
	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/extdb"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"
	mcpkg "cobol-ingestor/internal/mcp"
	"cobol-ingestor/internal/modernize"
	n4j "cobol-ingestor/internal/neo4j"
	"cobol-ingestor/internal/parser"
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

// SelectDirectory opens a native directory picker dialog with a custom title.
func (s *IngestService) SelectDirectory(title string) (string, error) {
	if title == "" {
		title = "Select Directory"
	}
	dir, err := runtime.OpenDirectoryDialog(s.app.ctx, runtime.OpenDialogOptions{
		Title: title,
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// StartIngestion starts the pipeline in a background goroutine.
// pass=0 runs all passes; pass=1-5 runs a specific pass.
func (s *IngestService) StartIngestion(dir string, pass int, contentDetect bool) error {
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

		if err := s.runPipeline(ctx, dir, pass, contentDetect); err != nil {
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

func (s *IngestService) runPipeline(ctx context.Context, dir string, passFlag int, contentDetect bool) error {
	cfg := s.app.cfg
	cfg.Ingest.RootDir = dir
	logger := s.newEventLogger()

	// Scan filesystem
	s.emit("progress", map[string]any{"phase": "scanning", "message": "Scanning files..."})
	detect := contentDetect || cfg.Ingest.ContentDetect
	scanResult, err := scanner.Scan(ctx, dir, logger, scanner.ScanOptions{
		ContentDetect: detect,
	})
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

	// Classify .txt files if content detection is enabled
	if detect && len(scanResult.Snippets) > 0 {
		s.emit("progress", map[string]any{"phase": "classifying", "message": "Classifying .txt files..."})
		if err := scanner.ClassifyPendingFiles(ctx, scanResult, provider, cfg.Claude.SonnetModel, logger, fileCache); err != nil {
			logger.Warn("content classification had errors", zap.Error(err))
		}
	}

	claudeClient, err := claude.NewClient(provider, cfg.Claude, logger)
	if err != nil {
		return fmt.Errorf("creating claude client: %w", err)
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)

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

// StartBWIngestion starts BusinessWare ingestion in a background goroutine.
func (s *IngestService) StartBWIngestion(dir, extensions string) error {
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

		s.emit("start", map[string]any{"dir": dir, "type": "bw"})

		if err := s.runBWPipeline(ctx, dir, extensions); err != nil {
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

func (s *IngestService) runBWPipeline(ctx context.Context, dir, extensionsStr string) error {
	cfg := s.app.cfg
	logger := s.newEventLogger()

	// Override config with parameters
	if dir != "" {
		cfg.BW.Dir = dir
	}
	if extensionsStr != "" {
		cfg.BW.Extensions = extensionsStr
	}

	// Apply defaults
	if cfg.BW.MaxWorkers <= 0 {
		cfg.BW.MaxWorkers = 5
	}
	if cfg.BW.MaxTokens <= 0 {
		cfg.BW.MaxTokens = 16000
	}
	if cfg.BW.TokenLimit <= 0 {
		cfg.BW.TokenLimit = 30000
	}

	// Parse extensions
	var extensions []string
	for _, ext := range strings.Split(cfg.BW.Extensions, ",") {
		ext = strings.TrimSpace(ext)
		if ext != "" {
			extensions = append(extensions, ext)
		}
	}

	// Scan BW files
	s.emit("progress", map[string]any{"phase": "scanning", "message": "Scanning BusinessWare files..."})
	bwScanResult, err := scanner.ScanBW(ctx, cfg.BW.Dir, extensions, logger, cfg.BW.MaxJARDepth, cfg.BW.MaxWorkers)
	if err != nil {
		return fmt.Errorf("scanning BW files: %w", err)
	}
	scanResult := bwScanResult.ScanResult
	if len(scanResult.Files) == 0 {
		return fmt.Errorf("no BusinessWare files found in %s", cfg.BW.Dir)
	}
	s.emit("progress", map[string]any{
		"phase":   "scanned",
		"message": fmt.Sprintf("Found %d BusinessWare files", len(scanResult.Files)),
	})

	neo4jClient := s.app.Neo4jService.client

	// Run migrations
	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		logger.Warn("migrations failed (may already be applied)", zap.Error(err))
	}

	// Open cache
	fileCache, err := cache.New(cfg.Ingest.CacheDB)
	if err != nil {
		return fmt.Errorf("opening cache: %w", err)
	}
	defer fileCache.Close()

	// Resolve Copilot token if needed
	if cfg.LLM.Provider == "copilot" && cfg.LLM.CopilotGitHubToken == "" {
		if st, err := auth.LoadToken(); err == nil && st != nil {
			cfg.LLM.CopilotGitHubToken = st.GitHubToken
		}
	}

	// Create LLM provider + Claude client
	s.emit("progress", map[string]any{"phase": "init", "message": "Initializing LLM provider..."})
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		return fmt.Errorf("creating LLM provider: %w", err)
	}
	defer provider.Close()

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

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)

	// Query enriched COBOL graph context for BW prompts
	bwGraphCtx, bwCtxErr := neo4jClient.QueryBWGraphContext(ctx)
	if bwCtxErr != nil {
		logger.Warn("failed to query BW graph context, falling back to flat IDs", zap.Error(bwCtxErr))
		existingPrograms := queryProgramIDs(ctx, neo4jClient, logger)
		bwGraphCtx = &n4j.BWGraphContext{RemainingPrograms: strings.Split(existingPrograms, ", ")}
	}
	interfaceProgs, domains, fileIOProgs, remaining := n4j.FormatBWGraphContext(bwGraphCtx, 8000)
	bwPromptCtx := claude.BWPromptContext{
		InterfacePrograms: interfaceProgs,
		BusinessDomains:   domains,
		FileIOPrograms:    fileIOProgs,
		RemainingPrograms: remaining,
	}
	bwPromptOverhead := computeBWPromptOverheadTiered(interfaceProgs, domains, fileIOProgs, remaining)

	// Build work items, skipping cached files
	const bwPassNumber = 99
	type bwWorkItem struct {
		file   graph.FileInfo
		chunks []chunker.BWChunk
	}

	var workItems []bwWorkItem
	skipped := 0
	for _, f := range scanResult.Files {
		changed, cacheErr := fileCache.IsChangedForPass(f.Path, f.Hash, bwPassNumber)
		if cacheErr != nil {
			logger.Warn("cache check failed", zap.String("file", f.Path), zap.Error(cacheErr))
			changed = true
		}
		if !changed {
			skipped++
			continue
		}

		var content []byte
		var readErr error
		if jarContent, ok := bwScanResult.JARContents[f.Path]; ok {
			content = jarContent
		} else {
			content, readErr = os.ReadFile(f.Path)
		}
		if readErr != nil {
			logger.Error("failed to read file", zap.String("file", f.Path), zap.Error(readErr))
			continue
		}

		chunks, chunkErr := chunker.ChunkBWFile(f.Path, content, cfg.BW.TokenLimit, bwPromptOverhead)
		if chunkErr != nil {
			logger.Error("failed to chunk file", zap.String("file", f.Path), zap.Error(chunkErr))
			continue
		}

		workItems = append(workItems, bwWorkItem{file: f, chunks: chunks})
	}

	s.emit("progress", map[string]any{
		"phase":   "running",
		"message": fmt.Sprintf("Processing %d files (%d cached, skipped)", len(workItems), skipped),
	})

	// Process with bounded concurrency
	sem := make(chan struct{}, cfg.BW.MaxWorkers)
	type bwResult struct {
		file   graph.FileInfo
		result *graph.BWResult
		err    error
	}
	resultCh := make(chan bwResult, len(workItems))

	for _, item := range workItems {
		sem <- struct{}{}
		go func(wi bwWorkItem) {
			defer func() { <-sem }()

			var allEntities []graph.BWEntity
			var allRels []graph.BWRelationship
			var allRefs []graph.BWCobolReference
			var summary string

			fileType := classifyBWExtension(wi.file.Path)

			for _, chunk := range wi.chunks {
				resp, analyzeErr := claudeClient.AnalyzeBW(ctx, wi.file.Path, fileType, chunk.Content, bwPromptCtx, cfg.BW.MaxTokens)
				if analyzeErr != nil {
					resultCh <- bwResult{file: wi.file, err: fmt.Errorf("analyzing %s chunk %d: %w", wi.file.Path, chunk.Index, analyzeErr)}
					return
				}

				parsed, parseErr := parser.ParseBWResponse(resp, wi.file.Path)
				if parseErr != nil {
					resultCh <- bwResult{file: wi.file, err: fmt.Errorf("parsing %s chunk %d: %w", wi.file.Path, chunk.Index, parseErr)}
					return
				}

				allEntities = append(allEntities, parsed.Entities...)
				allRels = append(allRels, parsed.Relationships...)
				allRefs = append(allRefs, parsed.CobolReferences...)
				if summary == "" {
					summary = parsed.File.Summary
				}
			}

			merged := &graph.BWResult{
				File: graph.BWFile{
					Path:     wi.file.Path,
					FileType: fileType,
					Summary:  summary,
				},
				Entities:        allEntities,
				Relationships:   allRels,
				CobolReferences: allRefs,
			}

			resultCh <- bwResult{file: wi.file, result: merged}
		}(item)
	}

	// Collect results and write to Neo4j
	processed := 0
	errors := 0
	totalEntities := 0
	totalRefs := 0
	for range len(workItems) {
		res := <-resultCh
		if res.err != nil {
			logger.Error("BW processing failed", zap.String("file", res.file.Path), zap.Error(res.err))
			errors++
			continue
		}

		if writeErr := writer.WriteBWResult(ctx, res.result); writeErr != nil {
			logger.Error("failed to write BW result", zap.String("file", res.file.Path), zap.Error(writeErr))
			errors++
			continue
		}

		if cacheErr := fileCache.MarkProcessedForPass(res.file.Path, res.file.Hash, bwPassNumber); cacheErr != nil {
			logger.Warn("failed to update cache", zap.String("file", res.file.Path), zap.Error(cacheErr))
		}

		processed++
		totalEntities += len(res.result.Entities)
		totalRefs += len(res.result.CobolReferences)

		s.emit("progress", map[string]any{
			"phase":          "running",
			"message":        fmt.Sprintf("Processed %s", filepath.Base(res.file.Path)),
			"filesProcessed": processed,
			"filesTotal":     len(workItems),
		})
	}

	s.emit("progress", map[string]any{
		"phase":   "done",
		"message": fmt.Sprintf("BW complete: %d processed, %d skipped, %d errors, %d entities, %d COBOL refs", processed, skipped, errors, totalEntities, totalRefs),
	})

	return nil
}

// StartOracleAnalysis starts Oracle/external DB gap analysis in a background goroutine.
func (s *IngestService) StartOracleAnalysis() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("ingestion already running")
	}
	s.running = true
	s.mu.Unlock()

	if s.app.Neo4jService.client == nil {
		if s.app.cfg.Neo4j.URI == "" {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return fmt.Errorf("Neo4j not configured (set connection details in Settings)")
		}
		if err := s.app.Neo4jService.tryConnect(s.app.ctx); err != nil {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
			return fmt.Errorf("Neo4j connection failed: %w", err)
		}
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

		s.emit("start", map[string]any{"type": "oracle"})

		if err := s.runOracleAnalysis(ctx); err != nil {
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

func (s *IngestService) runOracleAnalysis(ctx context.Context) error {
	cfg := s.app.cfg
	logger := s.newEventLogger()

	// Validate required fields
	if cfg.ExternalDB.DatabaseName == "" {
		return fmt.Errorf("database name is required (configure in Settings)")
	}
	if cfg.ExternalDB.DatabaseType == "" {
		return fmt.Errorf("database type is required (configure in Settings)")
	}
	hasOracle := strings.EqualFold(cfg.ExternalDB.DatabaseType, "oracle") && cfg.ExternalDB.OracleService != ""
	if cfg.ExternalDB.DBMCPCommand == "" && cfg.ExternalDB.DBMCPServerURL == "" && !hasOracle {
		return fmt.Errorf("Oracle connection not configured (set Oracle service in Settings)")
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

	chatProvider, ok := provider.(llm.ChatProvider)
	if !ok {
		return fmt.Errorf("LLM provider %q does not support chat completions", cfg.LLM.Provider)
	}

	model := cfg.Claude.OpusModel
	if cfg.LLM.Provider == "copilot" {
		if resolved, err := llm.ResolveCopilotModels(ctx, provider, cfg.Claude.OpusModel, cfg.Claude.SonnetModel); err == nil {
			model = resolved.OpusModel
		}
	}

	// Create MCP client for external DB
	s.emit("progress", map[string]any{"phase": "init", "message": "Connecting to external database..."})
	var dbClient *modernize.MCPClient
	switch {
	case cfg.ExternalDB.DBMCPServerURL != "":
		dbClient, err = modernize.NewMCPClientHTTP(ctx, cfg.ExternalDB.DBMCPServerURL)
	case cfg.ExternalDB.DBMCPCommand != "":
		parts := strings.Fields(cfg.ExternalDB.DBMCPCommand)
		if len(parts) == 0 {
			return fmt.Errorf("empty db-mcp-cmd")
		}
		dbClient, err = modernize.NewMCPClient(ctx, parts[0], parts[1:])
	case hasOracle:
		oracleConfig := extdb.OracleConfig{
			SQLclPath:  cfg.ExternalDB.OracleSQLclPath,
			Host:       cfg.ExternalDB.OracleHost,
			Port:       cfg.ExternalDB.OraclePort,
			Service:    cfg.ExternalDB.OracleService,
			User:       cfg.ExternalDB.OracleUser,
			Password:   cfg.ExternalDB.OraclePassword,
			WalletPath: cfg.ExternalDB.OracleWalletPath,
			TNSAdmin:   cfg.ExternalDB.OracleTNSAdmin,
		}
		dbClient, err = extdb.NewOracleMCPClient(ctx, oracleConfig, logger)
	default:
		return fmt.Errorf("no external DB connection configured")
	}
	if err != nil {
		return fmt.Errorf("connecting to external DB MCP: %w", err)
	}
	defer dbClient.Close()

	// Create in-process MCP client for COBOL graph
	s.emit("progress", map[string]any{"phase": "init", "message": "Initializing COBOL graph MCP..."})
	var batchWriter *n4j.BatchWriter
	if s.app.Neo4jService.client != nil {
		batchWriter = n4j.NewBatchWriter(s.app.Neo4jService.client, 500, "default", logger)
	}
	server := mcpkg.NewServer(s.app.Neo4jService.client, batchWriter, nil)
	graphClient, err := modernize.NewMCPClientInProcess(ctx, server)
	if err != nil {
		return fmt.Errorf("creating in-process graph MCP: %w", err)
	}
	defer graphClient.Close()

	// Create bridge and analyzer
	bridge := extdb.NewMCPBridge(graphClient, dbClient)
	analyzer := extdb.NewAnalyzer(
		bridge,
		chatProvider,
		model,
		cfg.ExternalDB.MaxIterations,
		cfg.ExternalDB.MaxTokens,
		cfg.ExternalDB.DatabaseName,
		cfg.ExternalDB.DatabaseType,
		logger,
	)

	s.emit("progress", map[string]any{"phase": "running", "message": "Running gap analysis..."})

	result, err := analyzer.Run(ctx)
	if err != nil {
		return fmt.Errorf("external DB analysis: %w", err)
	}

	// Write results to Neo4j
	s.emit("progress", map[string]any{"phase": "writing", "message": "Writing results to Neo4j..."})
	neo4jClient := s.app.Neo4jService.client
	migrationsDir := filepath.Join("migrations", "neo4j")
	if err := neo4jClient.RunMigrations(ctx, migrationsDir); err != nil {
		logger.Warn("migrations failed (may already be applied)", zap.Error(err))
	}

	writer := n4j.NewBatchWriter(neo4jClient, cfg.Ingest.BatchSize, "default", logger)
	if err := writer.WriteExternalDBResult(ctx, result); err != nil {
		return fmt.Errorf("writing results to neo4j: %w", err)
	}

	s.emit("progress", map[string]any{
		"phase":   "done",
		"message": fmt.Sprintf("Analysis complete: %d tables, %d mappings, %d gaps, %d flows", len(result.Tables), len(result.Mappings), len(result.Gaps), len(result.Flows)),
	})

	return nil
}

// queryProgramIDs fetches all existing COBOL program IDs from Neo4j for BW prompt context.
// The result is capped to avoid unbounded token growth in the prompt.
func queryProgramIDs(ctx context.Context, client *n4j.Client, logger *zap.Logger) string {
	ids, err := client.QueryAllProgramIDs(ctx)
	if err != nil {
		logger.Warn("failed to query program IDs", zap.Error(err))
		return "(none found)"
	}
	if len(ids) == 0 {
		return "(none found)"
	}
	return capProgramIDs(ids, 4000)
}

// capProgramIDs joins IDs until the token estimate reaches maxTokens,
// then appends a summary of remaining IDs to prevent unbounded prompt growth.
func capProgramIDs(ids []string, maxTokens int) string {
	var sb strings.Builder
	tokens := 0
	for i, id := range ids {
		entry := id
		if i > 0 {
			entry = ", " + id
		}
		entryTokens := chunker.EstimateTokens(entry)
		if tokens+entryTokens > maxTokens {
			remaining := len(ids) - i
			fmt.Fprintf(&sb, " ... and %d more programs", remaining)
			break
		}
		sb.WriteString(entry)
		tokens += entryTokens
	}
	return sb.String()
}

// computeBWPromptOverhead returns the estimated token overhead for BW prompts
// (system message + template + separator + prefill + margin + existingPrograms).
func computeBWPromptOverhead(existingPrograms string) int {
	return 2100 + chunker.EstimateTokens(existingPrograms)
}

// computeBWPromptOverheadTiered returns the estimated token overhead for BW prompts
// using tiered context.
func computeBWPromptOverheadTiered(interfaceProgs, domains, fileIOProgs, remaining string) int {
	return 2600 + chunker.EstimateTokens(interfaceProgs) +
		chunker.EstimateTokens(domains) +
		chunker.EstimateTokens(fileIOProgs) +
		chunker.EstimateTokens(remaining)
}

// classifyBWExtension returns a human-readable file type from a path's extension.
func classifyBWExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".java":
		return "Java"
	case ".class":
		return "Java Bytecode"
	case ".md":
		return "Markdown"
	case ".xml":
		return "XML"
	case ".bw":
		return "Businessware"
	case ".txt":
		return "Text"
	case ".properties":
		return "Properties"
	case ".json":
		return "JSON"
	case ".yml", ".yaml":
		return "YAML"
	case ".mf":
		return "Manifest"
	case ".vsdx":
		return "Visio Diagram"
	case ".drawio":
		return "DrawIO Diagram"
	case ".svg":
		return "SVG Diagram"
	case ".puml", ".plantuml":
		return "PlantUML Diagram"
	default:
		return "Unknown"
	}
}
