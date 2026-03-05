package llm

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicProvider implements Provider using the Anthropic SDK directly.
type AnthropicProvider struct {
	client      anthropic.Client
	opusModel   string
	sonnetModel string
}

func NewAnthropicProvider(cfg *config.Config) (*AnthropicProvider, error) {
	if cfg.LLM.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for the anthropic provider")
	}

	client := anthropic.NewClient(option.WithAPIKey(cfg.LLM.APIKey))

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

	resp, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic completion: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &CompletionResponse{
		Content:      content,
		Model:        string(resp.Model),
		PromptTokens: int(resp.Usage.InputTokens),
		OutputTokens: int(resp.Usage.OutputTokens),
		StopReason:   string(resp.StopReason),
		Truncated:    resp.StopReason == "max_tokens",
	}, nil
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
