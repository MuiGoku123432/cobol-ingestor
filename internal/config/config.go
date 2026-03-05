package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Claude  ClaudeConfig
	Neo4j   Neo4jConfig
	Ingest  IngestConfig
	API     APIConfig
}

type ClaudeConfig struct {
	APIKey     string
	OpusModel  string
	SonnetModel string
	MaxWorkers int
	MaxRetries int
}

type Neo4jConfig struct {
	URI      string
	User     string
	Password string
	Database string
}

type IngestConfig struct {
	RootDir    string
	BatchSize  int
	CacheDB    string
	TokenLimit int
}

type APIConfig struct {
	Port     string
	LogLevel string
}

func Load() (*Config, error) {
	// Load .env file if present (ignore error if missing)
	_ = godotenv.Load()

	viper.AutomaticEnv()

	// Claude defaults
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

	// API defaults
	viper.SetDefault("API_PORT", "8080")
	viper.SetDefault("API_LOG_LEVEL", "info")

	cfg := &Config{
		Claude: ClaudeConfig{
			APIKey:      viper.GetString("ANTHROPIC_API_KEY"),
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
			RootDir:    viper.GetString("INGEST_ROOT_DIR"),
			BatchSize:  viper.GetInt("INGEST_BATCH_SIZE"),
			CacheDB:    viper.GetString("INGEST_CACHE_DB"),
			TokenLimit: viper.GetInt("INGEST_TOKEN_LIMIT"),
		},
		API: APIConfig{
			Port:     viper.GetString("API_PORT"),
			LogLevel: viper.GetString("API_LOG_LEVEL"),
		},
	}

	return cfg, nil
}
