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

// Client wraps an LLM provider with retry, rate limiting, and prompt templates.
type Client struct {
	provider           llm.Provider
	sonnetModel        string
	opusModel          string
	maxRetries         int
	pass1MaxTokens     int
	pass2MaxTokens     int
	pass3MaxTokens     int
	pass4MaxTokens     int
	maxOutputTokensCap int // upper limit for auto-retry max_tokens doubling
	requestTimeout     time.Duration
	limiter            *rate.Limiter
	logger             *zap.Logger
	pass1Tmpl          *template.Template
	pass2Tmpl          *template.Template
	pass3Tmpl          *template.Template
	jclTmpl            *template.Template
	pass4Tmpl          *template.Template
	pass5Tmpl          *template.Template
	bwTmpl             *template.Template
	calibrator         *chunker.TokenCalibrator
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

	bwTmpl, err := template.New("bw").Parse(prompts.BWIngest)
	if err != nil {
		return nil, fmt.Errorf("parsing bw template: %w", err)
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

	maxOutputCap := cfg.MaxOutputTokensCap
	if maxOutputCap <= 0 {
		maxOutputCap = 65536
	}

	return &Client{
		provider:           provider,
		sonnetModel:        cfg.SonnetModel,
		opusModel:          cfg.OpusModel,
		maxRetries:         cfg.MaxRetries,
		pass1MaxTokens:     cfg.Pass1MaxTokens,
		pass2MaxTokens:     cfg.Pass2MaxTokens,
		pass3MaxTokens:     cfg.Pass3MaxTokens,
		pass4MaxTokens:     cfg.Pass4MaxTokens,
		maxOutputTokensCap: maxOutputCap,
		requestTimeout:     requestTimeout,
		limiter:            limiter,
		logger:             logger,
		pass1Tmpl:          p1Tmpl,
		pass2Tmpl:          p2Tmpl,
		pass3Tmpl:          p3Tmpl,
		jclTmpl:            jclTmpl,
		pass4Tmpl:          p4Tmpl,
		pass5Tmpl:          p5Tmpl,
		bwTmpl:             bwTmpl,
	}, nil
}

// SetCalibrator sets the token calibrator for feeding calibration data.
func (c *Client) SetCalibrator(cal *chunker.TokenCalibrator) {
	c.calibrator = cal
}

// supportsPrefill returns true if the provider supports assistant message prefill.
// Only the Anthropic API supports appending a partial assistant message to steer output.
func (c *Client) supportsPrefill() bool {
	return c.provider.Name() == "anthropic"
}

// withJSONPrefill appends an assistant prefill message containing "{" to force
// the model to begin its response with JSON. For providers that don't support
// assistant prefill (e.g. Copilot/OpenAI), this is a no-op.
// When prefill is active, the caller must prepend "{" to the response content.
func (c *Client) withJSONPrefill(msgs []llm.Message) []llm.Message {
	if !c.supportsPrefill() {
		return msgs
	}
	return append(msgs, llm.Message{Role: llm.RoleAssistant, Content: "{"})
}

// prependPrefill conditionally prepends "{" to the response when the provider
// supports assistant prefill (since the prefill seeded "{" as the start of output).
func (c *Client) prependPrefill(resp string) string {
	if !c.supportsPrefill() {
		return resp
	}
	return "{" + resp
}

// AnalyzeStructural sends a file to Claude Sonnet for Pass 1 structural extraction.
// Returns the raw JSON response string.
func (c *Client) AnalyzeStructural(ctx context.Context, fileName, content string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass1Tmpl.Execute(&userMsg, map[string]any{
		"FileName": fileName,
		"FewShot":  prompts.Pass1Example,
	}); err != nil {
		return "", fmt.Errorf("rendering pass1 template: %w", err)
	}

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: c.pass1MaxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are a COBOL code analysis assistant. You extract structural information from COBOL source files and return it as JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n---\n\n" + content},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
}

// AnalyzeDeep sends a chunk to Claude Opus for Pass 2 deep semantic analysis.
// contextPreamble contains graph context from Pass 1 results.
func (c *Client) AnalyzeDeep(ctx context.Context, chunk chunker.Chunk, contextPreamble string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.pass2Tmpl.Execute(&userMsg, map[string]any{
		"FileName": chunk.FileName,
		"Index":    chunk.Index + 1,
		"Total":    chunk.Total,
		"FewShot":  prompts.Pass2Example,
	}); err != nil {
		return "", fmt.Errorf("rendering pass2 template: %w", err)
	}

	userContent := userMsg.String()
	if contextPreamble != "" {
		userContent = contextPreamble + "\n\n" + userContent
	}
	if chunk.LastParagraph != "" && chunk.Index > 0 {
		userContent += fmt.Sprintf("\n\nCHUNK BOUNDARY NOTE: The previous chunk ended at paragraph '%s'. If you see this paragraph in the overlap region at the start of this chunk, do NOT extract its items again — they were already captured in the previous chunk.", chunk.LastParagraph)
	}
	userContent += "\n\n---\n\n" + chunk.Content

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass2MaxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst performing deep semantic analysis. You extract detailed relationships, data flows, and control flows from COBOL source code and return structured JSON."},
			{Role: llm.RoleUser, Content: userContent},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
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

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass3MaxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL systems analyst. You analyze program relationships to identify business domains, dead code, and risk factors. Return structured JSON."},
			{Role: llm.RoleUser, Content: userContent},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
}

// AnalyzeJCL sends a JCL file to Claude Sonnet for structural extraction.
func (c *Client) AnalyzeJCL(ctx context.Context, fileName, content string) (string, error) {
	var userMsg bytes.Buffer
	if err := c.jclTmpl.Execute(&userMsg, map[string]string{
		"FileName": fileName,
	}); err != nil {
		return "", fmt.Errorf("rendering jcl template: %w", err)
	}

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: c.pass1MaxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are a mainframe JCL analysis assistant. You extract structural information from JCL files and return it as JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n---\n\n" + content},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
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

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.sonnetModel,
		MaxTokens: maxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst mapping data fields between programs through LINKAGE SECTION parameters. Return structured JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n" + fieldContext},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
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

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: c.pass2MaxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are an expert COBOL analyst repairing gaps in extracted program metadata. Return only valid JSON matching the requested schema."},
			{Role: llm.RoleUser, Content: userMsg.String()},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
}

// AnalyzeBW sends a Businessware file to Claude Opus for entity/relationship extraction.
func (c *Client) AnalyzeBW(ctx context.Context, fileName, fileType, content, existingPrograms string, maxTokens int) (string, error) {
	var userMsg bytes.Buffer
	if err := c.bwTmpl.Execute(&userMsg, map[string]any{
		"FileName":         fileName,
		"FileType":         fileType,
		"ExistingPrograms": existingPrograms,
		"IsDiagram":        isDiagramType(fileType),
	}); err != nil {
		return "", fmt.Errorf("rendering bw template: %w", err)
	}

	if maxTokens <= 0 {
		maxTokens = 16000
	}

	// Pre-flight token check: reject prompts that would exceed the context window
	// rather than getting a cryptic 413 from the API.
	fullUserMsg := userMsg.String() + "\n\n---\n\n" + content
	estimatedTokens := len(fullUserMsg) * 10 / 32
	if estimatedTokens > 160000 {
		return "", fmt.Errorf("BW prompt too large: ~%d tokens (limit ~160000) for file %s", estimatedTokens, fileName)
	}

	resp, err := c.completeWithRetry(ctx, llm.CompletionRequest{
		Model:     c.opusModel,
		MaxTokens: maxTokens,
		Messages: c.withJSONPrefill([]llm.Message{
			{Role: llm.RoleSystem, Content: "You are an enterprise software analyst. You extract entities, relationships, and COBOL cross-references from Businessware artifacts (Java code, documentation, configuration files, architecture diagrams). Return structured JSON."},
			{Role: llm.RoleUser, Content: userMsg.String() + "\n\n---\n\n" + content},
		}),
	})
	if err != nil {
		if resp != "" {
			resp = c.prependPrefill(resp)
		}
		return resp, err
	}
	return c.prependPrefill(resp), nil
}

// isDiagramType returns true if the file type represents a diagram format.
func isDiagramType(fileType string) bool {
	switch fileType {
	case "Visio Diagram", "DrawIO Diagram", "SVG Diagram", "PlantUML Diagram":
		return true
	default:
		return false
	}
}

// completeWithRetry calls the LLM provider with exponential backoff retries.
// On truncation (StopReason == "max_tokens"), it iteratively doubles max_tokens
// up to 3 times or the configured cap. On final truncation failure, the truncated
// content is returned alongside ErrResponseTruncated for partial JSON recovery.
func (c *Client) completeWithRetry(ctx context.Context, req llm.CompletionRequest) (string, error) {
	// Phase 6: Log estimated input tokens for context window monitoring
	var estimatedInputTokens int
	for _, msg := range req.Messages {
		estimatedInputTokens += len(msg.Content) * 10 / 32 // conservative estimate
	}
	if estimatedInputTokens > 150000 {
		c.logger.Warn("estimated input tokens approaching context window limit",
			zap.String("model", req.Model),
			zap.Int("estimated_input_tokens", estimatedInputTokens),
			zap.Int("warning_threshold", 150000),
		)
	}

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
			if !llm.IsRetriable(err) {
				c.logger.Error("LLM API call failed with non-retriable error",
					zap.String("provider", c.provider.Name()),
					zap.Error(err),
				)
				return "", fmt.Errorf("LLM API non-retriable error: %w", err)
			}
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

		// Phase 6: Log actual prompt tokens for calibration
		if resp.PromptTokens > 0 {
			c.logger.Debug("input token calibration",
				zap.Int("estimated", estimatedInputTokens),
				zap.Int("actual", resp.PromptTokens),
			)
			if c.calibrator != nil {
				c.calibrator.RecordSample(estimatedInputTokens, resp.PromptTokens)
			}
		}

		if resp.Content == "" {
			return "", fmt.Errorf("no text content in LLM response")
		}

		// Handle truncated responses: first try compression instructions, then double max_tokens.
		if resp.Truncated {
			truncatedContent := resp.Content
			currentMax := req.MaxTokens

			// Phase 1: Try progressive compression instructions before doubling tokens.
			compressionInstructions := []string{
				"\n\nIMPORTANT: Be maximally concise. Omit empty arrays (remove keys with [] values). Use minimal whitespace in JSON output.",
				"\n\nIMPORTANT: Output ONLY non-empty arrays and objects. Remove ALL keys whose values would be empty arrays [] or empty strings. Minimize output size.",
			}

			compressionSucceeded := false
			for compIdx, instruction := range compressionInstructions {
				c.logger.Warn("LLM response truncated, retrying with compression instruction",
					zap.String("model", req.Model),
					zap.Int("max_tokens", currentMax),
					zap.Int("compression_level", compIdx+1),
				)

				// Append compression instruction to the last user message.
				compReq := req
				compReq.Messages = make([]llm.Message, len(req.Messages))
				copy(compReq.Messages, req.Messages)
				for i := len(compReq.Messages) - 1; i >= 0; i-- {
					if compReq.Messages[i].Role == llm.RoleUser {
						compReq.Messages[i].Content += instruction
						break
					}
				}

				if err := c.limiter.Wait(ctx); err != nil {
					return truncatedContent, fmt.Errorf("rate limiter during compression retry: %w", err)
				}
				compCtx, compCancel := context.WithTimeout(ctx, c.requestTimeout)
				compResp, compErr := c.provider.Complete(compCtx, compReq)
				compCancel()

				if compErr != nil {
					c.logger.Warn("compression retry failed",
						zap.String("model", req.Model),
						zap.Int("compression_level", compIdx+1),
						zap.Error(compErr),
					)
					break
				}
				if compResp.Content == "" {
					break
				}
				if !compResp.Truncated {
					compressionSucceeded = true
					truncatedContent = compResp.Content
					break
				}
				// Still truncated — keep best content and try next compression level.
				truncatedContent = compResp.Content
			}

			if compressionSucceeded {
				return truncatedContent, nil
			}

			// Phase 2: Double max_tokens up to 3 times.
			for doublingIter := range 3 {
				doubled := currentMax * 2
				if doubled > c.maxOutputTokensCap {
					doubled = c.maxOutputTokensCap
				}
				if doubled <= currentMax {
					// At cap, can't increase further — return truncated content for partial recovery.
					c.logger.Error("LLM response truncated at max_tokens cap, returning for partial recovery",
						zap.String("model", req.Model),
						zap.Int("max_tokens", currentMax),
						zap.Int("output_tokens", resp.OutputTokens),
						zap.Int("doubling_attempts", doublingIter+1),
					)
					return truncatedContent, fmt.Errorf("%w: model=%s max_tokens=%d output_tokens=%d",
						ErrResponseTruncated, req.Model, currentMax, resp.OutputTokens)
				}

				c.logger.Warn("LLM response truncated, retrying with increased max_tokens",
					zap.String("model", req.Model),
					zap.Int("current_max_tokens", currentMax),
					zap.Int("new_max_tokens", doubled),
					zap.Int("output_tokens", resp.OutputTokens),
					zap.Int("doubling_iteration", doublingIter+1),
				)

				retryReq := req
				retryReq.MaxTokens = doubled
				currentMax = doubled

				if err := c.limiter.Wait(ctx); err != nil {
					return truncatedContent, fmt.Errorf("rate limiter during truncation retry: %w", err)
				}
				retryCtx, retryCancel := context.WithTimeout(ctx, c.requestTimeout)
				retryResp, retryErr := c.provider.Complete(retryCtx, retryReq)
				retryCancel()

				if retryErr != nil {
					c.logger.Warn("truncation retry failed",
						zap.String("model", retryReq.Model),
						zap.Int("doubling_iteration", doublingIter+1),
						zap.Error(retryErr),
					)
					// Fall through to return truncated content for partial recovery.
					break
				}

				if retryResp.Content == "" {
					break
				}

				if !retryResp.Truncated {
					return retryResp.Content, nil
				}

				// Still truncated — update content and continue doubling.
				truncatedContent = retryResp.Content
				resp = retryResp
			}

			// All attempts exhausted — return truncated content for partial recovery.
			c.logger.Error("LLM response still truncated after all retry attempts, returning for partial recovery",
				zap.String("model", req.Model),
				zap.Int("final_max_tokens", currentMax),
				zap.Int("output_tokens", resp.OutputTokens),
			)
			return truncatedContent, fmt.Errorf("%w: model=%s max_tokens=%d output_tokens=%d",
				ErrResponseTruncated, req.Model, currentMax, resp.OutputTokens)
		}

		return resp.Content, nil
	}

	return "", fmt.Errorf("LLM API failed after %d retries: %w", c.maxRetries, lastErr)
}
