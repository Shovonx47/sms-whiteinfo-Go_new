package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v3"

	"smswhiteinfo/superman-api/internal/config"
)

func TestValidateSenderID(t *testing.T) {
	validMasking := []string{"WHITEINFO", "Brand 1", "A_B-C.2", "SMS"}
	for _, id := range validMasking {
		if msg := validateSenderID(id, "alphanumeric"); msg != "" {
			t.Errorf("expected masking sender %q to be valid, got %q", id, msg)
		}
	}

	invalidMasking := []string{"AB", "123456", "ABCDEFGHIJKL", "BAD@ID"}
	for _, id := range invalidMasking {
		if msg := validateSenderID(id, "alphanumeric"); msg == "" {
			t.Errorf("expected masking sender %q to be rejected", id)
		}
	}

	validNumeric := []string{"16216", "8809644357020", "+8801712345678"}
	for _, id := range validNumeric {
		if msg := validateSenderID(id, "numeric"); msg != "" {
			t.Errorf("expected non-masking sender %q to be valid, got %q", id, msg)
		}
	}

	invalidNumeric := []string{"123", "WHITEINFO", "88017-1234"}
	for _, id := range invalidNumeric {
		if msg := validateSenderID(id, "numeric"); msg == "" {
			t.Errorf("expected non-masking sender %q to be rejected", id)
		}
	}
}

func TestNormalizeSenderType(t *testing.T) {
	cases := map[string]string{
		"Masking":     "alphanumeric",
		"masking":     "alphanumeric",
		"Brand ID":    "alphanumeric",
		"Non-masking": "numeric",
		"non_masking": "numeric",
		"non masking": "numeric",
		"number":      "numeric",
		"short-code":  "shortcode",
		"":            "alphanumeric",
	}
	for input, want := range cases {
		if got := normalizeSenderType(input); got != want {
			t.Errorf("normalizeSenderType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeRoutesAndMissingOperators(t *testing.T) {
	clean := normalizeRoutes(map[string]int64{
		" GP ":    10,
		"bl":      0,
		"unknown": 22,
		"robi":    14,
	})
	if len(clean) != 2 || clean["gp"] != 10 || clean["robi"] != 14 {
		t.Fatalf("unexpected normalized routes: %#v", clean)
	}

	missing := missingOperators(clean)
	wantMissing := []string{"Banglalink", "Teletalk", "Airtel"}
	if len(missing) != len(wantMissing) {
		t.Fatalf("missingOperators = %v, want %v", missing, wantMissing)
	}
	for i := range wantMissing {
		if missing[i] != wantMissing[i] {
			t.Fatalf("missing[%d] = %q, want %q", i, missing[i], wantMissing[i])
		}
	}
}

func openScratchSupermanServer(t *testing.T) (*Server, *fiber.App, string, func()) {
	t.Helper()

	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/sms_panel_scratch?parseTime=true")
	if err != nil {
		t.Skipf("scratch DB unavailable: %v", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		t.Skipf("scratch DB not reachable: %v", err)
	}

	// Ensure minimal schema tables exist if running on an empty scratch DB.
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(191) NOT NULL DEFAULT '',
			email VARCHAR(191) NULL,
			phone VARCHAR(32) NULL,
			password VARCHAR(191) NOT NULL DEFAULT '',
			type VARCHAR(32) NOT NULL DEFAULT 'customer',
			status VARCHAR(32) NOT NULL DEFAULT 'active',
			balance DECIMAL(15,4) NOT NULL DEFAULT 0,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL DEFAULT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS gateways (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			driver VARCHAR(64) NOT NULL DEFAULT 'log',
			credentials TEXT NOT NULL,
			is_enabled TINYINT(1) NOT NULL DEFAULT 1,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL DEFAULT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS senders (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT UNSIGNED NULL,
			requested_by BIGINT UNSIGNED NULL,
			gateway_id BIGINT UNSIGNED NULL,
			sender_id VARCHAR(100) NOT NULL,
			type VARCHAR(64) NOT NULL DEFAULT 'alphanumeric',
			approval_status VARCHAR(32) NOT NULL DEFAULT 'approved',
			is_enabled TINYINT(1) NOT NULL DEFAULT 1,
			reviewed_by BIGINT UNSIGNED NULL,
			reviewed_at TIMESTAMP NULL DEFAULT NULL,
			rejection_reason TEXT NULL,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP NULL DEFAULT NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS sender_user (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			sender_id BIGINT UNSIGNED NOT NULL,
			user_id BIGINT UNSIGNED NOT NULL,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uniq_sender_user (sender_id, user_id)
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS messages (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT UNSIGNED NOT NULL,
			sender_id BIGINT UNSIGNED NULL
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT UNSIGNED NOT NULL,
			transactionable_type VARCHAR(191) NULL,
			transactionable_id BIGINT UNSIGNED NULL,
			amount DECIMAL(15,4) NOT NULL DEFAULT 0,
			direction VARCHAR(16) NOT NULL DEFAULT 'debit',
			balance_before DECIMAL(15,4) NOT NULL DEFAULT 0,
			balance_after DECIMAL(15,4) NOT NULL DEFAULT 0,
			description TEXT NULL,
			created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS cache (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			value MEDIUMTEXT NOT NULL,
			expiration INT NOT NULL
		) ENGINE=InnoDB`,
	}
	for _, ddl := range ddls {
		_, _ = db.Exec(ddl)
	}

	cfg := config.Config{
		JWTSecret:        []byte("test-secret-key-for-superman-httpapi-12345"),
		FrontendOrigin:   "http://localhost:5173",
		TokenTTL:         time.Hour,
		RememberTokenTTL: time.Hour * 24,
	}
	srv := New(cfg, db)
	token, err := srv.tokens.Issue(1, "Test Superman", "superman@test.local", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	return srv, srv.App(), token, func() { _ = db.Close() }
}

func doJSON(t *testing.T, app *fiber.App, method, path, token string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestSenderRoutesLifecycleAndImport(t *testing.T) {
	srv, app, token, cleanup := openScratchSupermanServer(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	var createdGateways []int64
	var createdCustomers []int64
	var createdSenders []int64
	defer func() {
		for _, sid := range createdSenders {
			_, _ = srv.db.Exec("DELETE FROM sender_operator_gateways WHERE sender_id = ?", sid)
			_, _ = srv.db.Exec("DELETE FROM sender_user WHERE sender_id = ?", sid)
			_, _ = srv.db.Exec("DELETE FROM senders WHERE id = ?", sid)
		}
		for _, gid := range createdGateways {
			_, _ = srv.db.Exec("DELETE FROM gateways WHERE id = ?", gid)
		}
		for _, cid := range createdCustomers {
			_, _ = srv.db.Exec("DELETE FROM users WHERE id = ?", cid)
		}
	}()

	gwNames := []string{
		fmt.Sprintf("GP-GW-%d", suffix),
		fmt.Sprintf("BL-GW-%d", suffix),
		fmt.Sprintf("ICON-GW-%d", suffix),
	}
	for _, name := range gwNames {
		res, err := srv.db.Exec("INSERT INTO gateways (name, driver, credentials, is_enabled) VALUES (?, 'log', '{}', 1)", name)
		if err != nil {
			t.Fatalf("insert gateway: %v", err)
		}
		id, _ := res.LastInsertId()
		createdGateways = append(createdGateways, id)
	}

	custEmail := fmt.Sprintf("sender_cust_%d@test.local", suffix)
	uRes, err := srv.db.Exec(
		"INSERT INTO users (name, email, phone, password, type, status) VALUES (?, ?, ?, 'x', 'customer', 'active')",
		fmt.Sprintf("Sender Cust %d", suffix), custEmail, fmt.Sprintf("+88017%08d", suffix),
	)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	custID, _ := uRes.LastInsertId()
	createdCustomers = append(createdCustomers, custID)

	// 1. Masking sender missing an operator route must fail with 422.
	resp, _ := doJSON(t, app, "POST", "/api/v1/superman/senders", token, map[string]any{
		"sender_id": fmt.Sprintf("MK%d", suffix),
		"type":      "masking",
		"routes":    map[string]int64{"gp": createdGateways[0]},
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 when missing operator routes, got %d", resp.StatusCode)
	}

	// 2. Create masking sender with all 5 operator routes + customer assignment.
	maskCode := fmt.Sprintf("MK%d", suffix)
	resp, body := doJSON(t, app, "POST", "/api/v1/superman/senders", token, map[string]any{
		"sender_id":  maskCode,
		"type":       "masking",
		"is_enabled": true,
		"user_ids":   []int64{custID},
		"routes": map[string]int64{
			"gp":     createdGateways[0],
			"bl":     createdGateways[1],
			"tt":     createdGateways[2],
			"airtel": createdGateways[0],
			"robi":   createdGateways[1],
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create masking sender status = %d, body = %#v", resp.StatusCode, body)
	}
	dataMap := body["data"].(map[string]any)
	maskID := int64(dataMap["id"].(float64))
	createdSenders = append(createdSenders, maskID)

	// 3. Verify GET /senders/:id returns all 5 routes in stable order + assigned user.
	resp, detail := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/senders/%d", maskID), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get sender detail status = %d", resp.StatusCode)
	}
	d := detail["data"].(map[string]any)
	routesList := d["routes"].([]any)
	if len(routesList) != 5 {
		t.Fatalf("expected 5 routes, got %d", len(routesList))
	}
	if routesList[0].(map[string]any)["operator_key"] != "gp" || routesList[4].(map[string]any)["operator_key"] != "robi" {
		t.Fatalf("unexpected route ordering: %#v", routesList)
	}

	// 4. Update sender to non-masking -> should clear per-operator routes.
	resp, _ = doJSON(t, app, "PUT", fmt.Sprintf("/api/v1/superman/senders/%d", maskID), token, map[string]any{
		"sender_id":  fmt.Sprintf("88096%08d", suffix),
		"type":       "non-masking",
		"gateway_id": createdGateways[2],
		"user_ids":   []int64{},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("update sender to non-masking status = %d", resp.StatusCode)
	}
	resp, detail = doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/senders/%d", maskID), token, nil)
	if len(detail["data"].(map[string]any)["routes"].([]any)) != 0 {
		t.Fatalf("expected routes cleared after switching to non-masking")
	}

	// 5. CSV Import endpoint with mixed masking + non-masking + invalid row + customer column.
	impAlpha := fmt.Sprintf("IMP%d", suffix)
	impNum := fmt.Sprintf("8809644%06d", suffix)
	resp, impBody := doJSON(t, app, "POST", "/api/v1/superman/senders/import", token, map[string]any{
		"default_gateway_id": createdGateways[0],
		"default_type":       "masking",
		"rows": []map[string]any{
			{"sender_id": impAlpha, "type": "masking", "gateway": "", "customer": custEmail},
			{"sender_id": impNum, "type": "non-masking", "gateway": gwNames[2]},
			{"sender_id": "12345", "type": "masking", "gateway": ""},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import status = %d, body = %#v", resp.StatusCode, impBody)
	}
	impData := impBody["data"].(map[string]any)
	if int(impData["created"].(float64)) != 2 || int(impData["failed"].(float64)) != 1 {
		t.Fatalf("unexpected import counts: %#v", impData)
	}

	// Track imported sender IDs for cleanup and test bulk-delete.
	rows, _ := srv.db.Query("SELECT id FROM senders WHERE sender_id IN (?, ?) AND deleted_at IS NULL", impAlpha, impNum)
	var bulkIDs []int64
	for rows.Next() {
		var id int64
		_ = rows.Scan(&id)
		bulkIDs = append(bulkIDs, id)
		createdSenders = append(createdSenders, id)
	}
	rows.Close()
	if len(bulkIDs) != 2 {
		t.Fatalf("expected 2 imported senders in DB, got %d", len(bulkIDs))
	}

	resp, _ = doJSON(t, app, "POST", "/api/v1/superman/senders/bulk-delete", token, map[string]any{"ids": bulkIDs})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bulk-delete status = %d", resp.StatusCode)
	}
	var remaining int
	_ = srv.db.QueryRow("SELECT COUNT(*) FROM senders WHERE sender_id IN (?, ?) AND deleted_at IS NULL", impAlpha, impNum).Scan(&remaining)
	if remaining != 0 {
		t.Fatalf("expected bulk-deleted senders to have deleted_at set, remaining=%d", remaining)
	}

	// 6. Approve pending masking sender with all 5 operator routes.
	pendCode := fmt.Sprintf("PND%d", suffix)
	pRes, err := srv.db.Exec(
		"INSERT INTO senders (sender_id, type, approval_status, is_enabled, created_at, updated_at) VALUES (?, 'alphanumeric', 'pending', 0, NOW(), NOW())",
		pendCode,
	)
	if err != nil {
		t.Fatalf("insert pending sender: %v", err)
	}
	pendID, _ := pRes.LastInsertId()
	createdSenders = append(createdSenders, pendID)

	resp, appBody := doJSON(t, app, "POST", fmt.Sprintf("/api/v1/superman/senders/%d/approve", pendID), token, map[string]any{
		"routes": map[string]int64{
			"gp":     createdGateways[0],
			"bl":     createdGateways[1],
			"tt":     createdGateways[2],
			"airtel": createdGateways[0],
			"robi":   createdGateways[1],
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("approve pending sender status = %d, body = %#v", resp.StatusCode, appBody)
	}

	resp, pendDetail := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/senders/%d", pendID), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("detail after approve status = %d", resp.StatusCode)
	}
	pd := pendDetail["data"].(map[string]any)
	if pd["approval_status"] != "approved" || pd["is_enabled"] != true {
		t.Fatalf("expected approved & enabled sender, got approval=%v enabled=%v", pd["approval_status"], pd["is_enabled"])
	}
	if len(pd["routes"].([]any)) != 5 {
		t.Fatalf("expected 5 operator routes after approval, got %d", len(pd["routes"].([]any)))
	}
}

func TestBillingReportsPaginationAndExport(t *testing.T) {
	srv, app, token, cleanup := openScratchSupermanServer(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	uRes, err := srv.db.Exec(
		"INSERT INTO users (name, email, phone, password, type, status) VALUES (?, ?, ?, 'x', 'customer', 'active')",
		fmt.Sprintf("Report User %d", suffix), fmt.Sprintf("rep_%d@test.local", suffix), fmt.Sprintf("+88018%08d", suffix),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := uRes.LastInsertId()
	var txIDs []int64
	defer func() {
		for _, id := range txIDs {
			_, _ = srv.db.Exec("DELETE FROM transactions WHERE id = ?", id)
		}
		_, _ = srv.db.Exec("DELETE FROM users WHERE id = ?", uid)
	}()

	for i := 0; i < 15; i++ {
		dir := "debit"
		if i%3 == 0 {
			dir = "credit"
		}
		res, err := srv.db.Exec(
			"INSERT INTO transactions (user_id, transactionable_type, amount, direction, balance_before, balance_after, description, created_at) VALUES (?, 'App\\\\Models\\\\Batch', 10.0, ?, 100.0, 90.0, ?, NOW())",
			uid, dir, fmt.Sprintf("TX_TEST_%d_%02d", suffix, i),
		)
		if err != nil {
			t.Fatalf("insert tx: %v", err)
		}
		id, _ := res.LastInsertId()
		txIDs = append(txIDs, id)
	}

	// Page 1 with per_page=10
	resp, p1 := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/billing/reports?search=TX_TEST_%d&page=1&per_page=10", suffix), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("billing reports page 1 status = %d", resp.StatusCode)
	}
	if len(p1["data"].([]any)) != 10 || int(p1["meta"].(map[string]any)["total"].(float64)) != 15 {
		t.Fatalf("unexpected page 1 payload: %#v", p1["meta"])
	}

	// Page 2 with per_page=10 -> 5 items
	resp, p2 := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/billing/reports?search=TX_TEST_%d&page=2&per_page=10", suffix), token, nil)
	if resp.StatusCode != http.StatusOK || len(p2["data"].([]any)) != 5 {
		t.Fatalf("expected 5 items on page 2, status=%d", resp.StatusCode)
	}

	// Export=1 returns all 15
	resp, exp := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/billing/reports?search=TX_TEST_%d&export=1", suffix), token, nil)
	if resp.StatusCode != http.StatusOK || len(exp["data"].([]any)) != 15 {
		t.Fatalf("expected 15 items with export=1, status=%d", resp.StatusCode)
	}

	// Search by #ID
	resp, byID := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/billing/reports?search=%%23%d", txIDs[0]), token, nil)
	if resp.StatusCode != http.StatusOK || len(byID["data"].([]any)) != 1 {
		t.Fatalf("expected 1 item searching by #%d, got %#v", txIDs[0], byID)
	}
}

func TestMessagesSenderCategoryFilterAndExport(t *testing.T) {
	srv, app, token, cleanup := openScratchSupermanServer(t)
	defer cleanup()

	suffix := time.Now().UnixNano() % 100000
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS gateway_id BIGINT UNSIGNED NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS `to` VARCHAR(32) NOT NULL DEFAULT ''")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS body TEXT NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS status VARCHAR(32) NOT NULL DEFAULT 'delivered'")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS cost DECIMAL(10,4) NOT NULL DEFAULT 0")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS operator_cost DECIMAL(10,4) NOT NULL DEFAULT 0")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS primary_dipping_cost DECIMAL(10,4) NOT NULL DEFAULT 0")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'web'")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS created_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS parts INT NOT NULL DEFAULT 1")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS sent_at DATETIME NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS delivered_at DATETIME NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS failed_reason TEXT NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS tariff_name VARCHAR(100) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS source_ip VARCHAR(64) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS operator_name VARCHAR(64) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS gateway_message_id VARCHAR(255) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS batch_id BIGINT UNSIGNED NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS bill_msisdn VARCHAR(32) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS sms_type VARCHAR(32) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS channel VARCHAR(32) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS request_id CHAR(36) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS server_txn_id VARCHAR(128) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS server_response_code VARCHAR(32) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS mno_txn_id VARCHAR(128) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS mno_response_code VARCHAR(32) NULL")
	_, _ = srv.db.Exec("ALTER TABLE messages ADD COLUMN IF NOT EXISTS mno_response_message TEXT NULL")

	uRes, err := srv.db.Exec(
		"INSERT INTO users (name, email, phone, password, type, status) VALUES (?, ?, ?, 'x', 'customer', 'active')",
		fmt.Sprintf("Msg Report User %d", suffix), fmt.Sprintf("msgrep_%d@test.local", suffix), fmt.Sprintf("+88019%08d", suffix),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	uid, _ := uRes.LastInsertId()

	gwRes, err := srv.db.Exec("INSERT INTO gateways (name, driver, credentials, is_enabled) VALUES (?, 'log', '{}', 1)", fmt.Sprintf("GW-MSG-%d", suffix))
	if err != nil {
		t.Fatalf("insert gateway: %v", err)
	}
	gid, _ := gwRes.LastInsertId()

	sMaskRes, err := srv.db.Exec("INSERT INTO senders (gateway_id, sender_id, type, approval_status, is_enabled) VALUES (?, ?, 'alphanumeric', 'approved', 1)", gid, fmt.Sprintf("MS%d", suffix))
	if err != nil {
		t.Fatalf("insert masking sender: %v", err)
	}
	sMaskID, _ := sMaskRes.LastInsertId()

	sNumRes, err := srv.db.Exec("INSERT INTO senders (gateway_id, sender_id, type, approval_status, is_enabled) VALUES (?, ?, 'numeric', 'approved', 1)", gid, fmt.Sprintf("8809%06d", suffix))
	if err != nil {
		t.Fatalf("insert numeric sender: %v", err)
	}
	sNumID, _ := sNumRes.LastInsertId()

	m1Res, err := srv.db.Exec("INSERT INTO messages (user_id, sender_id, gateway_id, `to`, body, status, cost, operator_cost, primary_dipping_cost, source, created_at) VALUES (?, ?, ?, '8801700000001', ?, 'delivered', 0.5, 0.2, 0, 'web', NOW())", uid, sMaskID, gid, fmt.Sprintf("MASK_BODY_%d", suffix))
	if err != nil {
		t.Fatalf("insert masking msg: %v", err)
	}
	m1ID, _ := m1Res.LastInsertId()

	m2Res, err := srv.db.Exec("INSERT INTO messages (user_id, sender_id, gateway_id, `to`, body, status, cost, operator_cost, primary_dipping_cost, source, created_at) VALUES (?, ?, ?, '8801700000002', ?, 'delivered', 0.25, 0.1, 0, 'web', NOW())", uid, sNumID, gid, fmt.Sprintf("NUM_BODY_%d", suffix))
	if err != nil {
		t.Fatalf("insert non-masking msg: %v", err)
	}
	m2ID, _ := m2Res.LastInsertId()

	defer func() {
		_, _ = srv.db.Exec("DELETE FROM messages WHERE id IN (?, ?)", m1ID, m2ID)
		_, _ = srv.db.Exec("DELETE FROM senders WHERE id IN (?, ?)", sMaskID, sNumID)
		_, _ = srv.db.Exec("DELETE FROM gateways WHERE id = ?", gid)
		_, _ = srv.db.Exec("DELETE FROM users WHERE id = ?", uid)
	}()

	resp, maskBody := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/messages?sender_category=masking&search=%d", suffix), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("masking messages status=%d", resp.StatusCode)
	}
	maskRows := maskBody["data"].([]any)
	if len(maskRows) != 1 || maskRows[0].(map[string]any)["gateway"] != fmt.Sprintf("GW-MSG-%d", suffix) {
		t.Fatalf("expected 1 masking row with gateway populated, got %#v", maskRows)
	}

	resp, numBody := doJSON(t, app, "GET", fmt.Sprintf("/api/v1/superman/messages?sender_category=non_masking&search=%d", suffix), token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("non-masking messages status=%d", resp.StatusCode)
	}
	numRows := numBody["data"].([]any)
	if len(numRows) != 1 || numRows[0].(map[string]any)["recipient"] != "8801700000002" {
		t.Fatalf("expected 1 non-masking row, got %#v", numRows)
	}
}
