package llm

import (
	"context"
	"fmt"
	"strings"

	"cobol-ingestor/internal/config"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// OpenAIProvider implements Provider and ChatProvider using the OpenAI
// Chat Completions API. Supports OpenAI, Azure OpenAI, and any
// OpenAI-compatible endpoint via OPENAI_BASE_URL.
type OpenAIProvider struct {
	client      *openai.Client
	defaultModel string
	opusModel   string
	sonnetModel string
}

func NewOpenAIProvider(cfg *config.Config) (*OpenAIProvider, error) {
	if cfg.LLM.OpenAIAPIKey == "" {
		return nil, fmt.Errorf("openai: OPENAI_API_KEY is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(cfg.LLM.OpenAIAPIKey),
	}
	if cfg.LLM.OpenAIBaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.LLM.OpenAIBaseURL))
	}
	if cfg.LLM.OpenAIOrgID != "" {
		opts = append(opts, option.WithOrganization(cfg.LLM.OpenAIOrgID))
	}

	client := openai.NewClient(opts...)

	model := cfg.LLM.OpenAIModel
	if model == "" {
		model = "gpt-4o"
	}

	return &OpenAIProvider{
		client:       &client,
		defaultModel: model,
		opusModel:    cfg.Claude.OpusModel,
		sonnetModel:  cfg.Claude.SonnetModel,
	}, nil
}

// resolveModel maps Claude model names to the configured OpenAI model.
// Non-Claude model names pass through unchanged.
func (p *OpenAIProvider) resolveModel(model string) string {
	if model == "" {
		return p.defaultModel
	}
	if strings.HasPrefix(model, "claude-") {
		return p.defaultModel
	}
	return model
}

// normalizeOpenAIStopReason converts OpenAI finish reasons to the internal format.
func normalizeOpenAIStopReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}

func (p *OpenAIProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := p.resolveModel(req.Model)

	var msgs []openai.ChatCompletionMessageParamUnion
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			msgs = append(msgs, openai.SystemMessage(m.Content))
		case RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    shared.ChatModel(model),
		Messages: msgs,
	}
	if req.MaxTokens > 0 {
		params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
	}

	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("openai complete: %w", err)
	}

	var content string
	var stopReason string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
		stopReason = normalizeOpenAIStopReason(resp.Choices[0].FinishReason)
	}

	return &CompletionResponse{
		Content:      content,
		Model:        resp.Model,
		PromptTokens: int(resp.Usage.PromptTokens),
		OutputTokens: int(resp.Usage.CompletionTokens),
		StopReason:   stopReason,
		Truncated:    stopReason == "max_tokens",
	}, nil
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	_, err := p.Complete(ctx, CompletionRequest{
		Messages:  []Message{{Role: RoleUser, Content: "ping"}},
		MaxTokens: 16,
	})
	return err
}

func (p *OpenAIProvider) Close() error {
	return nil
}
