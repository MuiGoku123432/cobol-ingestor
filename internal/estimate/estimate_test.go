package estimate

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/graph"
	"cobol-ingestor/internal/scanner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testConfig() *config.Config {
	cfg, _ := config.Load()
	return cfg
}

func testLogger() *zap.Logger {
	l, _ := zap.NewDevelopment()
	return l
}

func makeScanResult(cobol, jcl, copybook int) *scanner.ScanResult {
	sr := &scanner.ScanResult{}
	for i := 0; i < cobol; i++ {
		sr.Files = append(sr.Files, graph.FileInfo{
			Path: "/fake/prog" + strings.Repeat("X", i%10) + ".cbl",
			Type: graph.FileTypeCOBOL,
			Hash: "abc123",
			Size: 5000,
		})
	}
	for i := 0; i < jcl; i++ {
		sr.Files = append(sr.Files, graph.FileInfo{
			Path: "/fake/job" + strings.Repeat("J", i%5) + ".jcl",
			Type: graph.FileTypeJCL,
			Hash: "def456",
			Size: 2000,
		})
	}
	for i := 0; i < copybook; i++ {
		sr.Files = append(sr.Files, graph.FileInfo{
			Path: "/fake/copy" + strings.Repeat("C", i%5) + ".cpy",
			Type: graph.FileTypeCopybook,
			Hash: "ghi789",
			Size: 1000,
		})
	}
	return sr
}

func TestFileCounts(t *testing.T) {
	sr := makeScanResult(10, 3, 5)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	assert.Equal(t, 10, r.Files.COBOL)
	assert.Equal(t, 3, r.Files.JCL)
	assert.Equal(t, 5, r.Files.Copybook)
	assert.Equal(t, 18, r.Files.Total)
}

func TestEmptyScan(t *testing.T) {
	sr := &scanner.ScanResult{}
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	assert.Equal(t, 0, r.Files.COBOL)
	assert.Equal(t, 0, r.Files.JCL)
	// No COBOL files → heuristic passes contribute 0 requests
	assert.Equal(t, 0, r.Total.Requests)
}

func TestPass3RequestCount(t *testing.T) {
	// 100 COBOL files, batch size 50 → 2 Pass 3 requests
	sr := makeScanResult(100, 0, 0)
	cfg := testConfig()
	cfg.Ingest.Pass3BatchSize = 50
	est := New(cfg, sr, nil, testLogger())
	r := est.Run()

	var pass3 *PassEstimate
	for i := range r.Passes {
		if r.Passes[i].Name == "Pass 3 (Cross-Cutting)" {
			pass3 = &r.Passes[i]
			break
		}
	}
	require.NotNil(t, pass3)
	assert.Equal(t, 2, pass3.Requests)
	assert.False(t, pass3.Deterministic)
}

func TestPass4Heuristic(t *testing.T) {
	sr := makeScanResult(100, 0, 0)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	var pass4 *PassEstimate
	for i := range r.Passes {
		if r.Passes[i].Name == "Pass 4 (Data Flow)" {
			pass4 = &r.Passes[i]
			break
		}
	}
	require.NotNil(t, pass4)
	// 100 * 0.25 = 25 requests
	assert.Equal(t, 25, pass4.Requests)
	assert.False(t, pass4.Deterministic)
}

func TestCostMath(t *testing.T) {
	// Anthropic: 1M input tokens at Sonnet pricing = $3
	cost := anthropicInputCost("sonnet", 1_000_000)
	assert.InDelta(t, 3.0, cost, 0.001)

	// Anthropic: 1M output tokens at Opus pricing = $75
	cost = anthropicOutputCost("opus", 1_000_000)
	assert.InDelta(t, 75.0, cost, 0.001)

	// Copilot: 10 Opus requests = 10 * $0.04 * 3x = $1.20
	cost = copilotRequestCost("opus", 10)
	assert.InDelta(t, 1.20, cost, 0.001)

	// Copilot: 10 Sonnet requests = 10 * $0.04 * 1x = $0.40
	cost = copilotRequestCost("sonnet", 10)
	assert.InDelta(t, 0.40, cost, 0.001)
}

func TestRetryMultiplierApplied(t *testing.T) {
	// Verify that Copilot cost uses the retry multiplier, not raw request count.
	// For 10 Opus requests: billable = ceil(10 * 1.8) = 18; cost = 18 * $0.04 * 3 = $2.16
	p := PassEstimate{
		Name:     "test",
		Model:    "opus",
		Requests: 10,
	}
	computeCosts(&p)
	billable := int(math.Ceil(float64(10) * truncationRetryMultiplier))
	expected := copilotRequestCost("opus", billable)
	assert.InDelta(t, expected, p.CopilotCost, 0.001)
	// Must be more expensive than without retry multiplier
	assert.Greater(t, p.CopilotCost, copilotRequestCost("opus", 10))
}

func TestPass5AnnotationsPresent(t *testing.T) {
	sr := makeScanResult(100, 0, 0)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	var found bool
	for _, p := range r.Passes {
		if p.Name == "Pass 5 (Annotations)" {
			found = true
			// 100 * 0.20 = 20 requests
			assert.Equal(t, 20, p.Requests)
			assert.Equal(t, "opus", p.Model)
			assert.False(t, p.Deterministic)
			break
		}
	}
	assert.True(t, found, "Pass 5 (Annotations) should be present")
}

func TestTotalsAggregation(t *testing.T) {
	sr := makeScanResult(5, 2, 1)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	// Total requests should be sum of all passes
	manualSum := 0
	for _, p := range r.Passes {
		manualSum += p.Requests
	}
	manualSum += r.Scanner.Requests
	assert.Equal(t, manualSum, r.Total.Requests)

	// Total cost low should be <= cost high
	assert.LessOrEqual(t, r.Total.InputCost+r.Total.OutputCostLow,
		r.Total.InputCost+r.Total.OutputCostHigh)
}

func TestPrintTableNoError(t *testing.T) {
	sr := makeScanResult(10, 2, 3)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	var buf bytes.Buffer
	PrintTable(&buf, r)
	out := buf.String()

	assert.Contains(t, out, "Pass 1 (Structural)")
	assert.Contains(t, out, "Pass 2 (Deep Analysis)")
	assert.Contains(t, out, "TOTAL")
	assert.Contains(t, out, "Anthropic")
	assert.Contains(t, out, "Copilot")
}

func TestPrintTableShowsBothPricingModels(t *testing.T) {
	sr := makeScanResult(5, 0, 0)
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	var buf bytes.Buffer
	PrintTable(&buf, r)
	out := buf.String()

	assert.Contains(t, out, "TOTAL")
	assert.Contains(t, out, "Anthropic (typ)")
	assert.Contains(t, out, "Copilot")
	assert.Contains(t, out, "$0.04/premium request")
}

func TestScannerClassificationEstimate(t *testing.T) {
	sr := makeScanResult(0, 0, 0)
	// Add pending files
	for i := 0; i < 20; i++ {
		sr.Files = append(sr.Files, graph.FileInfo{
			Path: "/fake/unknown.dat",
			Type: graph.FileTypePending,
			Size: 3000,
		})
	}
	est := New(testConfig(), sr, nil, testLogger())
	r := est.Run()

	assert.Equal(t, 20, r.Scanner.PendingFiles)
	// ceil(20/8) = 3 requests
	assert.Equal(t, 3, r.Scanner.Requests)
	assert.Greater(t, r.Scanner.InputTokens, 0)
}

func TestFormatInt(t *testing.T) {
	assert.Equal(t, "1,000", formatInt(1000))
	assert.Equal(t, "1,000,000", formatInt(1000000))
	assert.Equal(t, "42", formatInt(42))
	assert.Equal(t, "0", formatInt(0))
}

func TestFormatTokens(t *testing.T) {
	assert.Equal(t, "1.0M", formatTokens(1_000_000))
	assert.Equal(t, "500K", formatTokens(500_000))
	assert.Equal(t, "42", formatTokens(42))
}
