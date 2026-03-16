package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"cobol-ingestor/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

// BedrockProvider implements Provider and ChatProvider using AWS Bedrock's
// Converse / ConverseStream APIs. Authentication uses the standard AWS
// credential chain (env vars, instance profiles, ECS task roles, SSO).
type BedrockProvider struct {
	client      *bedrockruntime.Client
	opusModel   string
	sonnetModel string
}

// Default Bedrock model IDs for Claude models.
const (
	defaultBedrockOpusModel   = "anthropic.claude-opus-4-20250514-v1:0"
	defaultBedrockSonnetModel = "anthropic.claude-sonnet-4-20250514-v1:0"
)

func NewBedrockProvider(cfg *config.Config) (*BedrockProvider, error) {
	region := cfg.LLM.BedrockRegion
	if region == "" {
		region = "us-east-1"
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("bedrock: loading AWS config: %w", err)
	}

	client := bedrockruntime.NewFromConfig(awsCfg)

	return &BedrockProvider{
		client:      client,
		opusModel:   cfg.Claude.OpusModel,
		sonnetModel: cfg.Claude.SonnetModel,
	}, nil
}

// bedrockModelID maps a Claude model name to a Bedrock model identifier.
func bedrockModelID(model, override string) string {
	if override != "" {
		return override
	}
	switch model {
	case "claude-opus-4-6", "claude-opus-4-20250514":
		return defaultBedrockOpusModel
	case "claude-sonnet-4-6", "claude-sonnet-4-20250514":
		return defaultBedrockSonnetModel
	default:
		// If already a Bedrock-style ID (contains "."), use as-is.
		for _, c := range model {
			if c == '.' {
				return model
			}
		}
		return "anthropic." + model + "-v1:0"
	}
}

func (p *BedrockProvider) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.sonnetModel
	}

	maxTokens := int32(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Build messages.
	var msgs []types.Message
	var systemBlocks []types.SystemContentBlock
	for _, m := range req.Messages {
		switch m.Role {
		case RoleSystem:
			systemBlocks = append(systemBlocks, &types.SystemContentBlockMemberText{
				Value: m.Content,
			})
		case RoleUser:
			msgs = append(msgs, types.Message{
				Role:    types.ConversationRoleUser,
				Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: m.Content}},
			})
		case RoleAssistant:
			msgs = append(msgs, types.Message{
				Role:    types.ConversationRoleAssistant,
				Content: []types.ContentBlock{&types.ContentBlockMemberText{Value: m.Content}},
			})
		}
	}

	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(bedrockModelID(model, "")),
		Messages: msgs,
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(maxTokens),
		},
	}
	if len(systemBlocks) > 0 {
		input.System = systemBlocks
	}

	output, err := p.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock complete: %w", err)
	}

	var content string
	if msg, ok := output.Output.(*types.ConverseOutputMemberMessage); ok {
		for _, block := range msg.Value.Content {
			if tb, ok := block.(*types.ContentBlockMemberText); ok {
				content += tb.Value
			}
		}
	}

	stopReason := string(output.StopReason)
	var promptTokens, outputTokens int
	if output.Usage != nil {
		promptTokens = int(aws.ToInt32(output.Usage.InputTokens))
		outputTokens = int(aws.ToInt32(output.Usage.OutputTokens))
	}

	return &CompletionResponse{
		Content:      content,
		Model:        bedrockModelID(model, ""),
		PromptTokens: promptTokens,
		OutputTokens: outputTokens,
		StopReason:   stopReason,
		Truncated:    stopReason == "max_tokens",
	}, nil
}

func (p *BedrockProvider) Name() string {
	return "bedrock"
}

func (p *BedrockProvider) HealthCheck(ctx context.Context) error {
	_, err := p.Complete(ctx, CompletionRequest{
		Model:     p.sonnetModel,
		Messages:  []Message{{Role: RoleUser, Content: "ping"}},
		MaxTokens: 16,
	})
	return err
}

func (p *BedrockProvider) Close() error {
	return nil
}

// --- helpers for converting tool schemas to Bedrock document types ---

// mapToDocument converts a Go map to a bedrockruntime document.Interface.
func mapToDocument(m map[string]any) document.Interface {
	return document.NewLazyDocument(m)
}

// documentToJSON converts a bedrockruntime document.Interface to JSON bytes.
func documentToJSON(doc document.Interface) json.RawMessage {
	if doc == nil {
		return json.RawMessage("{}")
	}
	var v any
	if err := doc.UnmarshalSmithyDocument(&v); err != nil {
		return json.RawMessage("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return b
}
