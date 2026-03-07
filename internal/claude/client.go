package claude

import (
	"bytes"
	"context"
	"errors"
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

// ErrResponseTruncated is returned when the LLM response was truncated
// due to max_tokens limits even after retry with increased limits.
var ErrResponseTruncated = errors.New("LLM response truncated by max_tokens limit")

// maxTokensCap is the upper limit for max_tokens when auto-retrying truncated responses.
const maxTokensCap = 32000

// Client wraps an LLM provider with retry, rate limiting, and prompt templates.
type Client struct {
	provider       llm.Provider
	sonnetModel    string
	opusModel      string
	maxRetries     int
	pass1MaxTokens int
	pass2MaxTokens int
	pass3MaxTokens int
	pass4MaxTokens int
	requestTimeout time.Duration
	limiter        *rate.Limiter
	logger         *zap.Logger
	pass1Tmpl      *template.Template
	pass2Tmpl      *template.Template
	pass3Tmpl      *template.Template
	jclTmpl        *template.Template
	pass4Tmpl      *template.Template
	pass5Tmpl      *template.Template
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

	jclTmpl, err := template.New("jcl").Parse(prompts.Pass1JCL)
	if err != nil {
		return nil, fmt.Errorf("parsing jcl template: %w", err)
	}

	p4Tmpl, err := template.New("pass4").Parse(prompts.Pass4CrossProgram)
	if err != nil {
		return nil, fmt.Errorf("parsing pass4 template: %w", err)
	}

	p5Tmpl, err := template.New("pass5").Parse(prompts.Pass5Repair)
	if err != nil {
		return nil, fmt.Errorf("parsing pass5 template: %w", err)
	}

	// Rate limit: ~120 requests per minute to stay within API limits.
	// DISABLE_RATE_LIMIT=true removes the limit entirely.
	var limiter *rate.Limiter
	if cfg.DisableRateLimit {
		limiter = rate.NewLimiter(rate.Inf, 0)
	} else {
		limiter = rate.NewLimiter(rate.Every(time.Second), 2)
	}

	requestTimeout := cfg.RequestTimeout
	if requestTimeout == 0 {
		requestTimeout = 10 * time.Minute
	}

	return &Client{
		provider:       provider,
		sonnetModel:    cfg.SonnetModel,
		opusModel:      cfg.OpusModel,
		maxRetries:     cfg.MaxRetries,
		pass1MaxTokens: cfg.Pass1MaxTokens,
		pass2MaxTokens: cfg.Pass2MaxTokens,
		pass3MaxTokens: cfg.Pass3MaxTokens,
		pass4MaxTokens: cfg.Pass4MaxTokens,
		requestTimeout: requestTimeout,
		limiter:        limiter,
		logger:      logger,
		pass1Tmpl:   p1Tmpl,
		pass2Tmpl:   p2Tmpl,
		pass3Tmpl:   p3Tmpl,
		jclTmpl:     jclTmpl,
		pass4Tmpl:   p4Tmpl,
		pass5Tmpl:   p5Tmpl,
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
// existingDomains is a formatted list of already-assigned domain names to prevent duplication.
func (c *Client) AnalyzeCrossCutting(ctx context.Context, graphSlice, existingDomains string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass3Tmpl.Execute(&userMsg, map[string]string{
		"ExistingDomains": existingDomains,
	}); err != nil {
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

// AnalyzeJCL sends a JCL file to Claude Sonnet for structural extraction.
func (c *Client) AnalyzeJCL(ctx context.Context, fileName, content string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.jclTmpl.Execute(&userMsg, map[string]string{
		"FileName": fileName,
	}); err != nil {
		return "", fmt.Errorf("rendering jcl template: %w", err)
	}

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: c.pass1MaxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a mainframe JCL analysis assistant. You extract structural information from JCL files and return it as JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n---\n\n" + content},
		},
	})
}

// AnalyzeCrossProgramFlow sends caller/callee field context to Claude Sonnet for LINKAGE mapping.
func (c *Client) AnalyzeCrossProgramFlow(ctx context.Context, caller, callee, fieldContext string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass4Tmpl.Execute(&userMsg, map[string]string{
		"Caller": caller,
		"Callee": callee,
	}); err != nil {
		return "", fmt.Errorf("rendering pass4 template: %w", err)
	}

	maxTokens := c.pass4MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4000
	}

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: maxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst mapping data fields between programs through LINKAGE SECTION parameters. Return structured JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n" + fieldContext},
		},
	})
}

// AnalyzeRepair sends a targeted repair prompt to Opus for Pass 5 gap repair.
func (c *Client) AnalyzeRepair(ctx context.Context, repairType, programID, graphContext, sourceCode, missingParagraphs string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass5Tmpl.Execute(&userMsg, map[string]string{
		"ProgramID":         programID,
		"RepairType":        repairType,
		"GraphContext":      graphContext,
		"SourceCode":        sourceCode,
		"MissingParagraphs": missingParagraphs,
	}); err != nil {
		return "", fmt.Errorf("rendering pass5 template: %w", err)
	}

	return c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass2MaxTokens,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst repairing gaps in extracted program metadata. Return only valid JSON matching the requested schema."},
			{Role: llm.RoleUser, Content: userMsg.String()},
		},
	})
}

// completeWithRetry calls the LLM provider with exponential backoff retries.
// On truncation (StopReason == "max_tokens"), it auto-retries once with doubled max_tokens.
func (c *Client) completeWithRetry(ctx context.Context, req llm.CompletionRequest) (string, error) {
	var lastErr error
	for attempt := range c.maxRetries {
		if err := c.limiter.Wait(ctx); err != nil {
			return "", fmt.Errorf("rate limiter: %w", err)
		}

		reqCtx, reqCancel := context.WithTimeout(ctx, c.requestTimeout)
		resp, err := c.provider.Complete(reqCtx, req)
		reqCancel()
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

		// Handle truncated responses
		if resp.Truncated {
			doubled := req.MaxTokens * 2
			if doubled > maxTokensCap {
				doubled = maxTokensCap
			}
			if doubled <= req.MaxTokens {
				// Already at or above cap, can't increase further
				c.logger.Error("LLM response truncated at max_tokens cap",
					zap.String("model", req.Model),
					zap.Int("max_tokens", req.MaxTokens),
					zap.Int("output_tokens", resp.OutputTokens),
				)
				return "", fmt.Errorf("%w: model=%s max_tokens=%d output_tokens=%d",
					ErrResponseTruncated, req.Model, req.MaxTokens, resp.OutputTokens)
			}

			c.logger.Warn("LLM response truncated, retrying with increased max_tokens",
				zap.String("model", req.Model),
				zap.Int("original_max_tokens", req.MaxTokens),
				zap.Int("new_max_tokens", doubled),
				zap.Int("output_tokens", resp.OutputTokens),
			)

			// Retry with doubled max_tokens
			retryReq := req
			retryReq.MaxTokens = doubled

			if err := c.limiter.Wait(ctx); err != nil {
				return "", fmt.Errorf("rate limiter: %w", err)
			}
			retryCtx, retryCancel := context.WithTimeout(ctx, c.requestTimeout)
		retryResp, retryErr := c.provider.Complete(retryCtx, retryReq)
		retryCancel()
			if retryErr != nil {
				return "", fmt.Errorf("truncation retry failed: %w", retryErr)
			}
			if retryResp.Content == "" {
				return "", fmt.Errorf("no text content in truncation retry response")
			}
			if retryResp.Truncated {
				c.logger.Error("LLM response still truncated after retry",
					zap.String("model", retryReq.Model),
					zap.Int("max_tokens", retryReq.MaxTokens),
					zap.Int("output_tokens", retryResp.OutputTokens),
				)
				return "", fmt.Errorf("%w: model=%s max_tokens=%d output_tokens=%d",
					ErrResponseTruncated, retryReq.Model, retryReq.MaxTokens, retryResp.OutputTokens)
			}
			return retryResp.Content, nil
		}

		return resp.Content, nil
	}

	return "", fmt.Errorf("LLM API failed after %d retries: %w", c.maxRetries, lastErr)
}
