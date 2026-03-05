package claude

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"text/template"
	"time"

	"cobol-ingestor/internal/config"
	"cobol-ingestor/prompts"

	"github.com/anthropics/anthropic-sdk-go"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Client wraps the Anthropic Claude SDK with retry and rate limiting.
type Client struct {
	sdk        anthropic.Client
	model      string
	maxRetries int
	limiter    *rate.Limiter
	logger     *zap.Logger
	pass1Tmpl  *template.Template
}

// NewClient creates a Claude API client from config.
func NewClient(cfg config.ClaudeConfig, logger *zap.Logger) (*Client, error) {
	sdk := anthropic.NewClient()

	tmpl, err := template.New("pass1").Parse(prompts.Pass1Structural)
	if err != nil {
		return nil, fmt.Errorf("parsing pass1 template: %w", err)
	}

	// Rate limit: ~50 requests per minute to stay within API limits
	limiter := rate.NewLimiter(rate.Every(time.Second), 2)

	return &Client{
		sdk:        sdk,
		model:      cfg.SonnetModel,
		maxRetries: cfg.MaxRetries,
		limiter:    limiter,
		logger:     logger,
		pass1Tmpl:  tmpl,
	}, nil
}

// AnalyzeStructural sends a file to Claude Sonnet for Pass 1 structural extraction.
// Returns the raw JSON response string.
func (c *Client) AnalyzeStructural(ctx context.Context, fileName, content string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass1Tmpl.Execute(&userMsg, map[string]string{
		"FileName": fileName,
	}); err != nil {
		return "", fmt.Errorf("rendering pass1 template: %w", err)
	}

	var lastErr error
	for attempt := range c.maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limiter: %w", err)
		}

		resp, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
			Model:     anthropic.Model(c.model),
			MaxTokens: 4096,
			System: []anthropic.TextBlockParam{
				{Text: "You are a COBOL code analysis assistant. You extract structural information from COBOL source files and return it as JSON."},
			},
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewTextBlock(userMsg.String()+"\n\n---\n\n"+content),
				),
			},
		})
		if err != nil {
			lastErr = err
			c.logger.Warn("claude API call failed, retrying",
				zap.Int("attempt", attempt+1),
				zap.Error(err),
			)
			backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
			continue
		}

		// Extract text from response
		for _, block := range resp.Content {
			if block.Type == "text" {
				return block.Text, nil
			}
		}

		return "", fmt.Errorf("no text content in claude response")
	}

	return "", fmt.Errorf("claude API failed after %d retries: %w", c.maxRetries, lastErr)
}
