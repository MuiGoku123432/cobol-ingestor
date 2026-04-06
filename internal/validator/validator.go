package validator

import (
	"fmt"
	"strings"

	"cobol-ingestor/internal/graph"

	"go.uber.org/zap"
)

// ValidationResult holds the results of validating an LLM response.
type ValidationResult struct {
	Valid    bool
	Warnings []string
	Errors   []string
	Score    float64 // 0.0 to 1.0 completeness score
}

// Validator validates parsed LLM responses for schema correctness and completeness.
type Validator struct {
	logger *zap.Logger
}

// New creates a new Validator.
func New(logger *zap.Logger) *Validator {
	return &Validator{logger: logger}
}

// ValidatePass1 validates a Pass 1 result for completeness and correctness.
func (v *Validator) ValidatePass1(result *graph.Pass1Result, estimatedLines int) ValidationResult {
	vr := ValidationResult{Valid: true, Score: 1.0}

	if result == nil {
		vr.Valid = false
		vr.Errors = append(vr.Errors, "nil Pass1Result")
		vr.Score = 0
		return vr
	}

	// Check program ID
	if len(result.Programs) == 0 {
		vr.Errors = append(vr.Errors, "no programs extracted")
		vr.Valid = false
		vr.Score -= 0.5
	} else if result.Programs[0].ProgramID == "UNKNOWN" {
		vr.Warnings = append(vr.Warnings, "program ID is UNKNOWN")
		vr.Score -= 0.1
	}

	// Check execution mode
	if len(result.Programs) > 0 && result.Programs[0].ExecutionMode == "UNKNOWN" {
		vr.Warnings = append(vr.Warnings, "execution mode is UNKNOWN")
		vr.Score -= 0.05
	}

	// Completeness check: estimate expected entities based on file size
	if estimatedLines > 0 {
		// Heuristic: expect at least 1 paragraph per 50 lines in the PROCEDURE division
		expectedParagraphs := estimatedLines / 100 // conservative
		if expectedParagraphs > 0 && len(result.Paragraphs) < expectedParagraphs/2 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("low paragraph count: %d extracted, expected ~%d for %d lines",
					len(result.Paragraphs), expectedParagraphs, estimatedLines))
			vr.Score -= 0.1
		}

		// Heuristic: expect at least 1 data item per 30 lines
		expectedDataItems := estimatedLines / 60
		if expectedDataItems > 0 && len(result.DataItems) < expectedDataItems/3 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("low data item count: %d extracted, expected ~%d for %d lines",
					len(result.DataItems), expectedDataItems, estimatedLines))
			vr.Score -= 0.1
		}
	}

	// Check for partial recovery
	if result.Partial {
		vr.Warnings = append(vr.Warnings, "result was recovered from truncated response")
		vr.Score -= 0.2
	}

	if vr.Score < 0 {
		vr.Score = 0
	}

	return vr
}

// ValidatePass2 validates a Pass 2 result for completeness.
func (v *Validator) ValidatePass2(result *graph.Pass2Result, knownParagraphs []string) ValidationResult {
	vr := ValidationResult{Valid: true, Score: 1.0}

	if result == nil {
		vr.Valid = false
		vr.Errors = append(vr.Errors, "nil Pass2Result")
		vr.Score = 0
		return vr
	}

	// Cross-reference: check that PERFORMS reference known paragraphs
	if len(knownParagraphs) > 0 {
		known := make(map[string]bool, len(knownParagraphs))
		for _, p := range knownParagraphs {
			known[strings.ToUpper(p)] = true
		}

		unknownRefs := 0
		for _, p := range result.Performs {
			if p.ToParagraph != "" && !known[strings.ToUpper(p.ToParagraph)] {
				unknownRefs++
			}
			if p.FromParagraph != "" && !known[strings.ToUpper(p.FromParagraph)] {
				unknownRefs++
			}
		}
		if unknownRefs > 0 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("%d PERFORMS reference unknown paragraphs", unknownRefs))
			// Mild penalty — could be sections or paragraphs in other chunks
			vr.Score -= float64(unknownRefs) * 0.01
		}
	}

	// Check annotation quality: flag generic descriptions
	genericPhrases := []string{
		"main processing", "performs processing", "handles processing",
		"does processing", "main logic", "performs main",
	}
	genericCount := 0
	for _, a := range result.Annotations {
		desc := strings.ToLower(a.Description)
		for _, phrase := range genericPhrases {
			if strings.Contains(desc, phrase) {
				genericCount++
				break
			}
		}
	}
	if genericCount > 0 {
		vr.Warnings = append(vr.Warnings,
			fmt.Sprintf("%d annotations have generic descriptions", genericCount))
		vr.Score -= float64(genericCount) * 0.02
	}

	if result.Partial {
		vr.Warnings = append(vr.Warnings, "result was recovered from truncated response")
		vr.Score -= 0.2
	}

	if vr.Score < 0 {
		vr.Score = 0
	}

	return vr
}

// ValidatePass3 validates a Pass 3 result for completeness.
func (v *Validator) ValidatePass3(result *graph.Pass3Result, expectedProgramCount int) ValidationResult {
	vr := ValidationResult{Valid: true, Score: 1.0}

	if result == nil {
		vr.Valid = false
		vr.Errors = append(vr.Errors, "nil Pass3Result")
		vr.Score = 0
		return vr
	}

	// Check domain coverage
	assignedPrograms := make(map[string]bool)
	for _, dm := range result.DomainMembers {
		assignedPrograms[dm.ProgramID] = true
	}

	if expectedProgramCount > 0 {
		coverage := float64(len(assignedPrograms)) / float64(expectedProgramCount)
		if coverage < 0.8 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("low domain coverage: %d/%d programs assigned (%.0f%%)",
					len(assignedPrograms), expectedProgramCount, coverage*100))
			vr.Score -= (1.0 - coverage) * 0.5
		}
	}

	// Check risk flag coverage
	riskPrograms := make(map[string]bool)
	for _, rf := range result.RiskFlags {
		riskPrograms[rf.ProgramID] = true
	}
	if expectedProgramCount > 0 {
		riskCoverage := float64(len(riskPrograms)) / float64(expectedProgramCount)
		if riskCoverage < 0.8 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("low risk flag coverage: %d/%d programs", len(riskPrograms), expectedProgramCount))
			vr.Score -= 0.1
		}
	}

	// Check volume estimate coverage
	volPrograms := make(map[string]bool)
	for _, ve := range result.VolumeEstimates {
		volPrograms[ve.ProgramID] = true
	}
	if expectedProgramCount > 0 {
		volCoverage := float64(len(volPrograms)) / float64(expectedProgramCount)
		if volCoverage < 0.8 {
			vr.Warnings = append(vr.Warnings,
				fmt.Sprintf("low volume estimate coverage: %d/%d programs", len(volPrograms), expectedProgramCount))
			vr.Score -= 0.1
		}
	}

	if vr.Score < 0 {
		vr.Score = 0
	}

	return vr
}
