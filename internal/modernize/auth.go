package modernize

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"cobol-ingestor/internal/auth"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/llm"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/copilot"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ProviderState holds the LLM provider with thread-safe deferred init.
type ProviderState struct {
	mu        sync.RWMutex
	provider  llm.ChatProvider
	cfg       *config.Config
	logger    *zap.Logger
	pending   atomic.Bool
	model     string // selected model ID
	maxTokens int    // max tokens for selected model
}

// NewProviderState creates a new ProviderState.
func NewProviderState(cfg *config.Config, logger *zap.Logger) *ProviderState {
	return &ProviderState{cfg: cfg, logger: logger}
}

// Get returns the current ChatProvider, or nil if not yet initialized.
func (ps *ProviderState) Get() llm.ChatProvider {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.provider
}

// Set sets the ChatProvider.
func (ps *ProviderState) Set(p llm.ChatProvider) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.provider = p
}

// IsReady returns true if a provider is available.
func (ps *ProviderState) IsReady() bool {
	return ps.Get() != nil
}

// GetModel returns the selected model ID.
func (ps *ProviderState) GetModel() string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.model
}

// GetMaxTokens returns the max tokens for the selected model.
func (ps *ProviderState) GetMaxTokens() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	return ps.maxTokens
}

// SetModel sets the selected model and max tokens.
func (ps *ProviderState) SetModel(model string, maxTokens int) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.model = model
	ps.maxTokens = maxTokens
}

// TryInit attempts provider creation using env var or cached token.
// Returns true if provider is ready.
func (ps *ProviderState) TryInit() bool {
	// For copilot, try loading cached token if no token in config
	if ps.cfg.LLM.Provider == "copilot" && ps.cfg.LLM.CopilotGitHubToken == "" {
		if st, err := auth.LoadToken(); err == nil && st != nil {
			ps.cfg.LLM.CopilotGitHubToken = st.GitHubToken
			ps.logger.Info("using cached copilot token")
		}
	}

	provider, err := llm.NewProvider(ps.cfg)
	if err != nil {
		ps.logger.Warn("provider init deferred", zap.Error(err))
		return false
	}

	chatProvider, ok := provider.(llm.ChatProvider)
	if !ok {
		ps.logger.Warn("provider does not support ChatProvider interface")
		return false
	}

	ps.Set(chatProvider)
	return true
}

// AuthStatusHandler returns a handler for GET /api/auth/status.
func AuthStatusHandler(ps *ProviderState) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"authenticated": ps.IsReady(),
			"provider":      ps.cfg.LLM.Provider,
			"pending":       ps.pending.Load(),
		})
	}
}

// DeviceCodeHandler returns a handler for POST /api/auth/device-code.
func DeviceCodeHandler(ps *ProviderState) gin.HandlerFunc {
	return func(c *gin.Context) {
		if ps.IsReady() {
			c.JSON(http.StatusOK, gin.H{"authenticated": true})
			return
		}
		if ps.pending.Load() {
			c.JSON(http.StatusConflict, gin.H{"error": "device flow already in progress"})
			return
		}

		userCode, verificationURI, deviceCode, interval, expiresIn, err := requestDeviceCode(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("device code request failed: %v", err)})
			return
		}

		ps.pending.Store(true)

		// Start background polling
		go pollForToken(context.Background(), ps, deviceCode, interval, expiresIn)

		c.JSON(http.StatusOK, gin.H{
			"user_code":        userCode,
			"verification_uri": verificationURI,
		})
	}
}

// LogoutHandler returns a handler for POST /api/auth/logout.
func LogoutHandler(ps *ProviderState) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p := ps.Get(); p != nil {
			p.Close()
		}
		ps.Set(nil)
		ps.cfg.LLM.CopilotGitHubToken = ""
		c.JSON(http.StatusOK, gin.H{"status": "logged out"})
	}
}

func requestDeviceCode(ctx context.Context) (userCode, verificationURI, deviceCode string, interval, expiresIn int, err error) {
	reqBody, _ := json.Marshal(map[string]string{
		"client_id": copilot.GitHubClientID,
		"scope":     copilot.GitHubOAuthScopes,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", copilot.GitHubDeviceCodeURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", "", 0, 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", "", 0, 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	var dc copilot.GitHubDeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return "", "", "", 0, 0, fmt.Errorf("decoding response: %w", err)
	}

	return dc.UserCode, dc.VerificationURI, dc.DeviceCode, dc.Interval, dc.ExpiresIn, nil
}

func pollForToken(ctx context.Context, ps *ProviderState, deviceCode string, interval, expiresIn int) {
	defer ps.pending.Store(false)

	ticker := time.NewTicker(time.Duration(interval+1) * time.Second)
	defer ticker.Stop()

	expiry := time.Now().Add(time.Duration(expiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Now().After(expiry) {
				ps.logger.Warn("device code expired")
				return
			}

			token, err := checkAccessToken(ctx, deviceCode)
			if err != nil {
				continue // authorization_pending or transient error
			}

			// Save token and init provider
			if saveErr := auth.SaveToken(token); saveErr != nil {
				ps.logger.Warn("failed to save token", zap.Error(saveErr))
			}

			ps.cfg.LLM.CopilotGitHubToken = token
			if ps.TryInit() {
				ps.logger.Info("copilot provider initialized via device flow")
				return
			}

			ps.logger.Error("failed to create provider after obtaining token")
			return
		}
	}
}

// ModelLister is an optional interface that providers can implement to
// expose a dynamic list of available models (e.g. from Copilot's API).
type ModelLister interface {
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// ModelInfo describes a model returned by a ModelLister.
type ModelInfo struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	MaxTokens            int    `json:"max_tokens,omitempty"`
	SupportsToolCalling  bool   `json:"supports_tool_calling,omitempty"`
}

// ModelsHandler returns a handler for GET /api/models.
func ModelsHandler(ps *ProviderState) gin.HandlerFunc {
	return func(c *gin.Context) {
		provider := ps.Get()
		if provider == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		// If the provider can list models dynamically, use that.
		if lister, ok := provider.(ModelLister); ok {
			models, err := lister.ListModels(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("fetching models: %v", err)})
				return
			}
			result := make([]gin.H, 0, len(models))
			for _, m := range models {
				result = append(result, gin.H{
					"id":                    m.ID,
					"name":                  m.Name,
					"description":           m.Description,
					"max_tokens":            m.MaxTokens,
					"supports_tool_calling": m.SupportsToolCalling,
				})
			}
			c.JSON(http.StatusOK, result)
			return
		}

		// Fallback for providers without dynamic model listing:
		// check for the Copilot-specific path (backward compat).
		if cp, ok := provider.(*llm.CopilotProvider); ok {
			models, err := cp.GetCopilotProvider().GetModels(c.Request.Context())
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("fetching models: %v", err)})
				return
			}
			result := make([]gin.H, 0, len(models))
			for _, m := range models {
				result = append(result, gin.H{
					"id":                    m.ID,
					"name":                  m.Name,
					"description":           m.Description,
					"max_tokens":            m.MaxTokens,
					"supports_tool_calling": m.SupportsToolCalling,
				})
			}
			c.JSON(http.StatusOK, result)
			return
		}

		// Generic provider: return current model as a single entry.
		model := ps.GetModel()
		if model == "" {
			c.JSON(http.StatusOK, []gin.H{})
			return
		}
		c.JSON(http.StatusOK, []gin.H{{
			"id":   model,
			"name": model,
		}})
	}
}

// SelectModelHandler returns a handler for POST /api/models/select.
func SelectModelHandler(ps *ProviderState) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Model string `json:"model"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Model == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "model is required"})
			return
		}

		provider := ps.Get()
		if provider == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		// Try to find max_tokens from the provider's model list.
		maxTokens := 0
		if lister, ok := provider.(ModelLister); ok {
			if models, err := lister.ListModels(c.Request.Context()); err == nil {
				for _, m := range models {
					if m.ID == req.Model {
						maxTokens = m.MaxTokens
						break
					}
				}
			}
		} else if cp, ok := provider.(*llm.CopilotProvider); ok {
			if models, err := cp.GetCopilotProvider().GetModels(c.Request.Context()); err == nil {
				for _, m := range models {
					if m.ID == req.Model {
						maxTokens = m.MaxTokens
						break
					}
				}
			}
		}
		if maxTokens == 0 {
			maxTokens = 16384 // reasonable default
		}

		ps.SetModel(req.Model, maxTokens)
		ps.logger.Info("model selected", zap.String("model", req.Model), zap.Int("maxTokens", maxTokens))

		c.JSON(http.StatusOK, gin.H{
			"model":      req.Model,
			"max_tokens": maxTokens,
		})
	}
}

func checkAccessToken(ctx context.Context, deviceCode string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"client_id":   copilot.GitHubClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", copilot.GitHubAccessTokenURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp copilot.GitHubAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf("oauth error: %s", tokenResp.Error)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}
