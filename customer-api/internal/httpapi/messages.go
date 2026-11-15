package httpapi

import (
	"bytes"
	stdctx "context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
		totalToday, succToday, delivToday, sentToday, failToday int
		costToday                                               float64
		total7d, succ7d, deliv7d, sent7d, fail7d                int
		cost7d                                                  float64
		total30d, succ30d, deliv30d, sent30d, fail30d           int
		cost30d                                                 float64
		totalAll, succAll, delivAll, sentAll, failAll           int
		costAll                                                 float64
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

	where, args := buildCustomerMessagesWhere(claims.UserID, context)

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := server.db.QueryRow("SELECT COUNT(*) FROM messages m LEFT JOIN senders s ON s.id = m.sender_id "+where, countArgs...).Scan(&total); err != nil {
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
		       m.created_at, m.sent_at, m.delivered_at, m.failed_reason, COALESCE(m.operator_name, ''),
		       COALESCE(NULLIF(m.sms_type, ''), CASE WHEN m.batch_id IS NULL THEN 'Transactional' ELSE 'Promotional' END),
		       COALESCE(m.channel, ''), COALESCE(m.tariff_name, ''), COALESCE(m.source_ip, ''),
		       COALESCE(m.request_id, ''), COALESCE(m.delivery_status, ''), COALESCE(m.source, 'web'),
		       COALESCE(
		           (SELECT t.balance_before FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ) as previous_balance,
		       COALESCE(
		           (SELECT t.balance_after FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ) as next_balance
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
		var id, parts int
		var senderID, to, body, status, operatorName, smsType, channel, tariffName, sourceIP, requestID, deliveryStatus, source string
		var cost, prevBalance, nextBalance float64
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		var failedReason *string

		if err := rows.Scan(&id, &senderID, &to, &body, &status, &cost, &parts,
			&createdAt, &sentAt, &deliveredAt, &failedReason, &operatorName,
			&smsType, &channel, &tariffName, &sourceIP, &requestID, &deliveryStatus, &source,
			&prevBalance, &nextBalance); err != nil {
			continue
		}

		op := operatorName
		if op == "" {
			op = operatorFromNumber(to)
		}

		items = append(items, fiber.Map{
			"id":               id,
			"msg_id":           fmt.Sprintf("MSG-%06d", id),
			"sender_id":        senderID,
			"to":               to,
			"body":             body,
			"status":           status,
			"cost":             fmt.Sprintf("%.4f", cost),
			"parts":            parts,
			"sms_type":         smsType,
			"channel":          channel,
			"tariff_name":      tariffName,
			"source_ip":        sourceIP,
			"request_id":       requestID,
			"delivery_status":  deliveryStatus,
			"source":           source,
			"previous_balance": fmt.Sprintf("%.4f", prevBalance),
			"next_balance":     fmt.Sprintf("%.4f", nextBalance),
			"created_at":       createdAt.Format(time.RFC3339),
			"submitted_at":     createdAt.Format(time.RFC3339),
			"sent_at":          apiTimePtr(sentAt),
			"delivered_at":     apiTimePtr(deliveredAt),
			"error_message":    failedReason,
			"operator":         op,
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) messageDetail(context fiber.Ctx) error {
	claims := customer(context)
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Message not found."})
	}

	var (
		batchID                        sql.NullInt64
		senderDBID                     sql.NullInt64
		senderID, to, body, status     string
		operatorName, failedReason     *string
		deliveryStatus, source         sql.NullString
		smsType, channel, tariffName   sql.NullString
		sourceIP, requestID            sql.NullString
		parts                          int
		cost, prevBalance, nextBalance float64
		createdAt, updatedAt           time.Time
		sentAt, deliveredAt, failedAt  *time.Time
	)
	err = server.db.QueryRow(`
		SELECT m.batch_id, m.sender_id, m.to, m.body, m.status, m.parts, m.cost,
		       COALESCE(s.sender_id, ''), m.operator_name, m.failed_reason, m.delivery_status,
		       m.source, m.sms_type, m.channel, m.tariff_name, m.source_ip, m.request_id,
		       COALESCE(
		           (SELECT t.balance_before FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ),
		       COALESCE(
		           (SELECT t.balance_after FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ),
		       m.created_at, m.updated_at, m.sent_at, m.delivered_at, m.failed_at
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		WHERE m.id = ? AND m.user_id = ?
	`, id, claims.UserID).Scan(&batchID, &senderDBID, &to, &body, &status, &parts, &cost,
		&senderID, &operatorName, &failedReason, &deliveryStatus, &source,
		&smsType, &channel, &tariffName, &sourceIP, &requestID, &prevBalance, &nextBalance,
		&createdAt, &updatedAt, &sentAt, &deliveredAt, &failedAt)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Message not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	op := ""
	if operatorName != nil {
		op = *operatorName
	}
	if op == "" {
		op = operatorFromNumber(to)
	}
	reason := ""
	if failedReason != nil {
		reason = *failedReason
	}
	resolvedType := nullStringValue(smsType)
	if resolvedType == "" {
		if batchID.Valid {
			resolvedType = "Promotional"
		} else {
			resolvedType = "Transactional"
		}
	}

	return context.JSON(fiber.Map{"data": fiber.Map{
		"id":               id,
		"msg_id":           fmt.Sprintf("MSG-%06d", id),
		"batch_id":         nullableInt64(batchID),
		"sender_db_id":     nullableInt64(senderDBID),
		"sender_id":        senderID,
		"to":               to,
		"body":             body,
		"status":           status,
		"parts":            parts,
		"cost":             fmt.Sprintf("%.4f", cost),
		"operator":         op,
		"delivery_status":  nullStringValue(deliveryStatus),
		"source":           nullStringValue(source),
		"sms_type":         resolvedType,
		"channel":          nullStringValue(channel),
		"tariff_name":      nullStringValue(tariffName),
		"source_ip":        nullStringValue(sourceIP),
		"request_id":       nullStringValue(requestID),
		"previous_balance": fmt.Sprintf("%.4f", prevBalance),
		"next_balance":     fmt.Sprintf("%.4f", nextBalance),
		"error_message":    reason,
		"submitted_at":     createdAt.Format(time.RFC3339),
		"created_at":       createdAt.Format(time.RFC3339),
		"updated_at":       updatedAt.Format(time.RFC3339),
		"sent_at":          apiTimePtr(sentAt),
		"delivered_at":     apiTimePtr(deliveredAt),
		"failed_at":        apiTimePtr(failedAt),
	}})
}

func nullableInt64(value sql.NullInt64) any {
	if value.Valid {
		return value.Int64
	}
	return nil
}

func nullStringValue(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func (server *Server) exportMessages(context fiber.Ctx) error {
	claims := customer(context)

	where, args := buildCustomerMessagesWhere(claims.UserID, context)

	format := strings.ToLower(strings.TrimSpace(context.Query("format", "csv")))
	limitClause := ""
	if format == "pdf" {
		limitClause = " LIMIT 500"
	}

	rows, err := server.db.Query(`
		SELECT m.id, m.to, COALESCE(s.sender_id, ''), m.body, m.parts, m.cost, m.status,
		       COALESCE(m.failed_reason, ''), COALESCE(m.operator_name, ''), m.created_at, m.sent_at, m.delivered_at
		FROM messages m LEFT JOIN senders s ON s.id = m.sender_id
		`+where+`
		ORDER BY m.id DESC`+limitClause, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	var (
		buffer  bytes.Buffer
		writer  *csv.Writer
		pdfRows []messagePDFRow
	)
	if format != "pdf" {
		writer = csv.NewWriter(&buffer)
		_ = writer.Write([]string{
			"Message ID", "Recipient Number", "Sender ID", "Content", "SMS Count",
			"Cost (BDT)", "Status", "Failure Reason", "Operator", "Submit Time", "Delivery Time",
		})
	}

	for rows.Next() {
		var id int64
		var parts int
		var to, senderID, body, status, reason, operator string
		var cost float64
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		if rows.Scan(&id, &to, &senderID, &body, &parts, &cost, &status, &reason, &operator, &createdAt, &sentAt, &deliveredAt) != nil {
			continue
		}
		if operator == "" {
			operator = operatorFromNumber(to)
		}
		submitted := createdAt.Format("2006-01-02 15:04:05")
		if sentAt != nil {
			submitted = sentAt.Format("2006-01-02 15:04:05")
		}
		delivered := ""
		if deliveredAt != nil {
			delivered = deliveredAt.Format("2006-01-02 15:04:05")
		}
		if format == "pdf" {
			pdfRows = append(pdfRows, messagePDFRow{
				id:        id,
				to:        to,
				senderID:  senderID,
				body:      body,
				parts:     parts,
				cost:      cost,
				status:    status,
				reason:    reason,
				operator:  operator,
				submitted: submitted,
				delivered: delivered,
			})
			continue
		}
		_ = writer.Write([]string{
			fmt.Sprintf("MSG-%06d", id), to, senderID, body, strconv.Itoa(parts),
			fmt.Sprintf("%.4f", cost), status, reason, operator, submitted, delivered,
		})
	}

	stamp := time.Now().Format("2006-01-02-150405")
	if format == "pdf" {
		context.Set("Content-Type", "application/pdf")
		context.Set("Content-Disposition", "attachment; filename=sent-messages-"+stamp+".pdf")
		return context.Send(buildMessagesPDF(pdfRows))
	}

	writer.Flush()
	context.Set("Content-Type", "text/csv; charset=utf-8")
	context.Set("Content-Disposition", "attachment; filename=sent-messages-"+stamp+".csv")

	return context.Send(buffer.Bytes())
}

// ─────────────────────────────────────────────────────────────────────────────
// Recent Recipients
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) recentRecipients(context fiber.Ctx) error {
	claims := customer(context)

	var phones []string

	// 1. Pull from last 100 outgoing messages
	rows, err := server.db.Query(`
		SELECT DISTINCT `+"`to`"+` FROM messages
		WHERE user_id = ? AND `+"`to`"+` IS NOT NULL AND `+"`to`"+` != ''
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

func (server *Server) customerRequiresApproval(userID int64) bool {
	var autoApproved bool
	if err := server.db.QueryRow("SELECT COALESCE(sms_auto_approved, 1) FROM users WHERE id = ?", userID).Scan(&autoApproved); err != nil {
		return true
	}

	return !autoApproved
}

func senderTypeIsMasking(senderType string) bool {
	norm := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(senderType)), "_", "-")
	switch norm {
	case "", "numeric", "non-masking", "number":
		return false
	default:
		return true
	}
}

func nullableInt64Ptr(v *int64) any {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}

func (server *Server) validateOwnedTemplateID(userID int64, templateID *int64) *int64 {
	if templateID == nil || *templateID <= 0 {
		return nil
	}
	var id int64
	if err := server.db.QueryRow(
		"SELECT id FROM templates WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		*templateID, userID,
	).Scan(&id); err != nil {
		return nil
	}
	return &id
}

func (server *Server) resolveSenderAndRates(ctx stdctx.Context, userID, senderID int64) (senderLabel string, isMasking bool, maxRate float64, errMsg string, httpCode int) {
	var senderType string
	err := server.db.QueryRowContext(ctx, `
		SELECT s.sender_id, s.type FROM senders s
		LEFT JOIN sender_user su ON su.sender_id = s.id
		WHERE (su.user_id = ? OR s.requested_by = ?) AND s.id = ? AND s.is_enabled = 1 AND s.deleted_at IS NULL
		LIMIT 1
	`, userID, userID, senderID).Scan(&senderLabel, &senderType)
	if err == sql.ErrNoRows {
		return "", false, 0, "Selected Sender ID is not available.", http.StatusUnprocessableEntity
	}
	if err != nil {
		return "", false, 0, "Database error resolving Sender ID.", http.StatusInternalServerError
	}

	resolver, err := billing.ForUser(ctx, server.db, userID)
	if err != nil {
		return "", false, 0, "Unable to resolve your tariff rates.", http.StatusInternalServerError
	}

	isMasking = senderTypeIsMasking(senderType)
	rates := resolver.RatesForUser(isMasking)
	if len(rates) == 0 {
		kind := "Non-masking"
		if isMasking {
			kind = "Masking"
		}
		planMsg := " in your tariff plan"
		if p := resolver.PlanName(); p != "" {
			planMsg = fmt.Sprintf(" in your assigned tariff plan [%s]", p)
		}
		return "", false, 0, fmt.Sprintf("%s rate not configured%s before sending messages.", kind, planMsg), http.StatusUnprocessableEntity
	}

	for _, r := range rates {
		if r > maxRate {
			maxRate = r
		}
	}
	return senderLabel, isMasking, maxRate, "", 0
}

func (server *Server) sendMessage(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID   int64  `json:"sender_id"`
		TemplateID *int64 `json:"template_id"`
		Phones     any    `json:"phones"`
		Body       string `json:"body"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	body := strings.TrimSpace(req.Body)
	if req.SenderID == 0 || body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "sender_id and body are required."})
	}

	phones := normalizePhones(req.Phones)
	if len(phones) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "At least one valid phone number is required."})
	}
	if len(phones) > 1 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Quick Message only supports a single recipient. Use Bulk SMS for multiple recipients."})
	}

	_, _, maxRate, errMsg, code := server.resolveSenderAndRates(context.Context(), claims.UserID, req.SenderID)
	if code != 0 {
		return context.Status(code).JSON(fiber.Map{"message": errMsg})
	}

	parts, _ := billing.CalculateParts(body)
	estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(len(phones)), 4)

	var balance float64
	_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ?", claims.UserID).Scan(&balance)
	if balance < estimatedCost {
		return context.Status(http.StatusPaymentRequired).JSON(fiber.Map{
			"message": "Insufficient balance for this message.",
			"balance": balance,
		})
	}

	if !server.acquireSubmitLock(claims.UserID, "quick_sms") {
		return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
			"message": "Your message is already being submitted. Please do not double-click.",
		})
	}

	recipients := make([]any, len(phones))
	for i, p := range phones {
		recipients[i] = map[string]any{"phone": p, "values": map[string]string{}}
	}
	srcConfigBytes, _ := json.Marshal(map[string]any{"recipients": recipients, "ip": context.IP()})

	// Quick SMS is single-recipient and bypasses Bulk SMS approval (matching CreateQuickMessage.php:205).
	requiresApproval := len(phones) > 1 && server.requiresApprovalForCampaign(claims.UserID, req.TemplateID, body)
	batchStatus := "processing"
	var approvalValue any
	if requiresApproval {
		batchStatus = "pending"
		approvalValue = "pending"
	}

	templateID := server.validateOwnedTemplateID(claims.UserID, req.TemplateID)
	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, template_id, body, source_type, source_ip, source_config, status, approval, total_recipients, valid_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, ?, ?, ?, 0, NOW(), NOW())
	`, claims.UserID, req.SenderID, nullableInt64Ptr(templateID), body, context.IP(), string(srcConfigBytes), batchStatus, approvalValue, len(phones))
	if err != nil {
		return databaseError(context, err)
	}

	batchID, _ := result.LastInsertId()

	if !requiresApproval {
		if len(phones) <= 5 {
			_ = server.batchProcessor.ProcessBatch(stdctx.Background(), batchID)
		} else {
			go func(bid int64) {
				_ = server.batchProcessor.ProcessBatch(stdctx.Background(), bid)
			}(batchID)
		}
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":                batchID,
			"status":            batchStatus,
			"approval":          approvalValue,
			"requires_approval": requiresApproval,
			"recipients":        len(phones),
		},
	})
}

func (server *Server) scheduleMessage(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID        int64  `json:"sender_id"`
		TemplateID      *int64 `json:"template_id"`
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

	body := strings.TrimSpace(req.Body)
	if req.SenderID == 0 || body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Sender ID and message content are required."})
	}

	phones := normalizePhones(req.Phones)
	if len(phones) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "At least one valid phone number is required."})
	}
	if len(phones) > 1 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Quick Message only supports a single recipient. Use Bulk SMS for multiple recipients."})
	}

	scheduledAt, err := parseScheduledTime(req.ScheduledDate, req.ScheduledHour, req.ScheduledMinute, req.ScheduledPeriod)
	if err != nil || scheduledAt.Before(time.Now().Add(-time.Minute)) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The selected schedule time has already passed."})
	}

	_, _, maxRate, errMsg, code := server.resolveSenderAndRates(context.Context(), claims.UserID, req.SenderID)
	if code != 0 {
		return context.Status(code).JSON(fiber.Map{"message": errMsg})
	}

	parts, _ := billing.CalculateParts(body)
	estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(len(phones)), 4)

	var balance float64
	_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ?", claims.UserID).Scan(&balance)
	if balance < estimatedCost {
		return context.Status(http.StatusPaymentRequired).JSON(fiber.Map{
			"message": "Insufficient balance to schedule this message.",
			"balance": balance,
		})
	}

	now := time.Now()
	// Quick SMS scheduling is single-recipient and bypasses Bulk SMS approval (matching CreateQuickMessage.php:406).
	requiresApproval := len(phones) > 1 && server.requiresApprovalForCampaign(claims.UserID, req.TemplateID, body)
	var approvalValue any
	if requiresApproval {
		approvalValue = "pending"
	}

	templateID := server.validateOwnedTemplateID(claims.UserID, req.TemplateID)
	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, template_id, body, source_type, source_ip, source_config, status, approval, total_recipients, valid_count, scheduled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'manual', ?, ?, 'scheduled', ?, ?, ?, ?, ?, ?)
	`, claims.UserID, req.SenderID, nullableInt64Ptr(templateID), body, context.IP(), buildSourceConfig(phones), approvalValue, len(phones), len(phones), scheduledAt, now, now)
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

type bulkRecipientPayload struct {
	Phone  string            `json:"phone"`
	Values map[string]string `json:"values"`
}

func (server *Server) bulkSend(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID        int64                  `json:"sender_id"`
		TemplateID      *int64                 `json:"template_id"`
		Body            string                 `json:"body"`
		Source          string                 `json:"source"`
		Phones          any                    `json:"phones"`
		Recipients      []bulkRecipientPayload `json:"recipients"`
		Groups          []int64                `json:"groups"`
		TempPath        string                 `json:"temp_path"`
		Mapping         map[string]any         `json:"mapping"`
		HasHeaderRow    *bool                  `json:"has_header_row"`
		FileTotalRows   int                    `json:"file_total_rows"`
		Dedup           bool                   `json:"dedup"`
		ScheduledDate   string                 `json:"scheduled_date"`
		ScheduledHour   string                 `json:"scheduled_hour"`
		ScheduledMinute string                 `json:"scheduled_minute"`
		ScheduledPeriod string                 `json:"scheduled_period"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	body := strings.TrimSpace(req.Body)
	if req.SenderID == 0 || body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Sender ID and message content are required."})
	}

	source := strings.ToLower(strings.TrimSpace(req.Source))
	if source == "" {
		source = "manual"
	}

	var (
		recipientCount int
		sourceConfig   map[string]any
		cleanTempRel   string
	)

	switch source {
	case "manual":
		var recEntries []map[string]any
		if len(req.Recipients) > 0 {
			for _, r := range req.Recipients {
				vals := r.Values
				if vals == nil {
					vals = map[string]string{}
				}
				for _, p := range normalizePhones(r.Phone) {
					recEntries = append(recEntries, map[string]any{"phone": p, "values": vals})
				}
			}
		} else {
			for _, p := range normalizePhones(req.Phones) {
				recEntries = append(recEntries, map[string]any{"phone": p, "values": map[string]string{}})
			}
		}
		recipientCount = len(recEntries)
		sourceConfig = map[string]any{"recipients": recEntries, "dedup": req.Dedup, "ip": context.IP()}

	case "group":
		if len(req.Groups) == 0 {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Please select at least one contact group."})
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(req.Groups)), ",")
		args := []any{claims.UserID}
		for _, gid := range req.Groups {
			args = append(args, gid)
		}
		_ = server.db.QueryRow(fmt.Sprintf(`
			SELECT COUNT(DISTINCT c.id) FROM contacts c
			JOIN contact_group cg ON cg.contact_id = c.id
			WHERE c.user_id = ? AND cg.group_id IN (%s) AND c.deleted_at IS NULL
		`, placeholders), args...).Scan(&recipientCount)
		sourceConfig = map[string]any{"group_ids": req.Groups, "dedup": req.Dedup, "ip": context.IP()}

	case "upload":
		rel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(strings.TrimSpace(req.TempPath))), "/")
		if rel == "" || !strings.HasPrefix(rel, "uploads/recipient-files/") {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Please upload a valid recipient file first."})
		}
		absSrc := filepath.Join(server.storageRoot(), filepath.FromSlash(rel))
		if _, err := os.Stat(absSrc); err != nil {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Uploaded file expired or missing. Please re-upload."})
		}
		cleanTempRel = rel
		recipientCount = req.FileTotalRows
		if recipientCount <= 0 {
			_, _, totalRows, _, _ := parseUploadedFilePreview(absSrc, 1)
			recipientCount = totalRows
		}
		hasHdr := true
		if req.HasHeaderRow != nil {
			hasHdr = *req.HasHeaderRow
		}
		mapping := req.Mapping
		if mapping == nil {
			mapping = map[string]any{}
		}
		sourceConfig = map[string]any{
			"disk":           "local",
			"file_path":      rel,
			"mapping":        mapping,
			"has_header_row": hasHdr,
			"dedup":          req.Dedup,
			"ip":             context.IP(),
		}

	default:
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid recipient source."})
	}

	if recipientCount == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "No recipients found for this bulk campaign."})
	}

	_, _, maxRate, errMsg, code := server.resolveSenderAndRates(context.Context(), claims.UserID, req.SenderID)
	if code != 0 {
		return context.Status(code).JSON(fiber.Map{"message": errMsg})
	}

	var scheduledAt *time.Time
	if req.ScheduledDate != "" && req.ScheduledHour != "" {
		t, err := parseScheduledTime(req.ScheduledDate, req.ScheduledHour, req.ScheduledMinute, req.ScheduledPeriod)
		if err != nil || t.Before(time.Now().Add(-time.Minute)) {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The selected schedule time has already passed. Please select a time in the future."})
		}
		scheduledAt = &t
	}

	requiresApproval := server.requiresApprovalForCampaign(claims.UserID, req.TemplateID, body)

	// When admin approval is required, do not halt for balance at submission time (matching CreateSmartMessage.php:683).
	if !requiresApproval {
		parts, _ := billing.CalculateParts(body)
		estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(recipientCount), 4)
		var balance float64
		_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ?", claims.UserID).Scan(&balance)
		if balance < estimatedCost {
			return context.Status(http.StatusPaymentRequired).JSON(fiber.Map{
				"message": fmt.Sprintf("Insufficient balance. This batch needs up to ৳%.2f.", estimatedCost),
				"balance": balance,
			})
		}
	}

	if !server.acquireSubmitLock(claims.UserID, "smart_sms") {
		return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
			"message": "Your message is already being submitted. Please do not double-click.",
		})
	}

	now := time.Now()
	batchStatus := "processing"
	if scheduledAt != nil {
		batchStatus = "scheduled"
	} else if requiresApproval {
		batchStatus = "pending"
	}
	var approvalValue any
	if requiresApproval {
		approvalValue = "pending"
	}

	srcConfigBytes, _ := json.Marshal(sourceConfig)
	templateID := server.validateOwnedTemplateID(claims.UserID, req.TemplateID)

	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, template_id, body, source_type, source_ip, source_config, status, approval, total_recipients, valid_count, scheduled_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, claims.UserID, req.SenderID, nullableInt64Ptr(templateID), body, source, context.IP(), string(srcConfigBytes), batchStatus, approvalValue, recipientCount, recipientCount, scheduledAt, now, now)
	if err != nil {
		return databaseError(context, err)
	}

	batchID, _ := result.LastInsertId()

	if cleanTempRel != "" {
		absSrc := filepath.Join(server.storageRoot(), filepath.FromSlash(cleanTempRel))
		ext := strings.ToLower(filepath.Ext(cleanTempRel))
		if ext == "" {
			ext = ".csv"
		}
		destRel := fmt.Sprintf("batches/%d/source%s", batchID, ext)
		absDest := filepath.Join(server.storageRoot(), filepath.FromSlash(destRel))
		_ = os.MkdirAll(filepath.Dir(absDest), 0755)
		if moveErr := os.Rename(absSrc, absDest); moveErr == nil {
			sourceConfig["file_path"] = destRel
			updatedCfg, _ := json.Marshal(sourceConfig)
			_, _ = server.db.Exec("UPDATE batches SET source_config = ? WHERE id = ?", string(updatedCfg), batchID)
		}
	}

	if scheduledAt == nil && !requiresApproval {
		if recipientCount <= 5 {
			_ = server.batchProcessor.ProcessBatch(stdctx.Background(), batchID)
		} else {
			go func(bid int64) {
				_ = server.batchProcessor.ProcessBatch(stdctx.Background(), bid)
			}(batchID)
		}
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":                batchID,
			"status":            batchStatus,
			"approval":          approvalValue,
			"requires_approval": requiresApproval,
			"recipients":        recipientCount,
			"scheduled_at":      apiTimePtr(scheduledAt),
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
	conditions := []string{"m.user_id = ?", "m.batch_id IS NOT NULL"}
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
		       COALESCE(
		           (SELECT t.balance_after FROM transactions t
		            WHERE t.user_id = m.user_id
		              AND ((t.transactionable_type IN ('message_batch', 'App\\Models\\Batch') AND t.transactionable_id = m.batch_id)
		                OR (t.transactionable_type IN ('message', 'App\\Models\\Message') AND t.transactionable_id = m.id))
		            ORDER BY t.id DESC LIMIT 1),
		           0.00
		       ) as next_balance,
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
		var cost, prevBalance, nextBalance float64
		var createdAt time.Time
		var sentAt, deliveredAt *time.Time
		var failedReason *string
		var smsType string

		if err := rows.Scan(&id, &to, &body, &senderID, &status, &cost, &parts,
			&createdAt, &sentAt, &deliveredAt, &failedReason, &prevBalance, &nextBalance, &smsType); err != nil {
			continue
		}

		items = append(items, fiber.Map{
			"id":               id,
			"to":               to,
			"body":             body,
			"sender_id":        senderID,
			"status":           status,
			"cost":             fmt.Sprintf("%.4f", cost),
			"parts":            parts,
			"previous_balance": fmt.Sprintf("%.2f", prevBalance),
			"next_balance":     fmt.Sprintf("%.2f", nextBalance),
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

	var batchID, totalRec, validCnt, invalidCnt, sentCnt, deliveredCnt, failedCnt int
	var templateID *int
	var senderID, status, sourceType, body string
	var approval, failedReportPath *string
	var cost float64
	var scheduledAt, startedAt, completedAt *time.Time
	var createdAt time.Time

	err := server.db.QueryRow(`
		SELECT b.id, COALESCE(s.sender_id, '—'), b.template_id, b.status, b.approval, b.source_type, b.body,
		       b.total_recipients, COALESCE(b.valid_count, 0), COALESCE(b.invalid_count, 0), b.failed_report_path,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('sent', 'delivered')) as sent_count,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status = 'delivered') as delivered_count,
		       (SELECT COUNT(*) FROM messages m WHERE m.batch_id = b.id AND m.status IN ('failed', 'dlr_failed')) as failed_count,
		       (SELECT COALESCE(SUM(m.cost), 0) FROM messages m WHERE m.batch_id = b.id) as cost,
		       b.scheduled_at, b.started_at, b.completed_at, b.created_at
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		WHERE b.id = ? AND b.user_id = ?
	`, id, claims.UserID).Scan(
		&batchID, &senderID, &templateID, &status, &approval, &sourceType, &body,
		&totalRec, &validCnt, &invalidCnt, &failedReportPath,
		&sentCnt, &deliveredCnt, &failedCnt, &cost,
		&scheduledAt, &startedAt, &completedAt, &createdAt,
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
		LIMIT 200
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
				"id":                 batchID,
				"sender_id":          senderID,
				"template_id":        templateID,
				"status":             status,
				"approval":           approval,
				"source_type":        sourceType,
				"body":               body,
				"total_recipients":   totalRec,
				"valid_count":        validCnt,
				"invalid_count":      invalidCnt,
				"failed_report_path": failedReportPath,
				"sent_count":         sentCnt,
				"delivered_count":    deliveredCnt,
				"failed_count":       failedCnt,
				"cost":               fmt.Sprintf("%.2f", cost),
				"scheduled_at":       apiTimePtr(scheduledAt),
				"started_at":         apiTimePtr(startedAt),
				"completed_at":       apiTimePtr(completedAt),
				"created_at":         createdAt.Format(time.RFC3339),
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

func (server *Server) bulkDeleteScheduledMessages(context fiber.Ctx) error {
	claims := customer(context)
	var req struct {
		Ids []int64 `json:"ids"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil || len(req.Ids) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Please select at least one scheduled message."})
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(req.Ids)), ",")
	args := make([]any, 0, len(req.Ids)+1)
	args = append(args, claims.UserID)
	for _, id := range req.Ids {
		args = append(args, id)
	}

	res, err := server.db.Exec(
		fmt.Sprintf("DELETE FROM batches WHERE user_id = ? AND status IN ('scheduled', 'paused') AND id IN (%s)", placeholders),
		args...,
	)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	return context.JSON(fiber.Map{
		"message": fmt.Sprintf("%d scheduled message(s) deleted.", affected),
		"deleted": affected,
	})
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
