package claude_test

import (
	"context"
	"fmt"
	"testing"

	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/claude"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newTestClient(t *testing.T, mock *llm.MockProvider) *claude.Client {
	t.Helper()
	cfg := config.ClaudeConfig{
		OpusModel:   "claude-opus-4-6",
		SonnetModel: "claude-sonnet-4-5-20250929",
		MaxRetries:  3,
	}
	logger := zap.NewNop()
	client, err := claude.NewClient(mock, cfg, logger)
	require.NoError(t, err)
	return client
}

func TestAnalyzeStructural_UsesSonnetModel(t *testing.T) {
	mock := &llm.MockProvider{
		Responses: []string{`{"programId": "TESTPROG"}`},
	}
	client := newTestClient(t, mock)

	resp, err := client.AnalyzeStructural(context.Background(), "TEST.CBL", "IDENTIFICATION DIVISION.")
	require.NoError(t, err)
	assert.Contains(t, resp, "TESTPROG")

	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "claude-sonnet-4-5-20250929", mock.Calls[0].Model)
}

func TestAnalyzeDeep_UsesOpusModel(t *testing.T) {
	mock := &llm.MockProvider{
		Responses: []string{`{"performs": []}`},
	}
	client := newTestClient(t, mock)

	chunk := chunker.Chunk{
		FileName: "TEST.CBL",
		Content:  "PROCEDURE DIVISION.",
		FileInfo: graph.FileInfo{Path: "TEST.CBL"},
		Index:    0,
		Total:    1,
	}
	resp, err := client.AnalyzeDeep(context.Background(), chunk, "context preamble")
	require.NoError(t, err)
	assert.Contains(t, resp, "performs")

	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "claude-opus-4-6", mock.Calls[0].Model)
	assert.Contains(t, mock.Calls[0].Messages[1].Content, "context preamble")
}

func TestAnalyzeCrossCutting_UsesOpusModel(t *testing.T) {
	mock := &llm.MockProvider{
		Responses: []string{`{"domains": [], "deadCode": [], "riskFlags": []}`},
	}
	client := newTestClient(t, mock)

	resp, err := client.AnalyzeCrossCutting(context.Background(), "graph data here")
	require.NoError(t, err)
	assert.Contains(t, resp, "domains")

	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "claude-opus-4-6", mock.Calls[0].Model)
	assert.Contains(t, mock.Calls[0].Messages[1].Content, "graph data here")
}

func TestRetryOnError(t *testing.T) {
	callCount := 0
	mock := &llm.MockProvider{
		Responses: []string{`{"programId": "RETRY"}`},
	}
	// Override Complete to fail first 2 times
	originalComplete := mock.Complete
	_ = originalComplete
	failingMock := &failNTimesMock{
		inner:    mock,
		failN:    2,
		err:      fmt.Errorf("transient error"),
	}

	cfg := config.ClaudeConfig{
		OpusModel:   "claude-opus-4-6",
		SonnetModel: "claude-sonnet-4-5-20250929",
		MaxRetries:  3,
	}
	logger := zap.NewNop()
	client, err := claude.NewClient(failingMock, cfg, logger)
	require.NoError(t, err)

	resp, err := client.AnalyzeStructural(context.Background(), "TEST.CBL", "content")
	require.NoError(t, err)
	assert.Contains(t, resp, "RETRY")
	assert.Equal(t, 3, failingMock.callCount) // 2 failures + 1 success
	_ = callCount
}

// failNTimesMock fails the first N calls, then delegates to inner.
type failNTimesMock struct {
	inner     llm.Provider
	failN     int
	callCount int
	err       error
}

func (m *failNTimesMock) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.callCount++
	if m.callCount <= m.failN {
		return nil, m.err
	}
	return m.inner.Complete(ctx, req)
}

func (m *failNTimesMock) Name() string                        { return "fail-mock" }
func (m *failNTimesMock) HealthCheck(_ context.Context) error { return nil }
func (m *failNTimesMock) Close() error                        { return nil }
