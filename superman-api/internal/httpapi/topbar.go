package httpapi

import (
	"database/sql"
	"log"
	"regexp"
	"time"

	"github.com/gofiber/fiber/v3"
)

type topbarStat struct {
	Title string `json:"title"`
	Value string `json:"value"`
	URL   string `json:"url"`
	Theme string `json:"theme"`
}

// topbarScan runs one single-value stat query. A failure is logged with the stat name and
// leaves the stat at its zero value, so one broken query cannot take the whole top bar down
// (and the real database error is never silently swallowed).
func topbarScan(server *Server, name string, query string, args []any, dest any) {
	if err := server.db.QueryRow(query, args...).Scan(dest); err != nil {
		log.Printf("topbar-stats: %s query failed: %v", name, err)
	}
}

func (server *Server) topbarStats(context fiber.Ctx) error {
	var kycPending, smsSent, lowBalance, openTickets, ipWhitelist int
	var deposits, profit float64

	// "Today" is a half-open [start, end) range built in Go and passed as parameters. This
	// avoids CURDATE() / DATE(col), which are MySQL-only and break on PostgreSQL.
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dayEnd := dayStart.AddDate(0, 0, 1)
	today := []any{dayStart.Format("2006-01-02 15:04:05"), dayEnd.Format("2006-01-02 15:04:05")}

	topbarScan(server, "kyc_pending",
		`SELECT COUNT(*) FROM users WHERE type = 'customer' AND kyc_status = 'under_review' AND deleted_at IS NULL`,
		nil, &kycPending)
	topbarScan(server, "today_deposit",
		`SELECT COALESCE(SUM(amount), 0) FROM deposits WHERE created_at >= ? AND created_at < ? AND status = 'completed' AND deleted_at IS NULL`,
		today, &deposits)
	topbarScan(server, "today_sms",
		`SELECT COUNT(*) FROM messages WHERE created_at >= ? AND created_at < ? AND status IN ('sent', 'delivered')`,
		today, &smsSent)
	topbarScan(server, "today_profit",
		`SELECT COALESCE(SUM(cost - (operator_cost + primary_dipping_cost)), 0) FROM messages WHERE created_at >= ? AND created_at < ?`,
		today, &profit)
	topbarScan(server, "low_balance",
		`SELECT COUNT(*) FROM users WHERE type = 'customer' AND balance < 100 AND deleted_at IS NULL`,
		nil, &lowBalance)
	topbarScan(server, "ip_whitelist",
		`SELECT COUNT(*) FROM ip_whitelist_requests WHERE status = 'pending'`,
		nil, &ipWhitelist)

	if tableExists(server, "tickets") {
		topbarScan(server, "open_tickets", `SELECT COUNT(*) FROM tickets WHERE status IN ('open', 'pending')`, nil, &openTickets)
	}

	return context.JSON(fiber.Map{
		"data": map[string]topbarStat{
			"kyc_pending": {
				Title: "KYC Pending", Value: integer(kycPending), URL: "/superman/kyc/requests", Theme: "purple",
			},
			"today_deposit": {
				Title: "Today Deposit", Value: currency(deposits), URL: "/superman/finance/deposits", Theme: "green",
			},
			"today_sms": {
				Title: "Today SMS Sent", Value: integer(smsSent), URL: "/superman/system-logs/sms-logs", Theme: "blue",
			},
			"today_profit": {
				Title: "Today Profit", Value: currency(profit), URL: "/superman/profit/global", Theme: "orange",
			},
			"low_balance": {
				Title: "Low Balance", Value: integer(lowBalance), URL: "/superman/users/customers?filter=low_balance", Theme: "red",
			},
			"open_tickets": {
				Title: "Open Tickets", Value: integer(openTickets), URL: "/superman/sms/inquiry", Theme: "cyan",
			},
			"ip_whitelist": {
				Title: "IP Whitelist", Value: integer(ipWhitelist), URL: "/superman/approval-center/ip-whitelists", Theme: "indigo",
			},
		},
	})
}

var safeTableName = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// tableExists probes the table with a zero-row SELECT instead of querying
// information_schema with DATABASE(), which is MySQL-only. Callers pass hard-coded names;
// the pattern check keeps the identifier safe to interpolate.
func tableExists(server *Server, name string) bool {
	if !safeTableName.MatchString(name) {
		return false
	}

	var one int
	err := server.db.QueryRow("SELECT 1 FROM " + name + " WHERE 1 = 0").Scan(&one)

	// sql.ErrNoRows means the table exists and the zero-row probe returned nothing.
	return err == nil || err == sql.ErrNoRows
}

func integer(value int) string {
	return formatNumber(float64(value), 0)
}

func currency(value float64) string {
	return "৳" + formatNumber(value, 2)
}
