package neo4j

import "time"

// ProgramSummary is a lightweight program representation for list views.
type ProgramSummary struct {
	ProgramID     string `json:"programId"`
	FilePath      string `json:"filePath"`
	Language      string `json:"language"`
	CallCount     int    `json:"callCount"`
	DeadCode      bool   `json:"deadCode,omitempty"`
	ExecutionMode string `json:"executionMode,omitempty"`
	LineCount     int    `json:"lineCount,omitempty"`
}

// ProgramDetail is a full program representation with relationships.
type ProgramDetail struct {
	ProgramID      string           `json:"programId"`
	FilePath       string           `json:"filePath"`
	Language       string           `json:"language"`
	LineCount      int              `json:"lineCount,omitempty"`
	ExecutionMode  string           `json:"executionMode,omitempty"`
	DeadCode       bool             `json:"deadCode,omitempty"`
	DeadCodeReason string           `json:"deadCodeReason,omitempty"`
	RiskScore      float64          `json:"riskScore,omitempty"`
	RiskType       string           `json:"riskType,omitempty"`
	RiskDetails    string           `json:"riskDetails,omitempty"`
	Callers        []CallInfo       `json:"callers"`
	Callees        []CallInfo       `json:"callees"`
	Copybooks      []string         `json:"copybooks"`
	Paragraphs     []ParagraphInfo  `json:"paragraphs"`
	Sections       []string         `json:"sections"`
	DataItems      []DataItemInfo   `json:"dataItems"`
	FileDefs       []FileDefInfo    `json:"fileDefs"`
	Domains        []string         `json:"domains"`
}

// ParagraphInfo holds paragraph details for API responses.
type ParagraphInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// DataItemInfo holds data item details for API responses.
type DataItemInfo struct {
	Name    string `json:"name"`
	Level   int    `json:"level"`
	FQN     string `json:"fqn"`
	Picture string `json:"picture,omitempty"`
	Usage   string `json:"usage,omitempty"`
}

// CallChainNode represents a node in a call chain traversal.
type CallChainNode struct {
	ProgramID string          `json:"programId"`
	Depth     int             `json:"depth"`
	Children  []CallChainNode `json:"children,omitempty"`
}

// ImpactResult holds the blast radius analysis for a program.
type ImpactResult struct {
	ProgramID           string   `json:"programId"`
	DownstreamPrograms  []string `json:"downstreamPrograms"`
	UpstreamPrograms    []string `json:"upstreamPrograms"`
	SharedCopybooks     []string `json:"sharedCopybooks"`
	SharedFiles         []string `json:"sharedFiles"`
	TotalAffected       int      `json:"totalAffected"`
}

// DashboardStats holds aggregate counts for the dashboard.
type DashboardStats struct {
	ProgramCount          int `json:"programCount"`
	CopybookCount         int `json:"copybookCount"`
	ParagraphCount        int `json:"paragraphCount"`
	SectionCount          int `json:"sectionCount"`
	DataItemCount         int `json:"dataItemCount"`
	FileCount             int `json:"fileCount"`
	SQLStatementCount     int `json:"sqlStatementCount"`
	CICSTransactionCount  int `json:"cicsTransactionCount"`
	ExternalInterfaceCount int `json:"externalInterfaceCount"`
	ConditionCount        int `json:"conditionCount"`
	ParameterCount        int `json:"parameterCount"`
	RelationshipCount     int `json:"relationshipCount"`
	OrphanCount           int `json:"orphanCount"`
	DomainCount           int `json:"domainCount"`
}

// SearchResult holds a single full-text search match.
type SearchResult struct {
	Label string  `json:"label"`
	Name  string  `json:"name"`
	Score float64 `json:"score"`
	Path  string  `json:"path,omitempty"`
}

// BusinessDomainSummary holds a domain with its member count.
type BusinessDomainSummary struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	ProgramCount int    `json:"programCount"`
}

// BusinessDomainDetail holds a domain with its member programs.
type BusinessDomainDetail struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Programs    []string `json:"programs"`
}

// CopybookSummary holds a copybook with its usage count.
type CopybookSummary struct {
	Name       string `json:"name"`
	UsageCount int    `json:"usageCount"`
}

// CopybookUsage holds which programs include a copybook.
type CopybookUsage struct {
	Name     string   `json:"name"`
	Programs []string `json:"programs"`
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

// PagedResponse wraps data with pagination.
type PagedResponse struct {
	Data     any        `json:"data"`
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// ConditionInfo holds condition details for API responses.
type ConditionInfo struct {
	Name      string `json:"name"`
	Parent    string `json:"parent"`
	Value     string `json:"value"`
	FQN       string `json:"fqn"`
	ProgramID string `json:"programId"`
}

// ParameterInfo holds parameter details for API responses.
type ParameterInfo struct {
	Name      string `json:"name"`
	Level     int    `json:"level"`
	Direction string `json:"direction"`
	FQN       string `json:"fqn"`
	ProgramID string `json:"programId"`
}

// ConditionalLogicInfo holds conditional logic from paragraph nodes.
type ConditionalLogicInfo struct {
	Paragraph        string   `json:"paragraph"`
	ConditionalLogic []string `json:"conditionalLogic"`
}

// ErrorHandlerInfo holds error handling details from paragraph nodes.
type ErrorHandlerInfo struct {
	Paragraph    string `json:"paragraph"`
	ErrorPattern string `json:"errorPattern"`
	ErrorDetails string `json:"errorDetails"`
}

// ExternalInterfaceInfo holds external interface details.
type ExternalInterfaceInfo struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Details   string `json:"details"`
	Paragraph string `json:"paragraph,omitempty"`
	ProgramID string `json:"programId"`
}

// BridgeProgramInfo holds bridge program details.
type BridgeProgramInfo struct {
	ProgramID    string   `json:"programId"`
	Domains      []string `json:"domains"`
	BridgeReason string   `json:"bridgeReason"`
}

// CopybookRiskInfo holds copybook risk details.
type CopybookRiskInfo struct {
	Name         string `json:"name"`
	RiskLevel    string `json:"riskLevel"`
	ProgramCount int    `json:"programCount"`
	RiskReason   string `json:"riskReason"`
}

// ModernizationCandidateInfo holds modernization candidate details.
type ModernizationCandidateInfo struct {
	ProgramID string  `json:"programId"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason"`
	Approach  string  `json:"approach"`
}

// RiskProgramInfo holds risk program details.
type RiskProgramInfo struct {
	ProgramID   string  `json:"programId"`
	RiskScore   float64 `json:"riskScore"`
	RiskType    string  `json:"riskType"`
	RiskDetails string  `json:"riskDetails"`
}

// VolumeEstimateInfo holds volume estimate details.
type VolumeEstimateInfo struct {
	ProgramID    string `json:"programId"`
	Estimate     string `json:"volumeEstimate"`
	VolumeReason string `json:"volumeReason"`
}

// CallInfo holds caller/callee details with CALLS relationship properties.
type CallInfo struct {
	ProgramID     string `json:"programId"`
	IsDynamic     bool   `json:"isDynamic,omitempty"`
	ResolvedFrom  string `json:"resolvedFrom,omitempty"`
	FromParagraph string `json:"fromParagraph,omitempty"`
}

// FileDefInfo holds file definition details.
type FileDefInfo struct {
	Name          string `json:"name"`
	AccessType    string `json:"accessType,omitempty"`
	Organization  string `json:"organization,omitempty"`
	VSAMType      string `json:"vsamType,omitempty"`
	DataStoreType string `json:"dataStoreType,omitempty"`
}

// SQLStatementInfo holds SQL statement details.
type SQLStatementInfo struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Type        string `json:"type"`
	TargetTable string `json:"targetTable,omitempty"`
}

// CICSTransactionInfo holds CICS transaction details.
type CICSTransactionInfo struct {
	ID      string `json:"id"`
	Command string `json:"command"`
}

// ParagraphFlowInfo holds PERFORMS/PERFORMS_THRU relationship details.
type ParagraphFlowInfo struct {
	FromParagraph string `json:"fromParagraph"`
	ToParagraph   string `json:"toParagraph"`
	Type          string `json:"type"`
	IsLoop        bool   `json:"isLoop,omitempty"`
	Condition     string `json:"condition,omitempty"`
}

// DataFlowInfo holds MOVES_TO relationship details.
type DataFlowInfo struct {
	FromItem string `json:"fromItem"`
	ToItem   string `json:"toItem"`
	Context  string `json:"context,omitempty"`
}

// DataHierarchyInfo holds CHILD_OF and REDEFINES relationship details.
type DataHierarchyInfo struct {
	Name        string `json:"name"`
	Level       int    `json:"level,omitempty"`
	Parent      string `json:"parent,omitempty"`
	ParentLevel int    `json:"parentLevel,omitempty"`
	Redefines   string `json:"redefines,omitempty"`
	Relation    string `json:"relation"`
}

// Filter holds common query filters.
type Filter struct {
	Search string
}

// JobStatus tracks an async ingestion job.
type JobStatus struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // "pending", "running", "completed", "failed"
	StartedAt time.Time `json:"startedAt"`
	Error     string    `json:"error,omitempty"`
	Progress  string    `json:"progress,omitempty"`
}
