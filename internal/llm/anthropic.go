package llm

import (
	"context"
	"errors"
	"fmt"

	"cobol-ingestor/internal/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
)

// AnthropicProvider implements Provider using the Anthropic SDK directly.
// Uses streaming to avoid response header timeouts on large COBOL analysis.
type AnthropicProvider struct {
	client      anthropic.Client
	opusModel   string
	sonnetModel string
}

func NewAnthropicProvider(cfg *config.Config) (*AnthropicProvider, error) {
	if cfg.LLM.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for the anthropic provider")
	}

	client := anthropic.NewClient(
		option.WithAPIKey(cfg.LLM.APIKey),
		option.WithRequestTimeout(cfg.LLM.Timeout),
	)

	return &AnthropicProvider{
		client:      client,
		opusModel:   cfg.Claude.OpusModel,
		sonnetModel: cfg.Claude.SonnetModel,
	}, nil
}

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.sonnetModel
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	msgs := make([]anthropic.MessageParam, 0, len(req.Messages))
	var systemPrompt string

	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			systemPrompt = m.Content
		case RoleUser:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case RoleAssistant:
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	if req.Temperature > 0 {
		params.Temperature = anthropic.Float(req.Temperature)
	}

	stream := p.client.Messages.NewStreaming(ctx, params)
	defer stream.Close()

	var content string
	var inputTokens, outputTokens int
	var stopReason string

	if err := p.consumeStream(stream, &content, &inputTokens, &outputTokens, &stopReason); err != nil {
		// Pass through context cancellation/timeout without wrapping.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		// Classify API errors by status code.
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			retriable := isRetriableStatus(apiErr.StatusCode)
			return nil, &LLMError{
				StatusCode: apiErr.StatusCode,
				Retriable:  retriable,
				Message:    apiErr.Error(),
				Err:        err,
			}
		}
		return nil, fmt.Errorf("anthropic streaming: %w", err)
	}

	return &CompletionResponse{
		Content:      content,
		Model:        string(params.Model),
		PromptTokens: inputTokens,
		OutputTokens: outputTokens,
		StopReason:   stopReason,
		Truncated:    stopReason == "max_tokens",
	}, nil
}

func (p *AnthropicProvider) consumeStream(
	stream *ssestream.Stream[anthropic.MessageStreamEventUnion],
	content *string, inputTokens, outputTokens *int, stopReason *string,
) error {
	for stream.Next() {
		event := stream.Current()
		switch event.Type {
		case "content_block_delta":
			*content += event.Delta.Text
		case "message_delta":
			*stopReason = string(event.Delta.StopReason)
			*outputTokens = int(event.Usage.OutputTokens)
		case "message_start":
			*inputTokens = int(event.Message.Usage.InputTokens)
		}
	}
	return stream.Err()
}

// isRetriableStatus returns true for HTTP status codes that warrant a retry.
func isRetriableStatus(code int) bool {
	switch code {
	case 400, 401, 403:
		return false
	default:
		// 429, 500, 502, 503, 529, and anything else → retry
		return true
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	_, err := p.Complete(ctx, CompletionRequest{
		Model:     p.sonnetModel,
		Messages:  []Message{{Role: RoleUser, Content: "ping"}},
		MaxTokens: 16,
	})
	return err
}

func (p *AnthropicProvider) Close() error {
	return nil
}
