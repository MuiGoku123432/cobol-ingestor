package chunker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenCalibrator_InitialFactor(t *testing.T) {
	tc := NewTokenCalibrator(100, nil)
	assert.Equal(t, 1.0, tc.CorrectionFactor())
}

func TestTokenCalibrator_RecordSample(t *testing.T) {
	tc := NewTokenCalibrator(100, nil)

	// If actual is 20% higher than estimated, factor should be ~1.2
	tc.RecordSample(1000, 1200)
	assert.InDelta(t, 1.2, tc.CorrectionFactor(), 0.01)

	// Add another sample at 1.0, average should be ~1.1
	tc.RecordSample(1000, 1000)
	assert.InDelta(t, 1.1, tc.CorrectionFactor(), 0.01)
}

func TestTokenCalibrator_MaxSamples(t *testing.T) {
	tc := NewTokenCalibrator(5, nil)

	// Fill with 1.0 ratio samples
	for i := 0; i < 5; i++ {
		tc.RecordSample(1000, 1000)
	}
	assert.InDelta(t, 1.0, tc.CorrectionFactor(), 0.01)

	// Now add 5 samples at 2.0 ratio - should push out old ones
	for i := 0; i < 5; i++ {
		tc.RecordSample(1000, 2000)
	}
	assert.InDelta(t, 2.0, tc.CorrectionFactor(), 0.01)
}

func TestTokenCalibrator_CalibratedEstimate(t *testing.T) {
	tc := NewTokenCalibrator(100, nil)
	tc.SetCorrectionFactor(1.5)

	content := "test content for estimation"
	base := EstimateTokens(content)
	calibrated := tc.CalibratedEstimate(content)
	require.Greater(t, calibrated, base)
	assert.InDelta(t, float64(base)*1.5, float64(calibrated), 1.0)
}

func TestTokenCalibrator_IgnoresInvalidSamples(t *testing.T) {
	tc := NewTokenCalibrator(100, nil)
	tc.RecordSample(0, 100)
	tc.RecordSample(100, 0)
	tc.RecordSample(-1, 100)
	assert.Equal(t, 1.0, tc.CorrectionFactor()) // unchanged
}
