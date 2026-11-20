package httpapi

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

// smsLogs mirrors the old Filament SMS Management > SMS Logs table, which reads
// the sms_logs audit table.
func (server *Server) smsAuditLogs(context fiber.Ctx) error {
	page, perPage := pagination(context)
	search := strings.TrimSpace(context.Query("search"))

	empty := func() error {
		return listResponse(context, []map[string]any{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	columns := tableColumns(server, "sms_logs")
	if len(columns) == 0 {
		return empty()
	}

	selects := []string{
		"id",
		textColumn(columns, "sms_logs", "created_at"),
		textColumn(columns, "sms_logs", "event"),
		textColumn(columns, "sms_logs", "phone"),
		textColumn(columns, "sms_logs", "gateway_name"),
		textColumn(columns, "sms_logs", "status"),
		textColumn(columns, "sms_logs", "duration_ms"),
		textColumn(columns, "sms_logs", "error_message"),
	}

	where := "WHERE 1 = 1"
	args := []any{}
	if search != "" {
		clauses := []string{}
		for _, column := range []string{"event", "phone", "gateway_name", "status", "error_message"} {
			if columns[column] {
				clauses = append(clauses, column+" LIKE ?")
				args = append(args, "%"+search+"%")
			}
		}
		if len(clauses) > 0 {
			where += " AND (" + strings.Join(clauses, " OR ") + ")"
		}
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM sms_logs "+where, args...).Scan(&total); err != nil {
		return empty()
	}

	order := " ORDER BY id DESC"
	if columns["created_at"] {
		order = " ORDER BY created_at DESC"
	}
	direction := "ASC"
	if strings.EqualFold(context.Query("dir"), "desc") {
		direction = "DESC"
	}
	switch context.Query("sort") {
	case "event":
		if columns["event"] {
			order = " ORDER BY event " + direction
		}
	case "phone":
		if columns["phone"] {
			order = " ORDER BY phone " + direction
		}
	case "status":
		if columns["status"] {
			order = " ORDER BY status " + direction
		}
	case "created_at":
		if columns["created_at"] {
			order = " ORDER BY created_at " + direction
		}
	}

	rows, err := server.db.Query(
		"SELECT "+strings.Join(selects, ", ")+" FROM sms_logs "+where+order+" LIMIT ? OFFSET ?",
		append(args, perPage, (page-1)*perPage)...,
	)
	if err != nil {
		return empty()
	}
	defer rows.Close()

	items := make([]map[string]any, 0)
	for rows.Next() {
		var id int64
		var createdAt sql.NullTime
		var event, phone, gatewayName, status, errorMessage sql.NullString
		var duration sql.NullInt64
		if err := rows.Scan(&id, &createdAt, &event, &phone, &gatewayName, &status, &duration, &errorMessage); err != nil {
			return empty()
		}
		item := map[string]any{"id": id}
		if createdAt.Valid {
			item["created_at"] = createdAt.Time.Format(time.RFC3339)
		}
		item["event"] = event.String
		item["phone"] = phone.String
		item["gateway_name"] = gatewayName.String
		item["status"] = status.String
		if duration.Valid {
			item["duration_ms"] = duration.Int64
		}
		item["error_message"] = errorMessage.String
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

// smsInquiry implements the old SMS Inquiry page: one message looked up by its
// internal id or by the upstream gateway message id, with its full ledger.
func (server *Server) smsInquiry(context fiber.Ctx) error {
	query := strings.TrimSpace(context.Query("q"))
	if query == "" {
		return context.JSON(fiber.Map{"data": nil})
	}

	columns := tableColumns(server, "messages")
	if len(columns) == 0 {
		return context.JSON(fiber.Map{"data": nil})
	}

	where := "WHERE m.id = ?"
	args := []any{}
	if id, err := strconv.ParseInt(query, 10, 64); err == nil && id > 0 {
		args = append(args, id)
	} else {
		where = "WHERE m.gateway_message_id = ?"
		args = append(args, query)
	}

	row := server.db.QueryRow(`
		SELECT m.id, COALESCE(u.name, 'Unknown'), m.`+"`to`"+`, COALESCE(s.sender_id, 'System'),
		       COALESCE(g.name, sg.name, 'Direct'), COALESCE(m.status, 'unknown'), COALESCE(m.parts, 1),
		       COALESCE(m.cost, 0), COALESCE(m.operator_cost, 0), COALESCE(m.primary_dipping_cost, 0),
		       COALESCE(m.body, ''), COALESCE(m.gateway_message_id, 'N/A'),
		       m.created_at, m.sent_at, m.delivered_at
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		LEFT JOIN gateways sg ON sg.id = s.gateway_id
		`+where+`
		LIMIT 1
	`, args...)

	var id int64
	var customer, recipient, sender, gateway, status, body, gatewayMessageID string
	var parts int
	var cost, operatorCost, dippingCost float64
	var createdAt time.Time
	var sentAt, deliveredAt sql.NullTime
	if err := row.Scan(&id, &customer, &recipient, &sender, &gateway, &status, &parts,
		&cost, &operatorCost, &dippingCost, &body, &gatewayMessageID, &createdAt, &sentAt, &deliveredAt); err != nil {
		return context.JSON(fiber.Map{"data": nil})
	}

	data := fiber.Map{
		"id":                 id,
		"gateway_message_id": gatewayMessageID,
		"customer":           customer,
		"to":                 recipient,
		"sender":             sender,
		"gateway":            gateway,
		"status":             status,
		"parts":              parts,
		"cost":               cost,
		"operator_cost":      operatorCost,
		"dipping_cost":       dippingCost,
		"body":               body,
		"created_at":         createdAt.Format("02 Jan 2006, 03:04:05 PM"),
		"sent_at":            "—",
		"delivered_at":       "—",
	}
	if sentAt.Valid {
		data["sent_at"] = sentAt.Time.Format("02 Jan 2006, 03:04:05 PM")
	}
	if deliveredAt.Valid {
		data["delivered_at"] = deliveredAt.Time.Format("02 Jan 2006, 03:04:05 PM")
	}

	return context.JSON(fiber.Map{"data": data})
}

// smsSenderOptions feeds the Sender ID select of the admin SMS dispatcher.
func (server *Server) smsSenderOptions(context fiber.Ctx) error {
	items := make([]map[string]any, 0)
	columns := tableColumns(server, "senders")
	if len(columns) == 0 {
		return context.JSON(fiber.Map{"data": items})
	}

	where := ""
	if columns["is_enabled"] {
		where = " WHERE is_enabled = TRUE"
	}
	rows, err := server.db.Query("SELECT id, sender_id, COALESCE(type, '') FROM senders" + where + " ORDER BY sender_id ASC LIMIT 500")
	if err != nil {
		return context.JSON(fiber.Map{"data": items})
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var senderID, senderType string
		if err := rows.Scan(&id, &senderID, &senderType); err == nil {
			items = append(items, map[string]any{"id": id, "sender_id": senderID, "type": senderType})
		}
	}

	return context.JSON(fiber.Map{"data": items})
}

type sendSMSRequest struct {
	SenderID int64  `json:"sender_id"`
	To       string `json:"to"`
	Message  string `json:"message"`
}

// adminSendSMS mirrors the old Admin SMS Dispatcher page.
func (server *Server) adminSendSMS(context fiber.Ctx) error {
	var payload sendSMSRequest
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}

	cleaned := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, payload.To)

	var e164 string
	switch {
	case len(cleaned) == 11 && strings.HasPrefix(cleaned, "01"):
		e164 = "+88" + cleaned
	case len(cleaned) == 13 && strings.HasPrefix(cleaned, "880"):
		e164 = "+" + cleaned
	default:
		e164 = "+880" + strings.TrimLeft(cleaned, "0")
	}
	if strings.TrimSpace(payload.Message) == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The message field is required."})
	}

	columns := tableColumns(server, "messages")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Messages are unavailable right now."})
	}

	parts := int(math.Ceil(float64(max(len(payload.Message), 1)) / 160.0))
	claims := superman(context)

	names := []string{"user_id", "to", "body", "parts", "cost", "status"}
	values := []any{claims.AdminID, e164, payload.Message, parts, 0.0, "pending"}
	if payload.SenderID > 0 && columns["sender_id"] {
		names = append(names, "sender_id")
		values = append(values, payload.SenderID)
	}
	if columns["created_at"] {
		names = append(names, "created_at")
		values = append(values, time.Now())
	}
	if columns["updated_at"] {
		names = append(names, "updated_at")
		values = append(values, time.Now())
	}

	result, err := server.db.Exec(
		"INSERT INTO messages ("+strings.Join(names, ", ")+") VALUES ("+inPlaceholders(len(values))+")",
		values...,
	)
	if err != nil {
		return databaseError(context, err)
	}
	id, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "SMS dispatched to " + e164 + ".",
		"data":    fiber.Map{"id": id, "to": e164},
	})
}

// requeueMessage is the old Failed SMS "Requeue" action.
func (server *Server) requeueMessage(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Message not found."})
	}

	columns := tableColumns(server, "messages")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Messages are unavailable right now."})
	}

	sets := []string{"status = 'pending'"}
	if columns["failed_reason"] {
		sets = append(sets, "failed_reason = NULL")
	}
	if columns["updated_at"] {
		sets = append(sets, "updated_at = NOW()")
	}
	if _, err := server.db.Exec("UPDATE messages SET "+strings.Join(sets, ", ")+" WHERE id = ?", id); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Message requeued to the pending queue."})
}

// pauseBatch / resumeBatch back the Scheduled SMS row actions.
func (server *Server) pauseBatch(context fiber.Ctx) error {
	return server.setBatchScheduleStatus(context, "paused")
}

func (server *Server) resumeBatch(context fiber.Ctx) error {
	return server.setBatchScheduleStatus(context, "scheduled")
}

func (server *Server) setBatchScheduleStatus(context fiber.Ctx, status string) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Batch not found."})
	}

	result, err := server.db.Exec(
		"UPDATE batches SET status = ?, updated_at = NOW() WHERE id = ? AND status IN ('scheduled', 'paused')",
		status, id,
	)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusConflict).JSON(fiber.Map{"message": "This scheduled message can no longer be changed."})
	}

	if status == "paused" {
		return context.JSON(fiber.Map{"message": "The scheduled message has been paused."})
	}

	return context.JSON(fiber.Map{"message": "The message schedule is now active."})
}

// deleteScheduledBatch removes a scheduled or paused batch.
func (server *Server) deleteScheduledBatch(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Batch not found."})
	}

	if _, err := server.db.Exec("DELETE FROM batches WHERE id = ? AND status IN ('scheduled', 'paused')", id); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "The scheduled message has been removed successfully."})
}
