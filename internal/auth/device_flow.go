package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/copilot"
)

// RunDeviceFlow runs the GitHub OAuth device flow and returns the access token.
func RunDeviceFlow(ctx context.Context) (string, error) {
	// Step 1: Request device code
	reqBody, _ := json.Marshal(map[string]string{
		"client_id": copilot.GitHubClientID,
		"scope":     copilot.GitHubOAuthScopes,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", copilot.GitHubDeviceCodeURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("creating device code request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("requesting device code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("device code request HTTP %d: %s", resp.StatusCode, body)
	}

	var deviceCode copilot.GitHubDeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceCode); err != nil {
		return "", fmt.Errorf("decoding device code response: %w", err)
	}

	// Step 2: Display instructions
	fmt.Printf("\n=== GitHub Copilot Authentication ===\n")
	fmt.Printf("1. Visit: %s\n", deviceCode.VerificationURI)
	fmt.Printf("2. Enter code: %s\n", deviceCode.UserCode)
	fmt.Printf("Waiting for authorization...\n\n")

	// Step 3: Poll for access token
	interval := time.Duration(deviceCode.Interval+1) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	expiry := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if time.Now().After(expiry) {
				return "", fmt.Errorf("device code expired")
			}

			token, err := checkAccessToken(ctx, deviceCode.DeviceCode)
			if err == nil {
				return token, nil
			}
			// Continue polling on authorization_pending or other transient errors
		}
	}
}

func checkAccessToken(ctx context.Context, deviceCode string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"client_id":   copilot.GitHubClientID,
		"device_code": deviceCode,
		"grant_type":  "urn:ietf:params:oauth:grant-type:device_code",
	})

	req, err := http.NewRequestWithContext(ctx, "POST", copilot.GitHubAccessTokenURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp copilot.GitHubAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	if tokenResp.Error != "" {
		return "", fmt.Errorf(tokenResp.Error)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token in response")
	}

	return tokenResp.AccessToken, nil
}
