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
