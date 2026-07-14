package provider

import "strings"

// ModelPricing defines cost per 1K tokens for a model.
type ModelPricing struct {
	InputPer1K     float64 // Cost per 1K input tokens (USD)
	OutputPer1K    float64 // Cost per 1K output tokens (USD)
	CachedDiscount float64 // Discount for cached tokens (0.5 = 50% off, 0.9 = 90% off)
}

// PricingTable maps model patterns to their pricing.
// Prices are per 1K tokens in USD.
var PricingTable = map[string]ModelPricing{
	// OpenAI models
	"gpt-4o":        {InputPer1K: 0.0025, OutputPer1K: 0.01, CachedDiscount: 0.5},
	"gpt-4o-mini":   {InputPer1K: 0.00015, OutputPer1K: 0.0006, CachedDiscount: 0.5},
	"gpt-4-turbo":   {InputPer1K: 0.01, OutputPer1K: 0.03, CachedDiscount: 0.5},
	"gpt-4":         {InputPer1K: 0.03, OutputPer1K: 0.06, CachedDiscount: 0.0},
	"gpt-3.5-turbo": {InputPer1K: 0.0005, OutputPer1K: 0.0015, CachedDiscount: 0.5},
	"o1":            {InputPer1K: 0.015, OutputPer1K: 0.06, CachedDiscount: 0.5},
	"o1-mini":       {InputPer1K: 0.003, OutputPer1K: 0.012, CachedDiscount: 0.5},
	"o3":            {InputPer1K: 0.01, OutputPer1K: 0.04, CachedDiscount: 0.5},
	"o3-mini":       {InputPer1K: 0.0011, OutputPer1K: 0.0044, CachedDiscount: 0.5},

	// Anthropic models
	"claude-opus-4":     {InputPer1K: 0.015, OutputPer1K: 0.075, CachedDiscount: 0.9},
	"claude-sonnet-4":   {InputPer1K: 0.003, OutputPer1K: 0.015, CachedDiscount: 0.9},
	"claude-3-5-sonnet": {InputPer1K: 0.003, OutputPer1K: 0.015, CachedDiscount: 0.9},
	"claude-3-5-haiku":  {InputPer1K: 0.0008, OutputPer1K: 0.004, CachedDiscount: 0.9},
	"claude-3-opus":     {InputPer1K: 0.015, OutputPer1K: 0.075, CachedDiscount: 0.9},
	"claude-3-sonnet":   {InputPer1K: 0.003, OutputPer1K: 0.015, CachedDiscount: 0.9},
	"claude-3-haiku":    {InputPer1K: 0.00025, OutputPer1K: 0.00125, CachedDiscount: 0.9},

	// DeepSeek models
	"deepseek-v3": {InputPer1K: 0.00027, OutputPer1K: 0.0011, CachedDiscount: 0.5},
	"deepseek-r1": {InputPer1K: 0.00055, OutputPer1K: 0.00219, CachedDiscount: 0.5},
}

// FindPricing finds the pricing for a model by matching against the pricing table.
func FindPricing(model string) (ModelPricing, bool) {
	modelLower := strings.ToLower(model)

	// Try exact match first
	if p, ok := PricingTable[modelLower]; ok {
		return p, true
	}

	// Try prefix match (longest match wins)
	bestMatch := ""
	bestPricing := ModelPricing{}
	for pattern, pricing := range PricingTable {
		if strings.HasPrefix(modelLower, pattern) && len(pattern) > len(bestMatch) {
			bestMatch = pattern
			bestPricing = pricing
		}
	}

	if bestMatch != "" {
		return bestPricing, true
	}

	return ModelPricing{}, false
}

// CalculateCost calculates the cost of a request in USD.
func CalculateCost(model string, usage *TokenUsage) float64 {
	if usage == nil {
		return 0
	}

	pricing, ok := FindPricing(model)
	if !ok {
		return 0
	}

	// Calculate input cost (non-cached tokens at full price)
	nonCachedTokens := usage.PromptTokens - usage.CachedTokens
	if nonCachedTokens < 0 {
		nonCachedTokens = 0
	}
	inputCost := float64(nonCachedTokens) / 1000 * pricing.InputPer1K

	// Calculate cached token cost (at discounted rate)
	cachedCost := float64(usage.CachedTokens) / 1000 * pricing.InputPer1K * (1 - pricing.CachedDiscount)

	// Calculate output cost
	outputCost := float64(usage.CompletionTokens) / 1000 * pricing.OutputPer1K

	return inputCost + cachedCost + outputCost
}

// CalculateCacheSavings calculates how much was saved via caching.
func CalculateCacheSavings(model string, usage *TokenUsage) float64 {
	if usage == nil || usage.CachedTokens == 0 {
		return 0
	}

	pricing, ok := FindPricing(model)
	if !ok {
		return 0
	}

	// Savings = what we would have paid without cache minus what we actually paid
	fullPrice := float64(usage.CachedTokens) / 1000 * pricing.InputPer1K
	cachedPrice := fullPrice * (1 - pricing.CachedDiscount)
	return fullPrice - cachedPrice
}
