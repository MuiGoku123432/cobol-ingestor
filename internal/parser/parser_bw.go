package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// BWResponseJSON matches the JSON schema returned by Claude for BW analysis.
type BWResponseJSON struct {
	Summary         string               `json:"summary"`
	Entities        []BWEntityJSON        `json:"entities"`
	Relationships   []BWRelationshipJSON  `json:"relationships"`
	CobolReferences []BWCobolRefJSON      `json:"cobolReferences"`
}

// BWEntityJSON represents an entity in the LLM response.
type BWEntityJSON struct {
	Name        string         `json:"name"`
	EntityType  string         `json:"entityType"`
	Description string         `json:"description"`
	Properties  map[string]any `json:"properties"`
}

// BWRelationshipJSON represents a relationship in the LLM response.
type BWRelationshipJSON struct {
	FromEntity   string  `json:"fromEntity"`
	ToEntity     string  `json:"toEntity"`
	RelationType string  `json:"relationType"`
	Description  string  `json:"description"`
	Confidence   float64 `json:"confidence"`
}

// BWCobolRefJSON represents a COBOL reference in the LLM response.
type BWCobolRefJSON struct {
	EntityName    string `json:"entityName"`
	TargetName    string `json:"targetName"`
	TargetType    string `json:"targetType"`
	ReferenceType string `json:"referenceType"`
	Description   string `json:"description"`
}

// ParseBWResponse parses Claude's JSON response into a BWResult.
func ParseBWResponse(jsonStr, sourceFile string) (*graph.BWResult, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw BWResponseJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing BW JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &graph.BWResult{
		File: graph.BWFile{
			Path:     sourceFile,
			FileType: classifyBWFileType(sourceFile),
			Summary:  raw.Summary,
		},
	}

	for _, e := range raw.Entities {
		if e.Name == "" {
			continue
		}
		result.Entities = append(result.Entities, graph.BWEntity{
			Name:        e.Name,
			EntityType:  e.EntityType,
			Description: e.Description,
			SourceFile:  sourceFile,
			MergeID:     sourceFile + "." + e.Name,
			Properties:  e.Properties,
		})
	}

	for _, r := range raw.Relationships {
		if r.FromEntity == "" || r.ToEntity == "" {
			continue
		}
		confidence := r.Confidence
		if confidence == 0 {
			confidence = 0.5
		}
		result.Relationships = append(result.Relationships, graph.BWRelationship{
			FromEntity:   r.FromEntity,
			ToEntity:     r.ToEntity,
			RelationType: r.RelationType,
			Description:  r.Description,
			Confidence:   confidence,
		})
	}

	for _, ref := range raw.CobolReferences {
		if ref.EntityName == "" || ref.TargetName == "" {
			continue
		}
		targetType := ref.TargetType
		if targetType == "" {
			targetType = "Program"
		}
		result.CobolReferences = append(result.CobolReferences, graph.BWCobolReference{
			EntityName:    ref.EntityName,
			TargetName:    ref.TargetName,
			TargetType:    targetType,
			ReferenceType: ref.ReferenceType,
			Description:   ref.Description,
		})
	}

	return result, nil
}

// classifyBWFileType returns a human-readable file type based on extension.
func classifyBWFileType(path string) string {
	switch {
	case hasExtCI(path, ".java"):
		return "Java"
	case hasExtCI(path, ".md"):
		return "Markdown"
	case hasExtCI(path, ".xml"):
		return "XML"
	case hasExtCI(path, ".bw"):
		return "Businessware"
	case hasExtCI(path, ".txt"):
		return "Text"
	default:
		return "Unknown"
	}
}

func hasExtCI(path, ext string) bool {
	l := len(path)
	e := len(ext)
	if l < e {
		return false
	}
	return len(path) >= len(ext) &&
		(path[l-e:] == ext || toLowerASCII(path[l-e:]) == ext)
}

func toLowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
