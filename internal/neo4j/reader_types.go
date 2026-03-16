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
	Name     string `json:"name"`
	Level    int    `json:"level"`
	FQN      string `json:"fqn"`
	Picture  string `json:"picture,omitempty"`
	Usage    string `json:"usage,omitempty"`
	Copybook string `json:"copybook,omitempty"`
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
	DDCardCount           int `json:"ddCardCount"`
	DBTableCount          int `json:"dbTableCount"`
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

// DeadParagraphInfo holds dead paragraph details.
type DeadParagraphInfo struct {
	Name        string `json:"name"`
	ProgramID   string `json:"programId"`
	Reason      string `json:"reason"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// DeadCodeSummaryInfo holds aggregate dead paragraph counts per program.
type DeadCodeSummaryInfo struct {
	ProgramID       string `json:"programId"`
	TotalParagraphs int    `json:"totalParagraphs"`
	DeadParagraphs  int    `json:"deadParagraphs"`
}

// JCLJobInfo holds JCL job details for API responses.
type JCLJobInfo struct {
	JobName   string `json:"jobName"`
	Class     string `json:"class,omitempty"`
	MsgClass  string `json:"msgclass,omitempty"`
	StepCount int    `json:"stepCount"`
}

// JCLJobDetail holds full JCL job detail.
type JCLJobDetail struct {
	JobName  string         `json:"jobName"`
	Class    string         `json:"class,omitempty"`
	MsgClass string         `json:"msgclass,omitempty"`
	Steps    []JCLStepInfo  `json:"steps"`
}

// JCLStepInfo holds JCL step details.
type JCLStepInfo struct {
	StepName string       `json:"stepName"`
	Program  string       `json:"program,omitempty"`
	Proc     string       `json:"proc,omitempty"`
	Cond     string       `json:"cond,omitempty"`
	Order    int          `json:"order"`
	DDCards  []DDCardInfo `json:"ddCards,omitempty"`
}

// DDCardInfo holds DD card details.
type DDCardInfo struct {
	DDName   string `json:"ddName"`
	DSName   string `json:"dsname,omitempty"`
	Disp     string `json:"disp,omitempty"`
	IsInput  bool   `json:"isInput"`
	IsOutput bool   `json:"isOutput"`
}

// ProgramJCLInfo holds reverse JCL lookup results.
type ProgramJCLInfo struct {
	ProgramID string   `json:"programId"`
	Jobs      []string `json:"jobs"`
	Steps     []string `json:"steps"`
}

// DatasetUsageInfo holds dataset usage details.
type DatasetUsageInfo struct {
	DSName   string   `json:"dsname"`
	Jobs     []string `json:"jobs"`
	Steps    []string `json:"steps"`
	IsInput  bool     `json:"isInput"`
	IsOutput bool     `json:"isOutput"`
}

// DBTableInfo holds database table details.
type DBTableInfo struct {
	Name       string `json:"name"`
	Schema     string `json:"schema,omitempty"`
	AccessCount int   `json:"accessCount"`
}

// TableUsageInfo holds table usage details.
type TableUsageInfo struct {
	TableName  string              `json:"tableName"`
	Programs   []TableAccessInfo   `json:"programs"`
}

// TableAccessInfo holds per-program table access info.
type TableAccessInfo struct {
	ProgramID  string   `json:"programId"`
	Operations []string `json:"operations"`
	Columns    []string `json:"columns,omitempty"`
}

// ProgramTableAccessInfo holds tables accessed by a program.
type ProgramTableAccessInfo struct {
	ProgramID string          `json:"programId"`
	Tables    []TableAccessDetail `json:"tables"`
}

// TableAccessDetail holds table access details for a program.
type TableAccessDetail struct {
	Name       string   `json:"name"`
	Operations []string `json:"operations"`
	Columns    []string `json:"columns,omitempty"`
}

// CrossProgramFlowInfo holds cross-program data flow details.
type CrossProgramFlowInfo struct {
	ProgramID      string `json:"programId"`
	OtherProgram   string `json:"otherProgram"`
	Channel        string `json:"channel"`
	Direction      string `json:"direction"`
	Fields         string `json:"fields,omitempty"`
	SharedResource string `json:"sharedResource,omitempty"`
}

// FieldImpactInfo holds field impact trace results.
type FieldImpactInfo struct {
	ProgramID    string            `json:"programId"`
	FieldName    string            `json:"fieldName"`
	IntraTargets []string          `json:"intraTargets,omitempty"`
	CrossFlows   []CrossFlowTarget `json:"crossFlows,omitempty"`
}

// CrossFlowTarget holds a cross-program flow target.
type CrossFlowTarget struct {
	Callee  string `json:"callee"`
	Channel string `json:"channel"`
	Fields  string `json:"fields,omitempty"`
}

// SharedDataChannelInfo holds shared data channel details.
type SharedDataChannelInfo struct {
	Resource string   `json:"resource"`
	Channel  string   `json:"channel"`
	Writers  []string `json:"writers"`
	Readers  []string `json:"readers"`
}

// ProgramSourceInfo holds source code retrieved from disk for a program.
type ProgramSourceInfo struct {
	ProgramID string `json:"programId"`
	FilePath  string `json:"filePath"`
	Source    string `json:"source"`
	LineCount int    `json:"lineCount"`
}

// MigrationStep represents a dependency-ordered migration entry.
type MigrationStep struct {
	ProgramID string   `json:"programId"`
	Order     int      `json:"order"`
	BlockedBy []string `json:"blockedBy"`
	Domain    string   `json:"domain,omitempty"`
	Approach  string   `json:"approach,omitempty"`
	Score     float64  `json:"score"`
	Tier      string   `json:"tier"` // LEAF, MIDDLE, ROOT
}

// FileAccessInfo holds all accessors and DD card mappings for a file.
type FileAccessInfo struct {
	FileName  string             `json:"fileName"`
	Accessors []FileAccessorInfo `json:"accessors"`
	DDCards   []FileAccessDDInfo `json:"ddCards,omitempty"`
}

// FileAccessorInfo holds per-program file access info.
type FileAccessorInfo struct {
	ProgramID  string `json:"programId"`
	AccessType string `json:"accessType"` // READS, WRITES, READS_WRITES
}

// FileAccessDDInfo holds DD card mapping info for a file.
type FileAccessDDInfo struct {
	DDName   string `json:"ddName"`
	DSName   string `json:"dsname,omitempty"`
	JobName  string `json:"jobName,omitempty"`
	StepName string `json:"stepName,omitempty"`
	IsInput  bool   `json:"isInput"`
	IsOutput bool   `json:"isOutput"`
}

// EffortEstimate holds structural complexity metrics for a program.
type EffortEstimate struct {
	ProgramID       string `json:"programId"`
	ParagraphCount  int    `json:"paragraphCount"`
	CopybookCount   int    `json:"copybookCount"`
	DataItemCount   int    `json:"dataItemCount"`
	ExternalCount   int    `json:"externalInterfaceCount"`
	SQLCount        int    `json:"sqlStatementCount"`
	CICSCount       int    `json:"cicsTransactionCount"`
	LineCount       int    `json:"lineCount"`
	TShirtSize      string `json:"tShirtSize"` // S, M, L, XL
	ComplexityScore int    `json:"complexityScore"`
	Approach        string `json:"approach,omitempty"`
}

// IDMSRecordInfo holds IDMS record details for API responses.
type IDMSRecordInfo struct {
	Name      string `json:"name"`
	Area      string `json:"area,omitempty"`
	ProgramID string `json:"programId"`
}

// IDMSSchemaInfo holds IDMS schema details for API responses.
type IDMSSchemaInfo struct {
	SchemaName    string `json:"schemaName"`
	SubschemaName string `json:"subschemaName"`
	ProtocolMode  string `json:"protocolMode,omitempty"`
	ProgramID     string `json:"programId"`
}

// IDMSAreaInfo holds IDMS area details for API responses.
type IDMSAreaInfo struct {
	Name      string `json:"name"`
	UsageMode string `json:"usageMode,omitempty"`
}

// IDMSImpactInfo holds IDMS record impact analysis.
type IDMSImpactInfo struct {
	RecordName string   `json:"recordName"`
	Navigators []string `json:"navigators"` // programs that OBTAIN/FIND/GET
	Storers    []string `json:"storers"`    // programs that STORE
	Modifiers  []string `json:"modifiers"`  // programs that MODIFY
	Erasers    []string `json:"erasers"`    // programs that ERASE
}

// ExternalDBTableInfo holds external DB table details for API responses.
type ExternalDBTableInfo struct {
	Name         string `json:"name"`
	Schema       string `json:"schema,omitempty"`
	DatabaseName string `json:"databaseName"`
	DatabaseType string `json:"databaseType"`
	Columns      string `json:"columns,omitempty"` // JSON array of column objects
}

// ExternalDBMappingInfo holds a mapping between COBOL DB2 table and external table.
type ExternalDBMappingInfo struct {
	CobolTable     string  `json:"cobolTable"`
	ExternalTable  string  `json:"externalTable"`
	Confidence     float64 `json:"confidence"`
	Reason         string  `json:"reason,omitempty"`
	ColumnMappings string  `json:"columnMappings,omitempty"` // JSON array
}

// GapInfo holds gap analysis details.
type GapInfo struct {
	Side        string `json:"side"`
	TableName   string `json:"tableName"`
	ColumnName  string `json:"columnName,omitempty"`
	Description string `json:"description"`
}

// DataFlowPathInfo holds data flow path details.
type DataFlowPathInfo struct {
	CobolProgram  string `json:"cobolProgram"`
	Operation     string `json:"operation"`
	DB2Table      string `json:"db2Table"`
	ExternalTable string `json:"externalTable"`
	FlowType      string `json:"flowType"`
	Description   string `json:"description"`
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
