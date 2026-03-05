package llm

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/config"
)

// Role constants for chat messages.
const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// Message represents a single chat message sent to or received from the LLM.
type Message struct {
	Role    string
	Content string
}

// CompletionRequest holds the parameters for a chat completion call.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
}

// CompletionResponse holds the result of a chat completion call.
type CompletionResponse struct {
	Content      string
	Model        string
	PromptTokens int
	OutputTokens int
}

// Provider is the interface that both the Anthropic and Copilot backends implement.
type Provider interface {
	// Complete sends a chat completion request and returns the full response.
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Name returns a human-readable provider name (e.g. "anthropic", "copilot").
	Name() string

	// HealthCheck verifies the provider connection is working.
	HealthCheck(ctx context.Context) error

	// Close cleans up any resources held by the provider.
	Close() error
}

// NewProvider creates a Provider based on the configured provider name.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.LLM.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg)
	case "copilot":
		return NewCopilotProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (expected \"anthropic\" or \"copilot\")", cfg.LLM.Provider)
	}
}
