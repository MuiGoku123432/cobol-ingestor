package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// Pass3JSON matches the JSON schema returned by Claude for Pass 3.
type Pass3JSON struct {
	Domains   []Pass3DomainJSON   `json:"domains"`
	DeadCode  []Pass3DeadCodeJSON `json:"deadCode"`
	RiskFlags []Pass3RiskJSON     `json:"riskFlags"`
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

	return result, nil
}
