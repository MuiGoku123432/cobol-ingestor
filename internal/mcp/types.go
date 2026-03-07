package mcp

// Input structs for MCP tools.
// google/jsonschema-go v0.4.2 uses the jsonschema tag as a plain description.
// Required is inferred from the absence of json:"...,omitempty".

type GetProgramInput struct {
	ProgramID string `json:"programId" jsonschema:"The COBOL program ID"`
}

type SearchProgramsInput struct {
	Query string `json:"query" jsonschema:"Search query (supports fuzzy matching)"`
	Limit int    `json:"limit,omitempty" jsonschema:"Max results (default 20)"`
}

type ListProgramsInput struct {
	Search   string `json:"search,omitempty" jsonschema:"Filter programs by ID substring"`
	Page     int    `json:"page,omitempty" jsonschema:"Page number (default 1)"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"Results per page (default 20)"`
}

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
