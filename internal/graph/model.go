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
	Path string
	Type FileType
	Hash string // SHA-256
	Size int64
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
	RelDefinedIn    RelType = "DEFINED_IN"
	RelMovesTo      RelType = "MOVES_TO"
	RelRuns         RelType = "RUNS"
	RelExecutesSQL  RelType = "EXECUTES_SQL"
	RelExecutesCICS RelType = "EXECUTES_CICS"
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
	ID        string
	ProgramID string
	FilePath  string
	Language  string
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
}

// FileDefinition represents an FD (file description).
type FileDefinition struct {
	ID           string
	Name         string
	ProgramID    string
	Organization string
}

// SQLStatement represents an EXEC SQL block.
type SQLStatement struct {
	ID        string
	Text      string
	ProgramID string
	Type      string // SELECT, INSERT, UPDATE, DELETE, etc.
}

// CICSTransaction represents an EXEC CICS command.
type CICSTransaction struct {
	ID        string
	Command   string
	ProgramID string
}

// JCLJob represents a JCL job definition.
type JCLJob struct {
	ID      string
	JobName string
}

// JCLStep represents a JCL step.
type JCLStep struct {
	ID       string
	StepName string
	Program  string
	JobID    string
}

// BusinessDomain represents a business domain cluster (Pass 3).
type BusinessDomain struct {
	ID          string
	Name        string
	Description string
}

// Pass1Result aggregates all extracted data from a single file's Pass 1 analysis.
type Pass1Result struct {
	SourceFile    string
	Programs      []Program
	Paragraphs    []Paragraph
	Sections      []Section
	Copybooks     []Copybook
	DataItems     []DataItem
	FileDefs      []FileDefinition
	SQLStatements []SQLStatement
	CICSTxns      []CICSTransaction
	Relationships []Relationship
}

// Pass2Result aggregates deep semantic analysis from a single file's Pass 2 analysis.
type Pass2Result struct {
	SourceFile    string
	ProgramID     string
	Performs      []PerformRelation
	DataFlows     []DataFlowRelation
	FileOps       []FileOpRelation
	SQLDetails    []SQLStatement
	CICSDetails   []CICSTransaction
	DataHierarchy []DataHierarchyItem
	Redefines     []RedefineRelation
	CopybookDefs  []CopybookDefRelation
	Annotations   []Annotation
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

// Pass3Result aggregates cross-cutting analysis results.
type Pass3Result struct {
	BusinessDomains []BusinessDomain
	DomainMembers   []DomainMembership
	DeadCodeFlags   []DeadCodeFlag
	RiskFlags       []RiskFlag
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

// RiskFlag marks a program as high-risk.
type RiskFlag struct {
	ProgramID string
	RiskType  string
	Details   string
	Score     float64
}
