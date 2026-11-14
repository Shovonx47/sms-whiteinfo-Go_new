package httpapi

import (
	stdctx "context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"smswhiteinfo/customer-api/internal/billing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Dashboard
// ─────────────────────────────────────────────────────────────────────────────

func buildPeriodSummary(total, successful, delivered, sentTelco, failed int, cost float64) fiber.Map {
	successRate := 0.0
	deliveryRate := 0.0
	failedRate := 0.0
	if total > 0 {
		successRate = roundFloat(float64(successful)/float64(total)*100, 1)
		deliveryRate = roundFloat(float64(delivered)/float64(total)*100, 1)
		failedRate = roundFloat(float64(failed)/float64(total)*100, 1)
	}
	return fiber.Map{
		"total_sent":    total,
		"successful":    successful,
		"delivered":     delivered,
		"sent_telco":    sentTelco,
		"failed":        failed,
		"cost":          roundFloat(cost, 2),
		"success_rate":  successRate,
		"delivery_rate": deliveryRate,
		"failed_rate":   failedRate,
	}
}

func extractBatchRecipientsPreview(db *sql.DB, batchID int64, totalRecipients int, sourceConfigRaw sql.NullString) (string, int, []string) {
	var phones []string

	msgRows, err := db.Query("SELECT `to` FROM messages WHERE batch_id = ? LIMIT 5", batchID)
	if err == nil {
		for msgRows.Next() {
			var p string
			if msgRows.Scan(&p) == nil {
				p = strings.TrimSpace(p)
				if p != "" {
					phones = append(phones, p)
				}
			}
		}
		msgRows.Close()
	}

	var configPhones []string
	if sourceConfigRaw.Valid && strings.TrimSpace(sourceConfigRaw.String) != "" {
		var parsed struct {
			Recipients []struct {
				Phone string `json:"phone"`
			} `json:"recipients"`
		}
		if json.Unmarshal([]byte(sourceConfigRaw.String), &parsed) == nil {
			for _, r := range parsed.Recipients {
				p := strings.TrimSpace(r.Phone)
				if p != "" {
					configPhones = append(configPhones, p)
				}
			}
		}
	}

	if len(phones) == 0 {
		phones = configPhones
	}

	total := totalRecipients
	if len(configPhones) > total {
		total = len(configPhones)
	}
	if len(phones) > total {
		total = len(phones)
	}

	if len(phones) == 0 {
		return "—", total, configPhones
	}

	limit := 2
	if len(phones) < limit {
		limit = len(phones)
	}
	first := strings.Join(phones[:limit], ", ")
	if total > 2 {
		return fmt.Sprintf("%s (+%d more)", first, total-2), total, configPhones
	}
	return first, total, configPhones
}

func (server *Server) dashboardStats(context fiber.Ctx) error {
	claims := customer(context)

	now := time.Now()
	startToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	start7Days := now.AddDate(0, 0, -7)
	start30Days := now.AddDate(0, 0, -30)

	var (
		totalToday, succToday, delivToday, sentToday, failToday             int
		costToday                                                           float64
		total7d, succ7d, deliv7d, sent7d, fail7d                            int
		cost7d                                                              float64
		total30d, succ30d, deliv30d, sent30d, fail30d                       int
		cost30d                                                             float64
		totalAll, succAll, delivAll, sentAll, failAll                       int
		costAll                                                             float64
	)

	err := server.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('sent','delivered') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('failed','dlr_failed') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END), 0),

			COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('sent','delivered') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('failed','dlr_failed') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END), 0),

			COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('sent','delivered') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status = 'sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status IN ('failed','dlr_failed') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= ? AND status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END), 0),

			COUNT(*),
			COALESCE(SUM(CASE WHEN status IN ('sent','delivered') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status IN ('failed','dlr_failed') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END), 0)
		FROM messages
		WHERE user_id = ?
	`,
		startToday, startToday, startToday, startToday, startToday, startToday,
		start7Days, start7Days, start7Days, start7Days, start7Days, start7Days,
		start30Days, start30Days, start30Days, start30Days, start30Days, start30Days,
		claims.UserID,
	).Scan(
		&totalToday, &succToday, &delivToday, &sentToday, &failToday, &costToday,
		&total7d, &succ7d, &deliv7d, &sent7d, &fail7d, &cost7d,
		&total30d, &succ30d, &deliv30d, &sent30d, &fail30d, &cost30d,
		&totalAll, &succAll, &delivAll, &sentAll, &failAll, &costAll,
	)
	if err != nil && err != sql.ErrNoRows {
		return databaseError(context, err)
	}

	var balance float64
	_ = server.db.QueryRow("SELECT COALESCE(balance, 0) FROM users WHERE id = ?", claims.UserID).Scan(&balance)

	var contacts, groupsCount, templatesCount, scheduledCount int
	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM contacts WHERE user_id = ? AND deleted_at IS NULL", claims.UserID,
	).Scan(&contacts)
	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM `groups` WHERE user_id = ? AND deleted_at IS NULL", claims.UserID,
	).Scan(&groupsCount)
	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM templates WHERE user_id = ? AND deleted_at IS NULL", claims.UserID,
	).Scan(&templatesCount)
	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM batches WHERE user_id = ? AND status IN ('scheduled', 'paused')", claims.UserID,
	).Scan(&scheduledCount)

	// Recent 5 messages
	recentMessages := []fiber.Map{}
	msgRows, err := server.db.Query(`
		SELECT m.id, COALESCE(s.sender_id, '—'), m.to, m.body, m.status, m.cost, m.parts,
		       m.created_at, m.sent_at, m.delivered_at
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		WHERE m.user_id = ?
		ORDER BY m.id DESC
		LIMIT 5
	`, claims.UserID)
	if err == nil {
		for msgRows.Next() {
			var id, parts int
			var senderID, to, body, status string
			var cost float64
			var createdAt time.Time
			var sentAt, deliveredAt *time.Time
			if msgRows.Scan(&id, &senderID, &to, &body, &status, &cost, &parts, &createdAt, &sentAt, &deliveredAt) == nil {
				recentMessages = append(recentMessages, fiber.Map{
					"id":           id,
					"msg_id":       fmt.Sprintf("MSG-%06d", id),
					"sender_id":    senderID,
					"to":           to,
					"body":         body,
					"status":       status,
					"cost":         roundFloat(cost, 4),
					"parts":        parts,
					"created_at":   createdAt.Format(time.RFC3339),
					"sent_at":      apiTimePtr(sentAt),
					"delivered_at": apiTimePtr(deliveredAt),
				})
			}
		}
		msgRows.Close()
	}

	// Upcoming 5 scheduled/paused batches
	scheduledMessages := []fiber.Map{}
	type schedRow struct {
		id              int64
		senderDBID      sql.NullInt64
		senderID        string
		totalRecipients int
		body            string
		sourceType      string
		status          string
		sourceConfig    sql.NullString
		scheduledAt     *time.Time
		createdAt       time.Time
	}
	var schedRowsData []schedRow
	schedRows, err := server.db.Query(`
		SELECT b.id, b.sender_id, COALESCE(s.sender_id, '—'), b.total_recipients, b.body,
		       b.source_type, b.status, b.source_config, b.scheduled_at, b.created_at
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		WHERE b.user_id = ? AND b.status IN ('scheduled', 'paused')
		ORDER BY b.scheduled_at ASC, b.id DESC
		LIMIT 5
	`, claims.UserID)
	if err == nil {
		for schedRows.Next() {
			var r schedRow
			if schedRows.Scan(&r.id, &r.senderDBID, &r.senderID, &r.totalRecipients, &r.body,
				&r.sourceType, &r.status, &r.sourceConfig, &r.scheduledAt, &r.createdAt) == nil {
				schedRowsData = append(schedRowsData, r)
			}
		}
		schedRows.Close()
	}
	for _, r := range schedRowsData {
		preview, totalRec, phonesList := extractBatchRecipientsPreview(server.db, r.id, r.totalRecipients, r.sourceConfig)
		var senderDBID any
		if r.senderDBID.Valid {
			senderDBID = r.senderDBID.Int64
		}
		scheduledMessages = append(scheduledMessages, fiber.Map{
			"id":                r.id,
			"sender_db_id":      senderDBID,
			"sender_id":         r.senderID,
			"recipient_preview": preview,
			"recipients_list":   strings.Join(phonesList, ",\n"),
			"total_recipients":  totalRec,
			"body":              r.body,
			"source_type":       r.sourceType,
			"status":            r.status,
			"scheduled_at":      apiTimePtr(r.scheduledAt),
			"created_at":        r.createdAt.Format(time.RFC3339),
		})
	}

	deliveryRateToday := 0.0
	successRateToday := 100.0
	if totalToday > 0 {
		deliveryRateToday = float64(succToday) / float64(totalToday) * 100
		successRateToday = roundFloat(deliveryRateToday, 1)
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"balance":             roundFloat(balance, 2),
			"messages_today":      totalToday,
			"successful_today":    succToday,
			"delivered_today":     delivToday,
			"sent_telco_today":    sentToday,
			"failed_today":        failToday,
			"cost_today":          roundFloat(costToday, 2),
			"success_rate_today":  successRateToday,
			"delivery_rate":       roundFloat(deliveryRateToday, 1),
			"messages_all_time":   totalAll,
			"successful_all_time": succAll,
			"delivered_all_time":  delivAll,
			"sent_telco_all_time": sentAll,
			"failed_all_time":     failAll,
			"cost_all_time":       roundFloat(costAll, 2),
			"scheduled_count":     scheduledCount,
			"contacts_total":      contacts,
			"groups_total":        groupsCount,
			"templates_total":     templatesCount,
			"periods": fiber.Map{
				"today":    buildPeriodSummary(totalToday, succToday, delivToday, sentToday, failToday, costToday),
				"7_days":   buildPeriodSummary(total7d, succ7d, deliv7d, sent7d, fail7d, cost7d),
				"30_days":  buildPeriodSummary(total30d, succ30d, deliv30d, sent30d, fail30d, cost30d),
				"all_time": buildPeriodSummary(totalAll, succAll, delivAll, sentAll, failAll, costAll),
			},
			"recent_messages":    recentMessages,
			"scheduled_messages": scheduledMessages,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Messages
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) messages(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"m.user_id = ?"}
	args := []any{claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "(m.to LIKE ? OR m.body LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	if status := strings.TrimSpace(context.Query("status")); status != "" {
		parts := strings.Split(status, ",")
		var validParts []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				validParts = append(validParts, p)
			}
		}
		if len(validParts) == 1 {
			conditions = append(conditions, "m.status = ?")
			args = append(args, validParts[0])
		} else if len(validParts) > 1 {
			placeholders := strings.TrimSuffix(strings.Repeat("?,", len(validParts)), ",")
			conditions = append(conditions, fmt.Sprintf("m.status IN (%s)", placeholders))
			for _, vp := range validParts {
				args = append(args, vp)
			}
		}
	}
	if dateFrom := context.Query("date_from"); dateFrom != "" {
		conditions = append(conditions, "DATE(m.created_at) >= ?")
		args = append(args, dateFrom)
	}
	if dateTo := context.Query("date_to"); dateTo != "" {
		conditions = append(conditions, "DATE(m.created_at) <= ?")
		args = append(args, dateTo)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := server.db.QueryRow("SELECT COUNT(*) FROM messages m "+where, countArgs...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	sort := "m.id"
	dir := "DESC"
	if s := context.Query("sort"); s != "" {
		allowed := map[string]bool{"id": true, "to": true, "status": true, "cost": true, "created_at": true, "sent_at": true}
		if allowed[s] {
			sort = "m." + s
		}
	}
	if context.Query("dir") == "asc" {
		dir = "ASC"
	}

	args = append(args, perPage, offset)
	rows, err := server.db.Query(fmt.Sprintf(`
		SELECT m.id, COALESCE(s.sender_id, '—'), m.to, m.body, m.status, m.cost, m.parts,
		       m.created_at, m.sent_at, m.delivered_at, m.failed_reason, COALESCE(m.operator_name, '')
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		%s
		ORDER BY %s %s
		LIMIT ? OFFSET ?
	`, where, sort, dir), args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var senderID, to, body, status, operatorName string
		var cost float64
		var parts int
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		var failedReason *string

		if err := rows.Scan(&id, &senderID, &to, &body, &status, &cost, &parts,
			&createdAt, &sentAt, &deliveredAt, &failedReason, &operatorName); err != nil {
			continue
		}

		op := operatorName
		if op == "" {
			op = operatorFromNumber(to)
		}

		items = append(items, fiber.Map{
			"id":            id,
			"msg_id":        fmt.Sprintf("MSG-%06d", id),
			"sender_id":     senderID,
			"to":            to,
			"body":          body,
			"status":        status,
			"cost":          fmt.Sprintf("%.4f", cost),
			"parts":         parts,
			"created_at":    createdAt.Format(time.RFC3339),
			"submitted_at":  createdAt.Format(time.RFC3339),
			"sent_at":       apiTimePtr(sentAt),
			"delivered_at":  apiTimePtr(deliveredAt),
			"error_message": failedReason,
			"operator":      op,
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Recent Recipients
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) recentRecipients(context fiber.Ctx) error {
	claims := customer(context)

	var phones []string

	// 1. Pull from last 100 outgoing messages
	rows, err := server.db.Query(`
		SELECT DISTINCT ` + "`to`" + ` FROM messages
		WHERE user_id = ? AND ` + "`to`" + ` IS NOT NULL AND ` + "`to`" + ` != ''
		ORDER BY id DESC LIMIT 100
	`, claims.UserID)
	if err == nil {
		for rows.Next() {
			var p string
			if rows.Scan(&p) == nil && p != "" {
				phones = append(phones, p)
			}
		}
		rows.Close()
	}

	// 2. Pull from last 20 manual batch source_configs
	batchRows, err := server.db.Query(`
		SELECT source_config FROM batches
		WHERE user_id = ? AND source_type = 'manual'
		ORDER BY id DESC LIMIT 20
	`, claims.UserID)
	if err == nil {
		for batchRows.Next() {
			var cfg string
			if batchRows.Scan(&cfg) == nil && cfg != "" {
				var parsed struct {
					Recipients []struct {
						Phone string `json:"phone"`
					} `json:"recipients"`
				}
				if json.Unmarshal([]byte(cfg), &parsed) == nil {
					for _, r := range parsed.Recipients {
						if r.Phone != "" {
							phones = append(phones, r.Phone)
						}
					}
				}
			}
		}
		batchRows.Close()
	}

	// 3. Deduplicate and take up to 10
	seen := map[string]bool{}
	var unique []string
	for _, p := range phones {
		p = strings.TrimSpace(p)
		if p != "" && !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
		if len(unique) >= 10 {
			break
		}
	}

	if unique == nil {
		unique = []string{}
	}

	return context.JSON(fiber.Map{"data": unique})
}

func (server *Server) sendMessage(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID int64  `json:"sender_id"`
		Phones   any    `json:"phones"`
		Body     string `json:"body"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	if req.SenderID == 0 || req.Body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "sender_id and body are required."})
	}

	// Verify sender belongs to user or is approved
	var senderLabel, senderType string
	err := server.db.QueryRow(`
		SELECT s.sender_id, s.type FROM senders s
		LEFT JOIN sender_user su ON su.sender_id = s.id
		WHERE (su.user_id = ? OR s.requested_by = ?) AND s.id = ? AND s.deleted_at IS NULL
		LIMIT 1
	`, claims.UserID, claims.UserID, req.SenderID).Scan(&senderLabel, &senderType)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Selected Sender ID is not available."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	phones := normalizePhones(req.Phones)
	if len(phones) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "At least one valid phone number is required."})
	}

	body := strings.TrimSpace(req.Body)

	// Check rate resolver & balance estimate
	resolver, err := billing.ForUser(context.Context(), server.db, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	isMasking := strings.Contains(strings.ToLower(senderType), "masking")
	parts, _ := billing.CalculateParts(body)
	rates := resolver.RatesForUser(isMasking)
	maxRate := 0.0
	for _, r := range rates {
		if r > maxRate {
			maxRate = r
		}
	}
	estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(len(phones)), 4)

	var balance float64
	_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ?", claims.UserID).Scan(&balance)
	if balance < estimatedCost {
		return context.Status(http.StatusPaymentRequired).JSON(fiber.Map{
			"message": "Insufficient balance for this message.",
			"balance": balance,
		})
	}

	recipients := make([]any, len(phones))
	for i, p := range phones {
		recipients[i] = map[string]any{"phone": p, "values": map[string]string{}}
	}
	srcConfigBytes, _ := json.Marshal(map[string]any{"recipients": recipients})

	// Create batch with status='processing'
	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, body, source_type, source_ip, source_config, status, approval, total_recipients, valid_count, created_at, updated_at)
		VALUES (?, ?, ?, 'manual', ?, ?, 'processing', 'approved', ?, 0, NOW(), NOW())
	`, claims.UserID, req.SenderID, body, context.IP(), string(srcConfigBytes), len(phones))
	if err != nil {
		return databaseError(context, err)
	}

	batchID, _ := result.LastInsertId()

	// Dispatch batch processor asynchronously
	go func(bid int64) {
		_ = server.batchProcessor.ProcessBatch(stdctx.Background(), bid)
	}(batchID)

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":         batchID,
			"status":     "processing",
			"recipients": len(phones),
		},
	})
}

func (server *Server) scheduleMessage(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID        int64  `json:"sender_id"`
		Phones          any    `json:"phones"`
		Body            string `json:"body"`
		ScheduledDate   string `json:"scheduled_date"`
		ScheduledHour   string `json:"scheduled_hour"`
		ScheduledMinute string `json:"scheduled_minute"`
		ScheduledPeriod string `json:"scheduled_period"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	phones := normalizePhones(req.Phones)
	if len(phones) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "At least one valid phone number is required."})
	}

	scheduledAt, err := parseScheduledTime(req.ScheduledDate, req.ScheduledHour, req.ScheduledMinute, req.ScheduledPeriod)
	if err != nil || scheduledAt.Before(time.Now().Add(-time.Minute)) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The selected schedule time has already passed."})
	}

	body := strings.TrimSpace(req.Body)
	now := time.Now()

	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, body, source_type, source_ip, source_config, status, total_recipients, valid_count, scheduled_at, created_at, updated_at)
		VALUES (?, ?, ?, 'manual', ?, ?, 'scheduled', ?, ?, ?, ?, ?)
	`, claims.UserID, req.SenderID, body, context.IP(), buildSourceConfig(phones), len(phones), len(phones), scheduledAt, now, now)
	if err != nil {
		return databaseError(context, err)
	}

	batchID, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":           batchID,
			"status":       "scheduled",
			"scheduled_at": scheduledAt.Format(time.RFC3339),
		},
	})
}

func (server *Server) bulkSend(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID        int64   `json:"sender_id"`
		Body            string  `json:"body"`
		Source          string  `json:"source"`
		Phones          any     `json:"phones"`
		ScheduledDate   string  `json:"scheduled_date"`
		ScheduledHour   string  `json:"scheduled_hour"`
		ScheduledMinute string  `json:"scheduled_minute"`
		ScheduledPeriod string  `json:"scheduled_period"`
		Groups          []int64 `json:"groups"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	var phones []string

	switch req.Source {
	case "manual":
		phones = normalizePhones(req.Phones)
	case "group":
		if len(req.Groups) > 0 {
			placeholders := strings.Repeat("?,", len(req.Groups))
			placeholders = placeholders[:len(placeholders)-1]
			args := []any{claims.UserID}
			for _, gid := range req.Groups {
				args = append(args, gid)
			}
			rows, err := server.db.Query(fmt.Sprintf(`
				SELECT c.phone FROM contacts c
				JOIN contact_group cg ON cg.contact_id = c.id
				WHERE c.user_id = ? AND cg.group_id IN (%s) AND c.deleted_at IS NULL
			`, placeholders), args...)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var p string
					if rows.Scan(&p) == nil {
						phones = append(phones, p)
					}
				}
			}
		}
	default:
		phones = normalizePhones(req.Phones)
	}

	if len(phones) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "No valid recipients found for this bulk campaign."})
	}

	var scheduledAt *time.Time
	if req.ScheduledDate != "" && req.ScheduledHour != "" {
		t, err := parseScheduledTime(req.ScheduledDate, req.ScheduledHour, req.ScheduledMinute, req.ScheduledPeriod)
		if err == nil {
			scheduledAt = &t
		}
	}

	body := strings.TrimSpace(req.Body)
	now := time.Now()

	if scheduledAt != nil {
		result, err := server.db.Exec(`
			INSERT INTO batches (user_id, sender_id, body, source_type, source_ip, source_config, status, total_recipients, valid_count, scheduled_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 'scheduled', ?, 0, ?, ?, ?)
		`, claims.UserID, req.SenderID, body, req.Source, context.IP(), buildSourceConfig(phones), len(phones), scheduledAt, now, now)
		if err != nil {
			return databaseError(context, err)
		}

		batchID, _ := result.LastInsertId()
		return context.Status(http.StatusCreated).JSON(fiber.Map{
			"data": fiber.Map{
				"id":         batchID,
				"status":     "scheduled",
				"recipients": len(phones),
			},
		})
	}

	// Verify affordability
	var senderType string
	_ = server.db.QueryRow("SELECT type FROM senders WHERE id = ?", req.SenderID).Scan(&senderType)

	resolver, err := billing.ForUser(context.Context(), server.db, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	isMasking := strings.Contains(strings.ToLower(senderType), "masking")
	parts, _ := billing.CalculateParts(body)
	rates := resolver.RatesForUser(isMasking)
	maxRate := 0.0
	for _, r := range rates {
		if r > maxRate {
			maxRate = r
		}
	}
	estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(len(phones)), 4)

	var balance float64
	_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ?", claims.UserID).Scan(&balance)
	if balance < estimatedCost {
		return context.Status(http.StatusPaymentRequired).JSON(fiber.Map{
			"message": "Insufficient balance for this batch.",
			"balance": balance,
		})
	}

	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, body, source_type, source_ip, source_config, status, total_recipients, valid_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 'processing', ?, 0, ?, ?)
	`, claims.UserID, req.SenderID, body, req.Source, context.IP(), buildSourceConfig(phones), len(phones), now, now)
	if err != nil {
		return databaseError(context, err)
	}

	batchID, _ := result.LastInsertId()

	go func(bid int64) {
		_ = server.batchProcessor.ProcessBatch(stdctx.Background(), bid)
	}(batchID)

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":         batchID,
			"status":     "processing",
			"recipients": len(phones),
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Batches
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) batches(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	// If explicit batch-summary view requested, return raw batches
	if context.Query("view") == "batches" || context.Query("mode") == "batches" {
		conditions := []string{"b.user_id = ?"}
		args := []any{claims.UserID}

		if search := context.Query("search"); search != "" {
			conditions = append(conditions, "(b.id LIKE ? OR b.body LIKE ?)")
			like := "%" + search + "%"
			args = append(args, like, like)
		}

		where := "WHERE " + strings.Join(conditions, " AND ")
		countArgs := make([]any, len(args))
		copy(countArgs, args)

		var total int
		_ = server.db.QueryRow("SELECT COUNT(*) FROM batches b "+where, countArgs...).Scan(&total)

		args = append(args, perPage, offset)
		rows, err := server.db.Query(`
			SELECT b.id, COALESCE(s.sender_id, '—'), b.status, b.approval, b.source_type,
			       b.total_recipients,
			       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('sent', 'delivered')) as sent_count,
			       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('failed', 'dlr_failed')) as failed_count,
			       (SELECT COALESCE(SUM(m.cost), 0) FROM messages m WHERE m.batch_id = b.id) as cost,
			       b.scheduled_at, b.created_at
			FROM batches b
			LEFT JOIN senders s ON s.id = b.sender_id
			`+where+`
			ORDER BY b.id DESC
			LIMIT ? OFFSET ?
		`, args...)
		if err != nil {
			return databaseError(context, err)
		}
		defer rows.Close()

		items := []fiber.Map{}
		for rows.Next() {
			var id, totalRec, sentCnt, failedCnt int
			var senderID, status, sourceType string
			var approval *string
			var cost float64
			var scheduledAt *time.Time
			var createdAt time.Time

			if err := rows.Scan(&id, &senderID, &status, &approval, &sourceType,
				&totalRec, &sentCnt, &failedCnt, &cost, &scheduledAt, &createdAt); err != nil {
				continue
			}
			items = append(items, fiber.Map{
				"id":               id,
				"sender_id":        senderID,
				"status":           status,
				"approval":         approval,
				"source_type":      sourceType,
				"total_recipients": totalRec,
				"sent_count":       sentCnt,
				"failed_count":     failedCnt,
				"cost":             fmt.Sprintf("%.2f", cost),
				"scheduled_at":     apiTimePtr(scheduledAt),
				"created_at":       createdAt.Format(time.RFC3339),
			})
		}

		return context.JSON(fiber.Map{
			"data": items,
			"meta": pageMeta(total, perPage, page),
		})
	}

	// Default: Sent Message Logs (Individual messages view matching Laravel ListBatches / Screenshot 9 & 10)
	conditions := []string{"m.user_id = ?"}
	args := []any{claims.UserID}

	if search := strings.TrimSpace(context.Query("search")); search != "" {
		conditions = append(conditions, "(m.to LIKE ? OR m.body LIKE ? OR s.sender_id LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	if sender := strings.TrimSpace(context.Query("sender_id")); sender != "" {
		conditions = append(conditions, "(CAST(m.sender_id AS CHAR) = ? OR s.sender_id = ?)")
		args = append(args, sender, sender)
	}

	if status := strings.TrimSpace(context.Query("status")); status != "" && status != "all" {
		conditions = append(conditions, "m.status = ?")
		args = append(args, status)
	}

	if createdFrom := strings.TrimSpace(context.Query("created_from")); createdFrom != "" {
		conditions = append(conditions, "DATE(m.created_at) >= ?")
		args = append(args, createdFrom)
	}

	if createdUntil := strings.TrimSpace(context.Query("created_until")); createdUntil != "" {
		conditions = append(conditions, "DATE(m.created_at) <= ?")
		args = append(args, createdUntil)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM messages m LEFT JOIN senders s ON s.id = m.sender_id "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT m.id, m.to, m.body, COALESCE(s.sender_id, '—') as sender_id,
		       m.status, m.cost, m.parts, m.created_at, m.sent_at, m.delivered_at, m.failed_reason,
		       COALESCE(
		           (SELECT t.balance_before FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ) as previous_balance,
		       CASE
		           WHEN b.source_type = 'manual' AND (b.total_recipients = 1 OR b.total_recipients IS NULL) THEN 'Transactional'
		           WHEN b.source_type = 'single' OR m.batch_id IS NULL THEN 'Transactional'
		           ELSE 'Promotional'
		       END as sms_type
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN batches b ON b.id = m.batch_id
		`+where+`
		ORDER BY m.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, parts int
		var to, body, senderID, status string
		var cost, prevBalance float64
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		var failedReason *string
		var smsType string

		if err := rows.Scan(&id, &to, &body, &senderID, &status, &cost, &parts,
			&createdAt, &sentAt, &deliveredAt, &failedReason, &prevBalance, &smsType); err != nil {
			continue
		}

		items = append(items, fiber.Map{
			"id":               id,
			"to":               to,
			"body":             body,
			"sender_id":        senderID,
			"status":           status,
			"cost":             fmt.Sprintf("%.2f", cost),
			"parts":            parts,
			"previous_balance": fmt.Sprintf("%.2f", prevBalance),
			"sms_type":         smsType,
			"created_at":       createdAt.Format(time.RFC3339),
			"sent_at":          apiTimePtr(sentAt),
			"delivered_at":     apiTimePtr(deliveredAt),
			"failed_reason":    failedReason,
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) batchDetail(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	var batchID, totalRec, sentCnt, deliveredCnt, failedCnt int
	var senderID, status, sourceType, body string
	var approval *string
	var cost float64
	var scheduledAt *time.Time
	var createdAt time.Time

	err := server.db.QueryRow(`
		SELECT b.id, COALESCE(s.sender_id, '—'), b.status, b.approval, b.source_type, b.body,
		       b.total_recipients,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('sent', 'delivered')) as sent_count,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status = 'delivered') as delivered_count,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('failed', 'dlr_failed')) as failed_count,
		       (SELECT COALESCE(SUM(m.cost), 0) FROM messages m WHERE m.batch_id = b.id) as cost,
		       b.scheduled_at, b.created_at
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		WHERE b.id = ? AND b.user_id = ?
	`, id, claims.UserID).Scan(
		&batchID, &senderID, &status, &approval, &sourceType, &body,
		&totalRec, &sentCnt, &deliveredCnt, &failedCnt, &cost,
		&scheduledAt, &createdAt,
	)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Batch not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	// Get messages for this batch
	msgRows, err := server.db.Query(`
		SELECT id, `+"`to`"+`, body, status, cost, failed_reason, sent_at, delivered_at
		FROM messages
		WHERE batch_id = ?
		ORDER BY id DESC
		LIMIT 100
	`, batchID)
	if err != nil {
		return databaseError(context, err)
	}
	defer msgRows.Close()

	messages := []fiber.Map{}
	for msgRows.Next() {
		var mid int
		var to, msgBody, msgStatus string
		var msgCost float64
		var failedReason *string
		var sentAt, deliveredAt *time.Time

		if err := msgRows.Scan(&mid, &to, &msgBody, &msgStatus, &msgCost, &failedReason, &sentAt, &deliveredAt); err != nil {
			continue
		}
		messages = append(messages, fiber.Map{
			"id":            mid,
			"to":            to,
			"body":          msgBody,
			"status":        msgStatus,
			"cost":          msgCost,
			"error_message": failedReason,
			"sent_at":       apiTimePtr(sentAt),
			"delivered_at":  apiTimePtr(deliveredAt),
		})
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"batch": fiber.Map{
				"id":               batchID,
				"sender_id":        senderID,
				"status":           status,
				"approval":         approval,
				"source_type":      sourceType,
				"body":             body,
				"total_recipients": totalRec,
				"sent_count":       sentCnt,
				"delivered_count":  deliveredCnt,
				"failed_count":     failedCnt,
				"cost":             fmt.Sprintf("%.2f", cost),
				"scheduled_at":     apiTimePtr(scheduledAt),
				"created_at":       createdAt.Format(time.RFC3339),
			},
			"messages": messages,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Scheduled Messages
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) scheduledMessages(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"b.user_id = ?"}
	args := []any{claims.UserID}

	statusFilter := strings.TrimSpace(context.Query("status"))
	switch statusFilter {
	case "scheduled", "paused":
		conditions = append(conditions, "b.status = ?")
		args = append(args, statusFilter)
	default:
		conditions = append(conditions, "b.status IN ('scheduled', 'paused')")
	}

	if search := strings.TrimSpace(context.Query("search")); search != "" {
		conditions = append(conditions, "(b.body LIKE ? OR b.source_config LIKE ? OR s.sender_id LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like, like)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow(`
		SELECT COUNT(*)
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		`+where, countArgs...).Scan(&total)

	var allCount, scheduledCount, pausedCount int
	_ = server.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN status = 'scheduled' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'paused' THEN 1 ELSE 0 END), 0)
		FROM batches
		WHERE user_id = ? AND status IN ('scheduled', 'paused')
	`, claims.UserID).Scan(&allCount, &scheduledCount, &pausedCount)

	sortCol := "b.scheduled_at"
	if s := context.Query("sort"); s != "" {
		switch s {
		case "scheduled_at":
			sortCol = "b.scheduled_at"
		case "total_recipients":
			sortCol = "b.total_recipients"
		case "created_at":
			sortCol = "b.created_at"
		}
	}
	dir := "ASC"
	if strings.EqualFold(context.Query("dir"), "desc") {
		dir = "DESC"
	}

	args = append(args, perPage, offset)
	rows, err := server.db.Query(fmt.Sprintf(`
		SELECT b.id, b.sender_id, COALESCE(s.sender_id, '—'), b.total_recipients, b.body,
		       b.source_type, b.status, b.source_config, b.scheduled_at, b.created_at
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		%s
		ORDER BY %s %s, b.id DESC
		LIMIT ? OFFSET ?
	`, where, sortCol, dir), args...)
	if err != nil {
		return databaseError(context, err)
	}

	type schedRow struct {
		id              int64
		senderDBID      sql.NullInt64
		senderID        string
		totalRecipients int
		body            string
		sourceType      string
		status          string
		sourceConfig    sql.NullString
		scheduledAt     *time.Time
		createdAt       time.Time
	}
	var schedRowsData []schedRow
	for rows.Next() {
		var r schedRow
		if rows.Scan(&r.id, &r.senderDBID, &r.senderID, &r.totalRecipients, &r.body,
			&r.sourceType, &r.status, &r.sourceConfig, &r.scheduledAt, &r.createdAt) == nil {
			schedRowsData = append(schedRowsData, r)
		}
	}
	rows.Close()

	items := []fiber.Map{}
	for _, r := range schedRowsData {
		preview, totalRec, phonesList := extractBatchRecipientsPreview(server.db, r.id, r.totalRecipients, r.sourceConfig)
		var senderDBID any
		if r.senderDBID.Valid {
			senderDBID = r.senderDBID.Int64
		}
		items = append(items, fiber.Map{
			"id":                r.id,
			"sender_db_id":      senderDBID,
			"sender_id":         r.senderID,
			"recipient_preview": preview,
			"recipients_list":   strings.Join(phonesList, ",\n"),
			"total_recipients":  totalRec,
			"body":              r.body,
			"source_type":       r.sourceType,
			"status":            r.status,
			"scheduled_at":      apiTimePtr(r.scheduledAt),
			"created_at":        r.createdAt.Format(time.RFC3339),
		})
	}

	meta := pageMeta(total, perPage, page)
	meta["counts"] = fiber.Map{
		"all":       allCount,
		"scheduled": scheduledCount,
		"paused":    pausedCount,
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": meta,
	})
}

func (server *Server) updateScheduledMessage(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	var req struct {
		SenderID       int64  `json:"sender_id"`
		Body           string `json:"body"`
		ScheduledAt    string `json:"scheduled_at"`
		RecipientsList string `json:"recipients_list"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Message content is required."})
	}

	var parsedSched time.Time
	var err error
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		parsedSched, err = time.ParseInLocation(layout, strings.TrimSpace(req.ScheduledAt), time.Local)
		if err == nil {
			break
		}
	}
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Valid scheduled date & time is required."})
	}

	var existingStatus string
	var existingConfig sql.NullString
	var existingTotal int
	var existingSender sql.NullInt64
	err = server.db.QueryRow(`
		SELECT status, source_config, total_recipients, sender_id
		FROM batches
		WHERE id = ? AND user_id = ?
	`, id, claims.UserID).Scan(&existingStatus, &existingConfig, &existingTotal, &existingSender)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Scheduled message not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}
	if existingStatus != "scheduled" && existingStatus != "paused" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Only scheduled or paused messages can be edited."})
	}

	senderID := req.SenderID
	if senderID == 0 && existingSender.Valid {
		senderID = existingSender.Int64
	}

	newSourceConfig := existingConfig.String
	newTotal := existingTotal
	if strings.TrimSpace(req.RecipientsList) != "" {
		phones := normalizePhones(req.RecipientsList)
		if len(phones) > 0 {
			newSourceConfig = buildSourceConfig(phones)
			newTotal = len(phones)
		}
	}

	_, err = server.db.Exec(`
		UPDATE batches
		SET sender_id = ?, body = ?, scheduled_at = ?, source_config = ?, total_recipients = ?, valid_count = ?, updated_at = NOW()
		WHERE id = ? AND user_id = ?
	`, senderID, body, parsedSched, newSourceConfig, newTotal, newTotal, id, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{
		"message": "Scheduled message updated successfully.",
	})
}

func (server *Server) pauseScheduledMessage(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	res, err := server.db.Exec(`
		UPDATE batches
		SET status = 'paused', updated_at = NOW()
		WHERE id = ? AND user_id = ? AND status = 'scheduled'
	`, id, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Scheduled message cannot be paused."})
	}
	return context.JSON(fiber.Map{"message": "Scheduled message paused successfully."})
}

func (server *Server) resumeScheduledMessage(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	res, err := server.db.Exec(`
		UPDATE batches
		SET status = 'scheduled', updated_at = NOW()
		WHERE id = ? AND user_id = ? AND status = 'paused'
	`, id, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Scheduled message cannot be resumed."})
	}
	return context.JSON(fiber.Map{"message": "Schedule resumed successfully."})
}

func (server *Server) deleteScheduledMessage(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	res, err := server.db.Exec(`
		DELETE FROM batches
		WHERE id = ? AND user_id = ? AND status IN ('scheduled', 'paused')
	`, id, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Scheduled message not found."})
	}
	return context.JSON(fiber.Map{"message": "Scheduled message deleted permanently."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func operatorFromNumber(phone string) string {
	cleaned := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(phone), "+"), "88")
	if len(cleaned) >= 3 {
		switch cleaned[:3] {
		case "017", "013":
			return "Grameenphone"
		case "018":
			return "Robi"
		case "019", "014":
			return "Banglalink"
		case "015":
			return "Teletalk"
		case "016":
			return "Airtel"
		}
	}
	return "Other"
}

func normalizePhones(raw any) []string {
	var items []string
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					items = append(items, s)
				}
			}
		}
	case string:
		for _, p := range strings.FieldsFunc(v, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' '
		}) {
			p = strings.TrimSpace(p)
			if p != "" {
				items = append(items, p)
			}
		}
	}

	return items
}

func buildSourceConfig(phones []string) string {
	parts := make([]string, len(phones))
	for i, p := range phones {
		parts[i] = fmt.Sprintf(`{"phone":%q,"values":{}}`, p)
	}

	return fmt.Sprintf(`{"recipients":[%s]}`, strings.Join(parts, ","))
}

func parseScheduledTime(date, hour, minute, period string) (time.Time, error) {
	if hour == "" {
		hour = "12"
	}
	if minute == "" {
		minute = "00"
	}
	if period == "" {
		period = "AM"
	}
	period = strings.ToUpper(period)

	raw := fmt.Sprintf("%s %02s:%02s %s", date, hour, minute, period)
	loc, _ := time.LoadLocation("Asia/Dhaka")
	if loc == nil {
		loc = time.FixedZone("Asia/Dhaka", 6*60*60)
	}

	return time.ParseInLocation("2006-01-02 03:04 PM", raw, loc)
}

func roundFloat(val float64, prec int) float64 {
	shift := 1.0
	for i := 0; i < prec; i++ {
		shift *= 10
	}

	return float64(int64(val*shift+0.5)) / shift
}
