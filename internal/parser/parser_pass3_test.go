package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePass3Response_Valid(t *testing.T) {
	jsonStr := `{
		"domains": [
			{
				"name": "Customer Management",
				"description": "Programs related to customer data",
				"programs": [
					{"programId": "CUSTMAINT", "confidence": 0.95},
					{"programId": "CUSTINQ", "confidence": 0.90}
				]
			}
		],
		"deadCode": [
			{"programId": "OLDUTIL", "reason": "No callers, utility naming pattern"}
		],
		"riskFlags": [
			{"programId": "MAINPROG", "riskType": "hub", "details": "Called by 15 programs", "score": 0.8},
			{"programId": "CUSTMAINT", "riskType": "standard", "details": "Moderate complexity", "score": 0.3},
			{"programId": "CUSTINQ", "riskType": "standard", "details": "Simple inquiry program", "score": 0.1}
		],
		"bridgePrograms": [
			{"programId": "XFERPROG", "domains": ["Customer Management", "Account Processing"], "reason": "Bridges customer and account domains"}
		],
		"copybookRisk": [
			{"copybook": "CUSTREC", "programCount": 25, "riskLevel": "HIGH", "reason": "Used by 25 programs across 3 domains"}
		],
		"modernizationCandidates": [
			{"programId": "CUSTINQ", "score": 0.85, "reason": "Self-contained customer lookup", "approach": "API_EXTRACTION"}
		],
		"volumeEstimates": [
			{"programId": "CUSTMAINT", "estimate": "HIGH", "reason": "Core customer maintenance with heavy DB2 access"}
		]
	}`

	result, err := ParsePass3Response(jsonStr)
	require.NoError(t, err)

	assert.Len(t, result.BusinessDomains, 1)
	assert.Equal(t, "Customer Management", result.BusinessDomains[0].Name)
	assert.Equal(t, "Programs related to customer data", result.BusinessDomains[0].Description)

	assert.Len(t, result.DomainMembers, 2)
	assert.Equal(t, "CUSTMAINT", result.DomainMembers[0].ProgramID)
	assert.Equal(t, "Customer Management", result.DomainMembers[0].DomainName)
	assert.Equal(t, 0.95, result.DomainMembers[0].Confidence)

	assert.Len(t, result.DeadCodeFlags, 1)
	assert.Equal(t, "OLDUTIL", result.DeadCodeFlags[0].ProgramID)

	assert.Len(t, result.RiskFlags, 3)
	assert.Equal(t, "MAINPROG", result.RiskFlags[0].ProgramID)
	assert.Equal(t, "hub", result.RiskFlags[0].RiskType)
	assert.Equal(t, 0.8, result.RiskFlags[0].Score)
	assert.Equal(t, "CUSTMAINT", result.RiskFlags[1].ProgramID)
	assert.Equal(t, "standard", result.RiskFlags[1].RiskType)
	assert.Equal(t, 0.3, result.RiskFlags[1].Score)
	assert.Equal(t, "CUSTINQ", result.RiskFlags[2].ProgramID)
	assert.Equal(t, "standard", result.RiskFlags[2].RiskType)
	assert.Equal(t, 0.1, result.RiskFlags[2].Score)

	// Bridge programs
	require.Len(t, result.BridgePrograms, 1)
	assert.Equal(t, "XFERPROG", result.BridgePrograms[0].ProgramID)
	assert.Equal(t, []string{"Customer Management", "Account Processing"}, result.BridgePrograms[0].Domains)

	// Copybook risk
	require.Len(t, result.CopybookRisks, 1)
	assert.Equal(t, "CUSTREC", result.CopybookRisks[0].Copybook)
	assert.Equal(t, 25, result.CopybookRisks[0].ProgramCount)
	assert.Equal(t, "HIGH", result.CopybookRisks[0].RiskLevel)

	// Modernization candidates
	require.Len(t, result.ModernizationCandidates, 1)
	assert.Equal(t, "CUSTINQ", result.ModernizationCandidates[0].ProgramID)
	assert.Equal(t, 0.85, result.ModernizationCandidates[0].Score)
	assert.Equal(t, "API_EXTRACTION", result.ModernizationCandidates[0].Approach)

	// Volume estimates
	require.Len(t, result.VolumeEstimates, 1)
	assert.Equal(t, "CUSTMAINT", result.VolumeEstimates[0].ProgramID)
	assert.Equal(t, "HIGH", result.VolumeEstimates[0].Estimate)
	assert.Equal(t, "Core customer maintenance with heavy DB2 access", result.VolumeEstimates[0].Reason)
}

func TestParsePass3Response_WithMarkdownFences(t *testing.T) {
	jsonStr := "```json\n{\"domains\": [], \"deadCode\": [], \"riskFlags\": []}\n```"

	result, err := ParsePass3Response(jsonStr)
	require.NoError(t, err)

	assert.Empty(t, result.BusinessDomains)
	assert.Empty(t, result.DeadCodeFlags)
	assert.Empty(t, result.RiskFlags)
}

func TestParsePass3Response_PartialResponse(t *testing.T) {
	jsonStr := `{"domains": [{"name": "Batch", "description": "Batch jobs", "programs": []}], "deadCode": [], "riskFlags": []}`

	result, err := ParsePass3Response(jsonStr)
	require.NoError(t, err)
	assert.Len(t, result.BusinessDomains, 1)
	assert.Empty(t, result.DomainMembers)
}

func TestParsePass3Response_InvalidJSON(t *testing.T) {
	_, err := ParsePass3Response("not json at all")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing pass3 JSON")
}
