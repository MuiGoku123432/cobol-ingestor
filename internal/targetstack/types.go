package targetstack

import "cobol-ingestor/internal/graph"

// FileCategory classifies discovered source files.
type FileCategory string

const (
	FileCategorySource FileCategory = "SOURCE"
	FileCategoryConfig FileCategory = "CONFIG"
	FileCategorySchema FileCategory = "SCHEMA"
	FileCategoryTest   FileCategory = "TEST"
	FileCategoryBuild  FileCategory = "BUILD"
	FileCategoryOther  FileCategory = "OTHER"
)

// ScannedFile holds metadata for a discovered file.
type ScannedFile struct {
	Path     string
	RelPath  string // relative to repo root
	Hash     string // SHA-256
	Size     int64
	Language string
	Category FileCategory
}

// ScanResult aggregates all files discovered in a repo.
type ScanResult struct {
	RepoURL   string
	LocalPath string
	HeadSHA   string
	Files     []ScannedFile
	ByLang    map[string]int // language → file count
}

// ExtractionResult holds per-file LLM extraction output before merging.
type ExtractionResult struct {
	SourceFile string
	Services   []graph.TargetService
	Endpoints  []graph.TargetEndpoint
	Rules      []graph.TargetBusinessRule
	DataModels []graph.TargetDataModel
	Integrations []graph.TargetIntegration
	ErrorHandlers []graph.TargetErrorHandler
}

// RepoConfig holds connection details for a single target repository.
type RepoConfig struct {
	URL      string
	Branch   string
	Provider string // "github", "azure_devops", "generic"
	Token    string // PAT
}

// CloneResult holds information about a cloned/updated repository.
type CloneResult struct {
	Config    RepoConfig
	LocalPath string
	HeadSHA   string
	Updated   bool // true if HEAD SHA changed since last run
}
