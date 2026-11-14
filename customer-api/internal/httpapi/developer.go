package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const customerMorphType = "App\\Models\\User"

func devTimePtr(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	s := v.Time.Format(time.RFC3339)
	return &s
}

func (server *Server) requireCustomerAPIAccess(c fiber.Ctx) (devUserData, bool, error) {
	claims := customer(c)
	var u devUserData
	err := server.db.QueryRowContext(c.Context(), `
		SELECT id, COALESCE(name, ''), COALESCE(api_access, 0)
		FROM users
		WHERE id = ? AND deleted_at IS NULL
	`, claims.UserID).Scan(&u.ID, &u.Name, &u.APIAccess)
	if err != nil {
		return u, false, databaseError(c, err)
	}
	if !u.APIAccess {
		return u, false, c.Status(http.StatusForbidden).JSON(fiber.Map{
			"message": "API access is not enabled for your account.",
		})
	}
	return u, true, nil
}

// ─── Access Tokens (API Keys) ────────────────────────────────────────────────

func parseTokenExpiry(raw json.RawMessage, legacyDays *int) (*time.Time, bool) {
	now := time.Now()
	if len(raw) > 0 && string(raw) != "null" {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			str = strings.TrimSpace(strings.ToLower(str))
			if str == "never" {
				return nil, true
			}
			if days, err := strconv.Atoi(str); err == nil && days > 0 {
				t := now.AddDate(0, 0, days)
				return &t, true
			}
			return nil, false
		}
		var days int
		if err := json.Unmarshal(raw, &days); err == nil && days > 0 {
			t := now.AddDate(0, 0, days)
			return &t, true
		}
		return nil, false
	}

	if legacyDays != nil {
		if *legacyDays <= 0 {
			return nil, true
		}
		t := now.AddDate(0, 0, *legacyDays)
		return &t, true
	}

	// Default matches AccessTokenForm.php: 30 days
	t := now.AddDate(0, 0, 30)
	return &t, true
}

func (server *Server) devTokensList(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	page, perPage := parsePageArgs(c, 10)
	offset := (page - 1) * perPage

	conditions := []string{"tokenable_type = ?", "tokenable_id = ?"}
	args := []any{customerMorphType, u.ID}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+search+"%")
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countArgs := append([]any{}, args...)
	if err := server.db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM access_tokens "+where, countArgs...,
	).Scan(&total); err != nil {
		return databaseError(c, err)
	}

	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, name, last_used_at, expires_at, created_at
		FROM access_tokens `+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return databaseError(c, err)
	}
	defer rows.Close()

	list := make([]fiber.Map, 0)
	for rows.Next() {
		var id int64
		var name string
		var lastUsed, expiresAt, createdAt sql.NullTime
		if err := rows.Scan(&id, &name, &lastUsed, &expiresAt, &createdAt); err != nil {
			return databaseError(c, err)
		}
		list = append(list, fiber.Map{
			"id":           id,
			"name":         name,
			"last_used_at": devTimePtr(lastUsed),
			"expires_at":   devTimePtr(expiresAt),
			"created_at":   devTimePtr(createdAt),
		})
	}

	return c.JSON(fiber.Map{
		"data": list,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) devTokenCreate(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	var req struct {
		Name        string          `json:"name"`
		ExpiresAt   json.RawMessage `json:"expires_at"`
		ExpiresDays *int            `json:"expires_days"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 255 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Token name is required (max 255 characters)."})
	}

	expiresTime, validExpiry := parseTokenExpiry(req.ExpiresAt, req.ExpiresDays)
	if !validExpiry {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid expiration preset."})
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to generate secure token."})
	}
	plainToken := hex.EncodeToString(b)
	digest := sha256.Sum256([]byte(plainToken))
	hashedToken := hex.EncodeToString(digest[:])

	var expSQL sql.NullTime
	if expiresTime != nil {
		expSQL = sql.NullTime{Time: *expiresTime, Valid: true}
	}

	now := time.Now()
	result, err := server.db.ExecContext(c.Context(), `
		INSERT INTO access_tokens (tokenable_type, tokenable_id, name, token, abilities, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, '["*"]', ?, ?, ?)
	`, customerMorphType, u.ID, req.Name, hashedToken, expSQL, now, now)
	if err != nil {
		return databaseError(c, err)
	}

	tokenID, _ := result.LastInsertId()
	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "API token created.",
		"token":   fmt.Sprintf("%d|%s", tokenID, plainToken),
		"data": fiber.Map{
			"id":           tokenID,
			"name":         req.Name,
			"last_used_at": nil,
			"expires_at":   devTimePtr(expSQL),
			"created_at":   now.Format(time.RFC3339),
		},
	})
}

func (server *Server) devTokenUpdate(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	id := c.Params("id")
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 255 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Token name is required (max 255 characters)."})
	}

	var existingID int64
	if err := server.db.QueryRowContext(c.Context(),
		"SELECT id FROM access_tokens WHERE id = ? AND tokenable_id = ? AND tokenable_type = ?",
		id, u.ID, customerMorphType,
	).Scan(&existingID); err == sql.ErrNoRows {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Token not found."})
	} else if err != nil {
		return databaseError(c, err)
	}

	if _, err := server.db.ExecContext(c.Context(),
		"UPDATE access_tokens SET name = ?, updated_at = ? WHERE id = ? AND tokenable_id = ? AND tokenable_type = ?",
		req.Name, time.Now(), existingID, u.ID, customerMorphType,
	); err != nil {
		return databaseError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "API token updated.",
		"data": fiber.Map{
			"id":   existingID,
			"name": req.Name,
		},
	})
}

func (server *Server) devTokenDelete(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	id := c.Params("id")
	result, err := server.db.ExecContext(c.Context(),
		"DELETE FROM access_tokens WHERE id = ? AND tokenable_id = ? AND tokenable_type = ?",
		id, u.ID, customerMorphType,
	)
	if err != nil {
		return databaseError(c, err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Token not found."})
	}
	return c.JSON(fiber.Map{"message": "API token revoked."})
}

func (server *Server) devTokensBulkDelete(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil || len(req.IDs) == 0 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one token."})
	}

	placeholders := make([]string, 0, len(req.IDs))
	args := make([]any, 0, 2+len(req.IDs))
	args = append(args, u.ID, customerMorphType)
	for _, id := range req.IDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	query := fmt.Sprintf(
		"DELETE FROM access_tokens WHERE tokenable_id = ? AND tokenable_type = ? AND id IN (%s)",
		strings.Join(placeholders, ","),
	)
	res, err := server.db.ExecContext(c.Context(), query, args...)
	if err != nil {
		return databaseError(c, err)
	}
	deleted, _ := res.RowsAffected()
	return c.JSON(fiber.Map{
		"message": fmt.Sprintf("%d API token(s) revoked.", deleted),
		"deleted": deleted,
	})
}

// ─── IP Whitelist Requests ───────────────────────────────────────────────────

func (server *Server) devIPWhitelistsList(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	page, perPage := parsePageArgs(c, 10)
	offset := (page - 1) * perPage

	conditions := []string{"user_id = ?"}
	args := []any{u.ID}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		conditions = append(conditions, "(ip_address LIKE ? OR COALESCE(label, '') LIKE ?)")
		args = append(args, like, like)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countArgs := append([]any{}, args...)
	if err := server.db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM ip_whitelist_requests "+where, countArgs...,
	).Scan(&total); err != nil {
		return databaseError(c, err)
	}

	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, ip_address, COALESCE(label, ''), status, COALESCE(admin_note, ''), created_at, reviewed_at
		FROM ip_whitelist_requests `+where+`
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return databaseError(c, err)
	}
	defer rows.Close()

	list := make([]fiber.Map, 0)
	for rows.Next() {
		var id int64
		var ip, label, status, adminNote string
		var createdAt, reviewedAt sql.NullTime
		if err := rows.Scan(&id, &ip, &label, &status, &adminNote, &createdAt, &reviewedAt); err != nil {
			return databaseError(c, err)
		}
		var noteVal any
		if adminNote != "" {
			noteVal = adminNote
		}
		list = append(list, fiber.Map{
			"id":          id,
			"ip_address":  ip,
			"label":       label,
			"status":      status,
			"admin_note":  noteVal,
			"created_at":  devTimePtr(createdAt),
			"reviewed_at": devTimePtr(reviewedAt),
		})
	}

	return c.JSON(fiber.Map{
		"data": list,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) devIPWhitelistCreate(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	var req struct {
		IPAddress string `json:"ip_address"`
		Label     string `json:"label"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}

	req.IPAddress = strings.TrimSpace(req.IPAddress)
	req.Label = strings.TrimSpace(req.Label)
	if req.IPAddress == "" || len(req.IPAddress) > 45 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Public IP address or CIDR block is required (max 45 characters)."})
	}
	if len(req.Label) > 100 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Label must be 100 characters or less."})
	}
	if errMsg := validatePublicIPStr(req.IPAddress); errMsg != "" {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": errMsg})
	}

	now := time.Now()
	result, err := server.db.ExecContext(c.Context(), `
		INSERT INTO ip_whitelist_requests (user_id, ip_address, label, status, created_at, updated_at)
		VALUES (?, ?, ?, 'pending', ?, ?)
	`, u.ID, req.IPAddress, nullString(req.Label), now, now)
	if err != nil {
		return databaseError(c, err)
	}
	id, _ := result.LastInsertId()

	server.recordIPWhitelistAudit(u, id, "created", req.IPAddress,
		nil,
		map[string]any{"ip_address": req.IPAddress, "label": req.Label, "status": "pending"},
		"Requested by customer",
		c.IP(),
	)

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Your IP whitelist request has been submitted for admin approval.",
		"data": fiber.Map{
			"id":          id,
			"ip_address":  req.IPAddress,
			"label":       req.Label,
			"status":      "pending",
			"admin_note":  nil,
			"created_at":  now.Format(time.RFC3339),
			"reviewed_at": nil,
		},
	})
}

func (server *Server) devIPWhitelistUpdate(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	id := c.Params("id")
	var req struct {
		IPAddress string `json:"ip_address"`
		Label     string `json:"label"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}

	req.IPAddress = strings.TrimSpace(req.IPAddress)
	req.Label = strings.TrimSpace(req.Label)
	if req.IPAddress == "" || len(req.IPAddress) > 45 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Public IP address or CIDR block is required (max 45 characters)."})
	}
	if len(req.Label) > 100 {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Label must be 100 characters or less."})
	}
	if errMsg := validatePublicIPStr(req.IPAddress); errMsg != "" {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": errMsg})
	}

	var rowID int64
	var oldIP, oldLabel, oldStatus, oldAdminNote string
	var oldReviewedAt sql.NullTime
	err = server.db.QueryRowContext(c.Context(), `
		SELECT id, ip_address, COALESCE(label, ''), status, COALESCE(admin_note, ''), reviewed_at
		FROM ip_whitelist_requests
		WHERE id = ? AND user_id = ?
	`, id, u.ID).Scan(&rowID, &oldIP, &oldLabel, &oldStatus, &oldAdminNote, &oldReviewedAt)
	if err == sql.ErrNoRows {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"message": "IP whitelist entry not found."})
	}
	if err != nil {
		return databaseError(c, err)
	}

	ipChanged := req.IPAddress != strings.TrimSpace(oldIP)
	newStatus := oldStatus
	var newAdminNote any = nullString(oldAdminNote)
	var newReviewedAt any = oldReviewedAt

	if ipChanged {
		newStatus = "pending"
		newAdminNote = nil
		newReviewedAt = nil
		_, err = server.db.ExecContext(c.Context(), `
			UPDATE ip_whitelist_requests
			SET ip_address = ?, label = ?, status = 'pending', admin_note = NULL, reviewed_by = NULL, reviewed_at = NULL, updated_at = ?
			WHERE id = ? AND user_id = ?
		`, req.IPAddress, nullString(req.Label), time.Now(), rowID, u.ID)
	} else {
		_, err = server.db.ExecContext(c.Context(), `
			UPDATE ip_whitelist_requests
			SET ip_address = ?, label = ?, updated_at = ?
			WHERE id = ? AND user_id = ?
		`, req.IPAddress, nullString(req.Label), time.Now(), rowID, u.ID)
	}
	if err != nil {
		return databaseError(c, err)
	}

	note := "Customer edited label"
	msg := "IP whitelist label updated."
	if ipChanged {
		note = "Customer edited IP address (status reset to pending review)"
		msg = "Because the IP address was modified, it has been resubmitted for admin review."
	}

	server.recordIPWhitelistAudit(
		u,
		rowID,
		"updated",
		req.IPAddress,
		map[string]any{"ip_address": oldIP, "label": oldLabel, "status": oldStatus},
		map[string]any{"ip_address": req.IPAddress, "label": req.Label, "status": newStatus},
		note,
		c.IP(),
	)

	var revStr any
	if nt, ok := newReviewedAt.(sql.NullTime); ok {
		revStr = devTimePtr(nt)
	}

	return c.JSON(fiber.Map{
		"message":    msg,
		"ip_changed": ipChanged,
		"data": fiber.Map{
			"id":          rowID,
			"ip_address":  req.IPAddress,
			"label":       req.Label,
			"status":      newStatus,
			"admin_note":  newAdminNote,
			"reviewed_at": revStr,
		},
	})
}

func (server *Server) devIPWhitelistDelete(c fiber.Ctx) error {
	u, ok, err := server.requireCustomerAPIAccess(c)
	if !ok {
		return err
	}

	id := c.Params("id")
	var rowID int64
	var oldIP, oldLabel, oldStatus string
	err = server.db.QueryRowContext(c.Context(),
		"SELECT id, ip_address, COALESCE(label, ''), status FROM ip_whitelist_requests WHERE id = ? AND user_id = ?",
		id, u.ID,
	).Scan(&rowID, &oldIP, &oldLabel, &oldStatus)
	if err == sql.ErrNoRows {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"message": "IP whitelist entry not found."})
	}
	if err != nil {
		return databaseError(c, err)
	}

	if _, err := server.db.ExecContext(c.Context(),
		"DELETE FROM ip_whitelist_requests WHERE id = ? AND user_id = ?",
		rowID, u.ID,
	); err != nil {
		return databaseError(c, err)
	}

	server.recordIPWhitelistAudit(
		u,
		rowID,
		"deleted",
		oldIP,
		map[string]any{"ip_address": oldIP, "label": oldLabel, "status": oldStatus},
		nil,
		"Deleted by customer",
		c.IP(),
	)

	return c.JSON(fiber.Map{"message": "IP whitelist entry removed."})
}

// ─── Global Search ───────────────────────────────────────────────────────────

func (server *Server) globalSearch(c fiber.Ctx) error {
	claims := customer(c)
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return c.JSON(fiber.Map{"categories": []fiber.Map{}})
	}
	like := "%" + q + "%"
	categories := make([]fiber.Map, 0)

	// 1. Contacts
	contactItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, COALESCE(name, ''), phone
		FROM contacts
		WHERE user_id = ? AND deleted_at IS NULL AND (COALESCE(name, '') LIKE ? OR phone LIKE ?)
		ORDER BY id DESC LIMIT 5
	`, claims.UserID, like, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var name, phone string
			if rows.Scan(&id, &name, &phone) == nil {
				title := phone
				if name != "" {
					title = fmt.Sprintf("%s (%s)", name, phone)
				}
				contactItems = append(contactItems, fiber.Map{
					"id":       id,
					"title":    title,
					"subtitle": "Phone: " + phone,
					"url":      "/audience/contacts",
				})
			}
		}
	}
	if len(contactItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Contacts", "items": contactItems})
	}

	// 2. Messages
	msgItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), "SELECT m.id, COALESCE(m.`to`, ''), COALESCE(m.status, ''), COALESCE(s.sender_id, '—') FROM messages m LEFT JOIN senders s ON s.id = m.sender_id WHERE m.user_id = ? AND (m.`to` LIKE ? OR m.body LIKE ?) ORDER BY m.id DESC LIMIT 5",
		claims.UserID, like, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var to, status, sender string
			if rows.Scan(&id, &to, &status, &sender) == nil {
				msgItems = append(msgItems, fiber.Map{
					"id":       id,
					"title":    "SMS to " + to,
					"subtitle": fmt.Sprintf("Sender: %s • %s", sender, status),
					"url":      "/messaging/messages",
				})
			}
		}
	}
	if len(msgItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Messages", "items": msgItems})
	}

	// 3. Scheduled Messages
	schedItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, COALESCE(body, ''), COALESCE(status, ''), COALESCE(total_recipients, 0)
		FROM batches
		WHERE user_id = ? AND status IN ('scheduled', 'paused') AND body LIKE ?
		ORDER BY scheduled_at ASC LIMIT 5
	`, claims.UserID, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var body, status string
			var recipients int
			if rows.Scan(&id, &body, &status, &recipients) == nil {
				preview := strings.TrimSpace(body)
				if len(preview) > 40 {
					preview = preview[:40] + "..."
				}
				schedItems = append(schedItems, fiber.Map{
					"id":       id,
					"title":    "Scheduled: " + preview,
					"subtitle": fmt.Sprintf("%s • %d recipients", status, recipients),
					"url":      "/messaging/scheduled-messages",
				})
			}
		}
	}
	if len(schedItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Scheduled Messages", "items": schedItems})
	}

	// 4. Templates
	tplItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, name, COALESCE(body, '')
		FROM templates
		WHERE user_id = ? AND deleted_at IS NULL AND (name LIKE ? OR body LIKE ?)
		ORDER BY id DESC LIMIT 5
	`, claims.UserID, like, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var name, body string
			if rows.Scan(&id, &name, &body) == nil {
				preview := strings.TrimSpace(body)
				if len(preview) > 40 {
					preview = preview[:40] + "..."
				}
				tplItems = append(tplItems, fiber.Map{
					"id":       id,
					"title":    name,
					"subtitle": preview,
					"url":      "/messaging/templates",
				})
			}
		}
	}
	if len(tplItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Templates", "items": tplItems})
	}

	// 5. Payment History (Deposits)
	depItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, amount, COALESCE(transaction_id, '—'), COALESCE(payment_method, '—'), COALESCE(status, '')
		FROM deposits
		WHERE user_id = ? AND (COALESCE(transaction_id, '') LIKE ? OR COALESCE(payment_method, '') LIKE ? OR CAST(amount AS CHAR) LIKE ?)
		ORDER BY id DESC LIMIT 5
	`, claims.UserID, like, like, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var amount float64
			var txID, method, status string
			if rows.Scan(&id, &amount, &txID, &method, &status) == nil {
				depItems = append(depItems, fiber.Map{
					"id":       id,
					"title":    fmt.Sprintf("Deposit: ৳%.2f", amount),
					"subtitle": fmt.Sprintf("TxID: %s • %s • %s", txID, method, status),
					"url":      "/billing/deposits",
				})
			}
		}
	}
	if len(depItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Payment History", "items": depItems})
	}

	// 6. Transactional Report
	txItems := make([]fiber.Map, 0)
	if rows, err := server.db.QueryContext(c.Context(), `
		SELECT id, direction, amount, COALESCE(description, '—')
		FROM transactions
		WHERE user_id = ? AND (CAST(id AS CHAR) LIKE ? OR COALESCE(description, '') LIKE ?)
		ORDER BY id DESC LIMIT 5
	`, claims.UserID, like, like); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var dir, desc string
			var amount float64
			if rows.Scan(&id, &dir, &amount, &desc) == nil {
				sign := "-"
				if dir == "credit" {
					sign = "+"
				}
				txItems = append(txItems, fiber.Map{
					"id":       id,
					"title":    fmt.Sprintf("Transaction #%d: %s৳%.2f", id, sign, amount),
					"subtitle": desc,
					"url":      "/billing/transactions",
				})
			}
		}
	}
	if len(txItems) > 0 {
		categories = append(categories, fiber.Map{"category": "Transactional Report", "items": txItems})
	}

	return c.JSON(fiber.Map{"categories": categories})
}
