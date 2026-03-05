package neo4j

import "time"

// ProgramSummary is a lightweight program representation for list views.
type ProgramSummary struct {
	ProgramID string `json:"programId"`
	FilePath  string `json:"filePath"`
	Language  string `json:"language"`
	CallCount int    `json:"callCount"`
	DeadCode  bool   `json:"deadCode,omitempty"`
}

// ProgramDetail is a full program representation with relationships.
type ProgramDetail struct {
	ProgramID    string           `json:"programId"`
	FilePath     string           `json:"filePath"`
	Language     string           `json:"language"`
	DeadCode     bool             `json:"deadCode,omitempty"`
	RiskScore    float64          `json:"riskScore,omitempty"`
	RiskType     string           `json:"riskType,omitempty"`
	Callers      []string         `json:"callers"`
	Callees      []string         `json:"callees"`
	Copybooks    []string         `json:"copybooks"`
	Paragraphs   []ParagraphInfo  `json:"paragraphs"`
	Sections     []string         `json:"sections"`
	DataItems    []DataItemInfo   `json:"dataItems"`
	FileDefs     []string         `json:"fileDefs"`
	Domains      []string         `json:"domains"`
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
	ProgramCount      int `json:"programCount"`
	CopybookCount     int `json:"copybookCount"`
	ParagraphCount    int `json:"paragraphCount"`
	DataItemCount     int `json:"dataItemCount"`
	RelationshipCount int `json:"relationshipCount"`
	OrphanCount       int `json:"orphanCount"`
	DomainCount       int `json:"domainCount"`
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
