package llm

import (
	"context"
	"fmt"
	"io"

	"cobol-ingestor/internal/config"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/providers/copilot"
	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// CopilotProvider implements Provider using GitHub Copilot as the LLM backend.
// Copilot proxies access to Claude and other models through your GitHub subscription.
type CopilotProvider struct {
	provider *copilot.CopilotProvider
	model    string
}

func NewCopilotProvider(cfg *config.Config) (*CopilotProvider, error) {
	providerConfig := map[string]interface{}{}

	if cfg.LLM.CopilotGitHubToken != "" {
		providerConfig["github_token"] = cfg.LLM.CopilotGitHubToken
	}
	if cfg.LLM.CopilotAccountType != "" {
		providerConfig["account_type"] = cfg.LLM.CopilotAccountType
	}

	providerCfg := types.ProviderConfig{
		Name:           "copilot",
		DefaultModel:   cfg.Claude.SonnetModel,
		ProviderConfig: providerConfig,
	}

	if cfg.LLM.APIKey != "" {
		providerCfg.APIKey = cfg.LLM.APIKey
	}

	cp := copilot.NewCopilotProvider(providerCfg)

	// Configure will pick up the github_token and account_type from ProviderConfig
	if err := cp.Configure(providerCfg); err != nil {
		return nil, fmt.Errorf("copilot configuration: %w", err)
	}

	return &CopilotProvider{
		provider: cp,
		model:    cfg.Claude.SonnetModel,
	}, nil
}

func (p *CopilotProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	msgs := make([]types.ChatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		msgs = append(msgs, types.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	opts := types.GenerateOptions{
		Model:     model,
		Messages:  msgs,
		MaxTokens: req.MaxTokens,
		Stream:    false,
	}

	stream, err := p.provider.GenerateChatCompletion(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("copilot completion: %w", err)
	}
	defer stream.Close()

	var content string
	var usage types.Usage

	for {
		chunk, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading copilot stream: %w", err)
		}
		content += chunk.Content
		usage = chunk.Usage
	}

	return &CompletionResponse{
		Content:      content,
		Model:        model,
		PromptTokens: usage.PromptTokens,
		OutputTokens: usage.CompletionTokens,
	}, nil
}

func (p *CopilotProvider) Name() string {
	return "copilot"
}

// GetCopilotProvider returns the underlying copilot provider for model discovery.
func (p *CopilotProvider) GetCopilotProvider() *copilot.CopilotProvider {
	return p.provider
}

func (p *CopilotProvider) HealthCheck(ctx context.Context) error {
	return p.provider.HealthCheck(ctx)
}

func (p *CopilotProvider) Close() error {
	return nil
}
