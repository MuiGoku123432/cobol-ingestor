package parser

import (
	"encoding/json"
	"fmt"
)

// BWPass2ResponseJSON matches the JSON schema returned by Claude for BW cross-file synthesis.
type BWPass2ResponseJSON struct {
	NewCobolReferences     []BWPass2CobolRefJSON     `json:"newCobolReferences"`
	CrossFileRelationships []BWPass2CrossRelJSON      `json:"crossFileRelationships"`
	Clusters               []BWPass2ClusterJSON       `json:"clusters"`
}

// BWPass2CobolRefJSON represents a new COBOL reference discovered in cross-file analysis.
type BWPass2CobolRefJSON struct {
	EntityMergeID string  `json:"entityMergeId"`
	TargetName    string  `json:"targetName"`
	TargetType    string  `json:"targetType"`
	ReferenceType string  `json:"referenceType"`
	Description   string  `json:"description"`
	Confidence    float64 `json:"confidence"`
}

// BWPass2CrossRelJSON represents a cross-file relationship between BW entities.
type BWPass2CrossRelJSON struct {
	FromMergeID   string `json:"fromMergeId"`
	ToMergeID     string `json:"toMergeId"`
	RelationType  string `json:"relationType"`
	Description   string `json:"description"`
}

// BWPass2ClusterJSON represents a logical integration cluster.
type BWPass2ClusterJSON struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	EntityMergeIDs []string `json:"entityMergeIds"`
}

// BWPass2Result is the parsed synthesis result.
type BWPass2Result struct {
	NewCobolRefs     []BWPass2CobolRef
	CrossFileRels    []BWPass2CrossRel
	Clusters         []BWPass2Cluster
}

// BWPass2CobolRef is a parsed new COBOL reference.
type BWPass2CobolRef struct {
	EntityMergeID string
	TargetName    string
	TargetType    string
	ReferenceType string
	Description   string
	Confidence    float64
}

// BWPass2CrossRel is a parsed cross-file relationship.
type BWPass2CrossRel struct {
	FromMergeID  string
	ToMergeID    string
	RelationType string
	Description  string
}

// BWPass2Cluster is a parsed integration cluster.
type BWPass2Cluster struct {
	Name           string
	Description    string
	EntityMergeIDs []string
}

// ParseBWPass2Response parses Claude's JSON response for BW cross-file synthesis.
func ParseBWPass2Response(jsonStr string) (*BWPass2Result, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw BWPass2ResponseJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing BW pass 2 JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &BWPass2Result{}

	for _, ref := range raw.NewCobolReferences {
		if ref.EntityMergeID == "" || ref.TargetName == "" {
			continue
		}
		confidence := ref.Confidence
		if confidence == 0 {
			confidence = 0.5
		}
		result.NewCobolRefs = append(result.NewCobolRefs, BWPass2CobolRef{
			EntityMergeID: ref.EntityMergeID,
			TargetName:    ref.TargetName,
			TargetType:    ref.TargetType,
			ReferenceType: ref.ReferenceType,
			Description:   ref.Description,
			Confidence:    confidence,
		})
	}

	for _, rel := range raw.CrossFileRelationships {
		if rel.FromMergeID == "" || rel.ToMergeID == "" {
			continue
		}
		result.CrossFileRels = append(result.CrossFileRels, BWPass2CrossRel{
			FromMergeID:  rel.FromMergeID,
			ToMergeID:    rel.ToMergeID,
			RelationType: rel.RelationType,
			Description:  rel.Description,
		})
	}

	for _, c := range raw.Clusters {
		if c.Name == "" || len(c.EntityMergeIDs) == 0 {
			continue
		}
		result.Clusters = append(result.Clusters, BWPass2Cluster{
			Name:           c.Name,
			Description:    c.Description,
			EntityMergeIDs: c.EntityMergeIDs,
		})
	}

	return result, nil
}
