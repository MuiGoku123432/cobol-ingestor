package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/cecil-the-coder/ai-provider-kit/pkg/types"
)

// ResolvedModels holds the resolved model IDs for opus and sonnet.
type ResolvedModels struct {
	OpusModel   string
	SonnetModel string
	AllModels   []types.Model // all Claude models found
}

// ResolveCopilotModels queries the Copilot provider for available models
// and resolves the best match for opus and sonnet. Falls back to the
// provided defaults if discovery fails or no matches are found.
func ResolveCopilotModels(ctx context.Context, provider Provider, defaultOpus, defaultSonnet string) (*ResolvedModels, error) {
	cp, ok := provider.(*CopilotProvider)
	if !ok {
		return nil, fmt.Errorf("provider is not a CopilotProvider")
	}

	models, err := cp.GetCopilotProvider().GetModels(ctx)
	if err != nil {
		return &ResolvedModels{
			OpusModel:   defaultOpus,
			SonnetModel: defaultSonnet,
		}, fmt.Errorf("fetching models: %w", err)
	}

	resolved := &ResolvedModels{
		OpusModel:   defaultOpus,
		SonnetModel: defaultSonnet,
	}

	for _, m := range models {
		idLower := strings.ToLower(m.ID)
		if !strings.Contains(idLower, "claude") {
			continue
		}
		resolved.AllModels = append(resolved.AllModels, m)

		if strings.Contains(idLower, "opus") {
			resolved.OpusModel = m.ID
		} else if strings.Contains(idLower, "sonnet") {
			resolved.SonnetModel = m.ID
		}
	}

	return resolved, nil
}
