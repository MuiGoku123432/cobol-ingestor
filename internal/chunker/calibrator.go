package chunker

import (
	"sync"

	"go.uber.org/zap"
)

// TokenCalibrator maintains a running average of actual/estimated token ratios
// to improve token estimation accuracy over time.
type TokenCalibrator struct {
	mu               sync.RWMutex
	samples          []float64
	maxSamples       int
	correctionFactor float64 // running average of actual/estimated ratios
	logger           *zap.Logger
}

// NewTokenCalibrator creates a calibrator with the given max sample count.
func NewTokenCalibrator(maxSamples int, logger *zap.Logger) *TokenCalibrator {
	if maxSamples <= 0 {
		maxSamples = 100
	}
	return &TokenCalibrator{
		maxSamples:       maxSamples,
		correctionFactor: 1.0, // start with no correction
		logger:           logger,
	}
}

// RecordSample records an actual vs estimated token measurement.
// estimatedInputTokens is what EstimateTokens() returned for the input content.
// actualInputTokens is the PromptTokens from the LLM response.
func (tc *TokenCalibrator) RecordSample(estimatedInputTokens, actualInputTokens int) {
	if estimatedInputTokens <= 0 || actualInputTokens <= 0 {
		return
	}

	ratio := float64(actualInputTokens) / float64(estimatedInputTokens)

	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.samples = append(tc.samples, ratio)
	if len(tc.samples) > tc.maxSamples {
		tc.samples = tc.samples[len(tc.samples)-tc.maxSamples:]
	}

	// Recompute running average
	var sum float64
	for _, s := range tc.samples {
		sum += s
	}
	tc.correctionFactor = sum / float64(len(tc.samples))

	if len(tc.samples)%10 == 0 && tc.logger != nil {
		tc.logger.Debug("token calibration updated",
			zap.Float64("correction_factor", tc.correctionFactor),
			zap.Int("samples", len(tc.samples)),
		)
	}
}

// CorrectionFactor returns the current calibration factor.
// Multiply EstimateTokens() results by this to get calibrated estimates.
func (tc *TokenCalibrator) CorrectionFactor() float64 {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.correctionFactor
}

// CalibratedEstimate returns an adjusted token estimate.
func (tc *TokenCalibrator) CalibratedEstimate(content string) int {
	base := EstimateTokens(content)
	tc.mu.RLock()
	factor := tc.correctionFactor
	tc.mu.RUnlock()
	return int(float64(base) * factor)
}

// SetCorrectionFactor sets the correction factor directly (e.g., loaded from cache).
func (tc *TokenCalibrator) SetCorrectionFactor(factor float64) {
	if factor <= 0 {
		return
	}
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.correctionFactor = factor
}
