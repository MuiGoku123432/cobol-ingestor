package mcp

// Input structs for MCP tools. Tags drive auto-schema generation.

type GetProgramInput struct {
	ProgramID string `json:"programId" jsonschema:"required,description=The COBOL program ID"`
}

type SearchProgramsInput struct {
	Query string `json:"query" jsonschema:"required,description=Search query (supports fuzzy matching)"`
	Limit int    `json:"limit,omitempty" jsonschema:"description=Max results (default 20)"`
}

type ListProgramsInput struct {
	Search   string `json:"search,omitempty" jsonschema:"description=Filter programs by ID substring"`
	Page     int    `json:"page,omitempty" jsonschema:"description=Page number (default 1)"`
	PageSize int    `json:"pageSize,omitempty" jsonschema:"description=Results per page (default 20)"`
}

type GetCallChainInput struct {
	ProgramID string `json:"programId" jsonschema:"required,description=The program ID to trace"`
	Direction string `json:"direction,omitempty" jsonschema:"description=upstream or downstream (default downstream)"`
	Depth     int    `json:"depth,omitempty" jsonschema:"description=Traversal depth 1-10 (default 3)"`
}

type GetImpactAnalysisInput struct {
	ProgramID string `json:"programId" jsonschema:"required,description=The program ID to analyze"`
}

type GetCopybookUsageInput struct {
	Name string `json:"name" jsonschema:"required,description=The copybook name"`
}

type GetDataItemsInput struct {
	ProgramID string `json:"programId" jsonschema:"required,description=The program ID"`
}

type GetBusinessDomainInput struct {
	Name string `json:"name" jsonschema:"required,description=The business domain name"`
}
