package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// StoredToken represents a persisted GitHub OAuth token.
type StoredToken struct {
	GitHubToken string    `json:"github_token"`
	ObtainedAt  time.Time `json:"obtained_at"`
}

// configDirFn is a variable so tests can override the config directory.
var configDirFn = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cobol-graph")
}

// TokenFilePath returns the path to the cached token file.
func TokenFilePath() string {
	return filepath.Join(configDirFn(), "copilot-token.json")
}

// SaveToken persists a GitHub OAuth token to disk.
func SaveToken(token string) error {
	dir := configDirFn()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	st := StoredToken{
		GitHubToken: token,
		ObtainedAt:  time.Now(),
	}

	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(TokenFilePath(), data, 0600)
}

// LoadToken reads a cached token from disk. Returns (nil, nil) if the file does not exist.
func LoadToken() (*StoredToken, error) {
	data, err := os.ReadFile(TokenFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var st StoredToken
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

// DeleteToken removes the cached token file. Returns nil if the file does not exist.
func DeleteToken() error {
	err := os.Remove(TokenFilePath())
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}
