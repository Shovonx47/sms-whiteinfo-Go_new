package httpapi

import (
	"database/sql"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type depositFinanceTotals struct {
	Add          float64 `json:"add"`
	Minus        float64 `json:"minus"`
	Net          float64 `json:"net"`
	Transactions int     `json:"transactions"`
}

type depositFinanceChannel struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Add   float64 `json:"add"`
	Minus float64 `json:"minus"`
}

type depositFinanceDay struct {
	Date         string  `json:"date"`
	Add          float64 `json:"add"`
	Minus        float64 `json:"minus"`
	Net          float64 `json:"net"`
	Transactions int     `json:"transactions"`
}

type depositFinanceReport struct {
	Period   string                  `json:"period"`
	From     string                  `json:"from"`
	To       string                  `json:"to"`
	Totals   depositFinanceTotals    `json:"totals"`
	Channels []depositFinanceChannel `json:"channels"`
	Daily    []depositFinanceDay     `json:"daily"`
}

type depositDetail struct {
	deposit
	GatewayName     string `json:"gateway_name"`
	GatewayResponse string `json:"gateway_response"`
}

// depositReportRange resolves the reporting window used by the finance report.
// "Add" is credited money (completed deposits) and "Minus" is money that never
// reached the wallet (failed or rejected deposits) inside the window.
func depositReportRange(context fiber.Ctx) (start time.Time, end time.Time, period string, err error) {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	period = strings.ToLower(strings.TrimSpace(context.Query("period")))
	if period == "" {
		period = "today"
	}

	start = today
	end = today.AddDate(0, 0, 1)

	switch period {
	case "today":
	case "yesterday":
		start = today.AddDate(0, 0, -1)
		end = today
	case "7days", "last_7_days", "last7days":
		period = "7days"
		start = today.AddDate(0, 0, -6)
	case "week", "this_week":
		period = "week"
		weekday := int(today.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = today.AddDate(0, 0, -(weekday - 1))
	case "month", "this_month":
		period = "month"
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
	case "custom":
		fromRaw := strings.TrimSpace(context.Query("from"))
		toRaw := strings.TrimSpace(context.Query("to"))
		fromParsed, fromErr := time.ParseInLocation("2006-01-02", fromRaw, today.Location())
		toParsed, toErr := time.ParseInLocation("2006-01-02", toRaw, today.Location())
		if fromErr != nil || toErr != nil {
			return time.Time{}, time.Time{}, "", fiber.NewError(http.StatusBadRequest, "Custom range requires valid from and to dates.")
		}
		if toParsed.Before(fromParsed) {
			toParsed = fromParsed
		}
		start = fromParsed
		end = toParsed.AddDate(0, 0, 1)
	default:
		period = "today"
	}

	return start, end, period, nil
}

func (server *Server) depositFinanceReport(context fiber.Ctx) error {
	start, end, period, err := depositReportRange(context)
	if err != nil {
		return err
	}

	startStamp := start.Format("2006-01-02 15:04:05")
	endStamp := end.Format("2006-01-02 15:04:05")

	report := depositFinanceReport{
		Period:   period,
		From:     start.Format("2006-01-02"),
		To:       end.AddDate(0, 0, -1).Format("2006-01-02"),
		Channels: make([]depositFinanceChannel, 0),
		Daily:    make([]depositFinanceDay, 0),
	}

	rows, err := server.db.Query(`
		SELECT DATE_FORMAT(d.created_at, '%Y-%m-%d') AS day,
			COALESCE(SUM(CASE WHEN d.status = 'completed' THEN d.amount ELSE 0 END), 0) AS add_total,
			COALESCE(SUM(CASE WHEN d.status IN ('failed', 'rejected') THEN d.amount ELSE 0 END), 0) AS minus_total,
			COALESCE(SUM(CASE WHEN d.status = 'completed' THEN 1 ELSE 0 END), 0) AS tx_count
		FROM deposits d
		WHERE d.deleted_at IS NULL AND d.created_at >= ? AND d.created_at < ?
		GROUP BY day
		ORDER BY day DESC
	`, startStamp, endStamp)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	for rows.Next() {
		var item depositFinanceDay
		if err := rows.Scan(&item.Date, &item.Add, &item.Minus, &item.Transactions); err != nil {
			return databaseError(context, err)
		}
		item.Net = item.Add - item.Minus
		report.Totals.Add += item.Add
		report.Totals.Minus += item.Minus
		report.Totals.Transactions += item.Transactions
		report.Daily = append(report.Daily, item)
	}
	if err := rows.Err(); err != nil {
		return databaseError(context, err)
	}
	report.Totals.Net = report.Totals.Add - report.Totals.Minus

	channelRows, err := server.db.Query(`
		SELECT
			CASE
				WHEN LOWER(COALESCE(d.payment_method, 'manual')) LIKE '%pipra%' THEN 'PIPRAPAY'
				WHEN LOWER(COALESCE(d.payment_method, 'manual')) IN ('manual', 'admin', 'bank', 'bank_transfer') THEN 'ADMIN'
				ELSE UPPER(COALESCE(d.payment_method, 'manual'))
			END AS channel,
			COALESCE(SUM(CASE WHEN d.status = 'completed' THEN d.amount ELSE 0 END), 0) AS add_total,
			COALESCE(SUM(CASE WHEN d.status IN ('failed', 'rejected') THEN d.amount ELSE 0 END), 0) AS minus_total
		FROM deposits d
		WHERE d.deleted_at IS NULL AND d.created_at >= ? AND d.created_at < ?
		GROUP BY channel
	`, startStamp, endStamp)
	if err != nil {
		return databaseError(context, err)
	}
	defer channelRows.Close()

	for channelRows.Next() {
		var item depositFinanceChannel
		if err := channelRows.Scan(&item.Key, &item.Add, &item.Minus); err != nil {
			return databaseError(context, err)
		}
		item.Label = item.Key
		report.Channels = append(report.Channels, item)
	}
	if err := channelRows.Err(); err != nil {
		return databaseError(context, err)
	}

	channelOrder := map[string]int{"ADMIN": 0, "PIPRAPAY": 1}
	sort.SliceStable(report.Channels, func(left, right int) bool {
		leftOrder, leftKnown := channelOrder[report.Channels[left].Key]
		rightOrder, rightKnown := channelOrder[report.Channels[right].Key]
		if leftKnown && rightKnown {
			return leftOrder < rightOrder
		}
		if leftKnown != rightKnown {
			return leftKnown
		}
		return report.Channels[left].Key < report.Channels[right].Key
	})

	return context.JSON(fiber.Map{"data": report})
}

func (server *Server) depositDetail(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil {
		return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Invalid deposit ID."})
	}

	var item depositDetail
	var createdAt time.Time
	var transactionID, depositedBy sql.NullString
	var paidAt sql.NullTime

	err = server.db.QueryRow(`
		SELECT d.id, d.user_id, u.name, COALESCE(u.email, ''), COALESCE(u.phone, ''),
			COALESCE(NULLIF(u.company_name, ''), NULLIF(u.business_name, ''), ''),
			d.amount, COALESCE(d.currency, 'BDT'), COALESCE(d.payment_method, 'manual'),
			d.transaction_id, admin.name, d.status, d.paid_at, d.created_at
		FROM deposits d
		INNER JOIN users u ON u.id = d.user_id
		LEFT JOIN users admin ON admin.id = d.deposited_by_id
		WHERE d.id = ? AND d.deleted_at IS NULL
	`, id).Scan(
		&item.ID, &item.UserID, &item.Customer, &item.CustomerEmail, &item.CustomerPhone,
		&item.Company, &item.Amount, &item.Currency, &item.PaymentMethod,
		&transactionID, &depositedBy, &item.Status, &paidAt, &createdAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Deposit not found."})
		}
		return databaseError(context, err)
	}

	if transactionID.Valid {
		value := transactionID.String
		item.TransactionID = &value
	}
	if depositedBy.Valid {
		value := depositedBy.String
		item.DepositedBy = &value
	}
	if paidAt.Valid {
		value := paidAt.Time.Format(time.RFC3339)
		item.PaidAt = &value
	}
	method := item.PaymentMethod
	item.Gateway = &method
	item.CreatedAt = createdAt.Format(time.RFC3339)

	// The raw gateway callback for this payment, when the gateway sent one.
	if item.TransactionID != nil && *item.TransactionID != "" {
		var payload, gateway string
		webhookErr := server.db.QueryRow(`
			SELECT COALESCE(payload, ''), COALESCE(gateway, '')
			FROM payment_webhooks
			WHERE event_id = ? OR payload LIKE ?
			ORDER BY id DESC
			LIMIT 1
		`, *item.TransactionID, "%"+*item.TransactionID+"%").Scan(&payload, &gateway)
		if webhookErr == nil {
			item.GatewayResponse = payload
			item.GatewayName = gateway
		}
	}

	return context.JSON(fiber.Map{"data": item})
}

type depositUpdatePayload struct {
	Amount        *float64 `json:"amount"`
	Currency      *string  `json:"currency"`
	PaymentMethod *string  `json:"payment_method"`
	TransactionID *string  `json:"transaction_id"`
	Status        *string  `json:"status"`
	PaidAt        *string  `json:"paid_at"`
}

// updateDeposit saves the editable deposit fields. Moving a deposit to
// completed goes through the same credit path as approval so the wallet and
// the ledger stay in sync, and an already approved deposit can never be
// un-credited from here.
func (server *Server) updateDeposit(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil {
		return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Invalid deposit ID."})
	}

	var payload depositUpdatePayload
	if err := context.Bind().Body(&payload); err != nil {
		return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Invalid request body."})
	}

	admin := superman(context)

	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(context, err)
	}
	defer tx.Rollback()

	var userID int64
	var amount float64
	var status, currency, method string
	var transactionID sql.NullString
	var paidAt sql.NullTime

	err = tx.QueryRow(`
		SELECT user_id, amount, status, COALESCE(currency, 'BDT'), COALESCE(payment_method, 'manual'), transaction_id, paid_at
		FROM deposits
		WHERE id = ? AND deleted_at IS NULL
		FOR UPDATE
	`, id).Scan(&userID, &amount, &status, &currency, &method, &transactionID, &paidAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Deposit not found."})
		}
		return databaseError(context, err)
	}

	newStatus := status
	if payload.Status != nil {
		candidate := strings.ToLower(strings.TrimSpace(*payload.Status))
		switch candidate {
		case "pending", "completed", "failed", "rejected":
			newStatus = candidate
		default:
			return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Status must be pending, completed, failed or rejected."})
		}
	}

	if status == "completed" && newStatus != "completed" {
		return context.Status(http.StatusBadRequest).JSON(fiber.Map{
			"message": "An approved deposit cannot be reopened. Adjust the customer balance from the billing dashboard instead.",
		})
	}

	newAmount := amount
	if payload.Amount != nil {
		if *payload.Amount <= 0 {
			return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Amount must be greater than zero."})
		}
		if status == "completed" {
			delta := *payload.Amount - amount
			if delta > 0.0001 || delta < -0.0001 {
				return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "The amount of an approved deposit cannot be changed."})
			}
		}
		newAmount = *payload.Amount
	}

	newCurrency := currency
	if payload.Currency != nil {
		candidate := strings.ToUpper(strings.TrimSpace(*payload.Currency))
		if candidate == "" || len(candidate) > 10 {
			return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Currency is required (max 10 characters)."})
		}
		newCurrency = candidate
	}

	newMethod := method
	if payload.PaymentMethod != nil {
		candidate := strings.ToLower(strings.TrimSpace(*payload.PaymentMethod))
		if candidate == "" || len(candidate) > 64 {
			return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Payment method is required (max 64 characters)."})
		}
		newMethod = candidate
	}

	newTransactionID := transactionID
	if payload.TransactionID != nil {
		candidate := strings.TrimSpace(*payload.TransactionID)
		if candidate == "" {
			newTransactionID = sql.NullString{}
		} else {
			newTransactionID = sql.NullString{String: candidate, Valid: true}
		}
	}

	newPaidAt := paidAt
	if payload.PaidAt != nil {
		candidate := strings.TrimSpace(*payload.PaidAt)
		if candidate == "" {
			newPaidAt = sql.NullTime{}
		} else {
			parsed, parseErr := parseDepositTime(candidate)
			if parseErr != nil {
				return context.Status(http.StatusBadRequest).JSON(fiber.Map{"message": "Paid At must be a valid date and time."})
			}
			newPaidAt = sql.NullTime{Time: parsed, Valid: true}
		}
	}

	completing := status != "completed" && newStatus == "completed"
	if completing {
		var balanceBefore float64
		var customerName string
		if err := tx.QueryRow("SELECT name, balance FROM users WHERE id = ? FOR UPDATE", userID).Scan(&customerName, &balanceBefore); err != nil {
			return databaseError(context, err)
		}

		balanceAfter := balanceBefore + newAmount
		if _, err := tx.Exec("UPDATE users SET balance = ?, updated_at = NOW() WHERE id = ?", balanceAfter, userID); err != nil {
			return databaseError(context, err)
		}

		description := fmt.Sprintf("Deposit of ৳%.2f via %s approved by %s (#%d)", newAmount, newMethod, admin.Name, id)
		if _, err := tx.Exec(`
			INSERT INTO transactions (user_id, transactionable_type, transactionable_id, amount, direction, balance_before, balance_after, description, created_at, updated_at)
			VALUES (?, 'App\\Models\\Deposit', ?, ?, 'credit', ?, ?, ?, NOW(), NOW())
		`, userID, id, newAmount, balanceBefore, balanceAfter, description); err != nil {
			return databaseError(context, err)
		}

		if !newPaidAt.Valid {
			newPaidAt = sql.NullTime{Time: time.Now(), Valid: true}
		}
	}

	if completing {
		_, err = tx.Exec(`
			UPDATE deposits
			SET amount = ?, currency = ?, payment_method = ?, transaction_id = ?, status = ?, paid_at = ?,
				deposited_by_id = COALESCE(deposited_by_id, ?), updated_at = NOW()
			WHERE id = ?
		`, newAmount, newCurrency, newMethod, newTransactionID, newStatus, newPaidAt, admin.AdminID, id)
	} else {
		_, err = tx.Exec(`
			UPDATE deposits
			SET amount = ?, currency = ?, payment_method = ?, transaction_id = ?, status = ?, paid_at = ?, updated_at = NOW()
			WHERE id = ?
		`, newAmount, newCurrency, newMethod, newTransactionID, newStatus, newPaidAt, id)
	}
	if err != nil {
		return databaseError(context, err)
	}

	if err := tx.Commit(); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{
		"message": fmt.Sprintf("Deposit #%d updated.", id),
		"data": fiber.Map{
			"id":     id,
			"status": newStatus,
			"amount": newAmount,
		},
	})
}

func parseDepositTime(value string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported time format: %s", value)
}
