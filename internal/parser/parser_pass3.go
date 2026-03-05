package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// Pass3JSON matches the JSON schema returned by Claude for Pass 3.
type Pass3JSON struct {
	Domains                 []Pass3DomainJSON                `json:"domains"`
	DeadCode                []Pass3DeadCodeJSON              `json:"deadCode"`
	RiskFlags               []Pass3RiskJSON                  `json:"riskFlags"`
	BridgePrograms          []Pass3BridgeProgramJSON         `json:"bridgePrograms"`
	CopybookRisk            []Pass3CopybookRiskJSON          `json:"copybookRisk"`
	ModernizationCandidates []Pass3ModernizationCandidateJSON `json:"modernizationCandidates"`
}

type Pass3DomainJSON struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Programs    []Pass3DomainProgramJSON  `json:"programs"`
}

type Pass3DomainProgramJSON struct {
	ProgramID  string  `json:"programId"`
	Confidence float64 `json:"confidence"`
}

type Pass3DeadCodeJSON struct {
	ProgramID string `json:"programId"`
	Reason    string `json:"reason"`
}

type Pass3RiskJSON struct {
	ProgramID string  `json:"programId"`
	RiskType  string  `json:"riskType"`
	Details   string  `json:"details"`
	Score     float64 `json:"score"`
}

type Pass3BridgeProgramJSON struct {
	ProgramID string   `json:"programId"`
	Domains   []string `json:"domains"`
	Reason    string   `json:"reason"`
}

type Pass3CopybookRiskJSON struct {
	Copybook     string `json:"copybook"`
	ProgramCount int    `json:"programCount"`
	RiskLevel    string `json:"riskLevel"`
	Reason       string `json:"reason"`
}

type Pass3ModernizationCandidateJSON struct {
	ProgramID string  `json:"programId"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
	Approach  string  `json:"approach"`
}

// ParsePass3Response parses Claude's JSON response into a Pass3Result.
func ParsePass3Response(jsonStr string) (*graph.Pass3Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw Pass3JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing pass3 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &graph.Pass3Result{}

	for _, d := range raw.Domains {
		domain := graph.BusinessDomain{
			ID:          newID(),
			Name:        d.Name,
			Description: d.Description,
		}
		result.BusinessDomains = append(result.BusinessDomains, domain)

		for _, p := range d.Programs {
			result.DomainMembers = append(result.DomainMembers, graph.DomainMembership{
				ProgramID:  p.ProgramID,
				DomainName: d.Name,
				Confidence: p.Confidence,
			})
		}
	}

	for _, dc := range raw.DeadCode {
		result.DeadCodeFlags = append(result.DeadCodeFlags, graph.DeadCodeFlag{
			ProgramID: dc.ProgramID,
			Reason:    dc.Reason,
		})
	}

	for _, rf := range raw.RiskFlags {
		result.RiskFlags = append(result.RiskFlags, graph.RiskFlag{
			ProgramID: rf.ProgramID,
			RiskType:  rf.RiskType,
			Details:   rf.Details,
			Score:     rf.Score,
		})
	}

	for _, bp := range raw.BridgePrograms {
		result.BridgePrograms = append(result.BridgePrograms, graph.BridgeProgram{
			ProgramID: bp.ProgramID,
			Domains:   bp.Domains,
			Reason:    bp.Reason,
		})
	}

	for _, cr := range raw.CopybookRisk {
		result.CopybookRisks = append(result.CopybookRisks, graph.CopybookRisk{
			Copybook:     cr.Copybook,
			ProgramCount: cr.ProgramCount,
			RiskLevel:    cr.RiskLevel,
			Reason:       cr.Reason,
		})
	}

	for _, mc := range raw.ModernizationCandidates {
		result.ModernizationCandidates = append(result.ModernizationCandidates, graph.ModernizationCandidate{
			ProgramID: mc.ProgramID,
			Score:     mc.Score,
			Reason:    mc.Reason,
			Approach:  mc.Approach,
		})
	}

	return result, nil
}
