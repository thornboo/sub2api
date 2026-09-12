//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalculateTokenCost_IntervalLongContextMarker(t *testing.T) {
	for _, tt := range []struct {
		name           string
		tokens         UsageTokens
		groupEnabled   bool
		accountEnabled bool
		multiplier     float64
		serviceTier    string
		highInputPrice float64
		highMaxTokens  *int
		wantCost       float64
		wantApplied    bool
	}{
		{name: "below threshold", tokens: UsageTokens{InputTokens: 271999}, groupEnabled: true, multiplier: 1, wantCost: 2.71999},
		{name: "at threshold", tokens: UsageTokens{InputTokens: 272000}, groupEnabled: true, multiplier: 1, wantCost: 2.72},
		{name: "above threshold", tokens: UsageTokens{InputTokens: 272001}, groupEnabled: true, multiplier: 1, wantCost: 5.44002, wantApplied: true},
		{name: "reported usage", tokens: UsageTokens{InputTokens: 659468, OutputTokens: 941, CacheReadTokens: 3712}, groupEnabled: true, multiplier: 1, wantCost: 13.267359, wantApplied: true},
		{name: "group and account disabled", tokens: UsageTokens{InputTokens: 659468, OutputTokens: 941, CacheReadTokens: 3712}, multiplier: 1, wantCost: 6.645442},
		{name: "account opt in", tokens: UsageTokens{InputTokens: 272001}, accountEnabled: true, multiplier: 1, wantCost: 5.44002, wantApplied: true},
		{name: "priority below threshold is not long context", tokens: UsageTokens{InputTokens: 272000}, groupEnabled: true, multiplier: 1, serviceTier: "priority", wantCost: 5.44},
		{name: "priority above threshold", tokens: UsageTokens{InputTokens: 272001}, groupEnabled: true, multiplier: 1, serviceTier: "priority", wantCost: 10.88004, wantApplied: true},
		{name: "cached read crosses threshold", tokens: UsageTokens{InputTokens: 1, CacheReadTokens: 272000}, groupEnabled: true, multiplier: 1, wantCost: 0.54402, wantApplied: true},
		{name: "cached write crosses threshold", tokens: UsageTokens{InputTokens: 1, CacheCreationTokens: 272000}, groupEnabled: true, multiplier: 1, wantCost: 6.80002, wantApplied: true},
		{name: "same price is not a surcharge", tokens: UsageTokens{InputTokens: 272001}, groupEnabled: true, multiplier: 1, highInputPrice: 10e-6, wantCost: 2.72001},
		{name: "discounted tier is not a surcharge", tokens: UsageTokens{InputTokens: 272001}, groupEnabled: true, multiplier: 1, highInputPrice: 5e-6, wantCost: 1.360005},
		{name: "free request", tokens: UsageTokens{InputTokens: 272001}, groupEnabled: true, wantCost: 0},
		{name: "outside configured intervals uses base", tokens: UsageTokens{InputTokens: 659468}, groupEnabled: true, multiplier: 1, highMaxTokens: testPtrInt(500000), wantCost: 6.59468},
	} {
		t.Run(tt.name, func(t *testing.T) {
			highInput := tt.highInputPrice
			if highInput == 0 {
				highInput = 20e-6
			}
			resolved := &ResolvedPricing{
				Mode: BillingModeToken, Source: PricingSourceChannel,
				BasePricing: &ModelPricing{
					InputPricePerToken: 10e-6, OutputPricePerToken: 50e-6,
					CacheCreationPricePerToken: 12.5e-6, CacheReadPricePerToken: 1e-6,
				},
				Intervals: []PricingInterval{
					{MinTokens: 0, MaxTokens: testPtrInt(272000), InputPrice: testPtrFloat64(10e-6), OutputPrice: testPtrFloat64(50e-6), CacheWritePrice: testPtrFloat64(12.5e-6), CacheReadPrice: testPtrFloat64(1e-6)},
					{MinTokens: 272000, MaxTokens: tt.highMaxTokens, InputPrice: &highInput, OutputPrice: testPtrFloat64(75e-6), CacheWritePrice: testPtrFloat64(25e-6), CacheReadPrice: testPtrFloat64(2e-6)},
				},
				longContextPricingEnabled: tt.groupEnabled,
			}
			got, err := (&BillingService{}).CalculateCostUnified(CostInput{
				Model: "gpt-6-astra", Tokens: tt.tokens, RateMultiplier: tt.multiplier,
				ServiceTier: tt.serviceTier,
				Resolver:    &ModelPricingResolver{}, Resolved: resolved,
				LongContextBillingEnabled: &tt.accountEnabled,
			})
			require.NoError(t, err)
			require.InDelta(t, tt.wantCost, got.ActualCost, 1e-10)
			require.Equal(t, tt.wantApplied, got.LongContextBillingApplied)
		})
	}
}
