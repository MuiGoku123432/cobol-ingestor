package graph

// FileType classifies source files in the COBOL codebase.
type FileType string

const (
	FileTypeCOBOL    FileType = "COBOL"
	FileTypeCopybook FileType = "COPYBOOK"
	FileTypeJCL      FileType = "JCL"
)

// FileInfo represents a discovered source file.
type FileInfo struct {
	Path      string
	Type      FileType
	Hash      string // SHA-256
	Size      int64
	LineCount int
}

// RelType enumerates Neo4j relationship types.
type RelType string

const (
	RelCalls        RelType = "CALLS"
	RelIncludes     RelType = "INCLUDES"
	RelReads        RelType = "READS"
	RelWrites       RelType = "WRITES"
	RelPerforms     RelType = "PERFORMS"
	RelPerformsThru RelType = "PERFORMS_THRU"
	RelBelongsTo    RelType = "BELONGS_TO"
	RelChildOf      RelType = "CHILD_OF"
	RelRedefines    RelType = "REDEFINES"
	RelConditionOf  RelType = "CONDITION_OF"
	RelParameterOf  RelType = "PARAMETER_OF"
	RelDefinedIn    RelType = "DEFINED_IN"
	RelMovesTo      RelType = "MOVES_TO"
	RelRuns         RelType = "RUNS"
	RelExecutesSQL  RelType = "EXECUTES_SQL"
	RelExecutesCICS        RelType = "EXECUTES_CICS"
	RelExternalInterface   RelType = "HAS_INTERFACE"
	RelStepOf              RelType = "STEP_OF"
	RelUsesDataset         RelType = "USES_DATASET"
	RelMapsToFile          RelType = "MAPS_TO_FILE"
	RelAccesses            RelType = "ACCESSES"
	RelDataFlowsTo         RelType = "DATA_FLOWS_TO"
	RelLinkageMapsTo       RelType = "LINKAGE_MAPS_TO"
)

// Relationship is a generic edge in the graph.
type Relationship struct {
	Type       RelType
	FromLabel  string
	FromKey    string
	ToLabel    string
	ToKey      string
	Properties map[string]any
}

// Program represents a COBOL program node.
type Program struct {
	ID            string
	ProgramID     string
	FilePath      string
	Language      string
	LineCount     int
	ExecutionMode string // BATCH, CICS, BATCH_AND_CICS, UNKNOWN
}

// Paragraph represents a PROCEDURE DIVISION paragraph.
type Paragraph struct {
	ID        string
	Name      string
	ProgramID string
}

// Section represents a PROCEDURE DIVISION section.
type Section struct {
	ID        string
	Name      string
	ProgramID string
}

// Copybook represents a COPY member.
type Copybook struct {
	ID   string
	Name string
}

// DataItem represents a data item declaration.
type DataItem struct {
	ID        string
	Name      string
	Level     int
	ProgramID string
	FQN       string // fully qualified name: PROGRAM.LEVEL.NAME
	Picture   string
	Usage     string // COMP, COMP-3, BINARY, POINTER, etc.
}

// Condition represents an 88-level condition variable.
type Condition struct {
	ID        string
	Name      string
	Parent    string // parent data item name
	Value     string // condition value(s)
	ProgramID string
	FQN       string // fully qualified name: programId.name
}

// Parameter represents a LINKAGE SECTION data item.
type Parameter struct {
	ID        string
	Name      string
	Level     int
	Direction string // IN, OUT, INOUT
	ProgramID string
	FQN       string // fully qualified name: programId.name
}

// FileDefinition represents an FD (file description).
type FileDefinition struct {
	ID            string
	Name          string
	ProgramID     string
	Organization  string
	VSAMType      string // KSDS, ESDS, RRDS, or empty
	DataStoreType string // VSAM, DB2, IMS, FLAT_FILE
}

// SQLStatement represents an EXEC SQL block.
type SQLStatement struct {
	ID          string
	Text        string
	ProgramID   string
	Type        string // SELECT, INSERT, UPDATE, DELETE, etc.
	TargetTable string
}

// CICSTransaction represents an EXEC CICS command.
type CICSTransaction struct {
	ID        string
	Command   string
	ProgramID string
}

// JCLJob represents a JCL job definition.
type JCLJob struct {
	ID       string
	JobName  string
	Class    string
	MsgClass string
	Region   string
	Cond     string
}

// JCLStep represents a JCL step.
type JCLStep struct {
	ID       string
	StepName string
	Program  string
	Proc     string
	Cond     string
	JobName  string
	Order    int
}

// BusinessDomain represents a business domain cluster (Pass 3).
type BusinessDomain struct {
	ID          string
	Name        string
	Description string
}

// ExternalInterface represents an external integration point.
type ExternalInterface struct {
	ID        string
	Type      string // MQ, CICS_LINK, CICS_XCTL, CICS_TS, CICS_TD, CICS_START, CICS_FILE, CICS_ENQ, IMS, IDMS, ADABAS, SORT, BATCH_UTIL, TCP, FILE_TRANSFER
	Details   string
	Paragraph string
	ProgramID string
}

// VolumeEstimate is a heuristic transaction volume estimate.
type VolumeEstimate struct {
	ProgramID string
	Estimate  string // HIGH, MEDIUM, LOW
	Reason    string
}

// Pass1Result aggregates all extracted data from a single file's Pass 1 analysis.
type Pass1Result struct {
	SourceFile         string
	Programs           []Program
	Paragraphs         []Paragraph
	Sections           []Section
	Copybooks          []Copybook
	DataItems          []DataItem
	Conditions         []Condition
	Parameters         []Parameter
	FileDefs           []FileDefinition
	SQLStatements      []SQLStatement
	CICSTxns           []CICSTransaction
	ExternalInterfaces []ExternalInterface
	DBTables           []DBTable
	Relationships      []Relationship
}

// Pass2Result aggregates deep semantic analysis from a single file's Pass 2 analysis.
type Pass2Result struct {
	SourceFile     string
	ProgramID      string
	Performs       []PerformRelation
	DataFlows      []DataFlowRelation
	FileOps        []FileOpRelation
	SQLDetails     []SQLStatement
	CICSDetails    []CICSTransaction
	DataHierarchy  []DataHierarchyItem
	Redefines      []RedefineRelation
	CopybookDefs   []CopybookDefRelation
	Annotations    []Annotation
	ConditionalLogic []ConditionalLogicItem
	DynamicCallResolutions []DynamicCallResolution
	ErrorHandlers  []ErrorHandler
}

// PerformRelation represents a PERFORM control flow.
type PerformRelation struct {
	FromParagraph string
	ToParagraph   string
	ThruParagraph string
	IsLoop        bool
	Condition     string
}

// DataFlowRelation represents a MOVE or data transfer.
type DataFlowRelation struct {
	FromItem string
	ToItem   string
	Context  string
}

// FileOpRelation represents a file I/O operation.
type FileOpRelation struct {
	Operation string
	FileName  string
	Paragraph string
}

// DataHierarchyItem represents a data item in the DATA DIVISION hierarchy.
type DataHierarchyItem struct {
	Name     string
	Level    int
	Parent   string
	Picture  string
	Copybook string
}

// RedefineRelation represents a REDEFINES clause.
type RedefineRelation struct {
	Item      string
	Redefines string
}

// CopybookDefRelation links a data item to its defining copybook.
type CopybookDefRelation struct {
	DataItem string
	Copybook string
}

// Annotation describes a paragraph's purpose.
type Annotation struct {
	Paragraph   string
	Description string
	Category    string
}

// ConditionalLogicItem represents an IF/EVALUATE decision point.
type ConditionalLogicItem struct {
	Paragraph string
	Condition string
	Variables []string
	Type      string // IF, EVALUATE
}

// DynamicCallResolution represents a resolved dynamic CALL target.
type DynamicCallResolution struct {
	Variable        string
	ResolvedTargets []string
	Paragraph       string
}

// ErrorHandler represents an error handling pattern in a paragraph.
type ErrorHandler struct {
	Paragraph string
	Pattern   string // STRUCTURED, AD-HOC, FILE-STATUS, SQLCODE, CICS-RESP
	Details   string
}

// BridgeProgram identifies a program connecting multiple domains.
type BridgeProgram struct {
	ProgramID string
	Domains   []string
	Reason    string
}

// CopybookRisk identifies high-risk shared copybooks.
type CopybookRisk struct {
	Copybook     string
	ProgramCount int
	RiskLevel    string
	Reason       string
}

// ModernizationCandidate identifies a program suitable for extraction.
type ModernizationCandidate struct {
	ProgramID  string
	Score      float64
	Reason     string
	Approach   string
}

// Pass3Result aggregates cross-cutting analysis results.
type Pass3Result struct {
	BusinessDomains         []BusinessDomain
	DomainMembers           []DomainMembership
	DeadCodeFlags           []DeadCodeFlag
	RiskFlags               []RiskFlag
	BridgePrograms          []BridgeProgram
	CopybookRisks           []CopybookRisk
	ModernizationCandidates []ModernizationCandidate
	VolumeEstimates         []VolumeEstimate
}

// DomainMembership links a program to a business domain.
type DomainMembership struct {
	ProgramID  string
	DomainName string
	Confidence float64
}

// DeadCodeFlag marks a program as potentially dead code.
type DeadCodeFlag struct {
	ProgramID string
	Reason    string
}

// RiskFlag captures a program's risk assessment (all programs receive one).
type RiskFlag struct {
	ProgramID string
	RiskType  string
	Details   string
	Score     float64
}

// DDCard represents a JCL DD card (dataset definition).
type DDCard struct {
	ID       string
	DDName   string
	DSName   string
	Disp     string
	IsInput  bool
	IsOutput bool
	JobName  string
	StepName string
}

// JCLAnalysisResult aggregates JCL extraction results.
type JCLAnalysisResult struct {
	SourceFile string
	Jobs       []JCLJob
	Steps      []JCLStep
	DDCards    []DDCard
	Relationships []Relationship
}

// DBTable represents a database table accessed by COBOL programs.
type DBTable struct {
	ID         string
	Name       string
	Schema     string
	Columns    []string
	Operations []string
}

// CrossProgramFlow represents a data flow between programs.
type CrossProgramFlow struct {
	FromProgram string
	ToProgram   string
	Channel     string // FILE, DB2, LINKAGE, CICS_COMMAREA
	Fields      []FieldPair
	SharedResource string // file name, table name, etc.
}

// FieldPair maps a source field to a target field.
type FieldPair struct {
	SourceField string
	TargetField string
	Transform   string // DIRECT_MOVE, COMPUTE, etc.
}

// Pass4Result aggregates cross-program data flow analysis.
type Pass4Result struct {
	Flows []CrossProgramFlow
}
