package estimate

// ModelPricing holds per-million-token costs for a model tier (Anthropic direct API).
// Last updated: 2025-04-08
type ModelPricing struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

// AnthropicPricing maps model tier names to their per-token costs.
var AnthropicPricing = map[string]ModelPricing{
	"opus":   {InputPerMTok: 15.0, OutputPerMTok: 75.0},
	"sonnet": {InputPerMTok: 3.0, OutputPerMTok: 15.0},
}

// CopilotPricing holds GitHub Copilot premium request pricing.
// Each API call counts as premium requests; the multiplier depends on the model.
// Last updated: 2025-04-08
type CopilotPricing struct {
	BasePerRequest float64 // cost per premium request once over included quota
	Multiplier     map[string]float64 // model tier → request multiplier
}

var DefaultCopilotPricing = CopilotPricing{
	BasePerRequest: 0.04,
	Multiplier: map[string]float64{
		"opus":   3.0,
		"sonnet": 1.0,
	},
}

func anthropicInputCost(tier string, tokens int) float64 {
	p, ok := AnthropicPricing[tier]
	if !ok {
		return 0
	}
	return float64(tokens) / 1_000_000 * p.InputPerMTok
}

func anthropicOutputCost(tier string, tokens int) float64 {
	p, ok := AnthropicPricing[tier]
	if !ok {
		return 0
	}
	return float64(tokens) / 1_000_000 * p.OutputPerMTok
}

// copilotRequestCost returns the dollar cost for a given number of requests on a model tier.
func copilotRequestCost(tier string, requests int) float64 {
	mult, ok := DefaultCopilotPricing.Multiplier[tier]
	if !ok {
		mult = 1.0
	}
	return float64(requests) * DefaultCopilotPricing.BasePerRequest * mult
}
