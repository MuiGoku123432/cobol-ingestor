package estimate

import "math"

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
	BasePerRequest float64            // cost per premium request once over included quota
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

// ─── Azure APIM Pricing ──────────────────────────────────────────────────────
// Two-model approach: GPT-5.4 (complex/opus) + GPT-5-mini (simple/sonnet).
// Last updated: 2026-04-09

// apimModelPricing holds per-million-token PAYG rates and PTU throughput for one model.
type apimModelPricing struct {
	InputPerMTok    float64 // PAYG input cost per million tokens
	OutputPerMTok   float64 // PAYG output cost per million tokens
	InputTPMPerPTU  float64 // input tokens per minute per PTU
	OutputInputRatio float64 // output tokens count as this many input tokens toward PTU utilization
}

var apimPricing = map[string]apimModelPricing{
	// GPT-5.4: opus-class, complex passes (Pass 2, 3, 5 repair/annotations)
	// PAYG: $2.50/$15.00 per MTok (in/out)
	// PTU: 2,400 input TPM per PTU; 1 output token = 6 input tokens utilization (matches price ratio)
	"opus": {
		InputPerMTok:     2.50,
		OutputPerMTok:    15.00,
		InputTPMPerPTU:   2400,
		OutputInputRatio: 6.0,
	},
	// GPT-5-mini: sonnet-class, simple passes (Pass 1, 4, 5 verify, scanner)
	// PAYG: $0.25/$2.00 per MTok (in/out)
	// PTU: 23,750 input TPM per PTU; 1 output token = 8 input tokens utilization (matches price ratio)
	"sonnet": {
		InputPerMTok:     0.25,
		OutputPerMTok:    2.00,
		InputTPMPerPTU:   23750,
		OutputInputRatio: 8.0,
	},
}

// APIMConfig holds the user's PTU allocation and hourly rate.
type APIMConfig struct {
	PTU        map[string]int // model tier → number of deployed PTUs
	HourlyRate float64        // $/PTU/hr (global = $1.00, regional = $2.00)
}

// DefaultAPIMConfig reflects the user's setup: 125 PTU for GPT-5.4, 15 PTU for GPT-5-mini.
var DefaultAPIMConfig = APIMConfig{
	PTU: map[string]int{
		"opus":   125,
		"sonnet": 15,
	},
	HourlyRate: 1.00, // global provisioned
}

// apimPaygCost returns the PAYG (no PTU) cost for a pass — used as max estimate.
func apimPaygCost(tier string, inputTokens, outputTokens int) float64 {
	p, ok := apimPricing[tier]
	if !ok {
		return 0
	}
	return float64(inputTokens)/1_000_000*p.InputPerMTok +
		float64(outputTokens)/1_000_000*p.OutputPerMTok
}

// apimPTUMinutes returns the estimated wall-clock minutes to process a pass on the
// allocated PTUs. Uses the PTU utilization model: output tokens are weighted as
// OutputInputRatio input-equivalent tokens when computing throughput consumption.
func apimPTUMinutes(tier string, inputTokens, outputTokens, ptuCount int) float64 {
	p, ok := apimPricing[tier]
	if !ok || ptuCount <= 0 {
		return 0
	}
	effectiveTPM := float64(ptuCount) * p.InputTPMPerPTU
	if effectiveTPM == 0 {
		return 0
	}
	utilTokens := float64(inputTokens) + float64(outputTokens)*p.OutputInputRatio
	return utilTokens / effectiveTPM
}

// apimPTUCost returns the PTU-based cost for a pass (min estimate when PTUs are available).
// Cost = (minutes / 60) * PTUs * hourlyRate.
func apimPTUCost(tier string, inputTokens, outputTokens, ptuCount int, hourlyRate float64) float64 {
	minutes := apimPTUMinutes(tier, inputTokens, outputTokens, ptuCount)
	return math.Ceil(minutes*100)/100 / 60 * float64(ptuCount) * hourlyRate
}
