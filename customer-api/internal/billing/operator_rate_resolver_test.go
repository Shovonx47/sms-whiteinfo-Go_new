package billing

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
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

func TestForUserActiveTariffPlanQueryParity(t *testing.T) {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/sms_panel_scratch?parseTime=true")
	if err != nil {
		t.Skip("Scratch DB not available")
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Skip("Scratch DB not pingable")
	}

	ctx := context.Background()
	now := time.Now().UnixNano()
	uRes, err := db.ExecContext(ctx, `
		INSERT INTO users (name, email, phone, password, balance, type, status, created_at, updated_at)
		VALUES ('Tariff Parity User', ?, ?, 'hash', 100.00, 'customer', 'approved', NOW(), NOW())
	`, fmt.Sprintf("tariff_%d@test.local", now), fmt.Sprintf("+88019%08d", now%100000000))
	if err != nil {
		t.Fatalf("failed creating user: %v", err)
	}
	userID, _ := uRes.LastInsertId()

	var opID int64
	if err := db.QueryRowContext(ctx, "SELECT id FROM operators WHERE is_active = 1 LIMIT 1").Scan(&opID); err != nil {
		t.Fatalf("expected at least 1 active operator in scratch DB: %v", err)
	}

	slugPrefix := fmt.Sprintf("parity-%d", now)
	pSup, err := db.ExecContext(ctx, "INSERT INTO tariff_plans (name, slug, type, status, currency, created_at, updated_at) VALUES ('Superseded Plan', ?, 'standard', 'active', 'BDT', NOW(), NOW())", slugPrefix+"-sup")
	if err != nil {
		t.Fatalf("insert plan sup: %v", err)
	}
	pSupID, _ := pSup.LastInsertId()

	pDraft, err := db.ExecContext(ctx, "INSERT INTO tariff_plans (name, slug, type, status, currency, created_at, updated_at) VALUES ('Draft Plan', ?, 'standard', 'draft', 'BDT', NOW(), NOW())", slugPrefix+"-draft")
	if err != nil {
		t.Fatalf("insert plan draft: %v", err)
	}
	pDraftID, _ := pDraft.LastInsertId()

	pAct, err := db.ExecContext(ctx, "INSERT INTO tariff_plans (name, slug, type, status, currency, created_at, updated_at) VALUES ('Active Parity Plan', ?, 'standard', 'active', 'BDT', NOW(), NOW())", slugPrefix+"-act")
	if err != nil {
		t.Fatalf("insert plan act: %v", err)
	}
	pActID, _ := pAct.LastInsertId()

	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM tariff_plan_assignments WHERE user_id = ?", userID)
		_, _ = db.ExecContext(ctx, "DELETE FROM tariff_rates WHERE tariff_plan_id IN (?, ?, ?)", pSupID, pDraftID, pActID)
		_, _ = db.ExecContext(ctx, "DELETE FROM tariff_plans WHERE id IN (?, ?, ?)", pSupID, pDraftID, pActID)
		_, _ = db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID)
	}()

	_, _ = db.ExecContext(ctx, "INSERT INTO tariff_rates (tariff_plan_id, operator_id, masking_price, non_masking_price, created_at, updated_at) VALUES (?, ?, 0.9900, 0.8800, NOW(), NOW())", pSupID, opID)
	_, _ = db.ExecContext(ctx, "INSERT INTO tariff_rates (tariff_plan_id, operator_id, masking_price, non_masking_price, created_at, updated_at) VALUES (?, ?, 0.7700, 0.6600, NOW(), NOW())", pDraftID, opID)
	_, _ = db.ExecContext(ctx, "INSERT INTO tariff_rates (tariff_plan_id, operator_id, masking_price, non_masking_price, created_at, updated_at) VALUES (?, ?, 0.5100, 0.2400, NOW(), NOW())", pActID, opID)

	// Assign superseded today, draft today, active effective tomorrow, and active effective yesterday
	_, err = db.ExecContext(ctx, `
		INSERT INTO tariff_plan_assignments (user_id, tariff_plan_id, effective_from, status, created_at, updated_at)
		VALUES
			(?, ?, CURDATE(), 'superseded', NOW(), NOW()),
			(?, ?, CURDATE(), 'active', NOW(), NOW()),
			(?, ?, DATE_ADD(CURDATE(), INTERVAL 1 DAY), 'active', NOW(), NOW()),
			(?, ?, DATE_SUB(CURDATE(), INTERVAL 1 DAY), 'active', NOW(), NOW())
	`, userID, pSupID, userID, pDraftID, userID, pSupID, userID, pActID)
	if err != nil {
		t.Fatalf("insert assignments: %v", err)
	}

	resolver, err := ForUser(ctx, db, userID)
	if err != nil {
		t.Fatalf("ForUser failed: %v", err)
	}
	if resolver.PlanName() != "Active Parity Plan" {
		t.Fatalf("expected active plan 'Active Parity Plan', got %q", resolver.PlanName())
	}

	rates := resolver.EffectiveOperatorRates()
	var matched *ResolvedOperatorRate
	for i := range rates {
		if rates[i].Operator.ID == opID {
			matched = &rates[i]
			break
		}
	}
	if matched == nil || matched.NonMaskingRate == nil || *matched.NonMaskingRate != 0.24 || matched.MaskingRate == nil || *matched.MaskingRate != 0.51 {
		t.Fatalf("expected resolved rates 0.51 / 0.24, got %+v", matched)
	}
}
