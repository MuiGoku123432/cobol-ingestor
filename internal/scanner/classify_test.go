package scanner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/llm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestClassifyByContent_COBOL(t *testing.T) {
	lines := []string{
		"       IDENTIFICATION DIVISION.",
		"       PROGRAM-ID. TESTPROG.",
		"       ENVIRONMENT DIVISION.",
		"       DATA DIVISION.",
		"       PROCEDURE DIVISION.",
		"           DISPLAY 'HELLO'.",
		"           STOP RUN.",
	}
	ft, ok := classifyByContent(lines)
	assert.True(t, ok)
	assert.Equal(t, graph.FileTypeCOBOL, ft)
}

func TestClassifyByContent_JCL(t *testing.T) {
	lines := []string{
		"//MYJOB   JOB (ACCT),'TEST',CLASS=A",
		"//STEP1   EXEC PGM=IEFBR14",
		"//SYSOUT  DD  SYSOUT=*",
		"//SYSIN   DD  DUMMY",
	}
	ft, ok := classifyByContent(lines)
	assert.True(t, ok)
	assert.Equal(t, graph.FileTypeJCL, ft)
}

func TestClassifyByContent_Copybook(t *testing.T) {
	lines := []string{
		"01 WS-CUSTOMER-RECORD.",
		"   05 WS-CUST-ID        PIC 9(5).",
		"   05 WS-CUST-NAME      PIC X(30).",
		"   05 WS-CUST-ADDR      PIC X(50).",
		"   05 WS-CUST-BAL       PIC 9(7)V99.",
		"   05 WS-CUST-TYPE      REDEFINES WS-CUST-BAL PIC X(9).",
	}
	ft, ok := classifyByContent(lines)
	assert.True(t, ok)
	assert.Equal(t, graph.FileTypeCopybook, ft)
}

func TestClassifyByContent_PlainText(t *testing.T) {
	lines := []string{
		"This is a plain text file.",
		"It contains no COBOL or JCL content.",
		"Just regular English prose.",
	}
	_, ok := classifyByContent(lines)
	assert.False(t, ok)
}

func TestClassifyByContent_Empty(t *testing.T) {
	_, ok := classifyByContent(nil)
	assert.False(t, ok)

	_, ok = classifyByContent([]string{})
	assert.False(t, ok)
}

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	responses []string
	calls     int
}

func (m *mockProvider) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.calls >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.calls]
	m.calls++
	return &llm.CompletionResponse{Content: resp}, nil
}

func (m *mockProvider) Name() string                              { return "mock" }
func (m *mockProvider) HealthCheck(_ context.Context) error       { return nil }
func (m *mockProvider) Close() error                              { return nil }

func TestClassifyPendingFiles_Heuristic(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/PROG.CBL", Type: graph.FileTypeCOBOL},
			{Path: "/tmp/COBOL1.txt", Type: graph.FileTypePending},
			{Path: "/tmp/JOB1.txt", Type: graph.FileTypePending},
			{Path: "/tmp/COPY1.txt", Type: graph.FileTypePending},
			{Path: "/tmp/README.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/COBOL1.txt": {
				"       IDENTIFICATION DIVISION.",
				"       PROGRAM-ID. TESTPROG.",
				"       PROCEDURE DIVISION.",
			},
			"/tmp/JOB1.txt": {
				"//MYJOB   JOB (ACCT),'TEST'",
				"//STEP1   EXEC PGM=IEFBR14",
				"//SYSOUT  DD  SYSOUT=*",
			},
			"/tmp/COPY1.txt": {
				"01 WS-REC.",
				"   05 WS-FLD1 PIC X(10).",
				"   05 WS-FLD2 PIC 9(5).",
				"   05 WS-FLD3 PIC X(20).",
			},
			"/tmp/README.txt": {
				"This is a readme file.",
				"No mainframe content here.",
			},
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, nil, "", logger)
	require.NoError(t, err)

	// README.txt should be removed (UNKNOWN), COBOL/JCL/Copybook should be classified
	assert.Len(t, result.Files, 4) // original COBOL + 3 classified

	typeMap := map[string]graph.FileType{}
	for _, f := range result.Files {
		typeMap[f.Path] = f.Type
	}
	assert.Equal(t, graph.FileTypeCOBOL, typeMap["/tmp/PROG.CBL"])
	assert.Equal(t, graph.FileTypeCOBOL, typeMap["/tmp/COBOL1.txt"])
	assert.Equal(t, graph.FileTypeJCL, typeMap["/tmp/JOB1.txt"])
	assert.Equal(t, graph.FileTypeCopybook, typeMap["/tmp/COPY1.txt"])

	// Snippets should be cleaned up
	assert.Empty(t, result.Snippets)
}

func TestClassifyPendingFiles_LLM(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending},
			{Path: "/tmp/FILE2.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
			"/tmp/FILE2.txt": {"//JOB1 JOB"},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "COBOL"}, {"file": "FILE2.txt", "type": "JCL"}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger)
	require.NoError(t, err)

	assert.Len(t, result.Files, 2)
	typeMap := map[string]graph.FileType{}
	for _, f := range result.Files {
		typeMap[f.Path] = f.Type
	}
	assert.Equal(t, graph.FileTypeCOBOL, typeMap["/tmp/FILE1.txt"])
	assert.Equal(t, graph.FileTypeJCL, typeMap["/tmp/FILE2.txt"])
	assert.Equal(t, 1, mock.calls)
}

func TestClassifyPendingFiles_LLM_MarkdownFence(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
		},
	}

	mock := &mockProvider{
		responses: []string{
			"```json\n[{\"file\": \"FILE1.txt\", \"type\": \"COBOL\"}]\n```",
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger)
	require.NoError(t, err)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
}

func TestClassifyPendingFiles_Batching(t *testing.T) {
	logger := zap.NewNop()

	// Create 25 pending files -> should result in ceil(25/12) = 3 batches
	var files []graph.FileInfo
	snippets := make(map[string][]string)
	var responses []string

	for i := 0; i < 25; i++ {
		path := fmt.Sprintf("/tmp/FILE%02d.txt", i)
		files = append(files, graph.FileInfo{Path: path, Type: graph.FileTypePending})
		snippets[path] = []string{"IDENTIFICATION DIVISION.", "PROGRAM-ID. TEST."}
	}

	// Build expected responses for each batch
	for batchStart := 0; batchStart < 25; batchStart += classifyBatchSize {
		batchEnd := batchStart + classifyBatchSize
		if batchEnd > 25 {
			batchEnd = 25
		}
		var items []string
		for i := batchStart; i < batchEnd; i++ {
			items = append(items, fmt.Sprintf(`{"file": "FILE%02d.txt", "type": "COBOL"}`, i))
		}
		responses = append(responses, "["+strings.Join(items, ",")+"]")
	}

	result := &ScanResult{Files: files, Snippets: snippets}
	mock := &mockProvider{responses: responses}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger)
	require.NoError(t, err)

	assert.Equal(t, 3, mock.calls, "should have made 3 batch calls")
	assert.Len(t, result.Files, 25, "all files should remain (classified as COBOL)")
	for _, f := range result.Files {
		assert.Equal(t, graph.FileTypeCOBOL, f.Type)
	}
}

func TestClassifyPendingFiles_NoPending(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/PROG.CBL", Type: graph.FileTypeCOBOL},
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, nil, "", logger)
	require.NoError(t, err)
	assert.Len(t, result.Files, 1)
}

func TestClassifyPendingFiles_LLM_UnknownRemoved(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"This is not mainframe code."},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "UNKNOWN"}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger)
	require.NoError(t, err)
	assert.Empty(t, result.Files, "UNKNOWN files should be removed")
}

func TestMapClassificationType(t *testing.T) {
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType("COBOL"))
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType("cobol"))
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType(" COBOL "))
	assert.Equal(t, graph.FileTypeCopybook, mapClassificationType("COPYBOOK"))
	assert.Equal(t, graph.FileTypeJCL, mapClassificationType("JCL"))
	assert.Equal(t, graph.FileType("UNKNOWN"), mapClassificationType("UNKNOWN"))
	assert.Equal(t, graph.FileType("UNKNOWN"), mapClassificationType("something-else"))
}
