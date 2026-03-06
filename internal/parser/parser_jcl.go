package parser

import (
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/graph"
)

// JCLResponseJSON matches the JSON schema returned by Claude for JCL analysis.
type JCLResponseJSON struct {
	JobName  string         `json:"jobName"`
	Class    string         `json:"class"`
	MsgClass string         `json:"msgclass"`
	Steps    []JCLStepJSON  `json:"steps"`
}

type JCLStepJSON struct {
	StepName string        `json:"stepName"`
	Pgm      string        `json:"pgm"`
	Proc     string        `json:"proc"`
	Cond     string        `json:"cond"`
	DDCards  []DDCardJSON  `json:"ddCards"`
}

type DDCardJSON struct {
	DDName   string `json:"ddName"`
	DSName   string `json:"dsname"`
	Disp     string `json:"disp"`
	IsInput  bool   `json:"isInput"`
	IsOutput bool   `json:"isOutput"`
}

// ParseJCLResponse parses Claude's JSON response into a JCLAnalysisResult.
func ParseJCLResponse(jsonStr, sourceFile string) (*graph.JCLAnalysisResult, error) {
	cleaned := stripMarkdownFences(jsonStr)

	var raw JCLResponseJSON
	if err := json.Unmarshal([]byte(cleaned), &raw); err != nil {
		return nil, fmt.Errorf("parsing JCL JSON: %w\nraw response: %.500s", err, cleaned)
	}

	result := &graph.JCLAnalysisResult{SourceFile: sourceFile}

	jobName := raw.JobName
	if jobName == "" {
		jobName = "UNKNOWN"
	}

	job := graph.JCLJob{
		ID:       newID(),
		JobName:  jobName,
		Class:    raw.Class,
		MsgClass: raw.MsgClass,
	}
	result.Jobs = append(result.Jobs, job)

	for i, s := range raw.Steps {
		stepName := s.StepName
		if stepName == "" {
			stepName = fmt.Sprintf("STEP%03d", i+1)
		}

		step := graph.JCLStep{
			ID:       newID(),
			StepName: stepName,
			Program:  s.Pgm,
			Proc:     s.Proc,
			Cond:     s.Cond,
			JobName:  jobName,
			Order:    i + 1,
		}
		result.Steps = append(result.Steps, step)

		// STEP_OF: JCLStep -> JCLJob
		result.Relationships = append(result.Relationships, graph.Relationship{
			Type:      graph.RelStepOf,
			FromLabel: "JCLStep",
			FromKey:   step.ID,
			ToLabel:   "JCLJob",
			ToKey:     jobName,
			Properties: map[string]any{
				"order": i + 1,
			},
		})

		// RUNS: JCLStep -> Program (if pgm is set and not a utility)
		if s.Pgm != "" {
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      graph.RelRuns,
				FromLabel: "JCLStep",
				FromKey:   step.ID,
				ToLabel:   "Program",
				ToKey:     s.Pgm,
			})
		}

		// DD Cards
		for _, dd := range s.DDCards {
			ddCard := graph.DDCard{
				ID:       newID(),
				DDName:   dd.DDName,
				DSName:   dd.DSName,
				Disp:     dd.Disp,
				IsInput:  dd.IsInput,
				IsOutput: dd.IsOutput,
				JobName:  jobName,
				StepName: stepName,
			}
			result.DDCards = append(result.DDCards, ddCard)

			// USES_DATASET: JCLStep -> DDCard
			result.Relationships = append(result.Relationships, graph.Relationship{
				Type:      graph.RelUsesDataset,
				FromLabel: "JCLStep",
				FromKey:   step.ID,
				ToLabel:   "DDCard",
				ToKey:     ddCard.ID,
			})
		}
	}

	return result, nil
}
