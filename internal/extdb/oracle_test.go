package extdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindSQLcl_ExplicitPath(t *testing.T) {
	// Create a temp file to simulate the SQLcl binary
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "sql")
	require.NoError(t, os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755))

	cfg := OracleConfig{SQLclPath: fakeBin}
	path, err := FindSQLcl(cfg)
	require.NoError(t, err)
	assert.Equal(t, fakeBin, path)
}

func TestFindSQLcl_NotFound(t *testing.T) {
	cfg := OracleConfig{SQLclPath: "/nonexistent/path/to/sql"}
	_, err := FindSQLcl(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestBuildSQLclMCPCommand_UserPass(t *testing.T) {
	cfg := OracleConfig{
		Host:     "dbhost",
		Port:     "1521",
		Service:  "ORCL",
		User:     "scott",
		Password: "tiger",
	}
	cmd, err := BuildSQLclMCPCommand("/usr/local/bin/sql", cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"/usr/local/bin/sql", "-mcpserver", "scott/tiger@dbhost:1521/ORCL"}, cmd.Args)
}

func TestBuildSQLclMCPCommand_Wallet(t *testing.T) {
	cfg := OracleConfig{
		Service:    "ORCL",
		WalletPath: "/opt/oracle/wallet",
		TNSAdmin:   "/opt/oracle/tns",
	}
	cmd, err := BuildSQLclMCPCommand("/usr/local/bin/sql", cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"/usr/local/bin/sql", "-mcpserver", "/@ORCL"}, cmd.Args)

	// Check TNS_ADMIN is set in env
	found := false
	for _, e := range cmd.Env {
		if e == "TNS_ADMIN=/opt/oracle/tns" {
			found = true
			break
		}
	}
	assert.True(t, found, "TNS_ADMIN should be set in command env")
}

func TestBuildSQLclMCPCommand_WalletDefaultTNSAdmin(t *testing.T) {
	cfg := OracleConfig{
		Service:    "ORCL",
		WalletPath: "/opt/oracle/wallet",
	}
	cmd, err := BuildSQLclMCPCommand("/usr/local/bin/sql", cfg)
	require.NoError(t, err)

	// TNS_ADMIN should default to WalletPath
	found := false
	for _, e := range cmd.Env {
		if e == "TNS_ADMIN=/opt/oracle/wallet" {
			found = true
			break
		}
	}
	assert.True(t, found, "TNS_ADMIN should default to WalletPath")
}

func TestBuildSQLclMCPCommand_Nolog(t *testing.T) {
	cfg := OracleConfig{
		Service: "ORCL",
	}
	cmd, err := BuildSQLclMCPCommand("/usr/local/bin/sql", cfg)
	require.NoError(t, err)

	assert.Equal(t, []string{"/usr/local/bin/sql", "-mcpserver", "/nolog"}, cmd.Args)
}
