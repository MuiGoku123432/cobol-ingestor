package parser

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"cobol-ingestor/internal/graph"
)

type customExtractJSON struct {
	Entities        []customEntityJSON     `json:"entities"`
	Relationships   []customRelJSON        `json:"relationships"`
	CrossReferences []customCrossRefJSON   `json:"crossReferences"`
}

type customEntityJSON struct {
	Name        string         `json:"name"`
	EntityType  string         `json:"entityType"`
	Description string         `json:"description"`
	Properties  map[string]any `json:"properties"`
}

type customRelJSON struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type customCrossRefJSON struct {
	EntityName    string `json:"entityName"`
	TargetName    string `json:"targetName"`
	TargetType    string `json:"targetType"`
	ReferenceType string `json:"referenceType"`
}

// CustomExtractResult holds generic custom-format analysis output.
type CustomExtractResult struct {
	SourceFile    string
	Entities      []graph.CustomEntity
	Relationships []graph.Relationship
}

// ParseCustomExtractResponse parses Claude's JSON response for custom-format extraction.
func ParseCustomExtractResponse(jsonStr, sourceFile string) (*CustomExtractResult, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw customExtractJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		recovered, recoverErr := RecoverPartialJSON(cleaned)
		if recoverErr != nil {
			return nil, fmt.Errorf("parsing custom extract JSON: %w (recovery: %v)", err, recoverErr)
		}
		if err2 := json.Unmarshal([]byte(recovered), &raw); err2 != nil {
			return nil, fmt.Errorf("parsing recovered custom extract JSON: %w", err2)
		}
	}

	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(sourceFile)), ".")
	result := &CustomExtractResult{SourceFile: sourceFile}

	for _, e := range raw.Entities {
		props := e.Properties
		if props == nil {
			props = map[string]any{}
		}
		props["extension"] = ext
		entity := graph.CustomEntity{
			ID:          newID(),
			Name:        e.Name,
			EntityType:  e.EntityType,
			Description: e.Description,
			SourceFile:  sourceFile,
			Extension:   ext,
			MergeID:     sourceFile + "::" + e.Name,
			Properties:  props,
		}
		result.Entities = append(result.Entities, entity)
	}

	// Internal relationships between entities in this file
	for _, r := range raw.Relationships {
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelType(r.Type),
			FromLabel: "CustomEntity",
			FromKey:   sourceFile + "::" + r.From,
			ToLabel:   "CustomEntity",
			ToKey:     sourceFile + "::" + r.To,
			Properties: map[string]any{"description": r.Description},
		})
	}

	// Cross-references to external artifacts
	for _, cr := range raw.CrossReferences {
		toLabel := cr.TargetType
		if toLabel == "" {
			toLabel = "Program"
		}
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelType(cr.ReferenceType),
			FromLabel: "CustomEntity",
			FromKey:   sourceFile + "::" + cr.EntityName,
			ToLabel:   toLabel,
			ToKey:     cr.TargetName,
			Properties: map[string]any{"referenceType": cr.ReferenceType},
		})
	}

	return result, nil
}
