package llm

import (
	"context"
	"fmt"

	"cobol-ingestor/internal/config"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/vertex"
)

// NewVertexProvider creates an AnthropicProvider backed by Google Vertex AI.
// Vertex AI serves the same Claude models through Google Cloud, so all existing
// Anthropic streaming and tool-use code is reused as-is. Authentication uses
// Google Application Default Credentials (ADC).
func NewVertexProvider(cfg *config.Config) (p *AnthropicProvider, err error) {
	if cfg.LLM.VertexProjectID == "" {
		return nil, fmt.Errorf("VERTEX_PROJECT_ID is required for the vertex provider")
	}
	region := cfg.LLM.VertexRegion
	if region == "" {
		region = "us-east5"
	}

	// vertex.WithGoogleAuth panics on credential errors; convert to error.
	defer func() {
		if r := recover(); r != nil {
			p = nil
			err = fmt.Errorf("vertex auth: %v", r)
		}
	}()

	client := anthropic.NewClient(
		vertex.WithGoogleAuth(context.Background(), region, cfg.LLM.VertexProjectID),
		option.WithRequestTimeout(cfg.LLM.Timeout),
	)

	return &AnthropicProvider{
		client:      client,
		opusModel:   cfg.Claude.OpusModel,
		sonnetModel: cfg.Claude.SonnetModel,
	}, nil
}
