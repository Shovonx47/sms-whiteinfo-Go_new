package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type campaignRow struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Customer    string  `json:"customer"`
	SenderID    *string `json:"sender_id"`
	Status      string  `json:"status"`
	ScheduledAt *string `json:"scheduled_at"`
	CompletedAt *string `json:"completed_at"`
	CreatedAt   string  `json:"created_at"`
}

// campaigns mirrors the old Filament Campaigns List / Campaign Reports tables.
func (server *Server) campaigns(context fiber.Ctx) error {
	page, perPage := pagination(context)
	search := strings.TrimSpace(context.Query("search"))

	empty := func() error {
		return listResponse(context, []campaignRow{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	columns := tableColumns(server, "campaigns")
	if len(columns) == 0 {
		return empty()
	}

	where := "WHERE 1 = 1"
	args := []any{}
	if columns["deleted_at"] {
		where = "WHERE c.deleted_at IS NULL"
	}
	statusFilter := strings.TrimSpace(context.Query("statuses"))
	if statusFilter == "" {
		statusFilter = strings.TrimSpace(context.Query("status"))
	}
	if statusFilter != "" && statusFilter != "all" && columns["status"] {
		placeholders := []string{}
		for _, value := range strings.Split(statusFilter, ",") {
			if value = strings.TrimSpace(value); value != "" {
				placeholders = append(placeholders, "?")
				args = append(args, value)
			}
		}
		if len(placeholders) > 0 {
			where += " AND c.status IN (" + strings.Join(placeholders, ", ") + ")"
		}
	}
	if search != "" {
		where += " AND (c.name LIKE ?" + func() string {
			if tableExists(server, "users") {
				return " OR u.name LIKE ?"
			}
			return ""
		}() + ")"
		term := "%" + search + "%"
		args = append(args, term)
		if tableExists(server, "users") {
			args = append(args, term)
		}
	}

	userJoin := ""
	customerExpr := "''"
	if tableExists(server, "users") {
		userJoin = " LEFT JOIN users u ON u.id = c.user_id"
		customerExpr = "COALESCE(u.name, '')"
	}
	senderJoin := ""
	senderExpr := "NULL"
	if tableExists(server, "senders") {
		senderJoin = " LEFT JOIN senders s ON s.id = c.sender_id"
		senderExpr = "s.sender_id"
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM campaigns c"+userJoin+" "+where, args...).Scan(&total); err != nil {
		return empty()
	}

	rows, err := server.db.Query(`
		SELECT c.id, COALESCE(c.name, ''), `+customerExpr+`, `+senderExpr+`,
		       COALESCE(c.status, 'draft'), `+textColumn(columns, "c", "scheduled_at")+`,
		       `+textColumn(columns, "c", "completed_at")+`, `+textColumn(columns, "c", "created_at")+`
		FROM campaigns c`+userJoin+senderJoin+`
		`+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return empty()
	}
	defer rows.Close()

	items := make([]campaignRow, 0)
	for rows.Next() {
		var item campaignRow
		var sender sql.NullString
		var scheduledAt, completedAt, createdAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Customer, &sender, &item.Status, &scheduledAt, &completedAt, &createdAt); err != nil {
			return empty()
		}
		if sender.Valid {
			value := sender.String
			item.SenderID = &value
		}
		if scheduledAt.Valid {
			value := scheduledAt.Time.Format(time.RFC3339)
			item.ScheduledAt = &value
		}
		if completedAt.Valid {
			value := completedAt.Time.Format(time.RFC3339)
			item.CompletedAt = &value
		}
		if createdAt.Valid {
			item.CreatedAt = createdAt.Time.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type campaignPayload struct {
	Name     string `json:"name"`
	SenderID int64  `json:"sender_id"`
	Message  string `json:"message"`
}

// createCampaign mirrors the old Create Campaign page, which stores a draft.
func (server *Server) createCampaign(context fiber.Ctx) error {
	var payload campaignPayload
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request payload."})
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The name field is required."})
	}

	columns := tableColumns(server, "campaigns")
	if len(columns) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Campaigns are unavailable right now."})
	}

	claims := superman(context)
	names := []string{"user_id", "name", "status"}
	values := []any{claims.AdminID, payload.Name, "draft"}
	if payload.SenderID > 0 && columns["sender_id"] {
		names = append(names, "sender_id")
		values = append(values, payload.SenderID)
	}
	if columns["body"] && payload.Message != "" {
		names = append(names, "body")
		values = append(values, payload.Message)
	}
	if columns["created_at"] {
		names = append(names, "created_at")
		values = append(values, time.Now())
	}
	if columns["updated_at"] {
		names = append(names, "updated_at")
		values = append(values, time.Now())
	}

	result, err := server.db.Exec("INSERT INTO campaigns ("+strings.Join(names, ", ")+") VALUES ("+inPlaceholders(len(values))+")", values...)
	if err != nil {
		return databaseError(context, err)
	}
	id, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Campaign '" + payload.Name + "' has been created in draft state.",
		"data":    fiber.Map{"id": id},
	})
}

// alertsDashboard mirrors the old Alerts Overview page metrics.
func (server *Server) alertsDashboard(context fiber.Ctx) error {
	count := func(query string, args ...any) int {
		var value int
		if err := server.db.QueryRow(query, args...).Scan(&value); err != nil {
			return 0
		}
		return value
	}

	disabledGateways := 0
	if tableExists(server, "gateways") {
		columns := tableColumns(server, "gateways")
		if columns["is_enabled"] {
			disabledGateways = count("SELECT COUNT(*) FROM gateways WHERE is_enabled = FALSE")
		}
	}

	lowBalanceUsers := 0
	if tableExists(server, "users") {
		columns := tableColumns(server, "users")
		if columns["balance"] {
			where := " WHERE balance < 50"
			if columns["type"] {
				where += " AND type = 'customer'"
			}
			if columns["deleted_at"] {
				where += " AND deleted_at IS NULL"
			}
			lowBalanceUsers = count("SELECT COUNT(*) FROM users" + where)
		}
	}

	failedSmsToday := 0
	if tableExists(server, "messages") {
		failedSmsToday = count("SELECT COUNT(*) FROM messages WHERE status = 'failed' AND DATE(created_at) = CURRENT_DATE")
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"disabled_gateways":  disabledGateways,
			"low_balance_users":  lowBalanceUsers,
			"failed_sms_today":   failedSmsToday,
		},
	})
}

// alertProviders mirrors the old Provider Outages table.
func (server *Server) alertProviders(context fiber.Ctx) error {
	items := make([]map[string]any, 0)
	columns := tableColumns(server, "gateways")
	if len(columns) == 0 {
		return listResponse(context, items, listMeta{Page: 1, PerPage: 25, Total: 0})
	}

	where := "WHERE 1 = 1"
	if columns["health_status"] && columns["is_enabled"] {
		where = "WHERE health_status = 'down' OR is_enabled = FALSE"
	} else if columns["is_enabled"] {
		where = "WHERE is_enabled = FALSE"
	}

	rows, err := server.db.Query(`
		SELECT id, name, `+textColumn(columns, "gateways", "driver")+`,
		       COALESCE(`+func() string {
		if columns["health_status"] {
			return "health_status"
		}
		return "'degraded'"
	}()+`, 'degraded'),
		       `+flagColumn(columns, "gateways", "is_enabled")+`,
		       `+textColumn(columns, "gateways", "last_health_response")+`,
		       `+textColumn(columns, "gateways", "updated_at")+`
		FROM gateways
		`+where+`
		ORDER BY id ASC
	`)
	if err != nil {
		return listResponse(context, items, listMeta{Page: 1, PerPage: 25, Total: 0})
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name, driver, health, lastResponse string
		var enabled int
		var updatedAt sql.NullTime
		if err := rows.Scan(&id, &name, &driver, &health, &enabled, &lastResponse, &updatedAt); err != nil {
			continue
		}
		state := health
		if enabled == 0 {
			state = "manually_disabled"
		}
		item := map[string]any{
			"id":                  id,
			"name":                name,
			"driver":              driver,
			"health_status":       state,
			"last_health_response": lastResponse,
		}
		if updatedAt.Valid {
			item["updated_at"] = updatedAt.Time.Format(time.RFC3339)
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: 1, PerPage: 25, Total: len(items)})
}

// restoreProvider is the old "Restore / Enable" action of Provider Outages.
func (server *Server) restoreProvider(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Gateway not found."})
	}

	columns := tableColumns(server, "gateways")
	sets := []string{}
	if columns["is_enabled"] {
		sets = append(sets, "is_enabled = TRUE")
	}
	if columns["health_status"] {
		sets = append(sets, "health_status = 'healthy'")
	}
	if columns["last_health_check_at"] {
		sets = append(sets, "last_health_check_at = NOW()")
	}
	if columns["updated_at"] {
		sets = append(sets, "updated_at = NOW()")
	}
	if len(sets) == 0 {
		return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"message": "Gateways are unavailable right now."})
	}

	if _, err := server.db.Exec("UPDATE gateways SET "+strings.Join(sets, ", ")+" WHERE id = ?", id); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Gateway restored and marked healthy."})
}

type dlrRow struct {
	ID               int64   `json:"id"`
	Customer         string  `json:"customer"`
	Recipient        string  `json:"recipient"`
	Sender           *string `json:"sender_id"`
	Gateway          *string `json:"gateway"`
	GatewayMessageID *string `json:"gateway_message_id"`
	Status           string  `json:"status"`
	CreatedAt        *string `json:"created_at"`
	SentAt           *string `json:"sent_at"`
	DeliveredAt      *string `json:"delivered_at"`
}

// dlrRows backs every old DLR page: live stream, logs, queue and retry.
func (server *Server) dlrRows(context fiber.Ctx, statuses []string, order string, limitOnly bool) error {
	page, perPage := pagination(context)
	search := strings.TrimSpace(context.Query("search"))
	if limitOnly {
		page, perPage = 1, 10
	}

	columns := tableColumns(server, "messages")
	if len(columns) == 0 {
		return listResponse(context, []dlrRow{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	where := "WHERE 1 = 1"
	args := []any{}
	if len(statuses) > 0 {
		placeholders := []string{}
		for _, status := range statuses {
			placeholders = append(placeholders, "?")
			args = append(args, status)
		}
		where += " AND m.status IN (" + strings.Join(placeholders, ", ") + ")"
	}
	if search != "" {
		where += " AND m.`to` LIKE ?"
		args = append(args, "%"+search+"%")
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM messages m "+where, args...).Scan(&total); err != nil {
		return listResponse(context, []dlrRow{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	rows, err := server.db.Query(`
		SELECT m.id, COALESCE(u.name, ''), m.`+"`to`"+`, s.sender_id, g.name,
		       `+textColumn(columns, "m", "gateway_message_id")+`, COALESCE(m.status, ''),
		       `+textColumn(columns, "m", "created_at")+`, `+textColumn(columns, "m", "sent_at")+`,
		       `+textColumn(columns, "m", "delivered_at")+`
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		`+where+`
		ORDER BY `+order+`
		LIMIT ? OFFSET ?
	`, append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return listResponse(context, []dlrRow{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}
	defer rows.Close()

	items := make([]dlrRow, 0)
	for rows.Next() {
		var item dlrRow
		var sender, gateway, gatewayMessageID sql.NullString
		var createdAt, sentAt, deliveredAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Customer, &item.Recipient, &sender, &gateway,
			&gatewayMessageID, &item.Status, &createdAt, &sentAt, &deliveredAt); err != nil {
			return listResponse(context, []dlrRow{}, listMeta{Page: page, PerPage: perPage, Total: 0})
		}
		if sender.Valid {
			value := sender.String
			item.Sender = &value
		}
		if gateway.Valid {
			value := gateway.String
			item.Gateway = &value
		}
		if gatewayMessageID.Valid {
			value := gatewayMessageID.String
			item.GatewayMessageID = &value
		}
		if createdAt.Valid {
			value := createdAt.Time.Format(time.RFC3339)
			item.CreatedAt = &value
		}
		if sentAt.Valid {
			value := sentAt.Time.Format(time.RFC3339)
			item.SentAt = &value
		}
		if deliveredAt.Valid {
			value := deliveredAt.Time.Format(time.RFC3339)
			item.DeliveredAt = &value
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

func (server *Server) dlrLive(context fiber.Ctx) error {
	return server.dlrRows(context, []string{"delivered"}, "COALESCE(m.delivered_at, m.created_at) DESC", false)
}

func (server *Server) dlrQueue(context fiber.Ctx) error {
	return server.dlrRows(context, []string{"sent"}, "COALESCE(m.sent_at, m.created_at) DESC", false)
}

func (server *Server) dlrRetry(context fiber.Ctx) error {
	return server.dlrRows(context, []string{"dlr_failed"}, "m.id DESC", false)
}

func (server *Server) dlrLogs(context fiber.Ctx) error {
	status := strings.TrimSpace(context.Query("statuses"))
	statuses := []string{}
	if status != "" && status != "all" {
		for _, value := range strings.Split(status, ",") {
			if value = strings.TrimSpace(value); value != "" {
				statuses = append(statuses, value)
			}
		}
	} else {
		statuses = []string{"delivered", "sent", "dlr_failed"}
	}
	return server.dlrRows(context, statuses, "m.id DESC", false)
}

// dlrCheck implements the old Delivery Check page: the last 10 messages sent to
// one destination number.
func (server *Server) dlrCheck(context fiber.Ctx) error {
	phone := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, context.Query("phone"))
	if phone == "" {
		return context.JSON(fiber.Map{"data": []dlrRow{}})
	}

	query := context.Request().URI().QueryString()
	_ = query
	context.Request().URI().SetQueryString("search=" + phone + "&per_page=10&page=1")

	page, perPage := 1, 10
	columns := tableColumns(server, "messages")
	if len(columns) == 0 {
		return context.JSON(fiber.Map{"data": []dlrRow{}})
	}

	rows, err := server.db.Query(`
		SELECT m.id, COALESCE(u.name, ''), m.`+"`to`"+`, s.sender_id, g.name,
		       `+textColumn(columns, "m", "gateway_message_id")+`, COALESCE(m.status, ''),
		       `+textColumn(columns, "m", "created_at")+`, `+textColumn(columns, "m", "sent_at")+`,
		       `+textColumn(columns, "m", "delivered_at")+`
		FROM messages m
		LEFT JOIN users u ON u.id = m.user_id
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		WHERE m.`+"`to`"+` LIKE ?
		ORDER BY m.id DESC
		LIMIT ? OFFSET ?
	`, "%"+phone+"%", perPage, (page-1)*perPage)
	if err != nil {
		return context.JSON(fiber.Map{"data": []dlrRow{}})
	}
	defer rows.Close()

	items := make([]dlrRow, 0)
	for rows.Next() {
		var item dlrRow
		var sender, gateway, gatewayMessageID sql.NullString
		var createdAt, sentAt, deliveredAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Customer, &item.Recipient, &sender, &gateway,
			&gatewayMessageID, &item.Status, &createdAt, &sentAt, &deliveredAt); err != nil {
			return context.JSON(fiber.Map{"data": []dlrRow{}})
		}
		if sender.Valid {
			value := sender.String
			item.Sender = &value
		}
		if gateway.Valid {
			value := gateway.String
			item.Gateway = &value
		}
		if gatewayMessageID.Valid {
			value := gatewayMessageID.String
			item.GatewayMessageID = &value
		}
		if createdAt.Valid {
			value := createdAt.Time.Format(time.RFC3339)
			item.CreatedAt = &value
		}
		if sentAt.Valid {
			value := sentAt.Time.Format(time.RFC3339)
			item.SentAt = &value
		}
		if deliveredAt.Valid {
			value := deliveredAt.Time.Format(time.RFC3339)
			item.DeliveredAt = &value
		}
		items = append(items, item)
	}

	return context.JSON(fiber.Map{"data": items})
}

// recheckDlr is the old DLR Retry "Re-Poll Status" action.
func (server *Server) recheckDlr(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Message not found."})
	}

	columns := tableColumns(server, "messages")
	sets := []string{"status = 'sent'"}
	if columns["updated_at"] {
		sets = append(sets, "updated_at = NOW()")
	}
	if _, err := server.db.Exec("UPDATE messages SET "+strings.Join(sets, ", ")+" WHERE id = ?", id); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Message re-queued for status verification."})
}
