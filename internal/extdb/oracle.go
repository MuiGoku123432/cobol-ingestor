package extdb

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"cobol-ingestor/internal/modernize"

	"go.uber.org/zap"
)

// OracleConfig holds connection details for Oracle SQLcl MCP auto-launch.
type OracleConfig struct {
	SQLclPath  string // Explicit path; auto-detected if empty
	Host       string // Default "localhost"
	Port       string // Default "1521"
	Service    string // Required — Oracle service name or SID
	User       string // Empty = wallet or /nolog
	Password   string
	WalletPath string // Oracle wallet directory
	TNSAdmin   string // TNS_ADMIN; defaults to WalletPath if set
}

// FindSQLcl locates the SQLcl binary. If cfg.SQLclPath is set, it verifies
// that path exists. Otherwise it searches PATH and common install locations.
func FindSQLcl(cfg OracleConfig) (string, error) {
	if cfg.SQLclPath != "" {
		if _, err := os.Stat(cfg.SQLclPath); err != nil {
			return "", fmt.Errorf("specified SQLcl path not found: %w", err)
		}
		return cfg.SQLclPath, nil
	}

	// Check PATH
	if p, err := exec.LookPath("sql"); err == nil {
		return p, nil
	}

	// Check common locations
	commonPaths := []string{
		"/usr/local/bin/sql",
		os.ExpandEnv("$HOME/sqlcl/bin/sql"),
		"/opt/oracle/sqlcl/bin/sql",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("SQLcl not found: install Oracle SQLcl 24.x+ or set --oracle-sqlcl-path")
}

// BuildSQLclMCPCommand constructs an exec.Cmd that launches SQLcl in MCP server mode.
// Three auth modes: wallet, user/password, or /nolog.
func BuildSQLclMCPCommand(sqlclPath string, cfg OracleConfig) (*exec.Cmd, error) {
	var connectString string

	switch {
	case cfg.WalletPath != "":
		// Wallet auth: /@SERVICE
		connectString = "/@" + cfg.Service
	case cfg.User != "" && cfg.Password != "":
		// User/password: user/pass@host:port/service
		connectString = fmt.Sprintf("%s/%s@%s:%s/%s",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Service)
	case cfg.User != "":
		// User without password (will prompt or use external auth)
		connectString = fmt.Sprintf("%s@%s:%s/%s",
			cfg.User, cfg.Host, cfg.Port, cfg.Service)
	default:
		// No credentials
		connectString = "/nolog"
	}

	cmd := exec.Command(sqlclPath, "-mcpserver", connectString)

	// Inherit current environment
	cmd.Env = os.Environ()

	// Set TNS_ADMIN if wallet is configured
	if cfg.WalletPath != "" {
		tnsAdmin := cfg.TNSAdmin
		if tnsAdmin == "" {
			tnsAdmin = cfg.WalletPath
		}
		cmd.Env = append(cmd.Env, "TNS_ADMIN="+tnsAdmin)
	}

	return cmd, nil
}

// NewOracleMCPClient finds SQLcl, builds the command, and connects via MCP.
func NewOracleMCPClient(ctx context.Context, cfg OracleConfig, logger *zap.Logger) (*modernize.MCPClient, error) {
	sqlclPath, err := FindSQLcl(cfg)
	if err != nil {
		return nil, err
	}

	logger.Info("launching SQLcl MCP server",
		zap.String("sqlcl_path", sqlclPath),
		zap.String("service", cfg.Service),
	)

	cmd, err := BuildSQLclMCPCommand(sqlclPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("building SQLcl command: %w", err)
	}

	client, err := modernize.NewMCPClientCommand(ctx, cmd, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to SQLcl MCP: %w", err)
	}

	return client, nil
}
