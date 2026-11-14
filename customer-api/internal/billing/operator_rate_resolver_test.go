package billing

import (
	"database/sql"
	"testing"
)

func TestEffectiveOperatorRatesStrictPlanAndLegacyFallback(t *testing.T) {
	operators := map[int64]Operator{
		1: {
			ID:              1,
			Name:            "Grameenphone",
			CountryID:       1,
			CountryCode:     "880",
			Codes:           []string{"017", "013"},
			MaskingPrice:    sql.NullFloat64{Float64: 0.50, Valid: true},
			NonMaskingPrice: sql.NullFloat64{Float64: 0.30, Valid: true},
		},
		2: {
			ID:              2,
			Name:            "Banglalink",
			CountryID:       1,
			CountryCode:     "880",
			Codes:           []string{"019", "014"},
			MaskingPrice:    sql.NullFloat64{},
			NonMaskingPrice: sql.NullFloat64{Float64: 0.28, Valid: true},
		},
	}

	// 1. Active Tariff Plan: strictly respects plan rates (never falls back to operator default).
	planResolver := &OperatorRateResolver{
		operators: operators,
		overrides: map[int64]RatePair{
			1: {
				MaskingPrice:    sql.NullFloat64{Float64: 0.42, Valid: true},
				NonMaskingPrice: sql.NullFloat64{},
			},
		},
		hasActivePlan: true,
		planName:      "Enterprise",
	}

	planRates := planResolver.EffectiveOperatorRates()
	if len(planRates) != 2 {
		t.Fatalf("expected 2 operators, got %d", len(planRates))
	}
	if planRates[0].Operator.Name != "Banglalink" || planRates[0].MaskingRate != nil || planRates[0].NonMaskingRate != nil {
		t.Fatalf("expected Banglalink under active plan to have nil rates (no operator default fallback), got %+v", planRates[0])
	}
	if planRates[1].Operator.Name != "Grameenphone" || planRates[1].MaskingRate == nil || *planRates[1].MaskingRate != 0.42 || planRates[1].NonMaskingRate != nil {
		t.Fatalf("expected Grameenphone masking=0.42 nonMasking=nil, got %+v", planRates[1])
	}

	// 2. Legacy override / default: cascades override -> operator nullable price without inventing defaults.
	legacyResolver := &OperatorRateResolver{
		operators: operators,
		overrides: map[int64]RatePair{
			2: {
				MaskingPrice:    sql.NullFloat64{},
				NonMaskingPrice: sql.NullFloat64{Float64: 0.22, Valid: true},
			},
		},
		hasActivePlan: false,
	}

	legacyRates := legacyResolver.EffectiveOperatorRates()
	if legacyRates[0].MaskingRate != nil {
		t.Fatalf("expected Banglalink masking rate nil when both override and operator price are NULL")
	}
	if legacyRates[0].NonMaskingRate == nil || *legacyRates[0].NonMaskingRate != 0.22 || legacyRates[0].RateSource != "custom" {
		t.Fatalf("expected Banglalink non-masking custom=0.22, got %+v", legacyRates[0])
	}
	if legacyRates[1].MaskingRate == nil || *legacyRates[1].MaskingRate != 0.50 || legacyRates[1].RateSource != "default" {
		t.Fatalf("expected Grameenphone default masking=0.50, got %+v", legacyRates[1])
	}
}
