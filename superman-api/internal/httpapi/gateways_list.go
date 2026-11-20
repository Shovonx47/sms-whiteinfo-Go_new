package httpapi

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// gatewayMetrics mirrors App\Services\Analytics\GatewayMetricsService::getMetrics().
// Pointer fields are null when the gateway does not support DLR (shown as "N/A").
type gatewayMetrics struct {
	MaskingSent           int64    `json:"masking_sent"`
	NonMaskingSent        int64    `json:"non_masking_sent"`
	TotalSent             int64    `json:"total_sent"`
	MaskingDelivered      *int64   `json:"masking_delivered"`
	NonMaskingDelivered   *int64   `json:"non_masking_delivered"`
	TotalDelivered        *int64   `json:"total_delivered"`
	DlrPending            *int64   `json:"dlr_pending"`
	DlrPendingPct         *float64 `json:"dlr_pending_pct"`
	SendFailed            int64    `json:"send_failed"`
	SendFailedPct         float64  `json:"send_failed_pct"`
	DlrFailed             *int64   `json:"dlr_failed"`
	DlrFailedPct          *float64 `json:"dlr_failed_pct"`
	Processing            int64    `json:"processing"`
	ProcessingPct         float64  `json:"processing_pct"`
	MaskingDeliveryPct    *float64 `json:"masking_delivery_pct"`
	NonMaskingDeliveryPct *float64 `json:"non_masking_delivery_pct"`
	MaskingCost           float64  `json:"masking_cost"`
	NonMaskingCost        float64  `json:"non_masking_cost"`
	OperatorCost          float64  `json:"operator_cost"`
	DippingCharge         float64  `json:"dipping_charge"`
	ThirdPartyDippingCost float64  `json:"third_party_dipping_cost"`
	TotalCost             float64  `json:"total_cost"`
	ProviderBalance       *string  `json:"provider_balance"`
	DippingBalance        *string  `json:"dipping_balance"`
	Revenue               float64  `json:"revenue"`
	Profit                float64  `json:"profit"`
	Verification          string   `json:"verification"`
	VerificationColor     string   `json:"verification_color"`
}

type GatewayListItem struct {
	ID                   int64          `json:"id"`
	Name                 string         `json:"name"`
	Driver               string         `json:"driver"`
	Priority             int            `json:"priority"`
	IsEnabled            bool           `json:"is_enabled"`
	Enabled              bool           `json:"enabled"`
	IsOtpEnabled         bool           `json:"is_otp_enabled"`
	OtpEnabled           bool           `json:"otp_enabled"`
	RequestsPerSecond    int            `json:"requests_per_second"`
	HealthStatus         string         `json:"health_status"`
	CostMode             string         `json:"cost_mode"`
	DlrSupport           bool           `json:"dlr_support"`
	HasThirdPartyDipping bool           `json:"has_third_party_dipping"`
	RouteCode            *string        `json:"route_code"`
	CreatedAt            string         `json:"created_at"`
	UpdatedAt            string         `json:"updated_at"`
	DeletedAt            *string        `json:"deleted_at"`
	Metrics              gatewayMetrics `json:"metrics"`
}

type gatewayAggregate struct {
	total, masking, nonMasking                         int64
	maskingDelivered, nonMaskingDelivered, delivered   int64
	dlrPending, sendFailed, dlrFailed, processing      int64
	revenue, operatorCost, dippingCost, thirdPartyCost float64
}

func roundTo(value float64, places int) float64 {
	factor := math.Pow(10, float64(places))
	return math.Round(value*factor) / factor
}

func (server *Server) gatewayAggregates() map[int64]*gatewayAggregate {
	result := map[int64]*gatewayAggregate{}
	rows, err := server.db.Query(`
		SELECT m.gateway_id,
		       COUNT(*),
		       COALESCE(SUM(CASE WHEN s.type IN ('alphanumeric', 'shortcode') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN s.type IS NULL OR s.type NOT IN ('alphanumeric', 'shortcode') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN s.type IN ('alphanumeric', 'shortcode') AND m.status = 'delivered' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN (s.type IS NULL OR s.type NOT IN ('alphanumeric', 'shortcode')) AND m.status = 'delivered' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status = 'delivered' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status = 'sent' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status = 'failed' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status = 'dlr_failed' THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status IN ('pending', 'sending') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status <> 'failed' THEN m.cost ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status <> 'failed' THEN m.operator_cost ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status <> 'failed' THEN m.primary_dipping_cost ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status <> 'failed' THEN m.third_party_dipping_cost ELSE 0 END), 0)
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		WHERE m.gateway_id IS NOT NULL
		GROUP BY m.gateway_id
	`)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var a gatewayAggregate
		if err := rows.Scan(&id, &a.total, &a.masking, &a.nonMasking, &a.maskingDelivered, &a.nonMaskingDelivered,
			&a.delivered, &a.dlrPending, &a.sendFailed, &a.dlrFailed, &a.processing,
			&a.revenue, &a.operatorCost, &a.dippingCost, &a.thirdPartyCost); err == nil {
			copied := a
			result[id] = &copied
		}
	}
	return result
}

func (server *Server) gatewayRateAverages() (masking map[int64]float64, nonMasking map[int64]float64) {
	masking = map[int64]float64{}
	nonMasking = map[int64]float64{}
	rows, err := server.db.Query(`
		SELECT gateway_id, COALESCE(AVG(masking_price), 0), COALESCE(AVG(non_masking_price), 0)
		FROM gateway_operator_rates GROUP BY gateway_id
	`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var m, n float64
		if rows.Scan(&id, &m, &n) == nil {
			masking[id] = m
			nonMasking[id] = n
		}
	}
	return
}

// latestPeriodCosts returns the manual cost of the most recent period per gateway.
func (server *Server) latestPeriodCosts(table string) map[int64]float64 {
	result := map[int64]float64{}
	rows, err := server.db.Query("SELECT gateway_id, manual_cost FROM " + table + " ORDER BY period_end DESC, id DESC")
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var cost sql.NullFloat64
		if rows.Scan(&id, &cost) != nil {
			continue
		}
		if _, seen := result[id]; !seen {
			result[id] = cost.Float64
		}
	}
	return result
}

func buildGatewayMetrics(item GatewayListItem, agg *gatewayAggregate, maskingRate, nonMaskingRate, periodCost, tpPeriodCost map[int64]float64) gatewayMetrics {
	if agg == nil {
		agg = &gatewayAggregate{}
	}
	hasDlr := item.DlrSupport
	m := gatewayMetrics{
		MaskingSent:    agg.masking,
		NonMaskingSent: agg.nonMasking,
		TotalSent:      agg.total,
		SendFailed:     agg.sendFailed,
		Processing:     agg.processing,
		MaskingCost:    maskingRate[item.ID],
		NonMaskingCost: nonMaskingRate[item.ID],
		Revenue:        agg.revenue,
	}

	pct := func(part, whole int64) float64 {
		if whole <= 0 {
			return 0
		}
		return roundTo(float64(part)/float64(whole)*100, 2)
	}
	i64 := func(v int64) *int64 { return &v }
	f64 := func(v float64) *float64 { return &v }

	m.SendFailedPct = pct(agg.sendFailed, agg.total)
	m.ProcessingPct = pct(agg.processing, agg.total)

	if hasDlr {
		m.MaskingDelivered = i64(agg.maskingDelivered)
		m.NonMaskingDelivered = i64(agg.nonMaskingDelivered)
		m.TotalDelivered = i64(agg.delivered)
		m.DlrPending = i64(agg.dlrPending)
		m.DlrFailed = i64(agg.dlrFailed)
		if agg.total > 0 {
			m.DlrPendingPct = f64(pct(agg.dlrPending, agg.total))
			m.DlrFailedPct = f64(pct(agg.dlrFailed, agg.total))
		}
		m.MaskingDeliveryPct = f64(0)
		m.NonMaskingDeliveryPct = f64(0)
		if agg.masking > 0 {
			m.MaskingDeliveryPct = f64(pct(agg.maskingDelivered, agg.masking))
		}
		if agg.nonMasking > 0 {
			m.NonMaskingDeliveryPct = f64(pct(agg.nonMaskingDelivered, agg.nonMasking))
		}
	}

	m.OperatorCost = agg.operatorCost
	if cost, ok := periodCost[item.ID]; ok {
		m.OperatorCost = cost
	}
	m.DippingCharge = agg.dippingCost
	if item.HasThirdPartyDipping {
		m.ThirdPartyDippingCost = agg.thirdPartyCost
		if cost, ok := tpPeriodCost[item.ID]; ok {
			m.ThirdPartyDippingCost = cost
		}
	}
	m.TotalCost = m.OperatorCost + m.DippingCharge + m.ThirdPartyDippingCost
	m.Profit = m.Revenue - m.TotalCost

	switch {
	case !hasDlr:
		m.Verification, m.VerificationColor = "Send Only", "gray"
	case agg.dlrPending > 0:
		m.Verification, m.VerificationColor = "Pending DLR", "warning"
	default:
		m.Verification, m.VerificationColor = "Verified", "success"
	}
	return m
}

func (server *Server) gateways(context fiber.Ctx) error {
	trashed := strings.ToLower(strings.TrimSpace(context.Query("trashed")))
	where := "WHERE deleted_at IS NULL"
	switch trashed {
	case "with":
		where = ""
	case "only":
		where = "WHERE deleted_at IS NOT NULL"
	}

	rows, err := server.db.Query(`
		SELECT id, name, driver, priority, is_otp_enabled, is_enabled, requests_per_second,
		       health_status, cost_mode, dlr_support, has_third_party_dipping, credentials,
		       DATE_FORMAT(created_at, '%b %d, %Y %H:%i:%s'),
		       DATE_FORMAT(updated_at, '%b %d, %Y %H:%i:%s'),
		       DATE_FORMAT(deleted_at, '%b %d, %Y %H:%i:%s')
		FROM gateways ` + where + `
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	enc, _ := server.getEncrypter()
	aggregates := server.gatewayAggregates()
	maskingRates, nonMaskingRates := server.gatewayRateAverages()
	periodCosts := server.latestPeriodCosts("gateway_operator_cost_periods")
	tpPeriodCosts := server.latestPeriodCosts("gateway_third_party_dipping_periods")

	items := make([]GatewayListItem, 0)
	for rows.Next() {
		var item GatewayListItem
		var rawCreds, created, updated, deleted sql.NullString

		if err := rows.Scan(
			&item.ID, &item.Name, &item.Driver, &item.Priority, &item.IsOtpEnabled, &item.IsEnabled,
			&item.RequestsPerSecond, &item.HealthStatus, &item.CostMode, &item.DlrSupport, &item.HasThirdPartyDipping,
			&rawCreds, &created, &updated, &deleted,
		); err != nil {
			return databaseError(context, err)
		}

		item.Enabled = item.IsEnabled
		item.OtpEnabled = item.IsOtpEnabled
		item.CreatedAt = created.String
		item.UpdatedAt = updated.String
		if deleted.Valid {
			value := deleted.String
			item.DeletedAt = &value
		}
		if rawCreds.Valid && rawCreds.String != "" && enc != nil {
			if credMap, err := enc.DecryptArray(rawCreds.String); err == nil {
				item.RouteCode = extractRouteCode(credMap)
			}
		}
		item.Metrics = buildGatewayMetrics(item, aggregates[item.ID], maskingRates, nonMaskingRates, periodCosts, tpPeriodCosts)
		items = append(items, item)
	}

	// Filters
	search := strings.ToLower(strings.TrimSpace(context.Query("search")))
	driverFilter := strings.ToLower(strings.TrimSpace(context.Query("driver")))
	enabledFilter := strings.ToLower(strings.TrimSpace(context.Query("enabled")))
	otpFilter := strings.ToLower(strings.TrimSpace(context.Query("otp")))

	filtered := items[:0:0]
	for _, item := range items {
		if search != "" {
			route := ""
			if item.RouteCode != nil {
				route = strings.ToLower(*item.RouteCode)
			}
			if !strings.Contains(strings.ToLower(item.Name), search) &&
				!strings.Contains(strings.ToLower(item.Driver), search) &&
				!strings.Contains(route, search) {
				continue
			}
		}
		if driverFilter != "" && strings.ToLower(item.Driver) != driverFilter {
			continue
		}
		if enabledFilter == "yes" && !item.IsEnabled || enabledFilter == "no" && item.IsEnabled {
			continue
		}
		if otpFilter == "yes" && !item.IsOtpEnabled || otpFilter == "no" && item.IsOtpEnabled {
			continue
		}
		filtered = append(filtered, item)
	}

	// Sorting (Filament sortable columns: name, priority, requests_per_second).
	sortKey := strings.TrimSpace(context.Query("sort"))
	descending := strings.EqualFold(context.Query("dir"), "desc")
	if sortKey != "" {
		less := func(a, b GatewayListItem) bool {
			switch sortKey {
			case "name":
				return strings.ToLower(a.Name) < strings.ToLower(b.Name)
			case "priority":
				return a.Priority < b.Priority
			case "requests_per_second":
				return a.RequestsPerSecond < b.RequestsPerSecond
			case "driver":
				return a.Driver < b.Driver
			case "total_sent":
				return a.Metrics.TotalSent < b.Metrics.TotalSent
			case "created_at":
				return a.ID < b.ID
			}
			return a.ID < b.ID
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			if descending {
				return less(filtered[j], filtered[i])
			}
			return less(filtered[i], filtered[j])
		})
	}

	total := len(filtered)
	exportAll := context.Query("export") == "1"
	paged := !exportAll && (context.Query("page") != "" || context.Query("per_page") != "")

	page, perPage := 1, total
	if paged {
		perPage = 10
		if value, err := strconv.Atoi(context.Query("per_page")); err == nil && value > 0 && value <= 100 {
			perPage = value
		}
		if value, err := strconv.Atoi(context.Query("page")); err == nil && value > 0 {
			page = value
		}
		start := (page - 1) * perPage
		if start > total {
			start = total
		}
		end := start + perPage
		if end > total {
			end = total
		}
		filtered = filtered[start:end]
	}
	if perPage < 1 {
		perPage = 1
	}
	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}

	return context.JSON(fiber.Map{
		"data": filtered,
		"meta": fiber.Map{"current_page": page, "per_page": perPage, "total": total, "last_page": lastPage},
	})
}

type gatewayBulkPayload struct {
	IDs    []int64 `json:"ids"`
	Action string  `json:"action"`
}

// bulkGateways implements Filament's Delete / Restore / Force-delete bulk actions.
func (server *Server) bulkGateways(context fiber.Ctx) error {
	var payload gatewayBulkPayload
	if err := json.Unmarshal(context.Body(), &payload); err != nil || len(payload.IDs) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one gateway."})
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(payload.IDs)), ",")
	args := make([]any, 0, len(payload.IDs))
	for _, id := range payload.IDs {
		args = append(args, id)
	}

	var query, message string
	switch payload.Action {
	case "delete":
		query = "UPDATE gateways SET deleted_at = NOW() WHERE deleted_at IS NULL AND id IN (" + placeholders + ")"
		message = "Selected gateways deleted."
	case "restore":
		query = "UPDATE gateways SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		message = "Selected gateways restored."
	case "force_delete":
		query = "DELETE FROM gateways WHERE deleted_at IS NOT NULL AND id IN (" + placeholders + ")"
		message = "Selected gateways permanently deleted."
	default:
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Unknown bulk action."})
	}

	result, err := server.db.Exec(query, args...)
	if err != nil {
		return databaseError(context, err)
	}
	affected, _ := result.RowsAffected()
	return context.JSON(fiber.Map{"message": message, "affected": affected})
}
