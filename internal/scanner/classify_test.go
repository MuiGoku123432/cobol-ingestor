package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"cobol-ingestor/internal/cache"
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

func TestClassifyByContent_COBOL_NoProgramID(t *testing.T) {
	// 3+ division headers but no PROGRAM-ID — should still classify as COBOL
	lines := []string{
		"       ENVIRONMENT DIVISION.",
		"       DATA DIVISION.",
		"       WORKING-STORAGE SECTION.",
		"       PROCEDURE DIVISION.",
		"           PERFORM MAIN-LOGIC.",
		"           STOP RUN.",
	}
	ft, ok := classifyByContent(lines)
	assert.True(t, ok)
	assert.Equal(t, graph.FileTypeCOBOL, ft)
}

func TestClassifyByContent_WeakCOBOL(t *testing.T) {
	// Only 1 COBOL signal, no PROGRAM-ID -> should fallback to COPYBOOK
	lines := []string{
		"       PERFORM SOME-PARAGRAPH.",
	}
	ft, ok := classifyByContent(lines)
	assert.True(t, ok)
	assert.Equal(t, graph.FileTypeCopybook, ft)
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

func TestClassifyByContent_Copybook_Relaxed(t *testing.T) {
	// Only 2 copybook signals (below old threshold of 3) — should now classify
	lines := []string{
		"01 WS-RECORD.",
		"   05 WS-FIELD PIC X(10).",
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

func TestIsLevelNumber_AllLevels(t *testing.T) {
	// All valid COBOL levels: 01-49, 66, 77, 88
	validLevels := []string{
		"01 ", "02 ", "03 ", "04 ", "05 ", "06 ", "07 ", "08 ", "09 ",
		"10 ", "11 ", "12 ", "13 ", "14 ", "15 ", "20 ", "25 ",
		"30 ", "35 ", "40 ", "45 ", "49 ",
		"66 ", "77 ", "88 ",
	}
	for _, lvl := range validLevels {
		assert.True(t, isLevelNumber(lvl+"WS-FIELD"), "expected level %q to match", lvl)
	}

	// Invalid levels
	invalidLevels := []string{"00 ", "50 ", "51 ", "67 ", "78 ", "89 ", "99 "}
	for _, lvl := range invalidLevels {
		assert.False(t, isLevelNumber(lvl+"WS-FIELD"), "expected level %q to NOT match", lvl)
	}
}

func TestTrimCommentHeader(t *testing.T) {
	lines := []string{
		"      * Copyright 2024 ACME Corp",
		"      * All rights reserved.",
		"      *",
		"",
		"       IDENTIFICATION DIVISION.",
		"       PROGRAM-ID. TEST.",
	}
	trimmed := trimCommentHeader(lines)
	require.Len(t, trimmed, 2)
	assert.Contains(t, trimmed[0], "IDENTIFICATION DIVISION")
}

func TestTrimCommentHeader_AllComments(t *testing.T) {
	lines := []string{
		"      * Only comments",
		"      * Nothing else",
	}
	trimmed := trimCommentHeader(lines)
	// When all lines are comments, return original so LLM has something
	assert.Equal(t, lines, trimmed)
}

func TestTrimCommentHeader_NoComments(t *testing.T) {
	lines := []string{
		"       IDENTIFICATION DIVISION.",
		"       PROGRAM-ID. TEST.",
	}
	trimmed := trimCommentHeader(lines)
	assert.Equal(t, lines, trimmed)
}

// mockProvider implements llm.Provider for testing. Thread-safe.
type mockProvider struct {
	mu        sync.Mutex
	responses []string // sequential responses (for single-batch tests)
	calls     int
}

func (m *mockProvider) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.calls >= len(m.responses) {
		return nil, fmt.Errorf("no more mock responses")
	}
	resp := m.responses[m.calls]
	m.calls++
	return &llm.CompletionResponse{Content: resp}, nil
}

func (m *mockProvider) Name() string                        { return "mock" }
func (m *mockProvider) HealthCheck(_ context.Context) error { return nil }
func (m *mockProvider) Close() error                        { return nil }

// dynamicMockProvider responds based on the prompt content — classifies everything as COBOL
// with high confidence.
type dynamicMockProvider struct {
	callCount atomic.Int32
}

// fileHeaderRe extracts just the filename (non-whitespace) from file headers,
// which may include optional hints like "(PDS member name format)".
var fileHeaderRe = regexp.MustCompile(`=== File: (\S+)`)

func (m *dynamicMockProvider) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.callCount.Add(1)
	// Parse filenames from prompt (may be in user message or second message)
	var prompt string
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleUser {
			prompt = msg.Content
			break
		}
	}
	matches := fileHeaderRe.FindAllStringSubmatch(prompt, -1)
	var items []llmClassification
	for _, match := range matches {
		items = append(items, llmClassification{File: match[1], Type: "COBOL", Confidence: 0.95})
	}
	body, _ := json.Marshal(items)
	return &llm.CompletionResponse{Content: string(body)}, nil
}

func (m *dynamicMockProvider) Name() string                        { return "dynamic-mock" }
func (m *dynamicMockProvider) HealthCheck(_ context.Context) error { return nil }
func (m *dynamicMockProvider) Close() error                        { return nil }

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

	err := ClassifyPendingFiles(context.Background(), result, nil, "", logger, nil)
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
			`[{"file": "FILE1.txt", "type": "COBOL", "confidence": 0.95}, {"file": "FILE2.txt", "type": "JCL", "confidence": 0.90}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
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
			"```json\n[{\"file\": \"FILE1.txt\", \"type\": \"COBOL\", \"confidence\": 0.95}]\n```",
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
}

func TestClassifyPendingFiles_Batching(t *testing.T) {
	logger := zap.NewNop()

	// Create 25 pending files -> should result in ceil(25/8) = 4 batches on attempt 1
	var files []graph.FileInfo
	snippets := make(map[string][]string)

	for i := 0; i < 25; i++ {
		path := fmt.Sprintf("/tmp/FILE%02d.txt", i)
		files = append(files, graph.FileInfo{Path: path, Type: graph.FileTypePending})
		snippets[path] = []string{"IDENTIFICATION DIVISION.", "PROGRAM-ID. TEST."}
	}

	result := &ScanResult{Files: files, Snippets: snippets}
	mock := &dynamicMockProvider{}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	// All files should be classified on attempt 1 (confidence 0.95 >= 0.7), so only 4 batches
	assert.Equal(t, int32(4), mock.callCount.Load(), "should have made 4 batch calls")
	assert.Len(t, result.Files, 25, "all files should remain (classified as COBOL)")
	for _, f := range result.Files {
		assert.Equal(t, graph.FileTypeCOBOL, f.Type)
	}
}

func TestClassifyPendingFiles_Batching_Large(t *testing.T) {
	logger := zap.NewNop()

	// Simulate a large codebase: 500 pending .txt files
	const n = 500
	var files []graph.FileInfo
	snippets := make(map[string][]string, n)

	for i := 0; i < n; i++ {
		path := fmt.Sprintf("/tmp/PROG%04d.txt", i)
		files = append(files, graph.FileInfo{Path: path, Type: graph.FileTypePending})
		snippets[path] = []string{"IDENTIFICATION DIVISION.", "PROGRAM-ID. TEST."}
	}

	result := &ScanResult{Files: files, Snippets: snippets}
	mock := &dynamicMockProvider{}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	expectedBatches := (n + classifyBatchSize - 1) / classifyBatchSize // ceil division
	assert.Equal(t, int32(expectedBatches), mock.callCount.Load())
	assert.Len(t, result.Files, n)
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

	err := ClassifyPendingFiles(context.Background(), result, nil, "", logger, nil)
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

	// UNKNOWN with high confidence — should not retry (accepted as UNKNOWN)
	// Provide 3 responses since UNKNOWN triggers retry (need all 3 attempts)
	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "UNKNOWN", "confidence": 0.90, "reasoning": "no mainframe signals"}]`,
			`[{"file": "FILE1.txt", "type": "UNKNOWN", "confidence": 0.90, "reasoning": "no mainframe signals"}]`,
			`[{"file": "FILE1.txt", "type": "UNKNOWN", "confidence": 0.90, "reasoning": "no mainframe signals"}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)
	assert.Empty(t, result.Files, "UNKNOWN files should be removed")
}

func TestClassifyPendingFiles_LLM_FallbackOnError(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/COBOL1.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/COBOL1.txt": {
				"       IDENTIFICATION DIVISION.",
				"       PROGRAM-ID. TEST.",
				"       PROCEDURE DIVISION.",
			},
		},
	}

	// Provider returns an error -> should fall back to heuristic after 3 attempts
	mock := &mockProvider{responses: nil} // will error immediately

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	assert.Error(t, err, "should report the LLM error")
	// But the file should still be classified via heuristic fallback
	assert.Len(t, result.Files, 1)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
}

func TestMapClassificationType(t *testing.T) {
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType("COBOL"))
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType("cobol"))
	assert.Equal(t, graph.FileTypeCOBOL, mapClassificationType(" COBOL "))
	assert.Equal(t, graph.FileTypeCopybook, mapClassificationType("COPYBOOK"))
	assert.Equal(t, graph.FileTypeJCL, mapClassificationType("JCL"))
	assert.Equal(t, graph.FileType("UNKNOWN"), mapClassificationType("UNKNOWN"))
	// Open-ended: unknown types pass through as uppercase
	assert.Equal(t, graph.FileType("BMS"), mapClassificationType("BMS"))
	assert.Equal(t, graph.FileType("BMS"), mapClassificationType("bms"))
	assert.Equal(t, graph.FileType("DCLGEN"), mapClassificationType("DCLGEN"))
	assert.Equal(t, graph.FileType("ASM"), mapClassificationType("asm"))
	assert.Equal(t, graph.FileType("REXX"), mapClassificationType("rexx"))
	assert.Equal(t, graph.FileType("SOMETHING-ELSE"), mapClassificationType("something-else"))
	assert.Equal(t, graph.FileType("UNKNOWN"), mapClassificationType(""))
	assert.Equal(t, graph.FileType("UNKNOWN"), mapClassificationType("  "))
}

func TestParseClassifyResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{"plain json", `[{"file":"a.txt","type":"COBOL"}]`, 1, false},
		{"markdown fence", "```json\n[{\"file\":\"a.txt\",\"type\":\"JCL\"}]\n```", 1, false},
		{"empty array", "[]", 0, false},
		{"invalid json", "not json at all and no braces", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseClassifyResponse(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.wantLen)
			}
		})
	}
}

func TestBuildClassifyPrompt(t *testing.T) {
	batch := []string{"/tmp/A.txt", "/tmp/B.txt"}
	snippets := map[string][]string{
		"/tmp/A.txt": {"line1", "line2"},
		"/tmp/B.txt": {"line3"},
	}
	prompt := buildClassifyPrompt(batch, snippets, false)
	assert.Contains(t, prompt, "=== File: A.txt")
	assert.Contains(t, prompt, "=== File: B.txt")
	assert.Contains(t, prompt, "line1")
	assert.Contains(t, prompt, "line3")
	assert.Contains(t, prompt, "mainframe source type")
	assert.Contains(t, prompt, "Tiebreaker")
	assert.Contains(t, prompt, "BMS")
	assert.Contains(t, prompt, "DCLGEN")
	assert.Contains(t, prompt, "Confidence Rubric")
	assert.Contains(t, prompt, "confidence")
	assert.Contains(t, prompt, "reasoning")
	assert.True(t, strings.HasPrefix(prompt, "Classify each file snippet"))
}

// panicProvider panics if Complete is called — used to verify cache hits avoid LLM calls.
type panicProvider struct{}

func (p *panicProvider) Complete(_ context.Context, _ llm.CompletionRequest) (*llm.CompletionResponse, error) {
	panic("LLM should not be called when cache hits cover all files")
}
func (p *panicProvider) Name() string                        { return "panic" }
func (p *panicProvider) HealthCheck(_ context.Context) error { return nil }
func (p *panicProvider) Close() error                        { return nil }

func TestClassifyPendingFiles_CacheHit(t *testing.T) {
	logger := zap.NewNop()
	dbPath := filepath.Join(t.TempDir(), "classify_hit.sqlite")
	c, err := cache.New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// Pre-populate cache
	require.NoError(t, c.BatchMarkClassified([]cache.ClassifyEntry{
		{Path: "/tmp/FILE1.txt", Hash: "hash1", FileType: "COBOL", Classifier: "LLM", Confidence: 0.95},
		{Path: "/tmp/FILE2.txt", Hash: "hash2", FileType: "JCL", Classifier: "LLM", Confidence: 0.90},
	}))

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending, Hash: "hash1"},
			{Path: "/tmp/FILE2.txt", Type: graph.FileTypePending, Hash: "hash2"},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
			"/tmp/FILE2.txt": {"//JOB1 JOB"},
		},
	}

	// panicProvider ensures LLM is never called
	err = ClassifyPendingFiles(context.Background(), result, &panicProvider{}, "test-model", logger, c)
	require.NoError(t, err)

	assert.Len(t, result.Files, 2)
	typeMap := map[string]graph.FileType{}
	for _, f := range result.Files {
		typeMap[f.Path] = f.Type
	}
	assert.Equal(t, graph.FileTypeCOBOL, typeMap["/tmp/FILE1.txt"])
	assert.Equal(t, graph.FileTypeJCL, typeMap["/tmp/FILE2.txt"])
}

func TestClassifyPendingFiles_CacheMiss_ThenPopulated(t *testing.T) {
	logger := zap.NewNop()
	dbPath := filepath.Join(t.TempDir(), "classify_miss.sqlite")
	c, err := cache.New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending, Hash: "hash1"},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "COBOL", "confidence": 0.95}]`,
		},
	}

	err = ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, c)
	require.NoError(t, err)

	assert.Len(t, result.Files, 1)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)

	// Verify cache was populated
	hits, misses, err := c.BatchLookupClassification(map[string]string{"/tmp/FILE1.txt": "hash1"})
	require.NoError(t, err)
	assert.Len(t, hits, 1)
	assert.Empty(t, misses)
	assert.Equal(t, "COBOL", hits["/tmp/FILE1.txt"].FileType)
	assert.Equal(t, "LLM", hits["/tmp/FILE1.txt"].Classifier)
}

func TestClassifyPendingFiles_CacheStale(t *testing.T) {
	logger := zap.NewNop()
	dbPath := filepath.Join(t.TempDir(), "classify_stale.sqlite")
	c, err := cache.New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// Pre-populate cache with old hash
	require.NoError(t, c.BatchMarkClassified([]cache.ClassifyEntry{
		{Path: "/tmp/FILE1.txt", Hash: "old-hash", FileType: "JCL", Classifier: "LLM", Confidence: 0.90},
	}))

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending, Hash: "new-hash"},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "COBOL", "confidence": 0.95}]`,
		},
	}

	err = ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, c)
	require.NoError(t, err)

	assert.Len(t, result.Files, 1)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
	assert.Equal(t, 1, mock.calls, "LLM should be called for stale cache entry")

	// Verify cache was updated with new hash
	hits, _, err := c.BatchLookupClassification(map[string]string{"/tmp/FILE1.txt": "new-hash"})
	require.NoError(t, err)
	assert.Equal(t, "COBOL", hits["/tmp/FILE1.txt"].FileType)
}

func TestClassifyPendingFiles_OpenEndedType(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/MAPDEF.txt", Type: graph.FileTypePending, Hash: "hash1"},
			{Path: "/tmp/ASMMOD.txt", Type: graph.FileTypePending, Hash: "hash2"},
		},
		Snippets: map[string][]string{
			"/tmp/MAPDEF.txt": {"DFHMSD TYPE=DSECT"},
			"/tmp/ASMMOD.txt": {"         CSECT"},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "MAPDEF.txt", "type": "BMS", "confidence": 0.90}, {"file": "ASMMOD.txt", "type": "ASM", "confidence": 0.85}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	// BMS and ASM should be kept (not filtered as UNKNOWN)
	assert.Len(t, result.Files, 2)
	typeMap := map[string]graph.FileType{}
	for _, f := range result.Files {
		typeMap[f.Path] = f.Type
	}
	assert.Equal(t, graph.FileType("BMS"), typeMap["/tmp/MAPDEF.txt"])
	assert.Equal(t, graph.FileType("ASM"), typeMap["/tmp/ASMMOD.txt"])
}

// === New tests for progressive retry, enhanced prompt, etc. ===

func TestClassifyWithLLMProgressive_AllConfident(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/A.txt", Type: graph.FileTypePending},
			{Path: "/tmp/B.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/A.txt": {"IDENTIFICATION DIVISION."},
			"/tmp/B.txt": {"//JOB1 JOB"},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "A.txt", "type": "COBOL", "confidence": 0.95}, {"file": "B.txt", "type": "JCL", "confidence": 0.92}]`,
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	assert.Equal(t, 1, mock.calls, "should only need 1 LLM call (no retry)")
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
	assert.Equal(t, graph.FileTypeJCL, result.Files[1].Type)
	assert.InDelta(t, 0.95, result.Files[0].Confidence, 0.01)
	assert.Equal(t, "LLM", result.Files[0].Classifier)
}

// confidenceMockProvider returns configurable confidence per attempt.
type confidenceMockProvider struct {
	mu        sync.Mutex
	callCount int
	// responses maps attempt number (0-based) to response generator
	attemptResponses map[int]func(filenames []string) string
}

func (m *confidenceMockProvider) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var prompt string
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleUser {
			prompt = msg.Content
			break
		}
	}

	matches := fileHeaderRe.FindAllStringSubmatch(prompt, -1)
	var filenames []string
	for _, match := range matches {
		filenames = append(filenames, match[1])
	}

	attempt := m.callCount
	m.callCount++

	if gen, ok := m.attemptResponses[attempt]; ok {
		return &llm.CompletionResponse{Content: gen(filenames)}, nil
	}

	// Default: return COBOL with high confidence
	var items []llmClassification
	for _, name := range filenames {
		items = append(items, llmClassification{File: name, Type: "COBOL", Confidence: 0.95})
	}
	body, _ := json.Marshal(items)
	return &llm.CompletionResponse{Content: string(body)}, nil
}

func (m *confidenceMockProvider) Name() string                        { return "confidence-mock" }
func (m *confidenceMockProvider) HealthCheck(_ context.Context) error { return nil }
func (m *confidenceMockProvider) Close() error                        { return nil }

func TestClassifyWithLLMProgressive_RetryOnLowConfidence(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/A.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/A.txt": {"SOME AMBIGUOUS CONTENT"},
		},
	}

	mock := &confidenceMockProvider{
		attemptResponses: map[int]func([]string) string{
			0: func(filenames []string) string {
				// Attempt 1: low confidence
				items := []llmClassification{{File: filenames[0], Type: "COBOL", Confidence: 0.55}}
				body, _ := json.Marshal(items)
				return string(body)
			},
			1: func(filenames []string) string {
				// Attempt 2: high confidence
				items := []llmClassification{{File: filenames[0], Type: "COBOL", Confidence: 0.90}}
				body, _ := json.Marshal(items)
				return string(body)
			},
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	assert.Equal(t, 2, mock.callCount, "should have 2 LLM calls (attempt 1 low confidence + attempt 2)")
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
	assert.InDelta(t, 0.90, result.Files[0].Confidence, 0.01)
}

func TestClassifyWithLLMProgressive_ThreeAttempts(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/A.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/A.txt": {"VERY AMBIGUOUS CONTENT"},
		},
	}

	mock := &confidenceMockProvider{
		attemptResponses: map[int]func([]string) string{
			0: func(filenames []string) string {
				items := []llmClassification{{File: filenames[0], Type: "COBOL", Confidence: 0.50}}
				body, _ := json.Marshal(items)
				return string(body)
			},
			1: func(filenames []string) string {
				items := []llmClassification{{File: filenames[0], Type: "COBOL", Confidence: 0.60}}
				body, _ := json.Marshal(items)
				return string(body)
			},
			2: func(filenames []string) string {
				items := []llmClassification{{File: filenames[0], Type: "COPYBOOK", Confidence: 0.85}}
				body, _ := json.Marshal(items)
				return string(body)
			},
		},
	}

	err := ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, nil)
	require.NoError(t, err)

	assert.Equal(t, 3, mock.callCount, "should need all 3 attempts")
	assert.Equal(t, graph.FileTypeCopybook, result.Files[0].Type)
	assert.InDelta(t, 0.85, result.Files[0].Confidence, 0.01)
}

func TestClassifyWithLLMProgressive_HeuristicOnlyOnError(t *testing.T) {
	logger := zap.NewNop()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/COBOL1.txt", Type: graph.FileTypePending},
		},
		Snippets: map[string][]string{
			"/tmp/COBOL1.txt": {
				"       IDENTIFICATION DIVISION.",
				"       PROGRAM-ID. TEST.",
				"       PROCEDURE DIVISION.",
			},
		},
	}

	// All LLM calls error out → heuristic should take over
	errorProvider := &mockProvider{responses: nil}

	err := ClassifyPendingFiles(context.Background(), result, errorProvider, "test-model", logger, nil)
	assert.Error(t, err)

	assert.Len(t, result.Files, 1)
	assert.Equal(t, graph.FileTypeCOBOL, result.Files[0].Type)
	assert.Equal(t, "HEURISTIC", result.Files[0].Classifier)
}

func TestBuildClassifyPrompt_Enhanced(t *testing.T) {
	batch := []string{"/tmp/TEST.txt"}
	snippets := map[string][]string{
		"/tmp/TEST.txt": {"IDENTIFICATION DIVISION."},
	}
	prompt := buildClassifyPrompt(batch, snippets, false)

	assert.Contains(t, prompt, "Confidence Rubric")
	assert.Contains(t, prompt, "0.95-1.0")
	assert.Contains(t, prompt, "reasoning")
	assert.Contains(t, prompt, "Classification Rules")
	assert.Contains(t, prompt, "PROGRAM-ID present")
}

func TestBuildClassifyPrompt_FilenameHints(t *testing.T) {
	batch := []string{"/tmp/CPYACCT.txt", "/tmp/DCLCUST.txt"}
	snippets := map[string][]string{
		"/tmp/CPYACCT.txt": {"01 WS-ACCT."},
		"/tmp/DCLCUST.txt": {"EXEC SQL DECLARE"},
	}
	prompt := buildClassifyPrompt(batch, snippets, false)

	assert.Contains(t, prompt, "name suggests: COPYBOOK")
	assert.Contains(t, prompt, "name suggests: DCLGEN")
}

func TestFilenameHint(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"CPYACCT.txt", "name suggests: COPYBOOK"},
		{"COPYDEF.txt", "name suggests: COPYBOOK"},
		{"DCLCUST.txt", "name suggests: DCLGEN"},
		{"MAPLOG.txt", "name suggests: BMS"},
		{"BMSMAP.txt", "name suggests: BMS"},
		{"PROCJOB.txt", "name suggests: PROC"},
		{"TESTPROG.txt", "PDS member name format"},
		{"readme-file.txt", ""},             // not a PDS name
		{"verylongfilename.txt", ""},        // too long
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filenameHint(tt.name)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseClassifyResponse_WithConfidence(t *testing.T) {
	input := `[{"file": "A.txt", "type": "COBOL", "confidence": 0.95, "reasoning": "PROGRAM-ID found"}]`
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "A.txt", result[0].File)
	assert.Equal(t, "COBOL", result[0].Type)
	assert.InDelta(t, 0.95, result[0].Confidence, 0.01)
	assert.Equal(t, "PROGRAM-ID found", result[0].Reasoning)
}

func TestParseClassifyResponse_SingleObject(t *testing.T) {
	input := `{"file": "A.txt", "type": "JCL", "confidence": 0.88}`
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "JCL", result[0].Type)
}

func TestParseClassifyResponse_PreambleText(t *testing.T) {
	input := `Here are the classifications:\n[{"file": "A.txt", "type": "COBOL", "confidence": 0.90}]`
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "COBOL", result[0].Type)
}

func TestSmartSnippet_LargeFile(t *testing.T) {
	// Build a 300-line file
	var allLines []string
	for i := 0; i < 300; i++ {
		allLines = append(allLines, fmt.Sprintf("LINE %d CONTENT", i+1))
	}

	snippet := buildSmartSnippet(allLines, 150)

	// Should contain first line (head region)
	assert.Contains(t, snippet[0], "LINE 1")

	// Should contain middle separator
	found := false
	for _, line := range snippet {
		if strings.HasPrefix(line, "--- [middle of file") {
			found = true
			break
		}
	}
	assert.True(t, found, "should contain middle separator")

	// Should contain tail separator
	foundTail := false
	for _, line := range snippet {
		if strings.HasPrefix(line, "--- [end of file") {
			foundTail = true
			break
		}
	}
	assert.True(t, foundTail, "should contain end separator")

	// Should contain last line
	assert.Contains(t, snippet[len(snippet)-1], "LINE 300")

	// Total snippet should be roughly around maxLines + separators
	assert.Greater(t, len(snippet), 100, "should capture substantial content")
	assert.Less(t, len(snippet), 200, "should not capture entire file")
}

func TestSmartSnippet_SmallFile(t *testing.T) {
	allLines := []string{"line1", "line2", "line3"}
	snippet := buildSmartSnippet(allLines, 150)
	assert.Equal(t, allLines, snippet, "small file should return all lines")
}

func TestReadLargerSnippet(t *testing.T) {
	dir := t.TempDir()

	// Create a file with 200 lines
	var content strings.Builder
	for i := 0; i < 200; i++ {
		content.WriteString(fmt.Sprintf("LINE %d CONTENT\n", i+1))
	}
	path := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(path, []byte(content.String()), 0644))

	snippet, err := readLargerSnippet(path, 400)
	require.NoError(t, err)

	// File has 200 lines, maxLines=400, so entire file should be returned
	assert.Len(t, snippet, 200)
	assert.Contains(t, snippet[0], "LINE 1")
	assert.Contains(t, snippet[199], "LINE 200")
}

func TestScan_ContentDetect_NoExtension(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()

	// File with no extension but COBOL content
	cobolContent := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. TESTPROG.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "TESTPROG"), []byte(cobolContent), 0644))

	result, err := Scan(context.Background(), dir, logger, ScanOptions{ContentDetect: true})
	require.NoError(t, err)

	assert.Len(t, result.Files, 1, "extensionless file should be captured")
	assert.Equal(t, graph.FileTypePending, result.Files[0].Type)
	assert.NotEmpty(t, result.Snippets)
}

func TestScan_ContentDetect_DatFile(t *testing.T) {
	logger := zap.NewNop()
	dir := t.TempDir()

	cobolContent := "       IDENTIFICATION DIVISION.\n       PROGRAM-ID. DATPROG.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "DATPROG.dat"), []byte(cobolContent), 0644))

	result, err := Scan(context.Background(), dir, logger, ScanOptions{ContentDetect: true})
	require.NoError(t, err)

	assert.Len(t, result.Files, 1, ".dat file should be captured")
	assert.Equal(t, graph.FileTypePending, result.Files[0].Type)
}

func TestCacheConfidence(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "conf_cache.sqlite")
	c, err := cache.New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	// Store entry with confidence
	require.NoError(t, c.BatchMarkClassified([]cache.ClassifyEntry{
		{Path: "/tmp/A.txt", Hash: "hash1", FileType: "COBOL", Classifier: "LLM", Confidence: 0.95},
	}))

	// Retrieve and verify confidence round-trips
	hits, _, err := c.BatchLookupClassification(map[string]string{"/tmp/A.txt": "hash1"})
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.InDelta(t, 0.95, hits["/tmp/A.txt"].Confidence, 0.01)
}

func TestLLMClassification_ConfidenceStored(t *testing.T) {
	logger := zap.NewNop()
	dbPath := filepath.Join(t.TempDir(), "conf_store.sqlite")
	c, err := cache.New(dbPath)
	require.NoError(t, err)
	defer c.Close()

	result := &ScanResult{
		Files: []graph.FileInfo{
			{Path: "/tmp/FILE1.txt", Type: graph.FileTypePending, Hash: "hash1"},
		},
		Snippets: map[string][]string{
			"/tmp/FILE1.txt": {"IDENTIFICATION DIVISION."},
		},
	}

	mock := &mockProvider{
		responses: []string{
			`[{"file": "FILE1.txt", "type": "COBOL", "confidence": 0.92}]`,
		},
	}

	err = ClassifyPendingFiles(context.Background(), result, mock, "test-model", logger, c)
	require.NoError(t, err)

	// Verify FileInfo has confidence
	assert.InDelta(t, 0.92, result.Files[0].Confidence, 0.01)
	assert.Equal(t, "LLM", result.Files[0].Classifier)

	// Verify cache has confidence
	hits, _, err := c.BatchLookupClassification(map[string]string{"/tmp/FILE1.txt": "hash1"})
	require.NoError(t, err)
	assert.InDelta(t, 0.92, hits["/tmp/FILE1.txt"].Confidence, 0.01)
}

// === Tests for extractJSONBlock and robust parsing ===

func TestExtractJSONBlock(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantOK  bool
	}{
		{
			"balanced array",
			`[{"file":"a.txt","type":"COBOL"}]`,
			`[{"file":"a.txt","type":"COBOL"}]`,
			true,
		},
		{
			"balanced object",
			`{"file":"a.txt","type":"COBOL"}`,
			`{"file":"a.txt","type":"COBOL"}`,
			true,
		},
		{
			"nested brackets",
			`[{"arr":[1,2],"obj":{"k":"v"}}]`,
			`[{"arr":[1,2],"obj":{"k":"v"}}]`,
			true,
		},
		{
			"no JSON",
			`This is plain text with no brackets`,
			"",
			false,
		},
		{
			"brackets in strings",
			`[{"note":"has [brackets] inside"}]`,
			`[{"note":"has [brackets] inside"}]`,
			true,
		},
		{
			"trailing text after array",
			`[{"file":"a.txt"}]\n\nNote: I classified this based on signals.`,
			`[{"file":"a.txt"}]`,
			true,
		},
		{
			"preamble and trailing text",
			`Here are the results:\n[{"file":"a.txt","type":"COBOL"}]\nLet me know if you need more.`,
			`[{"file":"a.txt","type":"COBOL"}]`,
			true,
		},
		{
			"escaped quotes in strings",
			`[{"note":"say \"hello\""}]`,
			`[{"note":"say \"hello\""}]`,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractJSONBlock(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseClassifyResponse_TrailingText(t *testing.T) {
	input := "[{\"file\":\"A.txt\",\"type\":\"COBOL\",\"confidence\":0.95}]\n\nNote: I classified this based on the PROGRAM-ID statement."
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "COBOL", result[0].Type)
	assert.InDelta(t, 0.95, result[0].Confidence, 0.01)
}

func TestParseClassifyResponse_PreambleAndTrailingText(t *testing.T) {
	input := "Here are my classifications:\n[{\"file\":\"A.txt\",\"type\":\"JCL\",\"confidence\":0.88}]\nLet me know if you need anything else."
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "JCL", result[0].Type)
}

func TestParseClassifyResponse_BracketInProse(t *testing.T) {
	input := "[word] then [{\"file\":\"A.txt\",\"type\":\"COBOL\",\"confidence\":0.90}]"
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "COBOL", result[0].Type)
}

func TestParseClassifyResponse_SingleObjectTrailingText(t *testing.T) {
	input := "{\"file\":\"A.txt\",\"type\":\"COPYBOOK\",\"confidence\":0.85} and here is my reasoning."
	result, err := parseClassifyResponse(input)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "COPYBOOK", result[0].Type)
}

func TestBuildClassifyPrompt_AntiUnknown(t *testing.T) {
	batch := []string{"/tmp/TEST.txt"}
	snippets := map[string][]string{"/tmp/TEST.txt": {"line1"}}
	prompt := buildClassifyPrompt(batch, snippets, false)

	assert.Contains(t, prompt, "NEVER return UNKNOWN")
	assert.NotContains(t, prompt, "consider UNKNOWN")
}

func TestBuildClassifyPrompt_ShortFileGuidance(t *testing.T) {
	batch := []string{"/tmp/TEST.txt"}
	snippets := map[string][]string{"/tmp/TEST.txt": {"line1"}}
	prompt := buildClassifyPrompt(batch, snippets, false)

	assert.Contains(t, prompt, "Short File Guidance")
	assert.Contains(t, prompt, "Short does not mean UNKNOWN")
}

func TestBuildClassifyPrompt_ExampleTypes(t *testing.T) {
	batch := []string{"/tmp/TEST.txt"}
	snippets := map[string][]string{"/tmp/TEST.txt": {"line1"}}
	prompt := buildClassifyPrompt(batch, snippets, false)

	assert.Contains(t, prompt, "EASYTRIEVE")
	assert.Contains(t, prompt, "CONTROL")
	assert.Contains(t, prompt, "IDMS")
	assert.Contains(t, prompt, "ADABAS")
}

func TestBuildRetryClassifyPrompt_NoUnknown(t *testing.T) {
	batch := []string{"/tmp/TEST.txt"}
	snippets := map[string][]string{"/tmp/TEST.txt": {"line1"}}
	prevResults := map[string]*llmClassification{
		"/tmp/TEST.txt": {File: "TEST.txt", Type: "UNKNOWN", Confidence: 0.40},
	}
	prompt := buildRetryClassifyPrompt(batch, snippets, prevResults, 2)

	assert.Contains(t, prompt, "Do NOT return UNKNOWN")
	assert.NotContains(t, prompt, "consider UNKNOWN")
}
