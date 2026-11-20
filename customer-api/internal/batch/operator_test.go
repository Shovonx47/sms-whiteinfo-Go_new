package batch

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestOperatorKeyForPhone(t *testing.T) {
	cases := map[string]string{
		"8801712345678":  "gp",
		"+8801312345678": "gp",
		"01412345678":    "bl",
		"8801912345678":  "bl",
		"8801512345678":  "tt",
		"8801612345678":  "airtel",
		"8801812345678":  "robi",
		"8801112345678":  "",
	}
	for phone, want := range cases {
		if got := OperatorKeyForPhone(phone); got != want {
			t.Errorf("OperatorKeyForPhone(%q) = %q, want %q", phone, got, want)
		}
	}
}

func TestBatchProcessor_MaskingPerOperatorGatewayRouting(t *testing.T) {
	db, userID, _, tmpDir, cleanup := setupTestBatchEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	_, _ = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS sender_operator_gateways (
			sender_id BIGINT NOT NULL,
			operator_key VARCHAR(20) NOT NULL,
			gateway_id BIGINT NOT NULL,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			PRIMARY KEY (sender_id, operator_key)
		) ENGINE=InnoDB
	`)

	now := time.Now().UnixNano()
	g1Res, err := db.ExecContext(ctx, "INSERT INTO gateways (name, driver, credentials, is_enabled) VALUES (?, 'log', '{}', 1)", fmt.Sprintf("GP-ROUTE-%d", now))
	if err != nil {
		t.Fatalf("insert gw1: %v", err)
	}
	gw1, _ := g1Res.LastInsertId()
	g2Res, err := db.ExecContext(ctx, "INSERT INTO gateways (name, driver, credentials, is_enabled) VALUES (?, 'log', '{}', 1)", fmt.Sprintf("ROBI-ROUTE-%d", now))
	if err != nil {
		t.Fatalf("insert gw2: %v", err)
	}
	gw2, _ := g2Res.LastInsertId()

	sRes, err := db.ExecContext(ctx, `
		INSERT INTO senders (gateway_id, sender_id, type, approval_status, is_enabled, created_at, updated_at)
		VALUES (?, ?, 'alphanumeric', 'approved', 1, NOW(), NOW())
	`, gw1, fmt.Sprintf("OP%d", now%100000))
	if err != nil {
		t.Fatalf("insert masking sender: %v", err)
	}
	senderID, _ := sRes.LastInsertId()

	_, _ = db.ExecContext(ctx, "INSERT INTO sender_operator_gateways (sender_id, operator_key, gateway_id) VALUES (?, 'gp', ?), (?, 'robi', ?)", senderID, gw1, senderID, gw2)

	defer func() {
		_, _ = db.ExecContext(ctx, "DELETE FROM sender_operator_gateways WHERE sender_id = ?", senderID)
		_, _ = db.ExecContext(ctx, "DELETE FROM senders WHERE id = ?", senderID)
		_, _ = db.ExecContext(ctx, "DELETE FROM gateways WHERE id IN (?, ?)", gw1, gw2)
	}()

	srcConfig, _ := json.Marshal(map[string]any{
		"recipients": []map[string]any{
			{"phone": "01711111111"},
			{"phone": "01811111111"},
		},
	})
	bRes, err := db.ExecContext(ctx, `
		INSERT INTO batches (user_id, sender_id, body, source_type, source_config, status, created_at, updated_at)
		VALUES (?, ?, 'Routing check', 'manual', ?, 'processing', NOW(), NOW())
	`, userID, senderID, string(srcConfig))
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	batchID, _ := bRes.LastInsertId()

	proc := NewBatchProcessor(db, tmpDir)
	if err := proc.ProcessBatch(ctx, batchID); err != nil {
		t.Fatalf("ProcessBatch failed: %v", err)
	}

	var gwForGP, gwForRobi int64
	if err := db.QueryRowContext(ctx, "SELECT gateway_id FROM messages WHERE batch_id = ? AND `to` LIKE '%1711111111'", batchID).Scan(&gwForGP); err != nil {
		t.Fatalf("query GP message: %v", err)
	}
	if err := db.QueryRowContext(ctx, "SELECT gateway_id FROM messages WHERE batch_id = ? AND `to` LIKE '%1811111111'", batchID).Scan(&gwForRobi); err != nil {
		t.Fatalf("query Robi message: %v", err)
	}
	if gwForGP != gw1 {
		t.Errorf("GP message got gateway_id=%d, want %d", gwForGP, gw1)
	}
	if gwForRobi != gw2 {
		t.Errorf("Robi message got gateway_id=%d, want %d", gwForRobi, gw2)
	}
}
