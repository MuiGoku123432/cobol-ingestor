package tokencount_test

import (
	"strings"
	"testing"

	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/tokencount"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeuristicCounterMatchesChunker(t *testing.T) {
	samples := []string{
		"IDENTIFICATION DIVISION.",
		"PERFORM VARYING WS-IDX FROM 1 BY 1 UNTIL WS-IDX > WS-MAX",
		strings.Repeat("MOVE WS-ACCOUNT-NUMBER TO WS-OUTPUT-FIELD\n", 100),
		"",
	}
	h := tokencount.HeuristicCounter{}
	for _, s := range samples {
		assert.Equal(t, chunker.EstimateTokens(s), h.Count(s),
			"HeuristicCounter should match chunker.EstimateTokens for %q", s[:min(len(s), 40)])
	}
}

func TestBPECounterReturnsPositive(t *testing.T) {
	bpe, err := tokencount.NewBPECounter()
	require.NoError(t, err)

	cobol := `IDENTIFICATION DIVISION.
PROGRAM-ID. ACCTPROC.
ENVIRONMENT DIVISION.
DATA DIVISION.
WORKING-STORAGE SECTION.
01 WS-ACCOUNT-NUMBER PIC 9(10).
01 WS-BALANCE        PIC S9(15)V99 COMP-3.
PROCEDURE DIVISION.
    PERFORM VARYING WS-IDX FROM 1 BY 1 UNTIL WS-IDX > 100
        MOVE WS-ACCOUNT-NUMBER TO WS-OUTPUT-FIELD
    END-PERFORM.
    STOP RUN.`

	count := bpe.Count(cobol)
	assert.Greater(t, count, 0, "BPE count should be positive for non-empty COBOL text")
}

func TestBPECounterDiffersFromHeuristic(t *testing.T) {
	bpe, err := tokencount.NewBPECounter()
	require.NoError(t, err)
	h := tokencount.HeuristicCounter{}

	// For non-trivial content the two methods should produce different results.
	cobol := strings.Repeat("PERFORM VARYING WS-IDX FROM 1 BY 1 UNTIL WS-IDX > WS-MAX\n", 50)
	bpeCount := bpe.Count(cobol)
	heuristicCount := h.Count(cobol)

	assert.NotEqual(t, bpeCount, heuristicCount,
		"BPE and heuristic should differ for substantial COBOL text")
}

func TestCountOrHeuristicNilFallsBack(t *testing.T) {
	content := "MOVE WS-ACCOUNT-NUMBER TO WS-OUTPUT-FIELD"
	result := tokencount.CountOrHeuristic(nil, content)
	assert.Equal(t, chunker.EstimateTokens(content), result,
		"CountOrHeuristic with nil counter should match chunker.EstimateTokens")
}

func TestCountOrHeuristicUsesBPE(t *testing.T) {
	bpe, err := tokencount.NewBPECounter()
	require.NoError(t, err)

	content := "MOVE WS-ACCOUNT-NUMBER TO WS-OUTPUT-FIELD"
	result := tokencount.CountOrHeuristic(bpe, content)
	assert.Equal(t, bpe.Count(content), result,
		"CountOrHeuristic with BPE counter should use BPE")
}

func TestMethodLabel(t *testing.T) {
	bpe, err := tokencount.NewBPECounter()
	require.NoError(t, err)

	assert.Contains(t, tokencount.MethodLabel(nil), "heuristic")
	assert.Contains(t, tokencount.MethodLabel(bpe), "BPE")
}

func BenchmarkHeuristicCounter(b *testing.B) {
	content := strings.Repeat("PERFORM VARYING WS-IDX FROM 1 BY 1 UNTIL WS-IDX > WS-MAX\n", 1000)
	h := tokencount.HeuristicCounter{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h.Count(content)
	}
}

func BenchmarkBPECounter(b *testing.B) {
	bpe, err := tokencount.NewBPECounter()
	require.NoError(b, err)
	content := strings.Repeat("PERFORM VARYING WS-IDX FROM 1 BY 1 UNTIL WS-IDX > WS-MAX\n", 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bpe.Count(content)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
