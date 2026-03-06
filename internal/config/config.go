package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	LLM       LLMConfig
	Claude    ClaudeConfig
	Neo4j     Neo4jConfig
	Ingest    IngestConfig
	API       APIConfig
	MCP       MCPConfig
	Modernize ModernizeConfig
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
	Provider              string        // "anthropic" or "copilot"
	APIKey                string        // ANTHROPIC_API_KEY or GitHub token depending on provider
	CopilotGitHubToken    string        // GitHub personal access token for Copilot auth
	CopilotAccountType    string        // "individual", "business", or "enterprise"
	Timeout               time.Duration // Overall HTTP client timeout (LLM_TIMEOUT)
	ResponseHeaderTimeout time.Duration // Time to wait for first response byte (LLM_RESPONSE_HEADER_TIMEOUT)
}

// ClaudeConfig holds model names and token settings (used by both providers).
type ClaudeConfig struct {
	OpusModel      string
	SonnetModel    string
	MaxRetries     int
	Pass1MaxTokens int
	Pass2MaxTokens int
	Pass3MaxTokens int
	Pass4MaxTokens int
	RequestTimeout time.Duration
}

type Neo4jConfig struct {
	URI      string
	User     string
	Password string
	Database string
}

type IngestConfig struct {
	RootDir         string
	BatchSize       int
	CacheDB         string
	TokenLimit      int
	MaxWorkers      int
	Pass2TokenLimit int
	OverlapLines    int
	Pass3BatchSize  int
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

	// Claude model defaults (used by both providers)
	viper.SetDefault("CLAUDE_OPUS_MODEL", "claude-opus-4-6")
	viper.SetDefault("CLAUDE_SONNET_MODEL", "claude-sonnet-4-5-20250929")
	viper.SetDefault("CLAUDE_MAX_RETRIES", 3)
	viper.SetDefault("CLAUDE_PASS1_MAX_TOKENS", 8192)
	viper.SetDefault("CLAUDE_PASS2_MAX_TOKENS", 16000)
	viper.SetDefault("CLAUDE_PASS3_MAX_TOKENS", 16000)
	viper.SetDefault("CLAUDE_PASS4_MAX_TOKENS", 4000)

	// Neo4j defaults
	viper.SetDefault("NEO4J_URI", "bolt://localhost:7687")
	viper.SetDefault("NEO4J_USER", "neo4j")
	viper.SetDefault("NEO4J_PASSWORD", "changeme")
	viper.SetDefault("NEO4J_DATABASE", "cobol")

	// Ingest defaults
	viper.SetDefault("INGEST_BATCH_SIZE", 500)
	viper.SetDefault("INGEST_CACHE_DB", "./cache.sqlite")
	viper.SetDefault("INGEST_TOKEN_LIMIT", 30000)
	viper.SetDefault("MAX_WORKERS", 15)
	viper.SetDefault("PASS2_TOKEN_LIMIT", 20000)
	viper.SetDefault("PASS2_OVERLAP_LINES", 20)
	viper.SetDefault("PASS3_BATCH_SIZE", 50)

	// API defaults
	viper.SetDefault("API_PORT", "8080")
	viper.SetDefault("API_LOG_LEVEL", "info")

	// MCP defaults
	viper.SetDefault("MCP_HTTP_PORT", "")

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
		LLM: LLMConfig{
			Provider:              viper.GetString("LLM_PROVIDER"),
			APIKey:                viper.GetString("ANTHROPIC_API_KEY"),
			CopilotGitHubToken:    viper.GetString("COPILOT_GITHUB_TOKEN"),
			CopilotAccountType:    viper.GetString("COPILOT_ACCOUNT_TYPE"),
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
			Pass4MaxTokens: viper.GetInt("CLAUDE_PASS4_MAX_TOKENS"),
			RequestTimeout: llmTimeout,
		},
		Neo4j: Neo4jConfig{
			URI:      viper.GetString("NEO4J_URI"),
			User:     viper.GetString("NEO4J_USER"),
			Password: viper.GetString("NEO4J_PASSWORD"),
			Database: viper.GetString("NEO4J_DATABASE"),
		},
		Ingest: IngestConfig{
			RootDir:         viper.GetString("INGEST_ROOT_DIR"),
			BatchSize:       viper.GetInt("INGEST_BATCH_SIZE"),
			CacheDB:         viper.GetString("INGEST_CACHE_DB"),
			TokenLimit:      viper.GetInt("INGEST_TOKEN_LIMIT"),
			MaxWorkers:      viper.GetInt("MAX_WORKERS"),
			Pass2TokenLimit: viper.GetInt("PASS2_TOKEN_LIMIT"),
			OverlapLines:    viper.GetInt("PASS2_OVERLAP_LINES"),
			Pass3BatchSize:  viper.GetInt("PASS3_BATCH_SIZE"),
		},
		API: APIConfig{
			Port:     viper.GetString("API_PORT"),
			LogLevel: viper.GetString("API_LOG_LEVEL"),
		},
		MCP: MCPConfig{
			HTTPPort: viper.GetString("MCP_HTTP_PORT"),
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
	if c.Neo4j.URI == "" {
		return fmt.Errorf("NEO4J_URI is required")
	}
	return nil
}
