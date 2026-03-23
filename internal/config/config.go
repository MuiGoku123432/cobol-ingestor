package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	DataDir    string // COBOL_GRAPH_DATA_DIR — base directory for persistent data (default ~/.cobol-graph)
	LLM        LLMConfig
	Claude     ClaudeConfig
	Neo4j      Neo4jConfig
	Ingest     IngestConfig
	API        APIConfig
	MCP        MCPConfig
	Modernize  ModernizeConfig
	ExternalDB ExternalDBConfig
	BW         BWConfig
}

// BWConfig holds settings for Businessware ingestion.
type BWConfig struct {
	Dir        string // BW_DIR — root directory of Businessware files
	Extensions string // BW_EXTENSIONS — comma-separated file extensions (default ".java,.md,.bw,.txt,.xml,.vsdx,.drawio,.svg,.puml,.plantuml,.jar")
	MaxWorkers int    // BW_MAX_WORKERS — concurrent analysis workers (default 5)
	MaxTokens  int    // BW_MAX_TOKENS — max output tokens per LLM call (default 16000)
	TokenLimit int    // BW_TOKEN_LIMIT — input chunking token limit (default 30000)
	JavapPath  string // BW_JAVAP_PATH — path to javap binary (auto-detected if empty)
}

// ExternalDBConfig holds settings for external database gap analysis via MCP.
type ExternalDBConfig struct {
	DBMCPCommand   string // EXTDB_MCP_CMD — shell command to start external DB MCP server
	DBMCPServerURL string // EXTDB_MCP_URL — HTTP endpoint alternative
	GraphMCPBin    string // EXTDB_GRAPH_MCP_BIN — path to cobol-graph-mcp binary
	GraphMCPURL    string // EXTDB_GRAPH_MCP_URL — HTTP endpoint alternative
	MaxIterations  int    // EXTDB_MAX_ITERATIONS (default 20)
	MaxTokens      int    // EXTDB_MAX_TOKENS (default 16000)
	DatabaseName   string // EXTDB_DATABASE_NAME — human label
	DatabaseType   string // EXTDB_DATABASE_TYPE — "oracle", "postgres", etc.

	// Oracle SQLcl auto-launch settings
	OracleHost       string // ORACLE_HOST (default "localhost")
	OraclePort       string // ORACLE_PORT (default "1521")
	OracleService    string // ORACLE_SERVICE
	OracleUser       string // ORACLE_USER
	OraclePassword   string // ORACLE_PASSWORD
	OracleWalletPath string // ORACLE_WALLET_PATH
	OracleTNSAdmin   string // ORACLE_TNS_ADMIN
	OracleSQLclPath  string // ORACLE_SQLCL_PATH
}

type MCPConfig struct {
	HTTPPort string // MCP_HTTP_PORT — if set, serve Streamable HTTP instead of stdio
}

type ModernizeConfig struct {
	Port          string // MODERNIZE_PORT
	MCPTransport  string // MCP_TRANSPORT: "command" or "http"
	MCPServerBin  string // MCP_SERVER_BIN
	MCPServerURL  string // MCP_SERVER_URL (for http transport)
	ChatModel     string // MODERNIZE_CHAT_MODEL
	ChatMaxTokens int    // MODERNIZE_CHAT_MAX_TOKENS
}

// LLMConfig selects which provider backend to use.
type LLMConfig struct {
	Provider              string        // "anthropic", "copilot", "vertex", "bedrock", or "openai"
	APIKey                string        // ANTHROPIC_API_KEY or GitHub token depending on provider
	CopilotGitHubToken    string        // GitHub personal access token for Copilot auth
	CopilotAccountType    string        // "individual", "business", or "enterprise"
	VertexProjectID       string        // Google Cloud project ID (for vertex provider)
	VertexRegion          string        // Google Cloud region (for vertex provider, default "us-east5")
	BedrockRegion         string        // AWS region (for bedrock provider, default "us-east-1")
	BedrockModelID        string        // Optional Bedrock model ID override
	OpenAIAPIKey          string        // OPENAI_API_KEY
	OpenAIBaseURL         string        // OPENAI_BASE_URL (for Azure or proxies)
	OpenAIOrgID           string        // OPENAI_ORG_ID
	OpenAIModel           string        // OPENAI_MODEL (default "gpt-4o")
	Timeout               time.Duration // Overall HTTP client timeout (LLM_TIMEOUT)
	ResponseHeaderTimeout time.Duration // Time to wait for first response byte (LLM_RESPONSE_HEADER_TIMEOUT)
}

// ClaudeConfig holds model names and token settings (used by both providers).
type ClaudeConfig struct {
	OpusModel        string
	SonnetModel      string
	MaxRetries       int
	Pass1MaxTokens   int
	Pass2MaxTokens   int
	Pass3MaxTokens   int
	Pass4MaxTokens   int
	RequestTimeout   time.Duration
	DisableRateLimit bool
}

type Neo4jConfig struct {
	URI      string
	User     string
	Password string
	Database string
}

type IngestConfig struct {
	RootDir         string
	Codebase        string // INGEST_CODEBASE — codebase identifier for multi-codebase support (default "default")
	BatchSize       int
	CacheDB         string
	TokenLimit      int
	MaxWorkers      int
	Pass1MaxWorkers int // PASS1_MAX_WORKERS — defaults to MaxWorkers if 0
	Pass2MaxWorkers int // PASS2_MAX_WORKERS — defaults to MaxWorkers if 0
	Pass5MaxWorkers int // PASS5_MAX_WORKERS — defaults to MaxWorkers if 0
	Pass2TokenLimit int
	OverlapLines    int
	Pass3BatchSize        int
	StripSequenceColumns  bool // STRIP_SEQUENCE_COLUMNS — strip columns 1-6 and 73-80 from fixed-format COBOL
}

// WorkersForPass returns the worker count for a specific pass, falling back to MaxWorkers.
func (c *IngestConfig) WorkersForPass(pass int) int {
	switch pass {
	case 1:
		if c.Pass1MaxWorkers > 0 {
			return c.Pass1MaxWorkers
		}
	case 2:
		if c.Pass2MaxWorkers > 0 {
			return c.Pass2MaxWorkers
		}
	case 5:
		if c.Pass5MaxWorkers > 0 {
			return c.Pass5MaxWorkers
		}
	}
	return c.MaxWorkers
}

type APIConfig struct {
	Port     string
	LogLevel string
}

func Load() (*Config, error) {
	// Load .env file if present (ignore error if missing)
	_ = godotenv.Load()

	viper.AutomaticEnv()

	// LLM provider defaults
	viper.SetDefault("LLM_PROVIDER", "anthropic")
	viper.SetDefault("COPILOT_ACCOUNT_TYPE", "individual")
	viper.SetDefault("LLM_TIMEOUT", "600s")
	viper.SetDefault("LLM_RESPONSE_HEADER_TIMEOUT", "300s")
	viper.SetDefault("OPENAI_MODEL", "gpt-4o")

	// Claude model defaults (used by both providers)
	viper.SetDefault("CLAUDE_OPUS_MODEL", "claude-opus-4-6")
	viper.SetDefault("CLAUDE_SONNET_MODEL", "claude-sonnet-4-6")
	viper.SetDefault("CLAUDE_MAX_RETRIES", 3)
	viper.SetDefault("CLAUDE_PASS1_MAX_TOKENS", 8192)
	viper.SetDefault("CLAUDE_PASS2_MAX_TOKENS", 16000)
	viper.SetDefault("CLAUDE_PASS3_MAX_TOKENS", 16000)
	viper.SetDefault("CLAUDE_PASS4_MAX_TOKENS", 4000)
	viper.SetDefault("DISABLE_RATE_LIMIT", false)

	// Neo4j defaults
	viper.SetDefault("NEO4J_URI", "bolt://localhost:7687")
	viper.SetDefault("NEO4J_USER", "neo4j")
	viper.SetDefault("NEO4J_PASSWORD", "changeme")
	viper.SetDefault("NEO4J_DATABASE", "cobol")

	// Ingest defaults
	viper.SetDefault("INGEST_CODEBASE", "default")
	viper.SetDefault("INGEST_BATCH_SIZE", 500)
	viper.SetDefault("INGEST_CACHE_DB", "./cache.sqlite")
	viper.SetDefault("INGEST_TOKEN_LIMIT", 30000)
	viper.SetDefault("MAX_WORKERS", 15)
	viper.SetDefault("PASS1_MAX_WORKERS", 0)
	viper.SetDefault("PASS2_MAX_WORKERS", 0)
	viper.SetDefault("PASS5_MAX_WORKERS", 0)
	viper.SetDefault("PASS2_TOKEN_LIMIT", 20000)
	viper.SetDefault("PASS2_OVERLAP_LINES", 20)
	viper.SetDefault("PASS3_BATCH_SIZE", 50)
	viper.SetDefault("STRIP_SEQUENCE_COLUMNS", true)

	// API defaults
	viper.SetDefault("API_PORT", "8080")
	viper.SetDefault("API_LOG_LEVEL", "info")

	// MCP defaults
	viper.SetDefault("MCP_HTTP_PORT", "")

	// Data directory default
	defaultDataDir := filepath.Join(func() string { h, _ := os.UserHomeDir(); return h }(), ".cobol-graph")
	viper.SetDefault("COBOL_GRAPH_DATA_DIR", defaultDataDir)

	// External DB defaults
	viper.SetDefault("EXTDB_MCP_CMD", "")
	viper.SetDefault("EXTDB_MCP_URL", "")
	viper.SetDefault("EXTDB_GRAPH_MCP_BIN", "./bin/cobol-graph-mcp")
	viper.SetDefault("EXTDB_GRAPH_MCP_URL", "")
	viper.SetDefault("EXTDB_MAX_ITERATIONS", 20)
	viper.SetDefault("EXTDB_MAX_TOKENS", 16000)
	viper.SetDefault("EXTDB_DATABASE_NAME", "")
	viper.SetDefault("EXTDB_DATABASE_TYPE", "")

	// BW defaults
	viper.SetDefault("BW_DIR", "")
	viper.SetDefault("BW_EXTENSIONS", ".java,.md,.bw,.txt,.xml,.vsdx,.drawio,.svg,.puml,.plantuml,.jar")
	viper.SetDefault("BW_MAX_WORKERS", 5)
	viper.SetDefault("BW_MAX_TOKENS", 16000)
	viper.SetDefault("BW_TOKEN_LIMIT", 30000)

	// Oracle SQLcl defaults
	viper.SetDefault("ORACLE_HOST", "localhost")
	viper.SetDefault("ORACLE_PORT", "1521")

	// Modernize defaults
	viper.SetDefault("MODERNIZE_PORT", "8081")
	viper.SetDefault("MCP_TRANSPORT", "command")
	viper.SetDefault("MCP_SERVER_BIN", "./bin/cobol-graph-mcp")
	viper.SetDefault("MCP_SERVER_URL", "")
	viper.SetDefault("MODERNIZE_CHAT_MODEL", "")
	viper.SetDefault("MODERNIZE_CHAT_MAX_TOKENS", 16384)

	llmTimeout, err := time.ParseDuration(viper.GetString("LLM_TIMEOUT"))
	if err != nil {
		llmTimeout = 600 * time.Second
	}
	llmResponseHeaderTimeout, err := time.ParseDuration(viper.GetString("LLM_RESPONSE_HEADER_TIMEOUT"))
	if err != nil {
		llmResponseHeaderTimeout = 300 * time.Second
	}

	cfg := &Config{
		DataDir: viper.GetString("COBOL_GRAPH_DATA_DIR"),
		LLM: LLMConfig{
			Provider:              viper.GetString("LLM_PROVIDER"),
			APIKey:                viper.GetString("ANTHROPIC_API_KEY"),
			CopilotGitHubToken:    viper.GetString("COPILOT_GITHUB_TOKEN"),
			CopilotAccountType:    viper.GetString("COPILOT_ACCOUNT_TYPE"),
			VertexProjectID:       viper.GetString("VERTEX_PROJECT_ID"),
			VertexRegion:          viper.GetString("VERTEX_REGION"),
			BedrockRegion:         viper.GetString("BEDROCK_REGION"),
			BedrockModelID:        viper.GetString("BEDROCK_MODEL_ID"),
			OpenAIAPIKey:          viper.GetString("OPENAI_API_KEY"),
			OpenAIBaseURL:         viper.GetString("OPENAI_BASE_URL"),
			OpenAIOrgID:           viper.GetString("OPENAI_ORG_ID"),
			OpenAIModel:          viper.GetString("OPENAI_MODEL"),
			Timeout:               llmTimeout,
			ResponseHeaderTimeout: llmResponseHeaderTimeout,
		},
		Claude: ClaudeConfig{
			OpusModel:      viper.GetString("CLAUDE_OPUS_MODEL"),
			SonnetModel:    viper.GetString("CLAUDE_SONNET_MODEL"),
			MaxRetries:     viper.GetInt("CLAUDE_MAX_RETRIES"),
			Pass1MaxTokens: viper.GetInt("CLAUDE_PASS1_MAX_TOKENS"),
			Pass2MaxTokens: viper.GetInt("CLAUDE_PASS2_MAX_TOKENS"),
			Pass3MaxTokens: viper.GetInt("CLAUDE_PASS3_MAX_TOKENS"),
			Pass4MaxTokens:   viper.GetInt("CLAUDE_PASS4_MAX_TOKENS"),
			RequestTimeout:   llmTimeout,
			DisableRateLimit: viper.GetBool("DISABLE_RATE_LIMIT"),
		},
		Neo4j: Neo4jConfig{
			URI:      viper.GetString("NEO4J_URI"),
			User:     viper.GetString("NEO4J_USER"),
			Password: viper.GetString("NEO4J_PASSWORD"),
			Database: viper.GetString("NEO4J_DATABASE"),
		},
		Ingest: IngestConfig{
			RootDir:         viper.GetString("INGEST_ROOT_DIR"),
			Codebase:        viper.GetString("INGEST_CODEBASE"),
			BatchSize:       viper.GetInt("INGEST_BATCH_SIZE"),
			CacheDB:         viper.GetString("INGEST_CACHE_DB"),
			TokenLimit:      viper.GetInt("INGEST_TOKEN_LIMIT"),
			MaxWorkers:      viper.GetInt("MAX_WORKERS"),
			Pass1MaxWorkers: viper.GetInt("PASS1_MAX_WORKERS"),
			Pass2MaxWorkers: viper.GetInt("PASS2_MAX_WORKERS"),
			Pass5MaxWorkers: viper.GetInt("PASS5_MAX_WORKERS"),
			Pass2TokenLimit: viper.GetInt("PASS2_TOKEN_LIMIT"),
			OverlapLines:    viper.GetInt("PASS2_OVERLAP_LINES"),
			Pass3BatchSize:       viper.GetInt("PASS3_BATCH_SIZE"),
			StripSequenceColumns: viper.GetBool("STRIP_SEQUENCE_COLUMNS"),
		},
		API: APIConfig{
			Port:     viper.GetString("API_PORT"),
			LogLevel: viper.GetString("API_LOG_LEVEL"),
		},
		MCP: MCPConfig{
			HTTPPort: viper.GetString("MCP_HTTP_PORT"),
		},
		ExternalDB: ExternalDBConfig{
			DBMCPCommand:    viper.GetString("EXTDB_MCP_CMD"),
			DBMCPServerURL:  viper.GetString("EXTDB_MCP_URL"),
			GraphMCPBin:     viper.GetString("EXTDB_GRAPH_MCP_BIN"),
			GraphMCPURL:     viper.GetString("EXTDB_GRAPH_MCP_URL"),
			MaxIterations:   viper.GetInt("EXTDB_MAX_ITERATIONS"),
			MaxTokens:       viper.GetInt("EXTDB_MAX_TOKENS"),
			DatabaseName:    viper.GetString("EXTDB_DATABASE_NAME"),
			DatabaseType:    viper.GetString("EXTDB_DATABASE_TYPE"),
			OracleHost:      viper.GetString("ORACLE_HOST"),
			OraclePort:      viper.GetString("ORACLE_PORT"),
			OracleService:   viper.GetString("ORACLE_SERVICE"),
			OracleUser:      viper.GetString("ORACLE_USER"),
			OraclePassword:  viper.GetString("ORACLE_PASSWORD"),
			OracleWalletPath: viper.GetString("ORACLE_WALLET_PATH"),
			OracleTNSAdmin:  viper.GetString("ORACLE_TNS_ADMIN"),
			OracleSQLclPath: viper.GetString("ORACLE_SQLCL_PATH"),
		},
		BW: BWConfig{
			Dir:        viper.GetString("BW_DIR"),
			Extensions: viper.GetString("BW_EXTENSIONS"),
			MaxWorkers: viper.GetInt("BW_MAX_WORKERS"),
			MaxTokens:  viper.GetInt("BW_MAX_TOKENS"),
			TokenLimit: viper.GetInt("BW_TOKEN_LIMIT"),
			JavapPath:  viper.GetString("BW_JAVAP_PATH"),
		},
		Modernize: ModernizeConfig{
			Port:          viper.GetString("MODERNIZE_PORT"),
			MCPTransport:  viper.GetString("MCP_TRANSPORT"),
			MCPServerBin:  viper.GetString("MCP_SERVER_BIN"),
			MCPServerURL:  viper.GetString("MCP_SERVER_URL"),
			ChatModel:     viper.GetString("MODERNIZE_CHAT_MODEL"),
			ChatMaxTokens: viper.GetInt("MODERNIZE_CHAT_MAX_TOKENS"),
		},
	}

	return cfg, nil
}

// Validate checks that required configuration fields are set.
func (c *Config) Validate() error {
	if c.LLM.Provider == "anthropic" && c.LLM.APIKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required when LLM_PROVIDER=anthropic")
	}
	// Copilot token is resolved at runtime (env var → cached file → device flow),
	// so we don't require it at config validation time.
	if c.LLM.Provider == "vertex" && c.LLM.VertexProjectID == "" {
		return fmt.Errorf("VERTEX_PROJECT_ID is required when LLM_PROVIDER=vertex")
	}
	// Bedrock uses the AWS credential chain, so no explicit key is required at
	// config validation time.
	if c.LLM.Provider == "openai" && c.LLM.OpenAIAPIKey == "" {
		return fmt.Errorf("OPENAI_API_KEY is required when LLM_PROVIDER=openai")
	}
	if c.Neo4j.URI == "" {
		return fmt.Errorf("NEO4J_URI is required")
	}
	return nil
}
