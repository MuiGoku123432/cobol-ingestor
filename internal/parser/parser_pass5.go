package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"cobol-ingestor/internal/graph"
)

// Pass5 repair response JSON types.

type pass5ChildOfJSON struct {
	ChildOfRelationships []struct {
		Child       string `json:"child"`
		ChildLevel  int    `json:"childLevel"`
		Parent      string `json:"parent"`
		ParentLevel int    `json:"parentLevel"`
	} `json:"childOfRelationships"`
}

type pass5DataFlowJSON struct {
	DataFlows []struct {
		FromItem string `json:"fromItem"`
		ToItem   string `json:"toItem"`
		Context  string `json:"context"`
	} `json:"dataFlows"`
}

type pass5CallsJSON struct {
	Calls []struct {
		Target        string `json:"target"`
		IsDynamic     bool   `json:"isDynamic"`
		FromParagraph string `json:"fromParagraph"`
		ResolvedFrom  string `json:"resolvedFrom"`
	} `json:"calls"`
}

type pass5AnnotationsJSON struct {
	Annotations []struct {
		Paragraph   string `json:"paragraph"`
		Description string `json:"description"`
		Category    string `json:"category"`
	} `json:"annotations"`
}

// ParseRepairChildOf parses LLM repair response into CHILD_OF relationships.
func ParseRepairChildOf(jsonResp, programID string) ([]graph.Relationship, error) {
	cleaned := stripMarkdownFences(jsonResp)
	var raw pass5ChildOfJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing CHILD_OF repair JSON: %w", err)
	}

	var rels []graph.Relationship
	for _, r := range raw.ChildOfRelationships {
		if r.Child == "" || r.Parent == "" {
			continue
		}
		childFQN := fmt.Sprintf("%s.%d.%s", programID, r.ChildLevel, strings.ToUpper(r.Child))
		parentFQN := fmt.Sprintf("%s.%d.%s", programID, r.ParentLevel, strings.ToUpper(r.Parent))

		rels = append(rels, graph.Relationship{
			Type:      graph.RelChildOf,
			FromLabel: "DataItem",
			FromKey:   childFQN,
			ToLabel:   "DataItem",
			ToKey:     parentFQN,
			Properties: map[string]any{
				"source": "pass5_repair",
			},
		})
	}
	return rels, nil
}

// ParseRepairMovesTo parses LLM repair response into MOVES_TO relationships.
func ParseRepairMovesTo(jsonResp, programID string) ([]graph.Relationship, error) {
	cleaned := stripMarkdownFences(jsonResp)
	var raw pass5DataFlowJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing MOVES_TO repair JSON: %w", err)
	}

	var rels []graph.Relationship
	for _, f := range raw.DataFlows {
		if f.FromItem == "" || f.ToItem == "" {
			continue
		}
		rels = append(rels, graph.Relationship{
			Type:      graph.RelMovesTo,
			FromLabel: "DataItem",
			FromKey:   programID + "." + strings.ToUpper(f.FromItem),
			ToLabel:   "DataItem",
			ToKey:     programID + "." + strings.ToUpper(f.ToItem),
			Properties: map[string]any{
				"context": f.Context,
				"source":  "pass5_repair",
			},
		})
	}
	return rels, nil
}

// ParseRepairCalls parses LLM repair response into CALLS relationships.
func ParseRepairCalls(jsonResp, programID string) ([]graph.Relationship, error) {
	cleaned := stripMarkdownFences(jsonResp)
	var raw pass5CallsJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing CALLS repair JSON: %w", err)
	}

	var rels []graph.Relationship
	for _, c := range raw.Calls {
		if c.Target == "" {
			continue
		}
		rels = append(rels, graph.Relationship{
			Type:      graph.RelCalls,
			FromLabel: "Program",
			FromKey:   programID,
			ToLabel:   "Program",
			ToKey:     strings.ToUpper(c.Target),
			Properties: map[string]any{
				"isDynamic":     c.IsDynamic,
				"fromParagraph": c.FromParagraph,
				"resolvedFrom":  c.ResolvedFrom,
				"source":        "pass5_repair",
			},
		})
	}
	return rels, nil
}

// ParagraphAnnotation holds a parsed paragraph annotation from Pass 5 repair.
type ParagraphAnnotation struct {
	Paragraph   string
	Description string
	Category    string
}

// ParseRepairAnnotations parses LLM repair response into paragraph annotations.
func ParseRepairAnnotations(jsonResp string) ([]ParagraphAnnotation, error) {
	cleaned := stripMarkdownFences(jsonResp)
	var raw pass5AnnotationsJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing annotations repair JSON: %w", err)
	}

	var annotations []ParagraphAnnotation
	for _, a := range raw.Annotations {
		if a.Paragraph == "" {
			continue
		}
		annotations = append(annotations, ParagraphAnnotation{
			Paragraph:   a.Paragraph,
			Description: a.Description,
			Category:    a.Category,
		})
	}
	return annotations, nil
}
