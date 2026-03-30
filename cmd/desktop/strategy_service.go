package main

import (
	"context"
	"fmt"
	"sync"

	"cobol-ingestor/internal/modernize"
	"cobol-ingestor/internal/strategy"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// StrategyService manages strategy generation runs from the desktop UI.
type StrategyService struct {
	app *App

	mu       sync.Mutex
	running  bool
	cancelFn context.CancelFunc
}

// strategyEmitter implements modernize.EventEmitter for strategy events.
type strategyEmitter struct {
	ctx context.Context
}

func (e *strategyEmitter) Emit(event string, data any) {
	runtime.EventsEmit(e.ctx, "strategy:"+event, data)
}

// GetQuestions returns the strategy questionnaire for frontend rendering.
func (s *StrategyService) GetQuestions() []strategy.Question {
	return strategy.GetQuestions()
}

// ValidateAnswers validates questionnaire answers and returns the context + any errors.
func (s *StrategyService) ValidateAnswers(answers map[string][]string) (*strategy.StrategyContext, []string) {
	ctx, err := strategy.ValidateAnswers(answers)
	if err != nil {
		return nil, []string{err.Error()}
	}
	return ctx, nil
}

// SelectOutputDirectory opens a native directory picker for strategy output.
func (s *StrategyService) SelectOutputDirectory() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(s.app.ctx, runtime.OpenDialogOptions{
		Title: "Select Strategy Output Directory",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// StartStrategy validates answers and starts the strategy pipeline in the background.
func (s *StrategyService) StartStrategy(answers map[string][]string, outputDir string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("strategy generation already running")
	}
	s.running = true
	s.mu.Unlock()

	// Validate answers
	stratCtx, err := strategy.ValidateAnswers(answers)
	if err != nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check prerequisites
	ps := s.app.ChatService.ps
	if ps == nil || !ps.IsReady() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("LLM provider not ready (configure in Settings)")
	}

	mcpClient := s.app.ChatService.mcpClient
	if mcpClient == nil {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return fmt.Errorf("MCP not connected (connect to Neo4j first)")
	}

	ctx, cancel := context.WithCancel(s.app.ctx)
	s.cancelFn = cancel

	emitter := &strategyEmitter{ctx: s.app.ctx}

	go func() {
		defer func() {
			cancel()
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}()

		emitter.Emit("start", map[string]string{})

		plan, runErr := strategy.RunStrategy(ctx, strategy.StrategyParams{
			Context:   *stratCtx,
			Provider:  ps,
			MCPClient: mcpClient,
			Emitter:   emitter,
			Logger:    s.app.logger,
		})
		if runErr != nil {
			if ctx.Err() != nil {
				emitter.Emit("error", map[string]string{"error": "cancelled"})
			} else {
				emitter.Emit("error", map[string]string{"error": runErr.Error()})
			}
			return
		}

		// Generate markdown files
		files, genErr := strategy.GenerateStrategyDocs(plan, outputDir)
		if genErr != nil {
			emitter.Emit("error", map[string]string{"error": genErr.Error()})
			return
		}

		// Emit file_written for each file
		for _, f := range files {
			emitter.Emit("file_written", map[string]string{
				"path": f,
				"name": fileBaseName(f),
			})
		}

		emitter.Emit("complete", map[string]any{
			"outputDir": outputDir,
			"files":     files,
		})
	}()

	return nil
}

// CancelStrategy cancels a running strategy generation.
func (s *StrategyService) CancelStrategy() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return fmt.Errorf("no strategy generation running")
	}
	if s.cancelFn != nil {
		s.cancelFn()
	}
	return nil
}

// IsRunning returns whether strategy generation is active.
func (s *StrategyService) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Ensure strategyEmitter implements modernize.EventEmitter.
var _ modernize.EventEmitter = (*strategyEmitter)(nil)

func fileBaseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[i+1:]
		}
	}
	return path
}
