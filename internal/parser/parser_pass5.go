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
		childFQN := fmt.Sprintf("%s.%02d.%s", programID, r.ChildLevel, strings.ToUpper(r.Child))
		parentFQN := fmt.Sprintf("%s.%02d.%s", programID, r.ParentLevel, strings.ToUpper(r.Parent))

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

// DeadCodeVerificationJSON matches the JSON schema for dead code verification.
type DeadCodeVerificationJSON struct {
	Verifications []DeadCodeVerdictJSON `json:"verifications"`
}

// DeadCodeVerdictJSON represents a single dead code verification result.
type DeadCodeVerdictJSON struct {
	ParagraphName string `json:"paragraphName"`
	Verdict       string `json:"verdict"` // "confirmed_dead" or "false_positive"
	Reason        string `json:"reason"`
}

// DeadCodeVerdict is the parsed result for a single paragraph verification.
type DeadCodeVerdict struct {
	ParagraphName   string
	IsFalsePositive bool
	Reason          string
}

// ParseDeadCodeVerification parses Claude's JSON response for dead code verification.
func ParseDeadCodeVerification(jsonStr string) ([]DeadCodeVerdict, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw DeadCodeVerificationJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing dead code verification JSON: %w\nraw response: %.500s", err, cleaned)
	}

	var verdicts []DeadCodeVerdict
	for _, v := range raw.Verifications {
		verdicts = append(verdicts, DeadCodeVerdict{
			ParagraphName:   v.ParagraphName,
			IsFalsePositive: v.Verdict == "false_positive",
			Reason:          v.Reason,
		})
	}

	return verdicts, nil
}

// DomainMergeDecisionJSON matches the JSON schema for domain merge decisions.
type DomainMergeDecisionJSON struct {
	Decisions []DomainMergeItemJSON `json:"decisions"`
}

// DomainMergeItemJSON represents a single domain merge decision.
type DomainMergeItemJSON struct {
	Domain1    string `json:"domain1"`
	Domain2    string `json:"domain2"`
	Action     string `json:"action"`     // "merge" or "keep_separate"
	KeepDomain string `json:"keepDomain"`
	Reason     string `json:"reason"`
}

// DomainMergeDecision is the parsed result for a single domain merge decision.
type DomainMergeDecision struct {
	Domain1     string
	Domain2     string
	ShouldMerge bool
	KeepDomain  string
	Reason      string
}

// ParseDomainMergeDecisions parses Claude's JSON response for domain merge decisions.
func ParseDomainMergeDecisions(jsonStr string) ([]DomainMergeDecision, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw DomainMergeDecisionJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing domain merge JSON: %w\nraw response: %.500s", err, cleaned)
	}

	var decisions []DomainMergeDecision
	for _, d := range raw.Decisions {
		decisions = append(decisions, DomainMergeDecision{
			Domain1:     d.Domain1,
			Domain2:     d.Domain2,
			ShouldMerge: d.Action == "merge",
			KeepDomain:  d.KeepDomain,
			Reason:      d.Reason,
		})
	}

	return decisions, nil
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
