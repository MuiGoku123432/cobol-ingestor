package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// Pass4JSON matches the JSON schema returned by Claude for Pass 4 field mapping.
type Pass4JSON struct {
	FieldMappings []FieldMappingJSON `json:"fieldMappings"`
}

type FieldMappingJSON struct {
	SourceField string `json:"sourceField"`
	TargetField string `json:"targetField"`
	Transform   string `json:"transform"`
}

// ParsePass4Response parses Claude's JSON response into field pairs.
func ParsePass4Response(jsonStr string) ([]graph.FieldPair, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw Pass4JSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing pass4 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	var pairs []graph.FieldPair
	for _, m := range raw.FieldMappings {
		pairs = append(pairs, graph.FieldPair{
			SourceField: m.SourceField,
			TargetField: m.TargetField,
			Transform:   m.Transform,
		})
	}

	return pairs, nil
}
