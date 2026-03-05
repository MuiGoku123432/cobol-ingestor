package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	LLM     LLMConfig
	Claude  ClaudeConfig
	Neo4j   Neo4jConfig
	Ingest  IngestConfig
	API     APIConfig
}

// LLMConfig selects which provider backend to use.
type LLMConfig struct {
	Provider           string // "anthropic" or "copilot"
	APIKey             string // ANTHROPIC_API_KEY or GitHub token depending on provider
	CopilotGitHubToken string // GitHub personal access token for Copilot auth
	CopilotAccountType string // "individual", "business", or "enterprise"
}

// ClaudeConfig holds model names and concurrency settings (used by both providers).
type ClaudeConfig struct {
	OpusModel   string
	SonnetModel string
	MaxWorkers  int
	MaxRetries  int
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
	Pass2Workers    int
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

	// Claude model defaults (used by both providers)
	viper.SetDefault("CLAUDE_OPUS_MODEL", "claude-opus-4-6")
	viper.SetDefault("CLAUDE_SONNET_MODEL", "claude-sonnet-4-5-20250929")
	viper.SetDefault("CLAUDE_MAX_WORKERS", 5)
	viper.SetDefault("CLAUDE_MAX_RETRIES", 3)

	// Neo4j defaults
	viper.SetDefault("NEO4J_URI", "bolt://localhost:7687")
	viper.SetDefault("NEO4J_USER", "neo4j")
	viper.SetDefault("NEO4J_PASSWORD", "changeme")
	viper.SetDefault("NEO4J_DATABASE", "cobol")

	// Ingest defaults
	viper.SetDefault("INGEST_BATCH_SIZE", 500)
	viper.SetDefault("INGEST_CACHE_DB", "./cache.sqlite")
	viper.SetDefault("INGEST_TOKEN_LIMIT", 150000)
	viper.SetDefault("PASS2_MAX_WORKERS", 3)
	viper.SetDefault("PASS2_TOKEN_LIMIT", 100000)
	viper.SetDefault("PASS2_OVERLAP_LINES", 20)
	viper.SetDefault("PASS3_BATCH_SIZE", 50)

	// API defaults
	viper.SetDefault("API_PORT", "8080")
	viper.SetDefault("API_LOG_LEVEL", "info")

	cfg := &Config{
		LLM: LLMConfig{
			Provider:           viper.GetString("LLM_PROVIDER"),
			APIKey:             viper.GetString("ANTHROPIC_API_KEY"),
			CopilotGitHubToken: viper.GetString("COPILOT_GITHUB_TOKEN"),
			CopilotAccountType: viper.GetString("COPILOT_ACCOUNT_TYPE"),
		},
		Claude: ClaudeConfig{
			OpusModel:   viper.GetString("CLAUDE_OPUS_MODEL"),
			SonnetModel: viper.GetString("CLAUDE_SONNET_MODEL"),
			MaxWorkers:  viper.GetInt("CLAUDE_MAX_WORKERS"),
			MaxRetries:  viper.GetInt("CLAUDE_MAX_RETRIES"),
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
			Pass2Workers:    viper.GetInt("PASS2_MAX_WORKERS"),
			Pass2TokenLimit: viper.GetInt("PASS2_TOKEN_LIMIT"),
			OverlapLines:    viper.GetInt("PASS2_OVERLAP_LINES"),
			Pass3BatchSize:  viper.GetInt("PASS3_BATCH_SIZE"),
		},
		API: APIConfig{
			Port:     viper.GetString("API_PORT"),
			LogLevel: viper.GetString("API_LOG_LEVEL"),
		},
	}

	return cfg, nil
}

// Validate checks that required configuration fields are set.
func (c *Config) Validate() error {
	if c.LLM.Provider == "anthropic" && c.LLM.APIKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required when LLM_PROVIDER=anthropic")
	}
	if c.LLM.Provider == "copilot" && c.LLM.CopilotGitHubToken == "" {
		return fmt.Errorf("COPILOT_GITHUB_TOKEN is required when LLM_PROVIDER=copilot")
	}
	if c.Neo4j.URI == "" {
		return fmt.Errorf("NEO4J_URI is required")
	}
	return nil
}
