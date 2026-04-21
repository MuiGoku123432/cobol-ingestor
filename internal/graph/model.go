package graph

// FileType classifies source files in the COBOL codebase.
type FileType string

const (
	FileTypeCOBOL    FileType = "COBOL"
	FileTypeCopybook FileType = "COPYBOOK"
	FileTypeJCL      FileType = "JCL"
	FileTypePending  FileType = "PENDING" // awaiting content-based classification
)

// FileInfo represents a discovered source file.
type FileInfo struct {
	Path       string
	Type       FileType
	Hash       string  // SHA-256
	Size       int64
	LineCount  int
	Confidence float64 // 0.0-1.0 classification confidence
	Classifier string  // "EXTENSION", "LLM", "HEURISTIC", "CACHE"
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
	RelNavigates           RelType = "NAVIGATES"     // Program → IDMSRecord (OBTAIN/FIND/GET)
	RelStoresIn            RelType = "STORES_IN"     // Program → IDMSRecord (STORE)
	RelModifiesRec         RelType = "MODIFIES"      // Program → IDMSRecord (MODIFY)
	RelErasesRec           RelType = "ERASES"        // Program → IDMSRecord (ERASE)
	RelBindsTo             RelType = "BINDS_TO"      // Program → IDMSSchema
	RelReadyArea           RelType = "READIES"       // Program → IDMSArea
	RelConnectsSet         RelType = "CONNECTS"      // Program → IDMSSet
	RelDisconnectsSet      RelType = "DISCONNECTS"   // Program → IDMSSet
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

// IDMSRecord represents an IDMS database record node.
type IDMSRecord struct {
	ID        string
	Name      string
	Area      string
	Schema    string
	ProgramID string
}

// IDMSSchema represents an IDMS schema/subschema binding.
type IDMSSchema struct {
	ID             string
	SchemaName     string
	SubschemaName  string
	ProgramID      string
	ProtocolMode   string
}

// IDMSArea represents an IDMS database area.
type IDMSArea struct {
	ID        string
	Name      string
	UsageMode string
	Schema    string
}

// IDMSSet represents an IDMS set relationship.
type IDMSSet struct {
	ID           string
	Name         string
	OwnerRecord  string
	MemberRecord string
	Schema       string
}

// IDMSOperation represents an IDMS DML operation extracted in Pass 2.
type IDMSOperation struct {
	Verb       string
	Record     string
	Area       string
	Set        string
	CalcKey    string
	Navigation string
	Paragraph  string
	UsageMode  string
}

// Pass1Result aggregates all extracted data from a single file's Pass 1 analysis.
type Pass1Result struct {
	SourceFile         string
	Partial            bool // true when recovered from a truncated LLM response
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
	IDMSRecords        []IDMSRecord
	IDMSSchemas        []IDMSSchema
	IDMSAreas          []IDMSArea
	IDMSSets           []IDMSSet
	Relationships      []Relationship
}

// Pass2Result aggregates deep semantic analysis from a single file's Pass 2 analysis.
type Pass2Result struct {
	SourceFile     string
	ProgramID      string
	Partial        bool // true when recovered from a truncated LLM response
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
	IDMSOperations []IDMSOperation
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
	Channel     string // FILE, DB2, LINKAGE, CICS_COMMAREA, CICS_TS, CICS_TD, MQ, JCL_STEP
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

// ExternalDatabase represents a modern database (Oracle, Postgres, etc.)
type ExternalDatabase struct {
	ID             string
	Name           string
	DatabaseType   string
	ConnectionInfo string
}

// ExternalDBTable represents a table in an external database.
type ExternalDBTable struct {
	ID           string
	Name         string
	Schema       string
	DatabaseName string
	DatabaseType string
	Columns      []ExtDBColumn
}

// ExtDBColumn represents a column in an external database table.
type ExtDBColumn struct {
	Name     string
	DataType string
	Nullable bool
	IsPK     bool
}

// GapInfo describes a table/column present only on one side.
type GapInfo struct {
	Side        string // "cobol_only" or "external_only"
	TableName   string
	ColumnName  string
	Description string
}

// DataFlowPath describes an end-to-end flow: COBOL program -> DB2 -> external DB.
type DataFlowPath struct {
	CobolProgram  string
	Operation     string
	DB2Table      string
	ExternalTable string
	FlowType      string
	Description   string
}

// DBTableMapping maps a COBOL DB2 table to an external table.
type DBTableMapping struct {
	CobolDBTable   string
	ExternalTable  string
	Confidence     float64
	Reason         string
	ColumnMappings []ColumnMapping
}

// ColumnMapping maps a COBOL column to an external column.
type ColumnMapping struct {
	CobolColumn    string
	ExternalColumn string
	Transform      string // EXACT, RENAMED, TYPE_CHANGED
}

// ExternalDBResult is the full analysis result from an external DB gap analysis.
type ExternalDBResult struct {
	Database ExternalDatabase
	Tables   []ExternalDBTable
	Mappings []DBTableMapping
	Gaps     []GapInfo
	Flows    []DataFlowPath
}

const (
	RelMapsToExtDB  RelType = "MAPS_TO_EXT_DB"
	RelHostedIn     RelType = "HOSTED_IN"
	RelBWContains   RelType = "BW_CONTAINS"
	RelBWRelatesTo  RelType = "BW_RELATES_TO"
	RelBWReferences RelType = "BW_REFERENCES"
)

// FileTypeBW classifies Businessware source files.
const FileTypeBW FileType = "BW"

// BWFile represents a Businessware source file node.
type BWFile struct {
	Path     string
	FileType string
	Summary  string
}

// BWEntity represents a flexible entity extracted by the LLM.
type BWEntity struct {
	Name        string
	EntityType  string
	Description string
	SourceFile  string
	MergeID     string         // sourceFile + "." + name (dedup key)
	Properties  map[string]any
}

// BWRelationship represents a relationship between two BW entities.
type BWRelationship struct {
	FromEntity   string
	ToEntity     string
	RelationType string
	Description  string
	Confidence   float64
}

// BWCobolReference represents a cross-link from a BW entity to a COBOL program or copybook.
type BWCobolReference struct {
	EntityName    string
	TargetName    string
	TargetType    string // "Program" or "Copybook"
	ReferenceType string
	Description   string
}

// BWResult aggregates extraction results for one Businessware file.
type BWResult struct {
	File            BWFile
	Entities        []BWEntity
	Relationships   []BWRelationship
	CobolReferences []BWCobolReference
}

// ---- Target Stack Types ----

const (
	// Target stack relationship types
	RelTSContains      RelType = "TS_CONTAINS"       // TargetRepo → TargetService
	RelTSExposes       RelType = "TS_EXPOSES"         // TargetService → TargetEndpoint
	RelTSEnforces      RelType = "TS_ENFORCES"        // TargetService → TargetBusinessRule
	RelTSModels        RelType = "TS_MODELS"          // TargetService → TargetDataModel
	RelTSIntegrates    RelType = "TS_INTEGRATES"      // TargetService → TargetIntegration
	RelTSHandlesError  RelType = "TS_HANDLES_ERROR"   // TargetService → TargetErrorHandler
	RelTSMapsToCobol   RelType = "TS_MAPS_TO_COBOL"  // TargetService → Program (coverage)
	RelTSRuleMapsTo    RelType = "TS_RULE_MAPS_TO"   // TargetBusinessRule → Paragraph (matched logic)
	RelGapFrom         RelType = "GAP_FROM"           // BusinessGap → source node
	RelGapTo           RelType = "GAP_TO"             // BusinessGap → target node
	RelRequirementFor  RelType = "REQUIREMENT_FOR"    // BusinessRequirement → BusinessGap
)

// TargetRepo represents a connected Git repository (the modern "target stack").
type TargetRepo struct {
	ID         string // URL-normalized unique key
	Name       string
	URL        string
	Provider   string // "github", "azure_devops", "generic"
	Branch     string
	LastCommit string // HEAD SHA at last analysis
	LocalPath  string // where cloned on disk
	Language   string // primary language detected
	Framework  string // primary framework detected
}

// TargetService represents a logical service or module extracted from a target repo.
type TargetService struct {
	ID          string // repoID + "::" + name
	Name        string
	ServiceType string // REST_API, GRPC, MESSAGE_CONSUMER, BATCH_JOB, LIBRARY
	Description string
	RepoURL     string
	BasePath    string // relative path within the repo
	Language    string
	Framework   string
}

// TargetEndpoint represents an API endpoint or message consumer entry point.
type TargetEndpoint struct {
	ID          string // serviceID + "::" + method + "::" + path
	Method      string // GET, POST, PUT, DELETE, CONSUME, PRODUCE
	Path        string
	Description string
	ServiceName string
	Parameters  string // JSON-encoded summary
}

// TargetBusinessRule represents an extracted business rule or validation.
type TargetBusinessRule struct {
	ID          string // serviceID + "::" + name
	Name        string
	Description string
	Category    string  // VALIDATION, CALCULATION, AUTHORIZATION, WORKFLOW, TRANSFORMATION
	ServiceName string
	SourceFile  string
	Confidence  float64
}

// TargetDataModel represents a data entity or model class.
type TargetDataModel struct {
	ID          string // serviceID + "::" + name
	Name        string
	Description string
	ServiceName string
	SourceFile  string
	Fields      string // JSON-encoded field definitions
	TableName   string // database table if ORM-mapped
}

// TargetIntegration represents an external integration point in the target stack.
type TargetIntegration struct {
	ID              string
	IntegrationType string // DATABASE, REST_CLIENT, MESSAGE_QUEUE, FILE_IO, CACHE, EXTERNAL_API
	Target          string // connection string, URL, queue name
	Description     string
	ServiceName     string
}

// TargetErrorHandler represents an error handling pattern in the target stack.
type TargetErrorHandler struct {
	ID          string
	Pattern     string // TRY_CATCH, ERROR_MIDDLEWARE, CIRCUIT_BREAKER, RETRY, FALLBACK
	Description string
	ServiceName string
	SourceFile  string
}

// BusinessGap represents an identified gap between COBOL and the target stack.
type BusinessGap struct {
	ID           string
	GapType      string  // COBOL_ONLY, TARGET_ONLY, PARTIAL_MATCH, SEMANTIC_MISMATCH
	Category     string  // BUSINESS_RULE, DATA_MODEL, INTEGRATION, ERROR_HANDLING, BATCH_PROCESSING
	Description  string
	Severity     string  // CRITICAL, HIGH, MEDIUM, LOW
	CobolSource  string  // Program/paragraph reference
	TargetSource string  // Service/file reference
	Confidence   float64
}

// BusinessRequirement represents a generated business requirement derived from gaps.
type BusinessRequirement struct {
	ID                 string
	Title              string
	Description        string
	Priority           string // P0, P1, P2, P3
	Category           string
	AcceptanceCriteria string // JSON-encoded []string
	EstimatedEffort    string
	GapID              string
}

// GlossaryTerm represents a company glossary entry (term or acronym).
type GlossaryTerm struct {
	Term       string   // canonical form, e.g. "PBT" or "Profit Before Tax"
	Kind       string   // "term" | "acronym"
	Definition string
	Aliases    []string // synonyms / alternate spellings
	Codebase   string   // scope (matches ingest codebase flag)
	SourceFile string   // path to the source HTML file
}

// GlossaryResult aggregates all terms extracted from one glossary file.
type GlossaryResult struct {
	SourceFile string
	Terms      []GlossaryTerm
}

// TargetStackResult aggregates extraction results for one target repo.
type TargetStackResult struct {
	Repo         TargetRepo
	Services     []TargetService
	Endpoints    []TargetEndpoint
	Rules        []TargetBusinessRule
	DataModels   []TargetDataModel
	Integrations []TargetIntegration
	ErrorHandlers []TargetErrorHandler
	Relationships []Relationship
}
