package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"smswhiteinfo/customer-api/internal/billing"
	"smswhiteinfo/customer-api/internal/payment"
)

const (
	minDepositAmountBDT = 50.0
	maxDepositAmountBDT = 100000.0
)

func (server *Server) publicBaseURL(c fiber.Ctx) string {
	for _, envKey := range []string{"CUSTOMER_API_PUBLIC_URL", "APP_URL"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return strings.TrimRight(c.BaseURL(), "/")
}

func (server *Server) initiateDeposit(c fiber.Ctx) error {
	claims := customer(c)

	var req struct {
		Amount  float64 `json:"amount"`
		Gateway string  `json:"gateway"`
	}
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	if req.Amount < minDepositAmountBDT {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Minimum recharge amount is ৳50."})
	}
	if req.Amount > maxDepositAmountBDT {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Maximum recharge amount is ৳100,000."})
	}

	method := strings.ToLower(strings.TrimSpace(req.Gateway))
	if method == "" {
		method = "piprapay"
	}
	if method != "piprapay" {
		return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unsupported payment gateway."})
	}

	apiKey := strings.TrimSpace(os.Getenv("PIPRAPAY_API_KEY"))
	baseURL := strings.TrimSpace(os.Getenv("PIPRAPAY_BASE_URL"))
	driver, err := payment.NewPipraPayDriver(apiKey, baseURL)
	if err != nil {
		return c.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
			"success": false,
			"message": "Payment gateway is not configured. Please contact support.",
		})
	}

	var customerName, customerEmail, customerPhone, userType string
	var spID sql.NullInt64
	if err := server.db.QueryRowContext(c.Context(), `
		SELECT COALESCE(name, ''), COALESCE(email, ''), COALESCE(phone, ''), COALESCE(type, 'customer'), sp_id
		FROM users
		WHERE id = ? AND deleted_at IS NULL
	`, claims.UserID).Scan(&customerName, &customerEmail, &customerPhone, &userType, &spID); err != nil {
		return databaseError(c, err)
	}

	tx, err := server.db.BeginTx(c.Context(), nil)
	if err != nil {
		return databaseError(c, err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Match Reference DepositAction: check reseller balance before launching gateway checkout.
	if userType == "customer" && spID.Valid && spID.Int64 > 0 {
		var spBalance float64
		err := tx.QueryRowContext(c.Context(),
			`SELECT COALESCE(balance, 0) FROM users WHERE id = ? AND deleted_at IS NULL FOR UPDATE`,
			spID.Int64,
		).Scan(&spBalance)
		if err != nil {
			return databaseError(c, err)
		}
		if spBalance < req.Amount {
			return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
				"success": false,
				"message": "Payment gateway temporarily unavailable. Please contact your service provider.",
			})
		}
	}

	now := time.Now()
	res, err := tx.ExecContext(c.Context(), `
		INSERT INTO deposits (user_id, amount, currency, payment_method, status, created_at, updated_at)
		VALUES (?, ?, 'BDT', ?, 'pending', ?, ?)
	`, claims.UserID, req.Amount, method, now, now)
	if err != nil {
		return databaseError(c, err)
	}
	depID, err := res.LastInsertId()
	if err != nil {
		return databaseError(c, err)
	}

	publicBase := server.publicBaseURL(c)
	returnURL := fmt.Sprintf("%s/payments/%s/callback/%d", publicBase, method, depID)
	webhookURL := fmt.Sprintf("%s/webhooks/payment/%s", publicBase, method)

	payRes, err := driver.Create(c.Context(), payment.PaymentRequest{
		CustomerName:  customerName,
		CustomerEmail: customerEmail,
		CustomerPhone: customerPhone,
		Amount:        req.Amount,
		Currency:      "BDT",
		Metadata: map[string]any{
			"deposit_id": depID,
			"user_id":    claims.UserID,
		},
		ReturnURL:  returnURL,
		WebhookURL: webhookURL,
	})
	if err != nil || payRes == nil || payRes.CheckoutURL == "" {
		msg := "Failed to initiate payment with gateway."
		if err != nil {
			msg = fmt.Sprintf("Failed to initiate payment: %v", err)
		}
		return c.Status(http.StatusBadGateway).JSON(fiber.Map{
			"success": false,
			"message": msg,
		})
	}

	metaJSON := "{}"
	if payRes.Raw != nil {
		if encoded, err := json.Marshal(payRes.Raw); err == nil {
			metaJSON = string(encoded)
		}
	}
	if _, err := tx.ExecContext(c.Context(),
		`UPDATE deposits SET transaction_id = ?, meta = ?, updated_at = ? WHERE id = ?`,
		payRes.GatewayPaymentID, metaJSON, time.Now(), depID,
	); err != nil {
		return databaseError(c, err)
	}

	if err := tx.Commit(); err != nil {
		return databaseError(c, err)
	}

	return c.Status(http.StatusCreated).JSON(fiber.Map{
		"success":      true,
		"message":      "Deposit initiated successfully.",
		"redirect_url": payRes.CheckoutURL,
		"data": fiber.Map{
			"id":             depID,
			"transaction_id": payRes.GatewayPaymentID,
			"amount":         fmt.Sprintf("%.2f", req.Amount),
			"status":         "pending",
			"redirect_url":   payRes.CheckoutURL,
		},
	})
}

func parsePageArgs(c fiber.Ctx, defaultPerPage int) (int, int) {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	perPage, _ := strconv.Atoi(c.Query("per_page", strconv.Itoa(defaultPerPage)))
	if perPage < 1 {
		perPage = defaultPerPage
	}
	if perPage > 100 {
		perPage = 100
	}
	return page, perPage
}

func (server *Server) transactionsReport(c fiber.Ctx) error {
	claims := customer(c)
	page, perPage := parsePageArgs(c, 10)
	offset := (page - 1) * perPage

	conditions := []string{"t.user_id = ?"}
	args := []any{claims.UserID}

	direction := strings.TrimSpace(c.Query("direction"))
	if direction == "" {
		direction = strings.TrimSpace(c.Query("type"))
	}
	if direction == "credit" || direction == "debit" {
		conditions = append(conditions, "t.direction = ?")
		args = append(args, direction)
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		conditions = append(conditions, "(t.description LIKE ? OR t.transactionable_type LIKE ? OR t.direction LIKE ?)")
		args = append(args, like, like, like)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	var total int
	countArgs := append([]any{}, args...)
	if err := server.db.QueryRowContext(c.Context(),
		"SELECT COUNT(*) FROM transactions t "+where, countArgs...,
	).Scan(&total); err != nil {
		return databaseError(c, err)
	}

	queryArgs := append(append([]any{}, args...), perPage, offset)
	rows, err := server.db.QueryContext(c.Context(), `
		SELECT t.id, t.direction, t.amount, t.balance_before, t.balance_after,
		       COALESCE(t.description, ''), COALESCE(t.transactionable_type, ''), t.created_at
		FROM transactions t `+where+`
		ORDER BY t.created_at DESC, t.id DESC
		LIMIT ? OFFSET ?
	`, queryArgs...)
	if err != nil {
		return databaseError(c, err)
	}
	defer rows.Close()

	items := make([]fiber.Map, 0)
	for rows.Next() {
		var id int64
		var dir, desc, txType string
		var amount, balanceBefore, balanceAfter float64
		var createdAt time.Time
		if err := rows.Scan(&id, &dir, &amount, &balanceBefore, &balanceAfter, &desc, &txType, &createdAt); err != nil {
			return databaseError(c, err)
		}
		items = append(items, fiber.Map{
			"id":                   id,
			"direction":            dir,
			"type":                 dir,
			"amount":               fmt.Sprintf("%.2f", amount),
			"balance_before":       fmt.Sprintf("%.2f", balanceBefore),
			"balance_after":        fmt.Sprintf("%.2f", balanceAfter),
			"description":          desc,
			"transactionable_type": txType,
			"created_at":           createdAt.Format(time.RFC3339),
		})
	}

	return c.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) pricingReport(c fiber.Ctx) error {
	claims := customer(c)
	resolver, err := billing.ForUser(c.Context(), server.db, claims.UserID)
	if err != nil {
		return databaseError(c, err)
	}

	countryNames := make(map[int64]string)
	cRows, err := server.db.QueryContext(c.Context(), `SELECT id, name FROM countries WHERE is_active = 1`)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var id int64
			var name string
			if err := cRows.Scan(&id, &name); err == nil {
				countryNames[id] = name
			}
		}
	}

	effective := resolver.EffectiveOperatorRates()
	rates := make([]fiber.Map, 0, len(effective))
	for _, entry := range effective {
		country := countryNames[entry.Operator.CountryID]
		if country == "" {
			country = "Bangladesh"
		}
		var maskingRate any
		if entry.MaskingRate != nil {
			maskingRate = *entry.MaskingRate
		}
		var nonMaskingRate any
		if entry.NonMaskingRate != nil {
			nonMaskingRate = *entry.NonMaskingRate
		}
		codes := entry.Operator.Codes
		if codes == nil {
			codes = []string{}
		}
		rates = append(rates, fiber.Map{
			"id":               entry.Operator.ID,
			"name":             entry.Operator.Name,
			"country":          country,
			"country_code":     entry.Operator.CountryCode,
			"codes":            codes,
			"masking_rate":     maskingRate,
			"non_masking_rate": nonMaskingRate,
			"rate_source":      entry.RateSource,
		})
	}

	var planName any
	if name := resolver.PlanName(); name != "" {
		planName = name
	}

	return c.JSON(fiber.Map{
		"tariff_plan": planName,
		"data":        rates,
	})
}

func resolveAnalyticsDateRange(rangeParam, fromParam, toParam string) (time.Time, time.Time) {
	now := time.Now()
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch strings.ToLower(strings.TrimSpace(rangeParam)) {
	case "today":
		return startOfToday, endOfToday
	case "yesterday":
		y := startOfToday.AddDate(0, 0, -1)
		return y, time.Date(y.Year(), y.Month(), y.Day(), 23, 59, 59, 0, now.Location())
	case "this_week":
		offset := (int(now.Weekday()) + 6) % 7 // Monday is the first day of the week.
		return startOfToday.AddDate(0, 0, -offset), endOfToday
	case "this_month":
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return first, endOfToday
	case "last_month":
		firstThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		firstLast := firstThis.AddDate(0, -1, 0)
		endLast := firstThis.Add(-time.Second)
		return firstLast, endLast
	case "this_year":
		return time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location()), endOfToday
	case "custom":
		from, fromErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(fromParam), now.Location())
		to, toErr := time.ParseInLocation("2006-01-02", strings.TrimSpace(toParam), now.Location())
		if fromErr == nil && toErr == nil {
			if to.Before(from) {
				from, to = to, from
			}
			return from, time.Date(to.Year(), to.Month(), to.Day(), 23, 59, 59, 0, now.Location())
		}
		return startOfToday.AddDate(0, 0, -29), endOfToday
	case "30d", "last_30_days":
		return startOfToday.AddDate(0, 0, -29), endOfToday
	case "90d", "last_90_days":
		return startOfToday.AddDate(0, 0, -89), endOfToday
	default: // "7d" / "last_7_days"
		return startOfToday.AddDate(0, 0, -6), endOfToday
	}
}

type messagingAnalyticsStats struct {
	Total       int     `json:"total"`
	Successful  int     `json:"successful"`
	Delivered   int     `json:"delivered"`
	Failed      int     `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
	TotalCost   float64 `json:"total_cost"`
	TotalParts  int     `json:"total_parts"`
	AvgParts    float64 `json:"avg_parts"`
	Unicode     int     `json:"unicode"`
}

type messagingDailyPoint struct {
	Date         string  `json:"date"`
	DateVal      string  `json:"date_val"`
	Total        int     `json:"total"`
	Successful   int     `json:"successful"`
	Delivered    int     `json:"delivered"`
	Failed       int     `json:"failed"`
	DeliveryRate float64 `json:"delivery_rate"`
	SuccessRate  float64 `json:"success_rate"`
	Cost         float64 `json:"cost"`
}

type messagingOperatorBreakdown struct {
	Name    string  `json:"name"`
	Count   int     `json:"count"`
	Percent float64 `json:"percent"`
}

type messagingAnalyticsPayload struct {
	Stats     messagingAnalyticsStats      `json:"stats"`
	Campaigns map[string]int               `json:"campaigns"`
	Daily     []messagingDailyPoint        `json:"daily"`
	Operators []messagingOperatorBreakdown `json:"operators"`
}

func (server *Server) buildMessagingAnalyticsData(ctx context.Context, userID int64, from, to time.Time) (*messagingAnalyticsPayload, error) {
	var total, successful, delivered, failed, totalParts, unicode int
	var totalCost, avgParts float64

	err := server.db.QueryRowContext(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN status IN ('sent', 'delivered') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status IN ('failed', 'dlr_failed') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN status NOT IN ('failed', 'dlr_failed') THEN cost ELSE 0 END), 0),
		       COALESCE(SUM(parts), 0),
		       COALESCE(AVG(parts), 0),
		       COALESCE(SUM(is_unicode), 0)
		FROM messages
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
	`, userID, from, to).Scan(&total, &successful, &delivered, &failed, &totalCost, &totalParts, &avgParts, &unicode)
	if err != nil {
		return nil, err
	}

	successRate := 0.0
	if total > 0 {
		successRate = math.Round((float64(successful)/float64(total))*1000) / 10
	}

	dailyRows, err := server.db.QueryContext(ctx, `
		SELECT DATE_FORMAT(created_at, '%Y-%m-%d') as date_val,
		       COUNT(*) as total,
		       COALESCE(SUM(CASE WHEN status IN ('sent', 'delivered') THEN 1 ELSE 0 END), 0) as successful,
		       COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0) as delivered,
		       COALESCE(SUM(CASE WHEN status IN ('failed', 'dlr_failed') THEN 1 ELSE 0 END), 0) as failed,
		       COALESCE(SUM(CASE WHEN status NOT IN ('failed', 'dlr_failed') THEN cost ELSE 0 END), 0) as cost
		FROM messages
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		GROUP BY date_val
		ORDER BY date_val ASC
	`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer dailyRows.Close()

	daily := make([]messagingDailyPoint, 0)
	for dailyRows.Next() {
		var dateVal string
		var dTotal, dSuccessful, dDelivered, dFailed int
		var dCost float64
		if err := dailyRows.Scan(&dateVal, &dTotal, &dSuccessful, &dDelivered, &dFailed, &dCost); err != nil {
			return nil, err
		}
		label := dateVal
		if parsed, err := time.Parse("2006-01-02", dateVal); err == nil {
			label = parsed.Format("Jan 02")
		}
		rate := 0.0
		if dTotal > 0 {
			rate = math.Round((float64(dSuccessful)/float64(dTotal))*1000) / 10
		}
		daily = append(daily, messagingDailyPoint{
			Date:         label,
			DateVal:      dateVal,
			Total:        dTotal,
			Successful:   dSuccessful,
			Delivered:    dDelivered,
			Failed:       dFailed,
			DeliveryRate: rate,
			SuccessRate:  rate,
			Cost:         math.Round(dCost*100) / 100,
		})
	}

	// Compute real operator breakdown using OperatorRateResolver prefix mapping.
	opCounts := make(map[string]int)
	if resolver, rErr := billing.ForUser(ctx, server.db, userID); rErr == nil {
		opRows, oErr := server.db.QueryContext(ctx, "SELECT COALESCE(`to`, '') FROM messages WHERE user_id = ? AND created_at >= ? AND created_at <= ?", userID, from, to)
		if oErr == nil {
			defer opRows.Close()
			for opRows.Next() {
				var recipient string
				if scanErr := opRows.Scan(&recipient); scanErr == nil {
					name := resolver.OperatorNameFor(recipient)
					if name == "" {
						name = "Other"
					}
					opCounts[name]++
				}
			}
		}
	}

	operators := make([]messagingOperatorBreakdown, 0, len(opCounts))
	for name, count := range opCounts {
		pct := 0.0
		if total > 0 {
			pct = math.Round((float64(count)/float64(total))*1000) / 10
		}
		operators = append(operators, messagingOperatorBreakdown{
			Name:    name,
			Count:   count,
			Percent: pct,
		})
	}
	sort.Slice(operators, func(i, j int) bool {
		if operators[i].Count == operators[j].Count {
			return operators[i].Name < operators[j].Name
		}
		return operators[i].Count > operators[j].Count
	})

	return &messagingAnalyticsPayload{
		Stats: messagingAnalyticsStats{
			Total:       total,
			Successful:  successful,
			Delivered:   delivered,
			Failed:      failed,
			SuccessRate: successRate,
			TotalCost:   math.Round(totalCost*100) / 100,
			TotalParts:  totalParts,
			AvgParts:    math.Round(avgParts*10) / 10,
			Unicode:     unicode,
		},
		Campaigns: map[string]int{"total": 0, "sent": 0},
		Daily:     daily,
		Operators: operators,
	}, nil
}

func (server *Server) messagingAnalytics(c fiber.Ctx) error {
	claims := customer(c)
	from, to := resolveAnalyticsDateRange(c.Query("range", "7d"), c.Query("from"), c.Query("to"))

	payload, err := server.buildMessagingAnalyticsData(c.Context(), claims.UserID, from, to)
	if err != nil {
		return databaseError(c, err)
	}

	return c.JSON(fiber.Map{"data": payload})
}

func (server *Server) exportMessagingAnalytics(c fiber.Ctx) error {
	claims := customer(c)
	from, to := resolveAnalyticsDateRange(c.Query("range", "7d"), c.Query("from"), c.Query("to"))
	format := strings.ToLower(strings.TrimSpace(c.Query("format", "csv")))

	payload, err := server.buildMessagingAnalyticsData(c.Context(), claims.UserID, from, to)
	if err != nil {
		return databaseError(c, err)
	}

	// Order daily rows DESC for tabular CSV/PDF reports (matches Reference MessagingAnalytics.php).
	dailyDesc := append([]messagingDailyPoint(nil), payload.Daily...)
	sort.Slice(dailyDesc, func(i, j int) bool {
		return dailyDesc[i].DateVal > dailyDesc[j].DateVal
	})

	timestamp := time.Now().Format("2006-01-02-150405")

	if format == "pdf" {
		var userName, userEmail string
		_ = server.db.QueryRowContext(c.Context(),
			`SELECT COALESCE(name, 'Customer'), COALESCE(email, '') FROM users WHERE id = ? AND deleted_at IS NULL`,
			claims.UserID,
		).Scan(&userName, &userEmail)

		periodLabel := fmt.Sprintf("%s - %s", from.Format("02 Jan 2006"), to.Format("02 Jan 2006"))
		pdfBytes := buildMessagingAnalyticsPDF(userName, userEmail, periodLabel, payload.Stats, dailyDesc)

		filename := fmt.Sprintf("messaging-analytics-%s.pdf", timestamp)
		c.Set("Content-Type", "application/pdf")
		c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		return c.Send(pdfBytes)
	}

	// Default: CSV with UTF-8 BOM matching ReportExportService::downloadCsv
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{
		"Date",
		"Total Messages",
		"Successful",
		"Delivered",
		"Failed",
		"Success Rate",
		"Spend (BDT)",
	})
	for _, day := range dailyDesc {
		rateStr := "0%"
		if day.Total > 0 {
			rateStr = fmt.Sprintf("%.1f%%", day.SuccessRate)
		}
		_ = w.Write([]string{
			day.DateVal,
			strconv.Itoa(day.Total),
			strconv.Itoa(day.Successful),
			strconv.Itoa(day.Delivered),
			strconv.Itoa(day.Failed),
			rateStr,
			fmt.Sprintf("%.2f", day.Cost),
		})
	}
	w.Flush()

	filename := fmt.Sprintf("messaging-analytics-%s.csv", timestamp)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return c.Send(buf.Bytes())
}

func buildMessagingAnalyticsPDF(
	userName, userEmail, period string,
	stats messagingAnalyticsStats,
	dailyDesc []messagingDailyPoint,
) []byte {
	const pageHeight = 842.0
	const pageWidth = 595.0
	const rowsPerPage = 32

	chunks := make([][]messagingDailyPoint, 0)
	if len(dailyDesc) == 0 {
		chunks = append(chunks, nil)
	} else {
		for start := 0; start < len(dailyDesc); start += rowsPerPage {
			end := start + rowsPerPage
			if end > len(dailyDesc) {
				end = len(dailyDesc)
			}
			chunks = append(chunks, dailyDesc[start:end])
		}
	}

	pages := make([]string, 0, len(chunks))
	for pageIdx, chunk := range chunks {
		var content strings.Builder
		content.WriteString(pdfText(40, pageHeight-44, "F2", 15, "Messaging Analytics Report"))
		content.WriteString(pdfText(40, pageHeight-62, "F1", 8, fmt.Sprintf(
			"Customer: %s (%s)  |  Period: %s  |  Page %d of %d",
			pdfTruncate(userName, 30),
			pdfTruncate(userEmail, 35),
			period,
			pageIdx+1,
			len(chunks),
		)))

		y := pageHeight - 86
		if pageIdx == 0 {
			content.WriteString(pdfText(40, y, "F2", 10, "Performance Summary"))
			y -= 16
			content.WriteString(pdfText(40, y, "F1", 8, fmt.Sprintf(
				"Total Messages: %d    Success Rate: %.1f%%    Delivered: %d    Failed: %d",
				stats.Total, stats.SuccessRate, stats.Delivered, stats.Failed,
			)))
			y -= 14
			content.WriteString(pdfText(40, y, "F1", 8, fmt.Sprintf(
				"Total Cost: %.2f BDT    Avg Parts: %.1f    Unicode: %d    Total Parts: %d",
				stats.TotalCost, stats.AvgParts, stats.Unicode, stats.TotalParts,
			)))
			y -= 24
		}

		content.WriteString(pdfText(40, y, "F2", 10, "Daily Messaging Breakdown"))
		y -= 16
		content.WriteString(pdfText(40, y, "F2", 8, "Date"))
		content.WriteString(pdfText(120, y, "F2", 8, "Total"))
		content.WriteString(pdfText(185, y, "F2", 8, "Successful"))
		content.WriteString(pdfText(255, y, "F2", 8, "Delivered"))
		content.WriteString(pdfText(325, y, "F2", 8, "Failed"))
		content.WriteString(pdfText(390, y, "F2", 8, "Success Rate"))
		content.WriteString(pdfText(475, y, "F2", 8, "Spend (BDT)"))
		y -= 6
		content.WriteString(fmt.Sprintf("0.5 w 40 %.1f m 555 %.1f l S\n", y, y))
		y -= 14

		if len(chunk) == 0 {
			content.WriteString(pdfText(40, y, "F1", 8, "No messaging data recorded in the selected period."))
		} else {
			for _, day := range chunk {
				dateStr := day.DateVal
				if parsed, err := time.Parse("2006-01-02", day.DateVal); err == nil {
					dateStr = parsed.Format("02 Jan 2006")
				}
				content.WriteString(pdfText(40, y, "F1", 8, dateStr))
				content.WriteString(pdfText(120, y, "F1", 8, strconv.Itoa(day.Total)))
				content.WriteString(pdfText(185, y, "F1", 8, strconv.Itoa(day.Successful)))
				content.WriteString(pdfText(255, y, "F1", 8, strconv.Itoa(day.Delivered)))
				content.WriteString(pdfText(325, y, "F1", 8, strconv.Itoa(day.Failed)))
				content.WriteString(pdfText(390, y, "F1", 8, fmt.Sprintf("%.1f%%", day.SuccessRate)))
				content.WriteString(pdfText(475, y, "F1", 8, fmt.Sprintf("%.2f", day.Cost)))
				y -= 16
			}
		}

		content.WriteString(pdfText(40, 28, "F1", 7, fmt.Sprintf(
			"Generated %s  |  White INFO SMS Platform",
			time.Now().Format("02 Jan 2006, 03:04 PM"),
		)))
		pages = append(pages, content.String())
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, 0, 5+len(pages)*2)
	writeObj := func(id int, body string) {
		offsets = append(offsets, out.Len())
		out.WriteString(fmt.Sprintf("%d 0 obj\n%s\nendobj\n", id, body))
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	pageObjStart := 5
	kids := make([]string, 0, len(pages))
	for i := range pages {
		kids = append(kids, fmt.Sprintf("%d 0 R", pageObjStart+i*2))
	}
	writeObj(2, fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pages), strings.Join(kids, " ")))
	writeObj(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	writeObj(4, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")

	for i, stream := range pages {
		pID := pageObjStart + i*2
		cID := pID + 1
		writeObj(pID, fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents %d 0 R >>",
			pageWidth, pageHeight, cID,
		))
		writeObj(cID, fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}

	xrefOffset := out.Len()
	out.WriteString(fmt.Sprintf("xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1))
	for _, off := range offsets {
		out.WriteString(fmt.Sprintf("%010d 00000 n \n", off))
	}
	out.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xrefOffset))
	return out.Bytes()
}
