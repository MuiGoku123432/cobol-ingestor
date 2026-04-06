package llm

import (
	"context"
	"fmt"
	"sync"
)

// MockProvider is a test double for the Provider interface.
// It returns pre-configured responses in order and records all calls.
type MockProvider struct {
	Responses    []string            // returned in order, cycling
	Calls        []CompletionRequest // recorded calls
	Error        error               // if set, all calls fail
	StopReason   string              // if set, used as StopReason in responses
	ProviderName string              // if set, returned by Name() instead of "mock"
	callIdx      int
	mu           sync.Mutex
}

func (m *MockProvider) Complete(_ context.Context, req CompletionRequest) (*CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Calls = append(m.Calls, req)

	if m.Error != nil {
		return nil, m.Error
	}

	if len(m.Responses) == 0 {
		return nil, fmt.Errorf("mock: no responses configured")
	}

	resp := m.Responses[m.callIdx%len(m.Responses)]
	m.callIdx++

	stopReason := m.StopReason
	if stopReason == "" {
		stopReason = "end_turn"
	}
	return &CompletionResponse{
		Content:      resp,
		Model:        req.Model,
		PromptTokens: 100,
		OutputTokens: 50,
		StopReason:   stopReason,
		Truncated:    stopReason == "max_tokens",
	}, nil
}

func (m *MockProvider) Name() string {
	if m.ProviderName != "" {
		return m.ProviderName
	}
	return "mock"
}
func (m *MockProvider) HealthCheck(_ context.Context) error { return m.Error }
func (m *MockProvider) Close() error         { return nil }
