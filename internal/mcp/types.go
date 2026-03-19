package mcp

// Input structs for MCP tools.
// google/jsonschema-go v0.4.2 uses the jsonschema tag as a plain description.
// Required is inferred from the absence of json:"...,omitempty".

type GetProgramInput struct {
	ProgramID string `json:"programId" jsonschema:"The COBOL program ID"`
}

type SearchProgramsInput struct {
	Query    string `json:"query" jsonschema:"Search query (supports fuzzy matching)"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default 20)"`
	Codebase string `json:"codebase,omitempty" jsonschema:"Filter to a specific codebase"`
}

type ListProgramsInput struct {
	Search   string `json:"search,omitempty" jsonschema:"Filter programs by ID substring"`
	Page     int    `json:"page,omitempty" jsonschema:"Page number (default 1)"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"Results per page (default 20)"`
	Codebase string `json:"codebase,omitempty" jsonschema:"Filter to a specific codebase"`
}

type ListCodebasesInput struct{}

type GetCrossCodebaseCallsInput struct{}

type GetCallChainInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to trace"`
	Direction string `json:"direction,omitempty" jsonschema:"upstream or downstream (default downstream)"`
	Depth     int    `json:"depth,omitempty" jsonschema:"Traversal depth 1-10 (default 3)"`
}

type GetImpactAnalysisInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to analyze"`
}

type GetCopybookUsageInput struct {
	Name string `json:"name" jsonschema:"The copybook name"`
}

type GetDataItemsInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID"`
}

type GetBusinessDomainInput struct {
	Name string `json:"name" jsonschema:"The business domain name"`
}

type ListRiskProgramsInput struct {
	MinScore float64 `json:"minScore,omitempty" jsonschema:"Minimum risk score threshold (default 0.5)"`
}

type GetJCLJobInput struct {
	JobName string `json:"jobName" jsonschema:"The JCL job name"`
}

type GetDatasetUsageInput struct {
	DSName string `json:"dsname" jsonschema:"Dataset name or substring to search"`
}

type GetTableUsageInput struct {
	TableName string `json:"tableName" jsonschema:"The database table name"`
}

type TraceFieldImpactInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID"`
	FieldName string `json:"fieldName" jsonschema:"The field name to trace"`
}

type GetCopybookStructureInput struct {
	Name string `json:"name" jsonschema:"The copybook name"`
}

type GetFileAccessorsInput struct {
	FileName string `json:"fileName" jsonschema:"The file name to look up"`
}

type GetTypeMappingsInput struct {
	ProgramID    string `json:"programId,omitempty" jsonschema:"Program ID to get type mappings for"`
	CopybookName string `json:"copybookName,omitempty" jsonschema:"Copybook name to get type mappings for"`
}

type ReassignDomainInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to reassign"`
	Domain    string `json:"domain" jsonschema:"The target business domain name"`
}

type GetIDMSRecordsInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to get IDMS records for"`
}

type GetIDMSSchemaInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to get IDMS schema for"`
}

type GetIDMSImpactInput struct {
	RecordName string `json:"recordName" jsonschema:"The IDMS record name to analyze"`
}

type GetIDMSAreasInput struct {
	ProgramID string `json:"programId" jsonschema:"The program ID to get IDMS areas for"`
}

type GetExternalDBMappingInput struct {
	TableName string `json:"tableName" jsonschema:"The external database table name"`
}

type GetCobolToExternalMappingsInput struct {
	CobolTable string `json:"cobolTable" jsonschema:"The COBOL DB2 table name"`
}

type GetDataFlowPathsInput struct {
	TableName string `json:"tableName,omitempty" jsonschema:"Filter by table name (DB2 or external). Leave empty for all flows."`
}
