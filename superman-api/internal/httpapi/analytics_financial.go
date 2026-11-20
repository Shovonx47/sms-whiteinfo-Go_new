package httpapi

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

type financialPeriodStats struct {
	Deposits        float64 `json:"deposits"`
	CustomerSpend   float64 `json:"customer_spend"`
	GatewayCost     float64 `json:"gateway_cost"`
	PendingDeposits float64 `json:"pending_deposits"`
	Transactions    int64   `json:"transactions"`
	SystemBalance   float64 `json:"system_balance"`
}

type financialRevenueStats struct {
	TotalDeposits   float64 `json:"total_deposits"`
	PendingDeposits float64 `json:"pending_deposits"`
	TotalSpent      float64 `json:"total_spent"`
	SystemBalance   float64 `json:"system_balance"`
}

type financialUserStats struct {
	NewUsers30d      int64   `json:"new_users_30d"`
	PendingApprovals int64   `json:"pending_approvals"`
	APIUsers         int64   `json:"api_users"`
	TotalBalance     float64 `json:"total_balance"`
}

type depositTrendPoint struct {
	Date  string  `json:"date"`
	Label string  `json:"label"`
	Total float64 `json:"total"`
}

type spendingTrendPoint struct {
	Date    string  `json:"date"`
	Label   string  `json:"label"`
	Revenue float64 `json:"revenue"`
	Cost    float64 `json:"cost"`
}

type depositMethodPoint struct {
	Method string  `json:"method"`
	Label  string  `json:"label"`
	Count  int64   `json:"count"`
	Total  float64 `json:"total"`
}

type transactionVolumePoint struct {
	Date    string `json:"date"`
	Label   string `json:"label"`
	Credits int64  `json:"credits"`
	Debits  int64  `json:"debits"`
}

type topPeriodCustomer struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name"`
	Email         string   `json:"email"`
	Status        string   `json:"status"`
	Balance       float64  `json:"balance"`
	PeriodSpent   *float64 `json:"period_spent"`
	MessagesCount int64    `json:"messages_count"`
}

func normalizeSQLDay(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 10 {
		trimmed = trimmed[:10]
	}
	if parsed, err := time.Parse("2006-01-02", trimmed); err == nil {
		return trimmed, parsed.Format("Jan 02")
	}
	return trimmed, trimmed
}

func formatDepositMethodLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "manual":
		return "Manual"
	case "piprapay":
		return "PipraPay"
	case "":
		return "Unknown"
	default:
		clean := strings.ReplaceAll(strings.TrimSpace(raw), "_", " ")
		if len(clean) == 0 {
			return "Unknown"
		}
		return strings.ToUpper(clean[:1]) + clean[1:]
	}
}

func (server *Server) analyticsFinancial(c fiber.Ctx) error {
	dr := parseAnalyticsDateRange(c)

	var systemBalance float64
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(balance), 0)
		FROM users
		WHERE type = 'customer' AND deleted_at IS NULL
	`).Scan(&systemBalance)

	// 1. FinancialStatsWidget (period-scoped)
	period := financialPeriodStats{SystemBalance: round2(systemBalance)}
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM deposits
		WHERE status = 'completed' AND deleted_at IS NULL AND created_at >= ? AND created_at <= ?
	`, dr.From, dr.To).Scan(&period.Deposits)
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(cost), 0)
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
	`, dr.From, dr.To).Scan(&period.CustomerSpend)
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(operator_cost + primary_dipping_cost), 0)
		FROM messages
		WHERE created_at >= ? AND created_at <= ? AND status <> 'failed'
	`, dr.From, dr.To).Scan(&period.GatewayCost)
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM deposits
		WHERE status = 'pending' AND deleted_at IS NULL AND created_at >= ? AND created_at <= ?
	`, dr.From, dr.To).Scan(&period.PendingDeposits)
	_ = server.db.QueryRow(`
		SELECT COUNT(*)
		FROM transactions
		WHERE created_at >= ? AND created_at <= ?
	`, dr.From, dr.To).Scan(&period.Transactions)

	period.Deposits = round2(period.Deposits)
	period.CustomerSpend = round2(period.CustomerSpend)
	period.GatewayCost = round2(period.GatewayCost)
	period.PendingDeposits = round2(period.PendingDeposits)

	// 2. RevenueStatsWidget (all-time)
	revenue := financialRevenueStats{SystemBalance: round2(systemBalance)}
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM deposits
		WHERE status = 'completed' AND deleted_at IS NULL
	`).Scan(&revenue.TotalDeposits)
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(amount), 0)
		FROM deposits
		WHERE status = 'pending' AND deleted_at IS NULL
	`).Scan(&revenue.PendingDeposits)
	_ = server.db.QueryRow(`SELECT COALESCE(SUM(cost), 0) FROM messages`).Scan(&revenue.TotalSpent)
	revenue.TotalDeposits = round2(revenue.TotalDeposits)
	revenue.PendingDeposits = round2(revenue.PendingDeposits)
	revenue.TotalSpent = round2(revenue.TotalSpent)

	// 3. UserStatsWidget
	users := financialUserStats{TotalBalance: round2(systemBalance)}
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	_ = server.db.QueryRow(`
		SELECT COUNT(*)
		FROM users
		WHERE type = 'customer' AND deleted_at IS NULL AND created_at >= ?
	`, thirtyDaysAgo).Scan(&users.NewUsers30d)
	_ = server.db.QueryRow(`
		SELECT COUNT(*)
		FROM users
		WHERE type = 'customer' AND deleted_at IS NULL AND status = 'pending'
	`).Scan(&users.PendingApprovals)
	_ = server.db.QueryRow(`
		SELECT COUNT(*)
		FROM users
		WHERE type = 'customer' AND deleted_at IS NULL AND api_access = TRUE
	`).Scan(&users.APIUsers)

	// 4. DepositTrendChart
	depositTrend := make([]depositTrendPoint, 0)
	if rows, err := server.db.Query(`
		SELECT DATE(created_at) AS day, COALESCE(SUM(amount), 0) AS total
		FROM deposits
		WHERE status = 'completed' AND deleted_at IS NULL AND created_at >= ? AND created_at <= ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, dr.From, dr.To); err == nil {
		defer rows.Close()
		for rows.Next() {
			var rawDay string
			var total float64
			if rows.Scan(&rawDay, &total) == nil {
				d, label := normalizeSQLDay(rawDay)
				depositTrend = append(depositTrend, depositTrendPoint{Date: d, Label: label, Total: round2(total)})
			}
		}
	}

	// 5. SpendingTrendChart (matches Laravel: SUM(cost) as revenue, SUM(primary_dipping_cost) as cost)
	spendingTrend := make([]spendingTrendPoint, 0)
	if rows, err := server.db.Query(`
		SELECT DATE(created_at) AS day,
		       COALESCE(SUM(cost), 0) AS revenue,
		       COALESCE(SUM(primary_dipping_cost), 0) AS cost
		FROM messages
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, dr.From, dr.To); err == nil {
		defer rows.Close()
		for rows.Next() {
			var rawDay string
			var rev, cost float64
			if rows.Scan(&rawDay, &rev, &cost) == nil {
				d, label := normalizeSQLDay(rawDay)
				spendingTrend = append(spendingTrend, spendingTrendPoint{Date: d, Label: label, Revenue: round2(rev), Cost: round2(cost)})
			}
		}
	}

	// 6. DepositMethodChart
	depositsByMethod := make([]depositMethodPoint, 0)
	if rows, err := server.db.Query(`
		SELECT COALESCE(payment_method, '') AS method, COUNT(*) AS cnt, COALESCE(SUM(amount), 0) AS total
		FROM deposits
		WHERE status = 'completed' AND deleted_at IS NULL AND created_at >= ? AND created_at <= ?
		GROUP BY COALESCE(payment_method, '')
		ORDER BY method ASC
	`, dr.From, dr.To); err == nil {
		defer rows.Close()
		for rows.Next() {
			var method string
			var cnt int64
			var total float64
			if rows.Scan(&method, &cnt, &total) == nil {
				depositsByMethod = append(depositsByMethod, depositMethodPoint{
					Method: method,
					Label:  formatDepositMethodLabel(method),
					Count:  cnt,
					Total:  round2(total),
				})
			}
		}
	}

	// 7. TransactionVolumeChart
	txVolume := make([]transactionVolumePoint, 0)
	if rows, err := server.db.Query(`
		SELECT DATE(created_at) AS day,
		       COALESCE(SUM(CASE WHEN direction = 'credit' THEN 1 ELSE 0 END), 0) AS credits,
		       COALESCE(SUM(CASE WHEN direction = 'debit' THEN 1 ELSE 0 END), 0) AS debits
		FROM transactions
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY DATE(created_at)
		ORDER BY DATE(created_at) ASC
	`, dr.From, dr.To); err == nil {
		defer rows.Close()
		for rows.Next() {
			var rawDay string
			var credits, debits int64
			if rows.Scan(&rawDay, &credits, &debits) == nil {
				d, label := normalizeSQLDay(rawDay)
				txVolume = append(txVolume, transactionVolumePoint{Date: d, Label: label, Credits: credits, Debits: debits})
			}
		}
	}

	// 8. TopCustomersByPeriodWidget
	search := strings.TrimSpace(c.Query("search"))
	sortCol := strings.ToLower(strings.TrimSpace(c.Query("sort", "period_spent")))
	sortDir := "DESC"
	if strings.EqualFold(strings.TrimSpace(c.Query("dir")), "asc") {
		sortDir = "ASC"
	}

	orderBy := "COALESCE(SUM(m.cost), 0) " + sortDir + ", u.id ASC"
	switch sortCol {
	case "name":
		orderBy = "u.name " + sortDir + ", u.id ASC"
	case "balance":
		orderBy = "u.balance " + sortDir + ", u.id ASC"
	case "messages_count":
		orderBy = "COUNT(m.id) " + sortDir + ", u.id ASC"
	case "period_spent":
		orderBy = "COALESCE(SUM(m.cost), 0) " + sortDir + ", u.id ASC"
	}

	custArgs := []any{dr.From, dr.To}
	custWhere := "WHERE u.type = 'customer' AND u.deleted_at IS NULL"
	if search != "" {
		custWhere += " AND (u.name LIKE ? OR u.email LIKE ?)"
		term := "%" + search + "%"
		custArgs = append(custArgs, term, term)
	}

	topCustomers := make([]topPeriodCustomer, 0)
	if rows, err := server.db.Query(`
		SELECT u.id, u.name, u.email, u.status, u.balance,
		       SUM(m.cost) AS period_spent,
		       COUNT(m.id) AS messages_count
		FROM users u
		LEFT JOIN messages m ON m.user_id = u.id AND m.created_at >= ? AND m.created_at <= ?
		`+custWhere+`
		GROUP BY u.id, u.name, u.email, u.status, u.balance
		ORDER BY `+orderBy+`
		LIMIT 10
	`, custArgs...); err == nil {
		defer rows.Close()
		for rows.Next() {
			var item topPeriodCustomer
			var spent sql.NullFloat64
			if rows.Scan(&item.ID, &item.Name, &item.Email, &item.Status, &item.Balance, &spent, &item.MessagesCount) == nil {
				item.Balance = round2(item.Balance)
				if spent.Valid {
					val := round2(spent.Float64)
					item.PeriodSpent = &val
				}
				topCustomers = append(topCustomers, item)
			}
		}
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"period_stats":       period,
			"revenue_stats":      revenue,
			"user_stats":         users,
			"deposit_trend":      depositTrend,
			"spending_trend":     spendingTrend,
			"deposits_by_method": depositsByMethod,
			"transaction_volume": txVolume,
			"top_customers":      topCustomers,
		},
	})
}
