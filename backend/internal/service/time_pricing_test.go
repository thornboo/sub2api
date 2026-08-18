//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func atShanghai(t *testing.T, hh, mm int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	return time.Date(2026, 8, 17, hh, mm, 0, 0, loc)
}

func TestValidateTimePricingForMode(t *testing.T) {
	defaultMultiplier := 0.8
	valid := &TimePricing{
		Enabled:           true,
		Timezone:          "Asia/Shanghai",
		DefaultLabel:      "regular",
		DefaultMultiplier: &defaultMultiplier,
		Rules: []TimePricingRule{{
			Label:      "morning",
			StartTime:  "09:00",
			EndTime:    "12:00",
			Multiplier: 2,
		}},
	}
	require.NoError(t, ValidateTimePricingForMode(valid, BillingModeToken))

	t.Run("non token rejected", func(t *testing.T) {
		err := ValidateTimePricingForMode(valid, BillingModePerRequest)
		require.Error(t, err)
		require.Contains(t, err.Error(), "token")
	})

	t.Run("no explicit rules uses configurable all-day default", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules = nil
		require.NoError(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("invalid default multiplier rejected", func(t *testing.T) {
		cp := valid.Clone()
		invalid := 101.0
		cp.DefaultMultiplier = &invalid
		err := ValidateTimePricingForMode(&cp, BillingModeToken)
		require.Error(t, err)
		require.Contains(t, err.Error(), "default_multiplier")
	})

	t.Run("zero and one hundred multipliers are accepted", func(t *testing.T) {
		cp := valid.Clone()
		zero := 0.0
		cp.DefaultMultiplier = &zero
		cp.Rules[0].Multiplier = 100
		require.NoError(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("customer facing labels are required and bounded", func(t *testing.T) {
		cp := valid.Clone()
		cp.DefaultLabel = " "
		err := ValidateTimePricingForMode(&cp, BillingModeToken)
		require.Error(t, err)
		require.Contains(t, err.Error(), "default_label")

		cp = valid.Clone()
		cp.Rules[0].Label = ""
		err = ValidateTimePricingForMode(&cp, BillingModeToken)
		require.Error(t, err)
		require.Contains(t, err.Error(), "rules[0].label")

		cp = valid.Clone()
		cp.DefaultLabel = "常常常常常常常常常常常常常常常常常常常常常常常常常常常常常常常常常"
		err = ValidateTimePricingForMode(&cp, BillingModeToken)
		require.Error(t, err)
		require.Contains(t, err.Error(), "at most 32 characters")
	})

	t.Run("strict HH:MM", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules[0].StartTime = "9:00"
		require.Error(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("same start end rejected", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules[0].EndTime = "09:00"
		require.Error(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("overlap rejected", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules = append(cp.Rules, TimePricingRule{Label: "lunch", StartTime: "11:00", EndTime: "13:00", Multiplier: 1.5})
		require.Error(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("cross midnight overlap rejected", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules = []TimePricingRule{
			{Label: "night", StartTime: "23:00", EndTime: "07:00", Multiplier: 0.5},
			{Label: "morning", StartTime: "06:30", EndTime: "08:00", Multiplier: 2},
		}
		require.Error(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})

	t.Run("cross midnight allowed", func(t *testing.T) {
		cp := valid.Clone()
		cp.Rules = []TimePricingRule{{Label: "night", StartTime: "23:00", EndTime: "07:00", Multiplier: 0.5}}
		require.NoError(t, ValidateTimePricingForMode(&cp, BillingModeToken))
	})
}

func TestTimePricingMultiplierAtBoundariesAndCrossMidnight(t *testing.T) {
	defaultMultiplier := 0.75
	p := &TimePricing{
		Enabled:           true,
		Timezone:          "Asia/Shanghai",
		DefaultLabel:      "平时",
		DefaultMultiplier: &defaultMultiplier,
		Rules: []TimePricingRule{
			{Label: "高峰", StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
			{Label: "夜间", StartTime: "23:00", EndTime: "07:00", Multiplier: 0.5},
		},
	}

	require.Equal(t, "高峰", p.MultiplierAt(atShanghai(t, 9, 0)).Label)
	require.Equal(t, 2.0, p.MultiplierAt(atShanghai(t, 9, 0)).Multiplier)
	require.Equal(t, "平时", p.MultiplierAt(atShanghai(t, 12, 0)).Label)
	require.Equal(t, 0.75, p.MultiplierAt(atShanghai(t, 12, 0)).Multiplier)
	require.Equal(t, 0.5, p.MultiplierAt(atShanghai(t, 23, 30)).Multiplier)
	require.Equal(t, 0.5, p.MultiplierAt(atShanghai(t, 6, 30)).Multiplier)
	require.Equal(t, 0.75, p.MultiplierAt(atShanghai(t, 7, 0)).Multiplier)
}

func TestTimePricingMultiplierAtDSTFallbackUsesWallClockRule(t *testing.T) {
	defaultMultiplier := 1.0
	p := &TimePricing{
		Enabled:           true,
		Timezone:          "America/New_York",
		DefaultLabel:      "regular",
		DefaultMultiplier: &defaultMultiplier,
		Rules: []TimePricingRule{{
			Label:      "repeated-hour",
			StartTime:  "01:00",
			EndTime:    "02:00",
			Multiplier: 2,
		}},
	}

	// 2026-11-01 01:30 occurs twice in New York, once in EDT and once in EST.
	first0130 := time.Date(2026, time.November, 1, 5, 30, 0, 0, time.UTC)
	second0130 := time.Date(2026, time.November, 1, 6, 30, 0, 0, time.UTC)
	require.Equal(t, "repeated-hour", p.MultiplierAt(first0130).Label)
	require.Equal(t, 2.0, p.MultiplierAt(first0130).Multiplier)
	require.Equal(t, "repeated-hour", p.MultiplierAt(second0130).Label)
	require.Equal(t, 2.0, p.MultiplierAt(second0130).Multiplier)
}

func TestTimePricingMultiplierAt_DefaultMultiplierCompatibilityAndSafety(t *testing.T) {
	t.Run("missing field remains one for legacy json", func(t *testing.T) {
		p := &TimePricing{Enabled: true, Timezone: "Asia/Shanghai"}
		applied := p.MultiplierAt(atShanghai(t, 12, 0))
		require.Equal(t, 1.0, applied.Multiplier)
		require.Equal(t, "Asia/Shanghai", applied.Timezone)
		require.Nil(t, applied.Rule)
	})

	t.Run("configured default applies with no explicit rule", func(t *testing.T) {
		defaultMultiplier := 1.1
		p := &TimePricing{
			Enabled:           true,
			Timezone:          "Asia/Shanghai",
			DefaultLabel:      "平时",
			DefaultMultiplier: &defaultMultiplier,
		}
		applied := p.MultiplierAt(atShanghai(t, 12, 0))
		require.Equal(t, 1.1, applied.Multiplier)
		require.Equal(t, "平时", applied.Label)
		require.Nil(t, applied.Rule)
	})

	t.Run("invalid persisted default safely falls back to one", func(t *testing.T) {
		invalid := math.NaN()
		p := &TimePricing{
			Enabled:           true,
			Timezone:          "Asia/Shanghai",
			DefaultMultiplier: &invalid,
		}
		require.Equal(t, 1.0, p.MultiplierAt(atShanghai(t, 12, 0)).Multiplier)
	})
}

func TestTimePricingIsActiveRequiresValidTimezone(t *testing.T) {
	require.False(t, (&TimePricing{Enabled: true, Timezone: ""}).IsActive())
	require.False(t, (&TimePricing{Enabled: true, Timezone: "Mars/Olympus"}).IsActive())
	require.True(t, (&TimePricing{Enabled: true, Timezone: "Asia/Shanghai"}).IsActive())
}

func TestTimePricingMultiplierAtSkipsInvalidPersistedMultiplier(t *testing.T) {
	p := &TimePricing{
		Enabled:  true,
		Timezone: "Asia/Shanghai",
		Rules: []TimePricingRule{
			{StartTime: "09:00", EndTime: "12:00", Multiplier: -1},
		},
	}

	applied := p.MultiplierAt(atShanghai(t, 9, 30))
	require.Equal(t, 1.0, applied.Multiplier)
	require.Nil(t, applied.Rule)
}

func TestChannelModelPricingCloneDeepCopiesTimeRules(t *testing.T) {
	defaultMultiplier := 0.75
	original := ChannelModelPricing{
		TimePricing: &TimePricing{
			Enabled:           true,
			Timezone:          "Asia/Shanghai",
			DefaultLabel:      "regular",
			DefaultMultiplier: &defaultMultiplier,
			Rules:             []TimePricingRule{{Label: "busy", StartTime: "09:00", EndTime: "12:00", Multiplier: 2}},
		},
	}
	cp := original.Clone()
	*cp.TimePricing.DefaultMultiplier = 0.25
	cp.TimePricing.Rules[0].Multiplier = 3
	require.Equal(t, 0.75, *original.TimePricing.DefaultMultiplier)
	require.Equal(t, 2.0, original.TimePricing.Rules[0].Multiplier)
}

func TestCalculateCostUnified_TimePricingReplacesGroupUserAndLegacyPeakRates(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	resolver := NewModelPricingResolver(nil, bs)
	group := &Group{
		ID:                 1,
		SubscriptionType:   SubscriptionTypeSubscription,
		PeakRateEnabled:    true,
		PeakStart:          "09:00",
		PeakEnd:            "12:00",
		PeakRateMultiplier: 3,
	}
	legacyMultiplied := 6.0

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Group:          group,
		Tokens:         UsageTokens{InputTokens: 1000},
		RateMultiplier: legacyMultiplied,
		PricingAt:      atShanghai(t, 9, 30),
		Resolver:       resolver,
		Resolved: &ResolvedPricing{
			Mode:        BillingModeToken,
			BasePricing: bs.fallbackPrices["claude-sonnet-4"],
			Source:      PricingSourceGroup,
			TimePricing: &TimePricing{
				Enabled:      true,
				Timezone:     "Asia/Shanghai",
				DefaultLabel: "regular",
				Rules:        []TimePricingRule{{Label: "busy", StartTime: "09:00", EndTime: "12:00", Multiplier: 4}},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 4.0, cost.EffectiveRateMultiplier)
	require.Equal(t, 4.0, cost.TimePricingMultiplier)
	require.Equal(t, "busy", cost.TimePricingLabel)
	require.InDelta(t, 1000*3e-6*4, cost.ActualCost, 1e-12)
}

func TestCalculateCostUnified_DefaultTimeMultiplierWorksWithoutExplicitRules(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	resolver := NewModelPricingResolver(nil, bs)
	defaultMultiplier := 0.5

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         UsageTokens{InputTokens: 1000},
		RateMultiplier: 6,
		PricingAt:      atShanghai(t, 13, 0),
		Resolver:       resolver,
		Resolved: &ResolvedPricing{
			Mode:        BillingModeToken,
			BasePricing: bs.fallbackPrices["claude-sonnet-4"],
			Source:      PricingSourceGroup,
			TimePricing: &TimePricing{
				Enabled:           true,
				Timezone:          "Asia/Shanghai",
				DefaultLabel:      "平时",
				DefaultMultiplier: &defaultMultiplier,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0.5, cost.EffectiveRateMultiplier)
	require.Equal(t, 0.5, cost.TimePricingMultiplier)
	require.Equal(t, "平时", cost.TimePricingLabel)
	require.Nil(t, cost.TimePricingRule)
	require.InDelta(t, 1000*3e-6*0.5, cost.ActualCost, 1e-12)
}

func TestCalculateCostUnified_LegacyPeakFallbackWhenNoModelSchedule(t *testing.T) {
	bs := newTestBillingServiceForResolver()
	resolver := NewModelPricingResolver(nil, bs)

	cost, err := bs.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		Tokens:         UsageTokens{InputTokens: 1000},
		RateMultiplier: 6,
		PricingAt:      atShanghai(t, 9, 30),
		Resolver:       resolver,
		Resolved: &ResolvedPricing{
			Mode:        BillingModeToken,
			BasePricing: bs.fallbackPrices["claude-sonnet-4"],
			Source:      PricingSourceFallback,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 6.0, cost.EffectiveRateMultiplier)
	require.InDelta(t, 1000*3e-6*6, cost.ActualCost, 1e-12)
}

func TestUsageScheduleMetaWithTimePricingSnapshotsDecision(t *testing.T) {
	pricingAt := atShanghai(t, 9, 30)
	original := &UsageScheduleMeta{Provider: "openai", ShadowReasonCodes: []string{"kept"}}
	cost := &CostBreakdown{
		TimePricingMultiplier: 2,
		TimePricingTimezone:   "Asia/Shanghai",
		TimePricingLabel:      "高峰",
		TimePricingRule: &TimePricingRule{
			Label:      "morning",
			StartTime:  "09:00",
			EndTime:    "12:00",
			Multiplier: 2,
		},
	}

	got := UsageScheduleMetaWithTimePricing(original, pricingAt, cost)
	require.NotSame(t, original, got)
	require.Equal(t, "openai", got.Provider)
	require.NotNil(t, got.PricingAt)
	require.True(t, got.PricingAt.Equal(pricingAt))
	require.Equal(t, "UTC", got.PricingAt.Location().String())
	require.NotNil(t, got.TimePricingMultiplier)
	require.Equal(t, 2.0, *got.TimePricingMultiplier)
	require.Equal(t, "Asia/Shanghai", got.TimePricingTimezone)
	require.Equal(t, "高峰", got.TimePricingLabel)
	require.Equal(t, "morning", got.TimePricingRule.Label)

	got.ShadowReasonCodes[0] = "changed"
	got.TimePricingRule.Label = "changed"
	require.Equal(t, "kept", original.ShadowReasonCodes[0])
	require.Equal(t, "morning", cost.TimePricingRule.Label)
}
