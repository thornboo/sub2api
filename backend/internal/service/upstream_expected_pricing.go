package service

import (
	"context"
	"strings"
)

type upstreamExpectedPricingContextKey struct{}

type upstreamExpectedPricingContext struct {
	channelID int64
	platform  string
}

func withUpstreamExpectedPricing(ctx context.Context, account *Account) context.Context {
	metadata := upstreamExpectedPricingContext{}
	if account != nil {
		metadata.platform = account.Platform
		if account.UpstreamOfficialPricingChannelID != nil {
			metadata.channelID = *account.UpstreamOfficialPricingChannelID
		}
	}
	return context.WithValue(ctx, upstreamExpectedPricingContextKey{}, metadata)
}

func isUpstreamExpectedPricing(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, enabled := ctx.Value(upstreamExpectedPricingContextKey{}).(upstreamExpectedPricingContext)
	return enabled
}

func upstreamExpectedPricingMetadata(ctx context.Context) (upstreamExpectedPricingContext, bool) {
	if ctx == nil {
		return upstreamExpectedPricingContext{}, false
	}
	metadata, ok := ctx.Value(upstreamExpectedPricingContextKey{}).(upstreamExpectedPricingContext)
	return metadata, ok
}

// upstreamPricingAPIKey supplies only the minimal Group shell required by the
// shared billing code. The explicit official catalog comes from context; no
// downstream group or channel identity survives into the audit calculation.
func upstreamPricingAPIKey(apiKey *APIKey) *APIKey {
	if apiKey == nil {
		return nil
	}
	cloned := *apiKey
	platform := ""
	if apiKey.Group != nil {
		platform = apiKey.Group.Platform
	}
	clonedGroup := &Group{
		ID:                        0,
		Platform:                  platform,
		LongContextPricingEnabled: true,
	}
	cloned.Group = clonedGroup
	return &cloned
}

func upstreamCurrencyAllowsResolvedPricing(currency string, resolved *ResolvedPricing) bool {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	switch currency {
	case UpstreamPriceReferenceCurrencyUSD:
		return true
	case UpstreamPriceReferenceCurrencyCNY:
		// The bundled model catalog is denominated in USD. A CNY binding must
		// therefore use an explicit channel price card whose numeric values the
		// administrator has confirmed are the upstream CNY official list price.
		return resolved != nil && resolved.Source == PricingSourceChannel
	default:
		return false
	}
}

func upstreamCurrencyIsCNY(currency string) bool {
	return strings.EqualFold(strings.TrimSpace(currency), UpstreamPriceReferenceCurrencyCNY)
}
