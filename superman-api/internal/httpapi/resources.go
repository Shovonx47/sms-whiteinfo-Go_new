package httpapi

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type listMeta struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
}

type customerSummary struct {
	TotalCustomers       int     `json:"total_customers"`
	TotalBalance         float64 `json:"total_balance"`
	AvgBalance           float64 `json:"avg_balance"`
	FundedCustomersCount int     `json:"funded_customers_count"`
	ZeroBalanceCount     int     `json:"zero_balance_count"`
	WithNidCount         int     `json:"with_nid_count"`
	VerifiedKycCount     int     `json:"verified_kyc_count"`
	UnderReviewKycCount  int     `json:"under_review_kyc_count"`
	OtherKycCount        int     `json:"other_kyc_count"`
	KycVerifiedRate      float64 `json:"kyc_verified_rate"`
	ApprovedStatusCount  int     `json:"approved_status_count"`
	PendingStatusCount   int     `json:"pending_status_count"`
	InactiveStatusCount  int     `json:"inactive_status_count"`
	ApprovedRate         float64 `json:"approved_rate"`
	FundedPercentage     float64 `json:"funded_percentage"`
}

type customer struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Email          string  `json:"email"`
	Phone          string  `json:"phone"`
	BusinessName   *string `json:"business_name"`
	Balance        float64 `json:"balance"`
	KYCStatus      string  `json:"kyc_status"`
	Status         string  `json:"status"`
	NIDNumber      *string `json:"nid_number"`
	Tariff         *string `json:"tariff"`
	LastLoginAt    *string `json:"last_login_at"`
	CreatedByAdmin bool    `json:"created_by_admin"`
	MessagesCount  int64   `json:"messages_count"`
	MessagesSpend  float64 `json:"messages_spend"`
	CreatedAt      string  `json:"created_at"`
}

func (server *Server) customers(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}
	search := strings.TrimSpace(context.Query("search"))
	creationSource := strings.TrimSpace(context.Query("creation_source"))
	kycStatus := strings.TrimSpace(context.Query("kyc_status"))
	status := strings.TrimSpace(context.Query("status"))
	tariffPlan := strings.TrimSpace(context.Query("tariff_plan"))
	balanceRange := strings.TrimSpace(context.Query("balance_range"))
	regFrom := strings.TrimSpace(context.Query("registered_from"))
	regUntil := strings.TrimSpace(context.Query("registered_until"))

	arguments := []any{"customer"}
	where := "WHERE u.type = ? AND u.deleted_at IS NULL"

	if search != "" {
		where += " AND (u.name LIKE ? OR u.email LIKE ? OR u.phone LIKE ? OR u.business_name LIKE ? OR u.nid_number LIKE ?)"
		term := "%" + search + "%"
		arguments = append(arguments, term, term, term, term, term)
	}

	if creationSource == "1" {
		where += " AND u.created_by_admin = true"
	} else if creationSource == "0" {
		where += " AND (u.created_by_admin = false OR u.created_by_admin IS NULL)"
	}

	if kycStatus != "" && kycStatus != "all" {
		where += " AND u.kyc_status = ?"
		arguments = append(arguments, kycStatus)
	}

	if status != "" && status != "all" {
		where += " AND u.status = ?"
		arguments = append(arguments, status)
	}

	if tariffPlan != "" && tariffPlan != "all" {
		where += " AND EXISTS (SELECT 1 FROM tariff_plan_assignments tpa WHERE tpa.user_id = u.id AND tpa.tariff_plan_id = ? AND tpa.status = 'active')"
		arguments = append(arguments, tariffPlan)
	}

	switch balanceRange {
	case "positive":
		where += " AND u.balance > 0"
	case "zero":
		where += " AND u.balance <= 0"
	case "low":
		where += " AND u.balance >= 0 AND u.balance < 100"
	case "high":
		where += " AND u.balance >= 1000"
	case "vip":
		where += " AND u.balance >= 10000"
	}

	if regFrom != "" {
		where += " AND DATE(u.created_at) >= ?"
		arguments = append(arguments, regFrom)
	}
	if regUntil != "" {
		where += " AND DATE(u.created_at) <= ?"
		arguments = append(arguments, regUntil)
	}

	// Calculate aggregates for current filtered subset
	summaryQuery := `
		SELECT
			COUNT(*),
			COALESCE(SUM(u.balance), 0),
			COALESCE(AVG(u.balance), 0),
			COALESCE(SUM(CASE WHEN u.balance > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.balance <= 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.nid_number IS NOT NULL AND u.nid_number != '' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.kyc_status = 'verified' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.kyc_status = 'under_review' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.status = 'approved' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN u.status IN ('rejected', 'banned') THEN 1 ELSE 0 END), 0)
		FROM users u ` + where

	var summary customerSummary
	err := server.db.QueryRow(summaryQuery, arguments...).Scan(
		&summary.TotalCustomers,
		&summary.TotalBalance,
		&summary.AvgBalance,
		&summary.FundedCustomersCount,
		&summary.ZeroBalanceCount,
		&summary.WithNidCount,
		&summary.VerifiedKycCount,
		&summary.UnderReviewKycCount,
		&summary.ApprovedStatusCount,
		&summary.PendingStatusCount,
		&summary.InactiveStatusCount,
	)
	if err != nil {
		return databaseError(context, err)
	}

	summary.OtherKycCount = summary.TotalCustomers - (summary.VerifiedKycCount + summary.UnderReviewKycCount)
	if summary.OtherKycCount < 0 {
		summary.OtherKycCount = 0
	}
	if summary.TotalCustomers > 0 {
		summary.KycVerifiedRate = float64(int((float64(summary.VerifiedKycCount)/float64(summary.TotalCustomers))*1000+0.5)) / 10.0
		summary.ApprovedRate = float64(int((float64(summary.ApprovedStatusCount)/float64(summary.TotalCustomers))*1000+0.5)) / 10.0
		summary.FundedPercentage = float64(int((float64(summary.FundedCustomersCount)/float64(summary.TotalCustomers))*1000+0.5)) / 10.0
	}

	total := summary.TotalCustomers
	itemsArguments := append(arguments, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT u.id, u.name, u.email, COALESCE(u.phone, ''), u.business_name, COALESCE(u.balance, 0), u.kyc_status, u.status,
			u.nid_number, u.last_login_at, COALESCE(u.created_by_admin, false),
			(SELECT tp.name FROM tariff_plan_assignments tpa JOIN tariff_plans tp ON tp.id = tpa.tariff_plan_id WHERE tpa.user_id = u.id AND tpa.status = 'active' LIMIT 1) AS tariff_name,
			(SELECT COUNT(*) FROM messages m WHERE m.user_id = u.id) AS messages_count,
			(SELECT COALESCE(SUM(m.cost), 0) FROM messages m WHERE m.user_id = u.id) AS messages_spend,
			u.created_at
		FROM users u `+where+`
		ORDER BY u.created_at DESC
		LIMIT ? OFFSET ?
	`, itemsArguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]customer, 0)
	for rows.Next() {
		var item customer
		var nidNumber, tariffName sql.NullString
		var lastLoginAt sql.NullTime
		var createdByAdmin bool
		var createdAt time.Time

		if err := rows.Scan(
			&item.ID, &item.Name, &item.Email, &item.Phone, &item.BusinessName, &item.Balance, &item.KYCStatus, &item.Status,
			&nidNumber, &lastLoginAt, &createdByAdmin, &tariffName, &item.MessagesCount, &item.MessagesSpend, &createdAt,
		); err != nil {
			return databaseError(context, err)
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.CreatedByAdmin = createdByAdmin
		if nidNumber.Valid && nidNumber.String != "" {
			val := nidNumber.String
			item.NIDNumber = &val
		}
		if tariffName.Valid && tariffName.String != "" {
			val := tariffName.String
			item.Tariff = &val
		}
		if lastLoginAt.Valid {
			val := lastLoginAt.Time.Format("Jan 02, 2006 03:04 PM")
			item.LastLoginAt = &val
		}
		items = append(items, item)
	}

	return context.JSON(fiber.Map{
		"data":    items,
		"meta":    listMeta{Page: page, PerPage: perPage, Total: total},
		"summary": summary,
	})
}

type transaction struct {
	ID            int64   `json:"id"`
	Customer      string  `json:"customer"`
	Reference     string  `json:"reference"`
	Direction     string  `json:"direction"`
	Amount        float64 `json:"amount"`
	BalanceBefore float64 `json:"balance_before"`
	BalanceAfter  float64 `json:"balance_after"`
	Description   *string `json:"description"`
	CreatedAt     string  `json:"created_at"`
}

func (server *Server) transactions(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}
	search := strings.TrimSpace(context.Query("search"))
	arguments := []any{}
	where := "WHERE 1 = 1"
	if search != "" {
		where += " AND (u.name LIKE ? OR t.transactionable_type LIKE ? OR t.description LIKE ?)"
		term := "%" + search + "%"
		arguments = append(arguments, term, term, term)
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM transactions t INNER JOIN users u ON u.id = t.user_id "+where, arguments...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	arguments = append(arguments, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT t.id, u.name, t.transactionable_type, t.direction, t.amount, t.balance_before, t.balance_after, t.description, t.created_at
		FROM transactions t
		INNER JOIN users u ON u.id = t.user_id
		`+where+`
		ORDER BY t.id DESC
		LIMIT ? OFFSET ?
	`, arguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]transaction, 0)
	for rows.Next() {
		var item transaction
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Customer, &item.Reference, &item.Direction, &item.Amount, &item.BalanceBefore, &item.BalanceAfter, &item.Description, &createdAt); err != nil {
			return databaseError(context, err)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type message struct {
	ID                 int64   `json:"id"`
	Customer           string  `json:"customer"`
	Recipient          string  `json:"recipient"`
	Body               string  `json:"body"`
	Status             string  `json:"status"`
	Cost               float64 `json:"cost"`
	OperatorCost       float64 `json:"operator_cost"`
	PrimaryDippingCost float64 `json:"primary_dipping_cost"`
	Parts              int     `json:"parts"`
	Source             string  `json:"source"`
	Gateway            *string `json:"gateway,omitempty"`
	GatewayDriver      string  `json:"gateway_driver"`
	SenderID           *string `json:"sender_id,omitempty"`
	SenderType         *string `json:"sender_type,omitempty"`
	BillMsisdn         string  `json:"bill_msisdn"`
	SmsType            string  `json:"sms_type"`
	Channel            string  `json:"channel"`
	Operator           string  `json:"operator"`
	Tariff             string  `json:"tariff"`
	APIUser            *string `json:"api_user"`
	SourceIP           string  `json:"source_ip"`
	RequestID          string  `json:"request_id"`
	ServerTxnID        string  `json:"server_txn_id"`
	ServerResponseCode string  `json:"server_response_code"`
	MnoTxnID           string  `json:"mno_txn_id"`
	MnoResponseCode    string  `json:"mno_response_code"`
	MnoResponseMessage string  `json:"mno_response_message"`
	PreviousBalance    *string `json:"previous_balance"`
	NextBalance        *string `json:"next_balance"`
	FailureReason      string  `json:"failure_reason"`
	SubmitTime         string  `json:"submit_time"`
	DeliveryTime       *string `json:"delivery_time"`
	CreatedAt          string  `json:"created_at"`
}

func (server *Server) messages(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}
	search := strings.TrimSpace(context.Query("search"))
	status := strings.TrimSpace(context.Query("status"))
	statuses := strings.TrimSpace(context.Query("statuses"))
	source := strings.ToLower(strings.TrimSpace(context.Query("source")))
	senderCategory := strings.ToLower(strings.TrimSpace(context.Query("sender_category")))
	customerID := strings.TrimSpace(context.Query("user_id"))
	gatewayID := strings.TrimSpace(context.Query("gateway_id"))
	dateFrom := strings.TrimSpace(context.Query("date_from"))
	dateTo := strings.TrimSpace(context.Query("date_to"))
	arguments := []any{}
	where := "WHERE 1 = 1"
	if search != "" {
		where += " AND (u.name LIKE ? OR m.`to` LIKE ? OR m.body LIKE ? OR COALESCE(s.sender_id, '') LIKE ? OR COALESCE(g.name, sg.name, '') LIKE ? OR COALESCE(m.gateway_message_id, '') LIKE ? OR COALESCE(m.tariff_name, '') LIKE ?)"
		term := "%" + search + "%"
		arguments = append(arguments, term, term, term, term, term, term, term)
	}
	if status != "" && status != "all" {
		where += " AND m.status = ?"
		arguments = append(arguments, status)
	}
	if statuses != "" {
		placeholders := []string{}
		for _, value := range strings.Split(statuses, ",") {
			if value = strings.TrimSpace(value); value != "" {
				placeholders = append(placeholders, "?")
				arguments = append(arguments, value)
			}
		}
		if len(placeholders) > 0 {
			where += " AND m.status IN (" + strings.Join(placeholders, ", ") + ")"
		}
	}
	if source == "api" || source == "web" {
		where += " AND m.source = ?"
		arguments = append(arguments, source)
	}
	if id, err := strconv.ParseInt(customerID, 10, 64); err == nil && id > 0 {
		where += " AND m.user_id = ?"
		arguments = append(arguments, id)
	}
	if id, err := strconv.ParseInt(gatewayID, 10, 64); err == nil && id > 0 {
		where += " AND COALESCE(m.gateway_id, s.gateway_id) = ?"
		arguments = append(arguments, id)
	}
	if dateFrom != "" {
		where += " AND DATE(COALESCE(m.sent_at, m.created_at)) >= ?"
		arguments = append(arguments, dateFrom)
	}
	if dateTo != "" {
		where += " AND DATE(COALESCE(m.sent_at, m.created_at)) <= ?"
		arguments = append(arguments, dateTo)
	}
	switch senderCategory {
	case "masking":
		where += " AND s.type IN ('alphanumeric', 'shortcode')"
	case "non_masking", "non-masking":
		where += " AND s.type = 'numeric'"
	}

	var total int
	if err := server.db.QueryRow(`
		SELECT COUNT(*)
		FROM messages m
		INNER JOIN users u ON u.id = m.user_id
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		LEFT JOIN gateways sg ON sg.id = s.gateway_id
		`+where, arguments...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	report := messageReportSchema(server)
	fields := []string{
		"m.id", "u.name", "m.`to`", "m.body", "m.status", "m.cost", "m.operator_cost", "m.primary_dipping_cost",
		"m.parts", "m.source", "COALESCE(g.name, sg.name)", "COALESCE(g.driver, sg.driver, '')",
		"COALESCE(s.sender_id, '')", "COALESCE(s.type, '')",
		"COALESCE(m.operator_name, '')", "COALESCE(m.tariff_name, '')", "COALESCE(m.source_ip, '')",
		"COALESCE(m.gateway_message_id, '')", "m.batch_id", "m.failed_reason",
		"m.created_at", "m.sent_at", "m.delivered_at",
		reportColumn(report.billMsisdn, "bill_msisdn"),
		reportColumn(report.smsType, "sms_type"),
		reportColumn(report.channel, "channel"),
		reportColumn(report.requestID, "request_id"),
		reportColumn(report.serverTxnID, "server_txn_id"),
		reportColumn(report.serverResponseCode, "server_response_code"),
		reportColumn(report.mnoTxnID, "mno_txn_id"),
		reportColumn(report.mnoResponseCode, "mno_response_code"),
		reportColumn(report.mnoResponseMessage, "mno_response_message"),
		balanceSnapshotSQL("balance_before"),
		balanceSnapshotSQL("balance_after"),
	}

	arguments = append(arguments, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT `+strings.Join(fields, ",\n\t\t\t")+`
		FROM messages m
		INNER JOIN users u ON u.id = m.user_id
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		LEFT JOIN gateways sg ON sg.id = s.gateway_id
		`+where+`
		ORDER BY m.id DESC
		LIMIT ? OFFSET ?
	`, arguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]message, 0)
	for rows.Next() {
		var item message
		var gatewayName, senderID, senderType, operatorName, tariff, sourceIP, gatewayMessageID sql.NullString
		var batchID sql.NullInt64
		var failedReason sql.NullString
		var createdAt time.Time
		var sentAt, deliveredAt sql.NullTime
		var billMsisdn, smsType, channel, requestID, serverTxnID, serverResponseCode, mnoTxnID, mnoResponseCode, mnoResponseMessage sql.NullString
		var previousBalance, nextBalance sql.NullFloat64
		if err := rows.Scan(
			&item.ID, &item.Customer, &item.Recipient, &item.Body, &item.Status, &item.Cost, &item.OperatorCost, &item.PrimaryDippingCost,
			&item.Parts, &item.Source, &gatewayName, &item.GatewayDriver,
			&senderID, &senderType,
			&operatorName, &tariff, &sourceIP,
			&gatewayMessageID, &batchID, &failedReason,
			&createdAt, &sentAt, &deliveredAt,
			&billMsisdn, &smsType, &channel, &requestID, &serverTxnID, &serverResponseCode, &mnoTxnID, &mnoResponseCode, &mnoResponseMessage,
			&previousBalance, &nextBalance,
		); err != nil {
			return databaseError(context, err)
		}
		if gatewayName.Valid && gatewayName.String != "" {
			val := gatewayName.String
			item.Gateway = &val
		}
		if senderID.Valid && senderID.String != "" {
			val := senderID.String
			item.SenderID = &val
		}
		if senderType.Valid && senderType.String != "" {
			val := senderType.String
			item.SenderType = &val
		}

		item.Tariff = tariff.String
		item.SourceIP = sourceIP.String
		item.FailureReason = failedReason.String
		item.Operator = operatorName.String
		if item.Operator == "" {
			item.Operator = operatorForRecipient(item.Recipient)
		}

		item.SmsType = strings.ToUpper(smsType.String)
		if item.SmsType == "" {
			item.SmsType = resolvedSmsType(batchID.Valid)
		}
		item.Channel = strings.ToUpper(channel.String)

		item.BillMsisdn = billMsisdn.String
		if item.BillMsisdn == "" && strings.EqualFold(senderType.String, "numeric") {
			item.BillMsisdn = billMsisdnFromSender(senderID.String)
		}

		item.RequestID = requestID.String
		item.ServerTxnID = serverTxnID.String
		if item.ServerTxnID == "" && strings.EqualFold(item.GatewayDriver, "infozillion") {
			item.ServerTxnID = gatewayMessageID.String
		}
		item.ServerResponseCode = serverResponseCode.String
		item.MnoTxnID = mnoTxnID.String
		item.MnoResponseCode = mnoResponseCode.String
		item.MnoResponseMessage = mnoResponseMessage.String

		if item.Source == "api" && item.Customer != "" {
			val := item.Customer
			item.APIUser = &val
		}

		if previousBalance.Valid {
			val := strconv.FormatFloat(previousBalance.Float64, 'f', 4, 64)
			item.PreviousBalance = &val
		}
		if nextBalance.Valid {
			val := strconv.FormatFloat(nextBalance.Float64, 'f', 4, 64)
			item.NextBalance = &val
		}

		item.CreatedAt = createdAt.Format(time.RFC3339)
		item.SubmitTime = item.CreatedAt
		if sentAt.Valid {
			item.SubmitTime = sentAt.Time.Format(time.RFC3339)
		}
		if deliveredAt.Valid {
			val := deliveredAt.Time.Format(time.RFC3339)
			item.DeliveryTime = &val
		}

		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type batch struct {
	ID              int64   `json:"id"`
	Customer        string  `json:"customer"`
	CustomerBalance float64 `json:"customer_balance"`
	SenderID        *string `json:"sender_id"`
	Body            string  `json:"body"`
	Source          string  `json:"source"`
	Status          string  `json:"status"`
	Approval        *string `json:"approval"`
	TotalRecipients int     `json:"total_recipients"`
	ValidCount      int     `json:"valid_count"`
	InvalidCount    int     `json:"invalid_count"`
	Remaining       int     `json:"remaining"`
	ProgressPercent int     `json:"progress_percent"`
	ScheduledAt     *string `json:"scheduled_at"`
	CreatedAt       string  `json:"created_at"`
}

func (server *Server) batches(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}

	approval := strings.ToLower(strings.TrimSpace(context.Query("approval")))
	search := strings.TrimSpace(context.Query("search"))
	statuses := strings.TrimSpace(context.Query("statuses"))
	if statuses == "" {
		statuses = strings.TrimSpace(context.Query("status"))
	}
	sortCol := strings.ToLower(strings.TrimSpace(context.Query("sort")))
	sortDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(context.Query("dir")), "asc") {
		sortDir = "ASC"
	}

	where := "WHERE 1 = 1"
	args := []any{}
	switch approval {
	case "", "all":
		// no approval filter
	case "any":
		where += " AND b.approval IS NOT NULL AND b.approval <> ''"
	default:
		where += " AND b.approval = ?"
		args = append(args, approval)
	}
	if statuses != "" && statuses != "all" {
		placeholders := []string{}
		for _, value := range strings.Split(statuses, ",") {
			if value = strings.TrimSpace(value); value != "" {
				placeholders = append(placeholders, "?")
				args = append(args, value)
			}
		}
		if len(placeholders) > 0 {
			where += " AND b.status IN (" + strings.Join(placeholders, ", ") + ")"
		}
	}
	if search != "" {
		term := "%" + search + "%"
		cleanedID := strings.TrimPrefix(strings.TrimPrefix(strings.ToUpper(search), "BATCH-"), "#")
		if numID, err := strconv.ParseInt(cleanedID, 10, 64); err == nil && numID > 0 {
			where += " AND (u.name LIKE ? OR COALESCE(s.sender_id, '') LIKE ? OR b.body LIKE ? OR b.id = ?)"
			args = append(args, term, term, term, numID)
		} else {
			where += " AND (u.name LIKE ? OR COALESCE(s.sender_id, '') LIKE ? OR b.body LIKE ?)"
			args = append(args, term, term, term)
		}
	}

	var total int
	if err := server.db.QueryRow(`
		SELECT COUNT(*)
		FROM batches b
		INNER JOIN users u ON u.id = b.user_id
		LEFT JOIN senders s ON s.id = b.sender_id
		`+where, args...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	orderBy := "b.id DESC"
	switch sortCol {
	case "customer":
		orderBy = "u.name " + sortDir + ", b.id DESC"
	case "customer_balance":
		orderBy = "u.balance " + sortDir + ", b.id DESC"
	case "created_at":
		orderBy = "b.created_at " + sortDir + ", b.id DESC"
	}

	queryArgs := append(args, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT b.id, u.name, u.balance, s.sender_id, COALESCE(b.body, ''), b.source_type,
		       COALESCE(b.source_config, ''), b.status, b.approval,
		       b.total_recipients, b.valid_count, b.invalid_count, b.scheduled_at, b.created_at
		FROM batches b
		INNER JOIN users u ON u.id = b.user_id
		LEFT JOIN senders s ON s.id = b.sender_id
		`+where+`
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]batch, 0)
	for rows.Next() {
		var item batch
		var sourceConfig string
		var scheduledAt sql.NullTime
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.Customer, &item.CustomerBalance, &item.SenderID, &item.Body,
			&item.Source, &sourceConfig, &item.Status, &item.Approval,
			&item.TotalRecipients, &item.ValidCount, &item.InvalidCount, &scheduledAt, &createdAt,
		); err != nil {
			return databaseError(context, err)
		}
		if sourceConfig != "" {
			var cfg struct {
				Recipients []any `json:"recipients"`
			}
			if json.Unmarshal([]byte(sourceConfig), &cfg) == nil && len(cfg.Recipients) > item.TotalRecipients {
				item.TotalRecipients = len(cfg.Recipients)
			}
		}
		item.Remaining = max(item.TotalRecipients-item.ValidCount-item.InvalidCount, 0)
		if item.TotalRecipients > 0 {
			item.ProgressPercent = (item.ValidCount + item.InvalidCount) * 100 / item.TotalRecipients
		}
		if scheduledAt.Valid {
			value := scheduledAt.Time.Format(time.RFC3339)
			item.ScheduledAt = &value
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

func (server *Server) approveBatch(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Batch not found."})
	}

	var scheduledAt sql.NullTime
	if err := server.db.QueryRow("SELECT scheduled_at FROM batches WHERE id = ?", id).Scan(&scheduledAt); err != nil {
		return databaseError(context, err)
	}
	targetStatus := "processing"
	if scheduledAt.Valid && scheduledAt.Time.After(time.Now()) {
		targetStatus = "scheduled"
	}

	res, err := server.db.Exec(`
		UPDATE batches
		SET status = ?, approval = 'approved', updated_at = NOW()
		WHERE id = ? AND approval = 'pending'
	`, targetStatus, id)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusConflict).JSON(fiber.Map{"message": "This batch has already been approved or rejected."})
	}

	return context.JSON(fiber.Map{"message": "Batch approved successfully."})
}

func (server *Server) rejectBatch(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Batch not found."})
	}

	res, err := server.db.Exec(`
		UPDATE batches
		SET status = 'failed', approval = 'rejected', updated_at = NOW()
		WHERE id = ? AND approval = 'pending'
	`, id)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusConflict).JSON(fiber.Map{"message": "This batch has already been approved or rejected."})
	}

	return context.JSON(fiber.Map{"message": "Batch rejected."})
}

type kycRequest struct {
	ID           int64   `json:"id"`
	CustomerName string  `json:"customer_name"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone"`
	NIDNumber    *string `json:"nid_number"`
	BusinessName *string `json:"business_name"`
	TradeLicense *string `json:"trade_license"`
	Status       string  `json:"status"`
	SubmittedAt  *string `json:"submitted_at"`
}

func (server *Server) kycRequests(context fiber.Ctx) error {
	if err := server.authorizeKYCPermission(context, "kyc.review"); err != nil {
		return err
	}

	page, perPage := pagination(context)
	status := context.Query("status")
	arguments := []any{"customer"}
	where := "WHERE type = ? AND deleted_at IS NULL AND kyc_status IN ('pending', 'under_review', 'resubmit_required')"
	if status != "" {
		where += " AND kyc_status = ?"
		arguments = append(arguments, status)
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM users "+where, arguments...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	arguments = append(arguments, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT id, name, email, phone, nid_number, business_name, trade_license_number, kyc_status, kyc_submitted_at
		FROM users `+where+`
		ORDER BY kyc_submitted_at ASC
		LIMIT ? OFFSET ?
	`, arguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]kycRequest, 0)
	for rows.Next() {
		var item kycRequest
		var submittedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.CustomerName, &item.Email, &item.Phone, &item.NIDNumber, &item.BusinessName, &item.TradeLicense, &item.Status, &submittedAt); err != nil {
			return databaseError(context, err)
		}
		if submittedAt.Valid {
			item.SubmittedAt = apiTime(submittedAt.Time)
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type deposit struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"user_id"`
	Customer      string  `json:"customer"`
	CustomerEmail string  `json:"customer_email"`
	CustomerPhone string  `json:"customer_phone"`
	Company       string  `json:"company"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Gateway       *string `json:"gateway"`
	PaymentMethod string  `json:"payment_method"`
	TransactionID *string `json:"transaction_id"`
	DepositedBy   *string `json:"deposited_by"`
	Status        string  `json:"status"`
	PaidAt        *string `json:"paid_at"`
	CreatedAt     string  `json:"created_at"`
}

// depositQuery builds the shared WHERE clause used by the list, its count and
// the finance report so filters always agree with the rows on screen.
func depositFilters(context fiber.Ctx) (string, []any) {
	where := "WHERE d.deleted_at IS NULL"
	arguments := []any{}

	status := strings.TrimSpace(context.Query("status"))
	if status != "" && status != "all" {
		where += " AND d.status = ?"
		arguments = append(arguments, status)
	}

	method := strings.TrimSpace(context.Query("method"))
	if method != "" && method != "all" {
		where += " AND LOWER(COALESCE(d.payment_method, 'manual')) = ?"
		arguments = append(arguments, strings.ToLower(method))
	}

	search := strings.TrimSpace(context.Query("search"))
	if search != "" {
		where += " AND (u.name LIKE ? OR u.email LIKE ? OR COALESCE(u.phone, '') LIKE ? OR COALESCE(d.transaction_id, '') LIKE ?)"
		term := "%" + search + "%"
		arguments = append(arguments, term, term, term, term)
	}

	return where, arguments
}

func (server *Server) deposits(context fiber.Ctx) error {
	page, perPage := pagination(context)
	where, arguments := depositFilters(context)

	var total int
	if err := server.db.QueryRow(`
		SELECT COUNT(*)
		FROM deposits d
		INNER JOIN users u ON u.id = d.user_id
		`+where, arguments...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	queryArguments := append(append([]any{}, arguments...), perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT d.id, d.user_id, u.name, COALESCE(u.email, ''), COALESCE(u.phone, ''),
			COALESCE(NULLIF(u.company_name, ''), NULLIF(u.business_name, ''), ''),
			d.amount, COALESCE(d.currency, 'BDT'), COALESCE(d.payment_method, 'manual'),
			d.transaction_id, admin.name, d.status, d.paid_at, d.created_at
		FROM deposits d
		INNER JOIN users u ON u.id = d.user_id
		LEFT JOIN users admin ON admin.id = d.deposited_by_id
		`+where+`
		ORDER BY d.created_at DESC
		LIMIT ? OFFSET ?
	`, queryArguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]deposit, 0)
	for rows.Next() {
		var item deposit
		var createdAt time.Time
		var transactionID, depositedBy sql.NullString
		var paidAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Customer, &item.CustomerEmail, &item.CustomerPhone,
			&item.Company, &item.Amount, &item.Currency, &item.PaymentMethod,
			&transactionID, &depositedBy, &item.Status, &paidAt, &createdAt,
		); err != nil {
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
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type ipWhitelistRequest struct {
	ID           int64   `json:"id"`
	CustomerID   int64   `json:"customer_id"`
	UserID       int64   `json:"user_id"`
	Customer     string  `json:"customer"`
	CustomerName string  `json:"customer_name"`
	Email        string  `json:"email"`
	IPAddress    string  `json:"ip_address"`
	Label        *string `json:"label"`
	Notes        *string `json:"notes"`
	Reason       *string `json:"reason"`
	AdminNote    *string `json:"admin_note"`
	Reviewer     *string `json:"reviewer"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	ReviewedAt   *string `json:"reviewed_at"`
}

func (server *Server) ipWhitelistRequests(context fiber.Ctx) error {
	page, perPage := pagination(context)

	empty := func() error {
		return listResponse(context, []ipWhitelistRequest{}, listMeta{Page: page, PerPage: perPage, Total: 0})
	}

	columns := tableColumns(server, "ip_whitelist_requests")
	if len(columns) == 0 {
		log.Printf("ip whitelist: table is not readable")
		return empty()
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM ip_whitelist_requests").Scan(&total); err != nil {
		log.Printf("ip whitelist count failed: %v", err)
		return empty()
	}

	reviewerJoin := ""
	reviewerExpr := "NULL"
	if columns["reviewer_id"] && tableExists(server, "users") {
		reviewerJoin = " LEFT JOIN users r ON r.id = w.reviewer_id"
		reviewerExpr = "r.name"
	}

	rows, err := server.db.Query(`
		SELECT w.id, COALESCE(w.user_id, 0), COALESCE(u.name, ''), COALESCE(u.email, ''),
			w.ip_address, `+textColumn(columns, "w", "label")+`, `+textColumn(columns, "w", "admin_note")+`,
			COALESCE(w.status, 'pending'), w.created_at, `+textColumn(columns, "w", "reviewed_at")+`, `+reviewerExpr+`
		FROM ip_whitelist_requests w
		LEFT JOIN users u ON u.id = w.user_id`+reviewerJoin+`
		ORDER BY w.created_at DESC
		LIMIT ? OFFSET ?
	`, perPage, (page-1)*perPage)
	if err != nil {
		log.Printf("ip whitelist query failed: %v", err)
		return empty()
	}
	defer rows.Close()

	items := make([]ipWhitelistRequest, 0)
	for rows.Next() {
		var item ipWhitelistRequest
		var label, adminNote, reviewer sql.NullString
		var createdAt time.Time
		var reviewedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.UserID, &item.Customer, &item.Email, &item.IPAddress,
			&label, &adminNote, &item.Status, &createdAt, &reviewedAt, &reviewer); err != nil {
			log.Printf("ip whitelist scan failed: %v", err)
			return empty()
		}
		item.CustomerID = item.UserID
		item.CustomerName = item.Customer
		if label.Valid {
			value := label.String
			item.Label = &value
			item.Notes = &value
			item.Reason = &value
		}
		if adminNote.Valid {
			value := adminNote.String
			item.AdminNote = &value
		}
		if reviewer.Valid {
			value := reviewer.String
			item.Reviewer = &value
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		if reviewedAt.Valid {
			value := reviewedAt.Time.Format(time.RFC3339)
			item.ReviewedAt = &value
		}
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

type smsLog struct {
	ID        int64   `json:"id"`
	Customer  string  `json:"customer"`
	Recipient string  `json:"recipient"`
	Body      string  `json:"body"`
	Gateway   *string `json:"gateway"`
	Cost      float64 `json:"cost"`
	Status    string  `json:"status"`
	CreatedAt string  `json:"created_at"`
}

func (server *Server) smsLogs(context fiber.Ctx) error {
	page, perPage := pagination(context)
	search := strings.TrimSpace(context.Query("search"))
	arguments := []any{}
	where := "WHERE 1 = 1"
	if search != "" {
		where += " AND (m.`to` LIKE ? OR m.body LIKE ? OR u.name LIKE ?)"
		term := "%" + search + "%"
		arguments = append(arguments, term, term, term)
	}

	var total int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM messages m INNER JOIN users u ON u.id = m.user_id "+where, arguments...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	arguments = append(arguments, perPage, (page-1)*perPage)
	rows, err := server.db.Query(`
		SELECT m.id, u.name, m.`+"`to`"+`, m.body, g.name, m.cost, m.status, m.created_at
		FROM messages m
		INNER JOIN users u ON u.id = m.user_id
		LEFT JOIN gateways g ON g.id = m.gateway_id
		`+where+`
		ORDER BY m.created_at DESC
		LIMIT ? OFFSET ?
	`, arguments...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]smsLog, 0)
	for rows.Next() {
		var item smsLog
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Customer, &item.Recipient, &item.Body, &item.Gateway, &item.Cost, &item.Status, &createdAt); err != nil {
			return databaseError(context, err)
		}
		item.CreatedAt = createdAt.Format(time.RFC3339)
		items = append(items, item)
	}

	return listResponse(context, items, listMeta{Page: page, PerPage: perPage, Total: total})
}

func listResponse(context fiber.Ctx, items any, meta listMeta) error {
	return context.Status(http.StatusOK).JSON(fiber.Map{"data": items, "meta": meta})
}
