package llm

import (
	"context"
	"errors"
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
	StopReason   string // "end_turn", "max_tokens", "stop_sequence"
	Truncated    bool   // true when StopReason == "max_tokens"
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

// LLMError wraps an LLM API error with classification metadata.
type LLMError struct {
	StatusCode int
	Retriable  bool
	Message    string
	Err        error
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("LLM error (status=%d, retriable=%v): %s", e.StatusCode, e.Retriable, e.Message)
}

func (e *LLMError) Unwrap() error {
	return e.Err
}

// IsRetriable checks if an error is an LLMError that should be retried.
func IsRetriable(err error) bool {
	var llmErr *LLMError
	if errors.As(err, &llmErr) {
		return llmErr.Retriable
	}
	// Unknown errors are retriable by default (network issues, etc.)
	return true
}

// NewProvider creates a Provider based on the configured provider name.
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.LLM.Provider {
	case "anthropic":
		return NewAnthropicProvider(cfg)
	case "copilot":
		return NewCopilotProvider(cfg)
	case "vertex":
		return NewVertexProvider(cfg)
	case "bedrock":
		return NewBedrockProvider(cfg)
	case "openai":
		return NewOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q (expected \"anthropic\", \"copilot\", \"vertex\", \"bedrock\", or \"openai\")", cfg.LLM.Provider)
	}
}
