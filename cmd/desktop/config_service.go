package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cobol-ingestor/internal/config"

	"github.com/joho/godotenv"
)

const appConfigDirName = "cobol-graph"

// ConfigService manages application configuration for the desktop UI.
type ConfigService struct {
	app *App
}

// ConfigDTO is a flat, UI-friendly representation of the configuration.
// API keys are masked when read; full values are accepted on save.
type ConfigDTO struct {
	// LLM
	LLMProvider        string `json:"llmProvider"`
	AnthropicAPIKey    string `json:"anthropicApiKey"`
	CopilotToken       string `json:"copilotToken"`
	CopilotAccountType string `json:"copilotAccountType"`
	VertexProjectID    string `json:"vertexProjectId"`
	VertexRegion       string `json:"vertexRegion"`
	BedrockRegion      string `json:"bedrockRegion"`
	OpenAIAPIKey       string `json:"openaiApiKey"`
	OpenAIBaseURL      string `json:"openaiBaseUrl"`
	OpenAIModel        string `json:"openaiModel"`

	// Neo4j
	Neo4jURI      string `json:"neo4jUri"`
	Neo4jUser     string `json:"neo4jUser"`
	Neo4jPassword string `json:"neo4jPassword"`
	Neo4jDatabase string `json:"neo4jDatabase"`

	// Ingest
	MaxWorkers      int  `json:"maxWorkers"`
	BatchSize       int  `json:"batchSize"`
	TokenLimit      int  `json:"tokenLimit"`
	StripSeqColumns bool `json:"stripSeqColumns"`

	// Chat
	ChatModel     string `json:"chatModel"`
	ChatMaxTokens int    `json:"chatMaxTokens"`

	// BusinessWare
	BWDir        string `json:"bwDir"`
	BWExtensions string `json:"bwExtensions"`
	BWMaxWorkers int    `json:"bwMaxWorkers"`
	BWTokenLimit int    `json:"bwTokenLimit"`

	// Oracle / External DB
	OracleHost       string `json:"oracleHost"`
	OraclePort       string `json:"oraclePort"`
	OracleService    string `json:"oracleService"`
	OracleUser       string `json:"oracleUser"`
	OraclePassword   string `json:"oraclePassword"`
	OracleWalletPath string `json:"oracleWalletPath"`
	OracleSQLclPath  string `json:"oracleSqlclPath"`
	ExtDBName        string `json:"extDbName"`
	ExtDBType        string `json:"extDbType"`
	ExtDBMaxIter     int    `json:"extDbMaxIterations"`
}

// GetConfig returns the current configuration with secrets masked.
func (s *ConfigService) GetConfig() ConfigDTO {
	cfg := s.app.cfg
	return ConfigDTO{
		LLMProvider:        cfg.LLM.Provider,
		AnthropicAPIKey:    maskSecret(cfg.LLM.APIKey),
		CopilotToken:       maskSecret(cfg.LLM.CopilotGitHubToken),
		CopilotAccountType: cfg.LLM.CopilotAccountType,
		VertexProjectID:    cfg.LLM.VertexProjectID,
		VertexRegion:       cfg.LLM.VertexRegion,
		BedrockRegion:      cfg.LLM.BedrockRegion,
		OpenAIAPIKey:       maskSecret(cfg.LLM.OpenAIAPIKey),
		OpenAIBaseURL:      cfg.LLM.OpenAIBaseURL,
		OpenAIModel:        cfg.LLM.OpenAIModel,
		Neo4jURI:           cfg.Neo4j.URI,
		Neo4jUser:          cfg.Neo4j.User,
		Neo4jPassword:      maskSecret(cfg.Neo4j.Password),
		Neo4jDatabase:      cfg.Neo4j.Database,
		MaxWorkers:         cfg.Ingest.MaxWorkers,
		BatchSize:          cfg.Ingest.BatchSize,
		TokenLimit:         cfg.Ingest.TokenLimit,
		StripSeqColumns:    cfg.Ingest.StripSequenceColumns,
		ChatModel:          cfg.Modernize.ChatModel,
		ChatMaxTokens:      cfg.Modernize.ChatMaxTokens,
		BWDir:              cfg.BW.Dir,
		BWExtensions:       cfg.BW.Extensions,
		BWMaxWorkers:       cfg.BW.MaxWorkers,
		BWTokenLimit:       cfg.BW.TokenLimit,
		OracleHost:         cfg.ExternalDB.OracleHost,
		OraclePort:         cfg.ExternalDB.OraclePort,
		OracleService:      cfg.ExternalDB.OracleService,
		OracleUser:         cfg.ExternalDB.OracleUser,
		OraclePassword:     maskSecret(cfg.ExternalDB.OraclePassword),
		OracleWalletPath:   cfg.ExternalDB.OracleWalletPath,
		OracleSQLclPath:    cfg.ExternalDB.OracleSQLclPath,
		ExtDBName:          cfg.ExternalDB.DatabaseName,
		ExtDBType:          cfg.ExternalDB.DatabaseType,
		ExtDBMaxIter:       cfg.ExternalDB.MaxIterations,
	}
}

// SaveConfig writes updated configuration to the .env file and reloads.
func (s *ConfigService) SaveConfig(dto ConfigDTO) error {
	envPath := s.envPath()

	// Read existing env vars
	existing, _ := godotenv.Read(envPath)
	if existing == nil {
		existing = make(map[string]string)
	}

	// Only update non-masked values
	setIfNotMasked(existing, "LLM_PROVIDER", dto.LLMProvider)
	setIfNotMasked(existing, "ANTHROPIC_API_KEY", dto.AnthropicAPIKey)
	setIfNotMasked(existing, "COPILOT_GITHUB_TOKEN", dto.CopilotToken)
	setIfNotMasked(existing, "COPILOT_ACCOUNT_TYPE", dto.CopilotAccountType)
	setIfNotMasked(existing, "VERTEX_PROJECT_ID", dto.VertexProjectID)
	setIfNotMasked(existing, "VERTEX_REGION", dto.VertexRegion)
	setIfNotMasked(existing, "BEDROCK_REGION", dto.BedrockRegion)
	setIfNotMasked(existing, "OPENAI_API_KEY", dto.OpenAIAPIKey)
	setIfNotMasked(existing, "OPENAI_BASE_URL", dto.OpenAIBaseURL)
	setIfNotMasked(existing, "OPENAI_MODEL", dto.OpenAIModel)
	setIfNotMasked(existing, "NEO4J_URI", dto.Neo4jURI)
	setIfNotMasked(existing, "NEO4J_USER", dto.Neo4jUser)
	setIfNotMasked(existing, "NEO4J_PASSWORD", dto.Neo4jPassword)
	setIfNotMasked(existing, "NEO4J_DATABASE", dto.Neo4jDatabase)
	setIfNotMasked(existing, "MAX_WORKERS", fmt.Sprintf("%d", dto.MaxWorkers))
	setIfNotMasked(existing, "INGEST_BATCH_SIZE", fmt.Sprintf("%d", dto.BatchSize))
	setIfNotMasked(existing, "INGEST_TOKEN_LIMIT", fmt.Sprintf("%d", dto.TokenLimit))
	setIfNotMasked(existing, "STRIP_SEQUENCE_COLUMNS", fmt.Sprintf("%t", dto.StripSeqColumns))
	setIfNotMasked(existing, "MODERNIZE_CHAT_MODEL", dto.ChatModel)
	setIfNotMasked(existing, "MODERNIZE_CHAT_MAX_TOKENS", fmt.Sprintf("%d", dto.ChatMaxTokens))

	// BusinessWare
	setIfNotMasked(existing, "BW_DIR", dto.BWDir)
	setIfNotMasked(existing, "BW_EXTENSIONS", dto.BWExtensions)
	if dto.BWMaxWorkers > 0 {
		existing["BW_MAX_WORKERS"] = fmt.Sprintf("%d", dto.BWMaxWorkers)
	}
	if dto.BWTokenLimit > 0 {
		existing["BW_TOKEN_LIMIT"] = fmt.Sprintf("%d", dto.BWTokenLimit)
	}

	// Oracle / External DB
	setIfNotMasked(existing, "ORACLE_HOST", dto.OracleHost)
	setIfNotMasked(existing, "ORACLE_PORT", dto.OraclePort)
	setIfNotMasked(existing, "ORACLE_SERVICE", dto.OracleService)
	setIfNotMasked(existing, "ORACLE_USER", dto.OracleUser)
	setIfNotMasked(existing, "ORACLE_PASSWORD", dto.OraclePassword)
	setIfNotMasked(existing, "ORACLE_WALLET_PATH", dto.OracleWalletPath)
	setIfNotMasked(existing, "ORACLE_SQLCL_PATH", dto.OracleSQLclPath)
	setIfNotMasked(existing, "EXTDB_DATABASE_NAME", dto.ExtDBName)
	setIfNotMasked(existing, "EXTDB_DATABASE_TYPE", dto.ExtDBType)
	if dto.ExtDBMaxIter > 0 {
		existing["EXTDB_MAX_ITERATIONS"] = fmt.Sprintf("%d", dto.ExtDBMaxIter)
	}

	if err := godotenv.Write(existing, envPath); err != nil {
		return fmt.Errorf("writing .env: %w", err)
	}
	// Set restrictive permissions
	_ = os.Chmod(envPath, 0600)

	// Reload config in memory
	cfg, err := reloadConfig(envPath)
	if err != nil {
		return fmt.Errorf("reloading config: %w", err)
	}
	s.app.cfg = cfg
	return nil
}

// ValidateConfig checks a config DTO for issues without saving.
func (s *ConfigService) ValidateConfig(dto ConfigDTO) []string {
	var issues []string
	if dto.LLMProvider == "" {
		issues = append(issues, "LLM provider is required")
	}
	if dto.LLMProvider == "anthropic" && (dto.AnthropicAPIKey == "" || isMasked(dto.AnthropicAPIKey)) {
		if s.app.cfg.LLM.APIKey == "" {
			issues = append(issues, "Anthropic API key is required")
		}
	}
	if dto.LLMProvider == "openai" && (dto.OpenAIAPIKey == "" || isMasked(dto.OpenAIAPIKey)) {
		if s.app.cfg.LLM.OpenAIAPIKey == "" {
			issues = append(issues, "OpenAI API key is required")
		}
	}
	if dto.Neo4jURI == "" {
		issues = append(issues, "Neo4j URI is required")
	}
	return issues
}

// GetEnvPath returns the path to the .env file.
func (s *ConfigService) GetEnvPath() string {
	return s.envPath()
}

func (s *ConfigService) envPath() string {
	// Use OS-native config directory:
	//   macOS:   ~/Library/Application Support/cobol-graph/.env
	//   Windows: %AppData%\cobol-graph\.env
	//   Linux:   $XDG_CONFIG_HOME/cobol-graph/.env (or ~/.config/cobol-graph/.env)
	if configDir, err := os.UserConfigDir(); err == nil {
		appDir := filepath.Join(configDir, appConfigDirName)
		_ = os.MkdirAll(appDir, 0700)
		return filepath.Join(appDir, ".env")
	}
	// Fallback: next to executable, then cwd
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), ".env")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ".env"
}

func reloadConfig(envPath string) (*config.Config, error) {
	// Set env vars from the file so viper picks them up
	_ = godotenv.Overload(envPath)
	return config.Load()
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func isMasked(s string) bool {
	return strings.Contains(s, "****")
}

func setIfNotMasked(m map[string]string, key, value string) {
	if value == "" || isMasked(value) {
		return
	}
	m[key] = value
}
