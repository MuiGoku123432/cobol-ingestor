package claude

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"text/template"
	"time"

	"cobol-ingestor/internal/chunker"
	"cobol-ingestor/internal/config"
	"cobol-ingestor/internal/llm"
	"cobol-ingestor/prompts"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Client wraps an LLM provider with retry, rate limiting, and prompt templates.
type Client struct {
	provider       llm.Provider
	sonnetModel    string
	opusModel      string
	maxRetries     int
	pass1MaxTokens int
	pass2MaxTokens int
	pass3MaxTokens int
	limiter        *rate.Limiter
	logger         *zap.Logger
	pass1Tmpl      *template.Template
	pass2Tmpl      *template.Template
	pass3Tmpl      *template.Template
}

// NewClient creates a Claude API client using an LLM provider.
func NewClient(provider llm.Provider, cfg config.ClaudeConfig, logger *zap.Logger) (*Client, error) {
	p1Tmpl, err := template.New("pass1").Parse(prompts.Pass1Structural)
	if err != nil {
		return nil, fmt.Errorf("parsing pass1 template: %w", err)
	}

	p2Tmpl, err := template.New("pass2").Parse(prompts.Pass2Deep)
	if err != nil {
		return nil, fmt.Errorf("parsing pass2 template: %w", err)
	}

	p3Tmpl, err := template.New("pass3").Parse(prompts.Pass3CrossCutting)
	if err != nil {
		return nil, fmt.Errorf("parsing pass3 template: %w", err)
	}

	// Rate limit: ~50 requests per minute to stay within API limits
	limiter := rate.NewLimiter(rate.Every(time.Second), 2)

	return &Client{
		provider:       provider,
		sonnetModel:    cfg.SonnetModel,
		opusModel:      cfg.OpusModel,
		maxRetries:     cfg.MaxRetries,
		pass1MaxTokens: cfg.Pass1MaxTokens,
		pass2MaxTokens: cfg.Pass2MaxTokens,
		pass3MaxTokens: cfg.Pass3MaxTokens,
		limiter:        limiter,
		logger:      logger,
		pass1Tmpl:   p1Tmpl,
		pass2Tmpl:   p2Tmpl,
		pass3Tmpl:   p3Tmpl,
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

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: c.pass1MaxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a COBOL code analysis assistant. You extract structural information from COBOL source files and return it as JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n---\n\n" + content},
		},
	})
}

// AnalyzeDeep sends a chunk to Claude Opus for Pass 2 deep semantic analysis.
// contextPreamble contains graph context from Pass 1 results.
func (c *Client) AnalyzeDeep(ctx context.Context, chunk chunker.Chunk, contextPreamble string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass2Tmpl.Execute(&userMsg, map[string]any{
		"FileName": chunk.FileName,
		"Index":    chunk.Index + 1,
		"Total":    chunk.Total,
	}); err != nil {
		return "", fmt.Errorf("rendering pass2 template: %w", err)
	}

	userContent := userMsg.String()
	if contextPreamble != "" {
		userContent = contextPreamble + "\n\n" + userContent
	}
	userContent += "\n\n---\n\n" + chunk.Content

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass2MaxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst performing deep semantic analysis. You extract detailed relationships, data flows, and control flows from COBOL source code and return structured JSON."},
			{Role: llm.RoleUser, Content: userContent},
		},
	})
}

// AnalyzeCrossCutting sends a graph data slice to Claude Opus for Pass 3 cross-cutting analysis.
func (c *Client) AnalyzeCrossCutting(ctx context.Context, graphSlice string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass3Tmpl.Execute(&userMsg, nil); err != nil {
		return "", fmt.Errorf("rendering pass3 template: %w", err)
	}

	userContent := userMsg.String() + "\n\n" + graphSlice

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass3MaxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL systems analyst. You analyze program relationships to identify business domains, dead code, and risk factors. Return structured JSON."},
			{Role: llm.RoleUser, Content: userContent},
		},
	})
}

// completeWithRetry calls the LLM provider with exponential backoff retries.
func (c *Client) completeWithRetry(ctx context.Context, req llm.CompletionRequest) (string, error) {
	var lastErr error
	for attempt := range c.maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limiter: %w", err)
		}

		resp, err := c.provider.Complete(ctx, req)
		if err != nil {
			lastErr = err
			c.logger.Warn("LLM API call failed, retrying",
				zap.String("provider", c.provider.Name()),
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

		if resp.Content == "" {
			return "", fmt.Errorf("no text content in LLM response")
		}

		return resp.Content, nil
	}

	return "", fmt.Errorf("LLM API failed after %d retries: %w", c.maxRetries, lastErr)
}
