package parser

import (
	"strings"

	"go.uber.org/zap"
)

// Allowed enum values for each field type.
var (
	validExecutionModes = map[string]bool{
		"BATCH": true, "CICS": true, "BATCH_AND_CICS": true,
		"IDMS_DC": true, "IDMS_BATCH": true, "UNKNOWN": true,
	}

	validCategories = map[string]bool{
		"INIT": true, "VALIDATION": true, "PROCESSING": true,
		"IO": true, "ERROR-HANDLING": true, "CLEANUP": true, "OTHER": true,
	}

	validErrorPatterns = map[string]bool{
		"STRUCTURED": true, "AD-HOC": true, "FILE-STATUS": true,
		"SQLCODE": true, "CICS-RESP": true, "IDMS-STATUS": true,
	}

	validConditionalTypes = map[string]bool{
		"IF": true, "EVALUATE": true,
	}

	validRiskTypes = map[string]bool{
		"hub": true, "external_dependencies": true, "complex_control_flow": true,
		"standard": true, "data_coupling": true, "dead_code": true,
	}

	validVolumeEstimates = map[string]bool{
		"HIGH": true, "MEDIUM": true, "LOW": true,
	}

	validApproaches = map[string]bool{
		"API_EXTRACTION": true, "STRANGLER_FIG": true,
		"EVENT_DRIVEN": true, "REWRITE": true,
	}

	validCopybookRiskLevels = map[string]bool{
		"LOW": true, "MEDIUM": true, "HIGH": true, "CRITICAL": true,
	}
)

// normalizeEnum uppercases the value and checks it against allowed values.
// If not valid, logs a warning and returns the fallback.
func normalizeEnum(value, fieldName string, allowed map[string]bool, fallback string) string {
	if value == "" {
		return fallback
	}
	upper := strings.ToUpper(value)
	// Try exact match first
	if allowed[upper] {
		return upper
	}
	// Try case-sensitive match (for mixed-case enums like riskType)
	if allowed[value] {
		return value
	}
	// Try lowercase match for riskType-style enums
	lower := strings.ToLower(value)
	if allowed[lower] {
		return lower
	}
	logWarn("unexpected enum value, using fallback",
		zap.String("field", fieldName),
		zap.String("value", value),
		zap.String("fallback", fallback),
	)
	return fallback
}
