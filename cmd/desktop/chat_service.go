package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"cobol-ingestor/internal/modernize"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ChatService handles chat/swarm interactions for the desktop UI.
type ChatService struct {
	app          *App
	ps           *modernize.ProviderState
	mcpClient    *modernize.MCPClient
	sessionStore modernize.SessionStore

	mu       sync.Mutex
	cancelFn context.CancelFunc
}

// ChatMessage mirrors the message structure used in the modernize package.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AuthStatus reports the LLM provider authentication state.
type AuthStatus struct {
	Ready    bool   `json:"ready"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Pending  bool   `json:"pending"`
}

// WailsEmitter implements modernize.EventEmitter using Wails runtime events.
type WailsEmitter struct {
	ctx    context.Context
	prefix string // event prefix, e.g. "chat:" or "swarm:"
}

func (e *WailsEmitter) Emit(event string, data any) {
	runtime.EventsEmit(e.ctx, e.prefix+event, data)
}

// initProvider creates the LLM provider state from config.
func (s *ChatService) initProvider() {
	s.ps = modernize.NewProviderState(s.app.cfg, s.app.logger)
	if s.ps.TryInit() {
		s.app.logger.Info("LLM provider ready at startup")
	} else {
		s.app.logger.Info("LLM provider deferred (configure in Settings)")
	}

	// Set default model
	chatModel := s.app.cfg.Modernize.ChatModel
	if chatModel == "" {
		chatModel = s.app.cfg.Claude.OpusModel
	}
	maxTokens := s.app.cfg.Modernize.ChatMaxTokens
	if maxTokens == 0 {
		maxTokens = 16384
	}
	s.ps.SetModel(chatModel, maxTokens)
}

// GetAuthStatus returns current authentication state.
func (s *ChatService) GetAuthStatus() AuthStatus {
	return AuthStatus{
		Ready:    s.ps.IsReady(),
		Provider: s.app.cfg.LLM.Provider,
		Model:    s.ps.GetModel(),
		Pending:  false, // TODO: wire up pending from device flow
	}
}

// StartDeviceFlow initiates GitHub Copilot device flow authentication.
func (s *ChatService) StartDeviceFlow() (map[string]string, error) {
	if s.app.cfg.LLM.Provider != "copilot" {
		return nil, fmt.Errorf("device flow only available for copilot provider")
	}
	// The device flow is handled by the provider state
	// Return instructions for the user
	return map[string]string{
		"message": "Please complete GitHub authentication in your browser",
	}, nil
}

// ListModels returns available LLM models.
func (s *ChatService) ListModels() ([]modernize.ModelInfo, error) {
	if s.ps == nil || !s.ps.IsReady() {
		// Return default model if not authenticated yet
		model := s.app.cfg.Modernize.ChatModel
		if model == "" {
			model = s.app.cfg.Claude.OpusModel
		}
		return []modernize.ModelInfo{
			{ID: model, Name: model},
		}, nil
	}
	return s.ps.ListModels()
}

// SelectModel changes the active chat model.
func (s *ChatService) SelectModel(modelID string, maxTokens int) error {
	if maxTokens == 0 {
		maxTokens = 16384
	}
	s.ps.SetModel(modelID, maxTokens)
	return nil
}

// ListSessions returns all chat sessions.
func (s *ChatService) ListSessions() ([]modernize.SessionSummary, error) {
	if s.sessionStore == nil {
		return nil, fmt.Errorf("session store not initialized")
	}
	return s.sessionStore.List()
}

// GetSession returns a session with full message history.
func (s *ChatService) GetSession(id string) (*modernize.Session, error) {
	if s.sessionStore == nil {
		return nil, fmt.Errorf("session store not initialized")
	}
	return s.sessionStore.Get(id)
}

// CreateSession creates a new chat session.
func (s *ChatService) CreateSession(title string) (*modernize.Session, error) {
	if s.sessionStore == nil {
		return nil, fmt.Errorf("session store not initialized")
	}
	return s.sessionStore.Create(title)
}

// UpdateSessionTitle updates a session's title.
func (s *ChatService) UpdateSessionTitle(id, title string) error {
	if s.sessionStore == nil {
		return fmt.Errorf("session store not initialized")
	}
	return s.sessionStore.Update(id, nil, title)
}

// DeleteSession removes a session.
func (s *ChatService) DeleteSession(id string) error {
	if s.sessionStore == nil {
		return fmt.Errorf("session store not initialized")
	}
	return s.sessionStore.Delete(id)
}

// ensureMCP lazily initializes the Neo4j connection and MCP client if not
// already connected. This allows the chat to work after the user configures
// Neo4j via Settings without requiring an explicit "Connect" click.
func (s *ChatService) ensureMCP() error {
	if s.mcpClient != nil {
		return nil
	}
	if s.app.Neo4jService.client == nil {
		if s.app.cfg.Neo4j.URI == "" {
			return fmt.Errorf("Neo4j not configured (set connection details in Settings)")
		}
		if err := s.app.Neo4jService.tryConnect(s.app.ctx); err != nil {
			return fmt.Errorf("Neo4j connection failed: %w", err)
		}
	}
	if err := s.app.initMCP(); err != nil {
		return fmt.Errorf("MCP initialization failed: %w", err)
	}
	return nil
}

// SendChat sends a chat message and streams responses via Wails events.
// Events: chat:text, chat:tool_start, chat:tool_result, chat:done, chat:error
func (s *ChatService) SendChat(sessionID string, messages []ChatMessage, discoveryMode, generateDiagram, unlimitedIterations bool, targetLang, framework, integrations string) error {
	if !s.ps.IsReady() {
		return fmt.Errorf("LLM provider not ready (configure in Settings)")
	}
	if err := s.ensureMCP(); err != nil {
		return err
	}

	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
	ctx, cancel := context.WithCancel(s.app.ctx)
	s.cancelFn = cancel
	s.mu.Unlock()

	emitter := &WailsEmitter{ctx: s.app.ctx, prefix: "chat:"}

	go func() {
		defer cancel()
		err := modernize.RunChat(ctx, modernize.ChatParams{
			Provider:            s.ps,
			MCPClient:           s.mcpClient,
			SessionStore:        s.sessionStore,
			SessionID:           sessionID,
			Messages:            toModernizeMessages(messages),
			DiscoveryMode:       discoveryMode,
			GenerateDiagram:     generateDiagram,
			UnlimitedIterations: unlimitedIterations,
			TargetLang:          targetLang,
			Framework:           framework,
			Integrations:        integrations,
			Emitter:             emitter,
			Logger:              s.app.logger,
			DiagramOutputDir:    filepath.Join(s.app.cfg.DataDir, "diagrams"),
		})
		if err != nil {
			emitter.Emit("error", map[string]string{"error": err.Error()})
		}
	}()

	return nil
}

// SendSwarm sends a swarm query and streams responses via Wails events.
// Events: swarm:agent_start, swarm:agent_progress, swarm:agent_complete, etc.
func (s *ChatService) SendSwarm(sessionID string, messages []ChatMessage, discoveryMode, multiRound, generateDiagram, unlimitedIterations bool, targetLang, framework, integrations string) error {
	if !s.ps.IsReady() {
		return fmt.Errorf("LLM provider not ready (configure in Settings)")
	}
	if err := s.ensureMCP(); err != nil {
		return err
	}

	s.mu.Lock()
	if s.cancelFn != nil {
		s.cancelFn()
	}
	ctx, cancel := context.WithCancel(s.app.ctx)
	s.cancelFn = cancel
	s.mu.Unlock()

	emitter := &WailsEmitter{ctx: s.app.ctx, prefix: "swarm:"}

	go func() {
		defer cancel()
		err := modernize.RunSwarm(ctx, modernize.SwarmParams{
			Provider:            s.ps,
			MCPClient:           s.mcpClient,
			SessionStore:        s.sessionStore,
			SessionID:           sessionID,
			Messages:            toModernizeMessages(messages),
			DiscoveryMode:       discoveryMode,
			MultiRound:          multiRound,
			GenerateDiagram:     generateDiagram,
			UnlimitedIterations: unlimitedIterations,
			TargetLang:          targetLang,
			Framework:           framework,
			Integrations:        integrations,
			Emitter:             emitter,
			Logger:              s.app.logger,
			DiagramOutputDir:    filepath.Join(s.app.cfg.DataDir, "diagrams"),
		})
		if err != nil {
			emitter.Emit("error", map[string]string{"error": err.Error()})
		}
	}()

	return nil
}

// CancelChat cancels the current chat or swarm operation.
func (s *ChatService) CancelChat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelFn != nil {
		s.cancelFn()
		s.cancelFn = nil
	}
}

func toModernizeMessages(msgs []ChatMessage) []modernize.InputMessage {
	out := make([]modernize.InputMessage, len(msgs))
	for i, m := range msgs {
		out[i] = modernize.InputMessage{Role: m.Role, Content: m.Content}
	}
	return out
}
