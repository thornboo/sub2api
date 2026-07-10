package service

import (
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func TestNormalizeUpstreamRechargeRecordInput_DefaultsToCNYUSD(t *testing.T) {
	values, err := normalizeUpstreamRechargeRecordInput(UpstreamRechargeRecordInput{
		PaidAmount:           7,
		ReceivedCreditAmount: 1,
	})
	if err != nil {
		t.Fatalf("normalizeUpstreamRechargeRecordInput() error = %v", err)
	}

	if values.PaidCurrency != upstreamRechargePaidCurrency {
		t.Fatalf("PaidCurrency = %q, want %q", values.PaidCurrency, upstreamRechargePaidCurrency)
	}
	if values.ReceivedCreditCurrency != upstreamRechargeCreditCurrency {
		t.Fatalf("ReceivedCreditCurrency = %q, want %q", values.ReceivedCreditCurrency, upstreamRechargeCreditCurrency)
	}
	if values.ReferenceFXRate != UpstreamRechargeDefaultReferenceFXRate {
		t.Fatalf("ReferenceFXRate = %v, want %v", values.ReferenceFXRate, UpstreamRechargeDefaultReferenceFXRate)
	}
	if values.EffectiveCNYPerUSD == nil || *values.EffectiveCNYPerUSD != 7 {
		t.Fatalf("EffectiveCNYPerUSD = %v, want 7", values.EffectiveCNYPerUSD)
	}
	if values.RechargeDiscount == nil || *values.RechargeDiscount != 1 {
		t.Fatalf("RechargeDiscount = %v, want 1", values.RechargeDiscount)
	}
}

func TestNormalizeUpstreamRechargeRecordInput_RejectsUnsupportedCurrency(t *testing.T) {
	_, err := normalizeUpstreamRechargeRecordInput(UpstreamRechargeRecordInput{
		PaidAmount:             1,
		PaidCurrency:           "USD",
		ReceivedCreditAmount:   1,
		ReceivedCreditCurrency: "USD",
	})
	if err == nil {
		t.Fatal("normalizeUpstreamRechargeRecordInput() error = nil, want currency error")
	}
	if got, want := infraerrors.Reason(err), "INVALID_UPSTREAM_RECHARGE_CURRENCY"; got != want {
		t.Fatalf("error reason = %q, want %q", got, want)
	}
}

func TestNormalizeUpstreamRechargeRecordInput_BonusDoesNotDefineUnitCost(t *testing.T) {
	values, err := normalizeUpstreamRechargeRecordInput(UpstreamRechargeRecordInput{
		Type:                 "bonus",
		PaidAmount:           7,
		ReceivedCreditAmount: 2,
		ReferenceFXRate:      7,
	})
	if err != nil {
		t.Fatalf("normalizeUpstreamRechargeRecordInput() error = %v", err)
	}
	if values.EffectiveCNYPerUSD != nil || values.RechargeDiscount != nil {
		t.Fatalf("bonus cost = (%v, %v), want nil unit cost and discount", values.EffectiveCNYPerUSD, values.RechargeDiscount)
	}
}

func TestNormalizeUpstreamCostBindingInput_DefaultsAndDeduplicatesFamilies(t *testing.T) {
	note := "  fast lane  "
	groupName := "  claude-sale  "
	values, err := normalizeUpstreamCostBindingInput(UpstreamCostBindingInput{
		AccountID:         1,
		CostPoolID:        2,
		UpstreamGroupName: &groupName,
		DefaultMultiplier: 0,
		ModelFamilyMultipliers: []UpstreamCostModelFamilyMultiplier{
			{Family: " Sonnet ", GroupMultiplier: 0.7, Note: &note},
			{Family: "sonnet", GroupMultiplier: 0.8},
			{Family: " ", GroupMultiplier: 1},
			{Family: "OPUS", GroupMultiplier: 1.2},
		},
	})
	if err != nil {
		t.Fatalf("normalizeUpstreamCostBindingInput() error = %v", err)
	}

	if values.DefaultMultiplier != 1 {
		t.Fatalf("DefaultMultiplier = %v, want 1", values.DefaultMultiplier)
	}
	if values.UpstreamGroupName == nil || *values.UpstreamGroupName != "claude-sale" {
		t.Fatalf("UpstreamGroupName = %v, want claude-sale", values.UpstreamGroupName)
	}
	if len(values.ModelFamilyMultipliers) != 2 {
		t.Fatalf("len(ModelFamilyMultipliers) = %d, want 2", len(values.ModelFamilyMultipliers))
	}
	if got, want := values.ModelFamilyMultipliers[0].Family, "sonnet"; got != want {
		t.Fatalf("first family = %q, want %q", got, want)
	}
	if got, want := values.ModelFamilyMultipliers[0].GroupMultiplier, 0.7; got != want {
		t.Fatalf("sonnet multiplier = %v, want %v", got, want)
	}
	if values.ModelFamilyMultipliers[0].Note == nil || *values.ModelFamilyMultipliers[0].Note != "fast lane" {
		t.Fatalf("sonnet note = %v, want fast lane", values.ModelFamilyMultipliers[0].Note)
	}
	if got, want := values.ModelFamilyMultipliers[1].Family, "opus"; got != want {
		t.Fatalf("second family = %q, want %q", got, want)
	}
}

func TestNormalizeUpstreamCostBindingInput_RejectsInvalidPoolAndFamilyMultiplier(t *testing.T) {
	_, err := normalizeUpstreamCostBindingInput(UpstreamCostBindingInput{
		AccountID:  1,
		CostPoolID: 0,
	})
	if err == nil {
		t.Fatal("normalizeUpstreamCostBindingInput() error = nil, want invalid pool error")
	}
	if got, want := infraerrors.Reason(err), "INVALID_UPSTREAM_COST_POOL_ID"; got != want {
		t.Fatalf("error reason = %q, want %q", got, want)
	}

	_, err = normalizeUpstreamCostBindingInput(UpstreamCostBindingInput{
		AccountID:         1,
		CostPoolID:        2,
		DefaultMultiplier: 1,
		ModelFamilyMultipliers: []UpstreamCostModelFamilyMultiplier{
			{Family: "haiku", GroupMultiplier: 0},
		},
	})
	if err == nil {
		t.Fatal("normalizeUpstreamCostBindingInput() error = nil, want invalid multiplier error")
	}
	if got, want := infraerrors.Reason(err), "INVALID_UPSTREAM_COST_MODEL_FAMILY_MULTIPLIER"; got != want {
		t.Fatalf("error reason = %q, want %q", got, want)
	}
}
