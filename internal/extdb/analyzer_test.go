package extdb

import (
	"context"
	"testing"

	"cobol-ingestor/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockChatProvider implements llm.ChatProvider for testing.
type mockChatProvider struct {
	response *llm.ChatResponse
	err      error
}

func (m *mockChatProvider) Complete(_ context.Context, _ llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, nil
}
func (m *mockChatProvider) Name() string                         { return "mock" }
func (m *mockChatProvider) HealthCheck(_ context.Context) error   { return nil }
func (m *mockChatProvider) Close() error                          { return nil }
func (m *mockChatProvider) CompleteChat(_ context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestAnalyzer_BuildSystemPrompt(t *testing.T) {
	a := &Analyzer{
		dbName: "testdb",
		dbType: "postgres",
	}
	prompt, err := a.buildSystemPrompt()
	require.NoError(t, err)
	assert.Contains(t, prompt, "postgres")
	assert.Contains(t, prompt, "testdb")
	assert.Contains(t, prompt, "cobol_")
	assert.Contains(t, prompt, "extdb_")
}

func TestSchemaToMap(t *testing.T) {
	m := schemaToMap(nil)
	assert.Equal(t, "object", m["type"])

	input := map[string]any{"type": "object", "properties": map[string]any{"foo": "bar"}}
	m = schemaToMap(input)
	assert.Equal(t, "object", m["type"])

	m = schemaToMap("not a map")
	assert.Equal(t, "object", m["type"])
}

func TestMockProviderAnalysis(t *testing.T) {
	resultJSON := `{
		"externalTables": [{"name": "users", "schema": "public", "columns": []}],
		"mappings": [],
		"gaps": [{"side": "external_only", "tableName": "users", "description": "New table"}],
		"dataFlows": []
	}`

	provider := &mockChatProvider{
		response: &llm.ChatResponse{
			Content: []llm.ContentBlock{
				llm.NewTextContent(resultJSON),
			},
			StopReason: "end_turn",
		},
	}

	bridge := &MCPBridge{}
	logger := zap.NewNop()
	a := NewAnalyzer(bridge, provider, "test-model", 5, 4096, "testdb", "postgres", logger)

	result, err := a.Run(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "testdb", result.Database.Name)
	assert.Equal(t, "postgres", result.Database.DatabaseType)
	require.Len(t, result.Tables, 1)
	assert.Equal(t, "users", result.Tables[0].Name)
	require.Len(t, result.Gaps, 1)
	assert.Equal(t, "external_only", result.Gaps[0].Side)
}

func TestMockProviderAnalysis_ErrorFromLLM(t *testing.T) {
	provider := &mockChatProvider{
		err: assert.AnError,
	}

	bridge := &MCPBridge{}
	logger := zap.NewNop()
	a := NewAnalyzer(bridge, provider, "test-model", 2, 4096, "testdb", "postgres", logger)

	_, err := a.Run(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM completion")
}
