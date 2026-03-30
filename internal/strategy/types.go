package strategy

import (
	"cobol-ingestor/internal/modernize"

	"go.uber.org/zap"
)

// QuestionType defines the input type for a questionnaire question.
type QuestionType string

const (
	QuestionTypeSingleSelect QuestionType = "single_select"
	QuestionTypeMultiSelect  QuestionType = "multi_select"
	QuestionTypeText         QuestionType = "text"
	QuestionTypeNumber       QuestionType = "number"
)

// Question defines a single questionnaire item.
type Question struct {
	ID          string       `json:"id"`
	Group       string       `json:"group"`
	Text        string       `json:"text"`
	Description string       `json:"description"`
	Type        QuestionType `json:"type"`
	Options     []Option     `json:"options,omitempty"`
	Required    bool         `json:"required"`
	DependsOn   *Dependency  `json:"dependsOn,omitempty"`
}

// Option defines a selectable value for single/multi select questions.
type Option struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Dependency defines a conditional visibility rule.
type Dependency struct {
	QuestionID string   `json:"questionId"`
	Values     []string `json:"values"`
}

// StrategyContext holds the validated answers from the questionnaire.
type StrategyContext struct {
	Strategy          string   `json:"strategy"`
	HybridStrategies  []string `json:"hybridStrategies,omitempty"`
	TargetLang        string   `json:"targetLang"`
	TargetLangOther   string   `json:"targetLangOther,omitempty"`
	TargetFramework   string   `json:"targetFramework"`
	TargetPlatform    string   `json:"targetPlatform"`
	Timeline          string   `json:"timeline"`
	TeamSize          int      `json:"teamSize"`
	TeamSkills        []string `json:"teamSkills"`
	Compliance        []string `json:"compliance"`
	Integrations      string   `json:"integrations"`
	PriorityCriteria  []string `json:"priorityCriteria"`
	PhasingApproach   string   `json:"phasingApproach"`
	Criticality       string   `json:"criticality"`
	DowntimeTolerance string   `json:"downtimeTolerance"`
	AdditionalNotes   string   `json:"additionalNotes"`

	// Current Stack
	CurrentDatabase      []string `json:"currentDatabase"`
	CurrentDatabaseOther string   `json:"currentDatabaseOther,omitempty"`
	CurrentMiddleware    []string `json:"currentMiddleware"`
	CurrentMiddlewareOther string `json:"currentMiddlewareOther,omitempty"`
	CurrentBatch         string   `json:"currentBatch"`
	CurrentMonitoring    string   `json:"currentMonitoring"`

	// Target Stack (expanded)
	TargetDatabase       string `json:"targetDatabase"`
	TargetMessaging      string `json:"targetMessaging"`
	TargetAPIStyle       string `json:"targetApiStyle"`
	TargetContainerization string `json:"targetContainerization"`
	TargetCICD           string `json:"targetCicd"`
}

// StrategyPlan holds the complete output of the strategy analysis.
type StrategyPlan struct {
	Context      StrategyContext   `json:"context"`
	AgentResults map[string]string `json:"agentResults"`
	Synthesis    string            `json:"synthesis"`
}

// StrategyParams holds all parameters for a strategy run.
type StrategyParams struct {
	Context   StrategyContext
	Provider  *modernize.ProviderState
	MCPClient *modernize.MCPClient
	Emitter   modernize.EventEmitter
	Logger    *zap.Logger
}
