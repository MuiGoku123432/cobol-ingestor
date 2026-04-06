package parser

import (
	"encoding/json"
	"fmt"
)

// BWPass3ResponseJSON matches the JSON schema returned by Claude for BW validation repair.
type BWPass3ResponseJSON struct {
	Repairs []BWPass3RepairJSON `json:"repairs"`
}

// BWPass3RepairJSON represents a single repair action.
type BWPass3RepairJSON struct {
	EntityMergeID string  `json:"entityMergeId"`
	TargetName    string  `json:"targetName"`
	TargetType    string  `json:"targetType"`
	ReferenceType string  `json:"referenceType"`
	Description   string  `json:"description"`
	Confidence    float64 `json:"confidence"`
	Action        string  `json:"action"` // "add_reference", "confirm", "reject"
}

// BWPass3Repair is a parsed repair action.
type BWPass3Repair struct {
	EntityMergeID string
	TargetName    string
	TargetType    string
	ReferenceType string
	Description   string
	Confidence    float64
	Action        string
}

// ParseBWPass3Response parses Claude's JSON response for BW validation repair.
func ParseBWPass3Response(jsonStr string) ([]BWPass3Repair, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw BWPass3ResponseJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing BW pass 3 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	var repairs []BWPass3Repair
	for _, r := range raw.Repairs {
		if r.EntityMergeID == "" || r.Action == "reject" {
			continue
		}
		confidence := r.Confidence
		if confidence == 0 {
			confidence = 0.5
		}
		repairs = append(repairs, BWPass3Repair{
			EntityMergeID: r.EntityMergeID,
			TargetName:    r.TargetName,
			TargetType:    r.TargetType,
			ReferenceType: r.ReferenceType,
			Description:   r.Description,
			Confidence:    confidence,
			Action:        r.Action,
		})
	}

	return repairs, nil
}
