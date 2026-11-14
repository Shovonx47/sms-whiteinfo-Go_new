package httpapi

import (
	stdctx "context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"smswhiteinfo/customer-api/internal/billing"
)

// ─────────────────────────────────────────────────────────────────────────────
// Old Laravel API response contract (App\Enums\ApiResponseCode / ApiResponse)
// ─────────────────────────────────────────────────────────────────────────────

const (
	codeSMSSubmitted         = 1000
	codeRequestAccepted      = 1002
	codeDeliveryReport       = 1003
	codeInvalidAPIKey        = 2000
	codeAPIKeyMissing        = 2001
	codeAccountSuspended     = 2002
	codeIPNotWhitelisted     = 2003
	codePermissionDenied     = 2004
	codeInsufficientBalance  = 3000
	codeInvalidMobileNumber  = 4000
	codeInvalidSenderID      = 4001
	codeMissingParameter     = 4003
	codeInvalidRequestFormat = 4004
	codeDuplicateRequest     = 4006
	codeInvalidScheduleTime  = 4007
	codeInternalServerError  = 5004
)

func apiCodeMessage(code int) string {
	switch code {
	case codeSMSSubmitted:
		return "SMS Submitted Successfully"
	case codeRequestAccepted:
		return "Request Accepted"
	case codeDeliveryReport:
		return "Delivery Report Available"
	case codeInvalidAPIKey:
		return "Invalid API Key"
	case codeAPIKeyMissing:
		return "API Key Missing"
	case codeAccountSuspended:
		return "Account Suspended"
	case codeIPNotWhitelisted:
		return "IP Not Whitelisted"
	case codePermissionDenied:
		return "Permission Denied"
	case codeInsufficientBalance:
		return "Insufficient Balance"
	case codeInvalidMobileNumber:
		return "Invalid Mobile Number"
	case codeInvalidSenderID:
		return "Invalid Sender ID"
	case codeMissingParameter:
		return "Missing Required Parameter"
	case codeInvalidRequestFormat:
		return "Invalid Request Format"
	case codeDuplicateRequest:
		return "Duplicate Request"
	case codeInvalidScheduleTime:
		return "Invalid Schedule Time"
	case codeInternalServerError:
		return "Internal Server Error"
	}

	return "Error occurred"
}

func apiCodeHTTPStatus(code int) int {
	switch code {
	case codeSMSSubmitted, codeDeliveryReport:
		return http.StatusOK
	case codeRequestAccepted:
		return http.StatusAccepted
	case codeInvalidAPIKey, codeAPIKeyMissing:
		return http.StatusUnauthorized
	case codeAccountSuspended, codeIPNotWhitelisted, codePermissionDenied:
		return http.StatusForbidden
	case codeInsufficientBalance:
		return http.StatusPaymentRequired
	case codeInvalidMobileNumber, codeInvalidSenderID, codeMissingParameter, codeInvalidRequestFormat, codeDuplicateRequest, codeInvalidScheduleTime:
		return http.StatusUnprocessableEntity
	case codeInternalServerError:
		return http.StatusInternalServerError
	}

	return http.StatusBadRequest
}

func apiSuccess(context fiber.Ctx, code int, message string, extra fiber.Map, httpStatus int) error {
	if message == "" {
		message = apiCodeMessage(code)
	}
	if httpStatus == 0 {
		httpStatus = apiCodeHTTPStatus(code)
	}

	payload := fiber.Map{"status": "success", "code": code, "message": message}
	for key, value := range extra {
		payload[key] = value
	}

	return context.Status(httpStatus).JSON(payload)
}

func apiError(context fiber.Ctx, code int, message string, extra fiber.Map, httpStatus int) error {
	if message == "" {
		message = apiCodeMessage(code)
	}
	if httpStatus == 0 {
		httpStatus = apiCodeHTTPStatus(code)
	}

	payload := fiber.Map{"status": "error", "code": code, "message": message}
	for key, value := range extra {
		payload[key] = value
	}

	return context.Status(httpStatus).JSON(payload)
}

// ─────────────────────────────────────────────────────────────────────────────
// External Developer API — token-authenticated (/api/v1/*)
// ─────────────────────────────────────────────────────────────────────────────

type devUserData struct {
	ID              int64
	Name            string
	Email           string
	Phone           *string
	Balance         float64
	Status          string
	APIAccess       bool
	SMSAutoApproved bool
	TokenID         int64
}

type securitySettings struct {
	IPWhitelistEnforced        bool
	AllowAllWhenWhitelistEmpty bool
	AutoApproveAPIRequests     bool
	WebPortalBypassEnabled     bool
	TrustedPortalIPs           string
	BlockedMessage             string
}

func defaultSecuritySettings() securitySettings {
	return securitySettings{
		IPWhitelistEnforced:        true,
		AllowAllWhenWhitelistEmpty: false,
		AutoApproveAPIRequests:     false,
		WebPortalBypassEnabled:     true,
		TrustedPortalIPs:           "127.0.0.1\n::1",
		BlockedMessage:             "Your IP address is not whitelisted. SMS sending through this IP is not allowed.",
	}
}

func (server *Server) securitySettings() securitySettings {
	settings := defaultSecuritySettings()

	rows, err := server.db.Query(`SELECT name, payload FROM settings WHERE ` + "`group`" + ` = 'security'`)
	if err != nil {
		return settings
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var payload []byte
		if rows.Scan(&name, &payload) != nil {
			continue
		}

		var value any
		if json.Unmarshal(payload, &value) != nil {
			continue
		}

		switch name {
		case "ip_whitelist_enforced":
			settings.IPWhitelistEnforced = settingBool(value, settings.IPWhitelistEnforced)
		case "allow_all_when_whitelist_empty":
			settings.AllowAllWhenWhitelistEmpty = settingBool(value, settings.AllowAllWhenWhitelistEmpty)
		case "auto_approve_api_requests":
			settings.AutoApproveAPIRequests = settingBool(value, settings.AutoApproveAPIRequests)
		case "web_portal_bypass_enabled":
			settings.WebPortalBypassEnabled = settingBool(value, settings.WebPortalBypassEnabled)
		case "trusted_portal_ips":
			if text, ok := value.(string); ok {
				settings.TrustedPortalIPs = text
			}
		case "blocked_message":
			if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
				settings.BlockedMessage = text
			}
		}
	}

	return settings
}

func settingBool(value any, fallback bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return typed == "1" || strings.EqualFold(typed, "true")
	case float64:
		return typed != 0
	}

	return fallback
}

func (server *Server) RegisterExternalAPI(app *fiber.App) {
	ext := app.Group("/api/v1", server.devAuthenticate)

	// Account
	ext.Get("/me", server.extMe)
	ext.Get("/balance", server.extBalance)
	ext.Get("/senders", server.extSenders)

	// Messaging
	ext.Get("/messages", server.extSendMessage)
	ext.Post("/messages", server.extSendMessage)
	ext.Get("/batches/:batch", server.extShowBatch)

	// IP Whitelisting
	ext.Get("/ip-whitelists", server.extIPWhitelists)
	ext.Post("/ip-whitelists", server.extCreateIPWhitelist)
	ext.Delete("/ip-whitelists/:id", server.extDeleteIPWhitelist)
}

func devTokenFromRequest(context fiber.Ctx) string {
	if header := context.Get(fiber.HeaderAuthorization); strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}

	for _, key := range []string{"api_key", "api_token"} {
		if value := strings.TrimSpace(context.Query(key)); value != "" {
			return value
		}
		if value := strings.TrimSpace(context.FormValue(key)); value != "" {
			return value
		}
	}

	return ""
}

func (server *Server) devAuthenticate(context fiber.Ctx) error {
	rawToken := devTokenFromRequest(context)
	if rawToken == "" {
		return apiError(context, codeAPIKeyMissing, "API Key Missing", nil, 0)
	}

	tokenParts := strings.SplitN(rawToken, "|", 2)
	if len(tokenParts) != 2 {
		return apiError(context, codeInvalidAPIKey, "Invalid API Key", nil, 0)
	}

	digest := sha256.Sum256([]byte(tokenParts[1]))
	hashedToken := hex.EncodeToString(digest[:])

	var user devUserData
	err := server.db.QueryRow(`
		SELECT u.id, u.name, u.email, u.phone, u.balance, u.status,
		       COALESCE(u.api_access, 0), COALESCE(u.sms_auto_approved, 1), t.id
		FROM access_tokens t
		JOIN users u ON u.id = t.tokenable_id
		WHERE t.id = ?
		  AND t.token = ?
		  AND t.tokenable_type = 'App\\Models\\User'
		  AND u.type = 'customer'
		  AND u.deleted_at IS NULL
		LIMIT 1
	`, tokenParts[0], hashedToken).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone,
		&user.Balance, &user.Status, &user.APIAccess, &user.SMSAutoApproved, &user.TokenID,
	)
	if err == sql.ErrNoRows {
		return apiError(context, codeInvalidAPIKey, "Invalid API Key", nil, 0)
	}
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	if !user.APIAccess {
		return apiError(context, codePermissionDenied, "Permission Denied: API access is disabled for this account.", nil, 0)
	}

	if user.Status != "approved" {
		return apiError(context, codeAccountSuspended, "Account Suspended: Account is not approved for API usage.", nil, 0)
	}

	clientIP := context.IP()

	// Customers must always be able to manage their whitelist, even when their
	// current IP is not approved yet.
	if !strings.HasPrefix(context.Path(), "/api/v1/ip-whitelists") {
		settings := server.securitySettings()
		if allowed, message := server.apiIPAllowed(user.ID, clientIP, settings); !allowed {
			return apiError(context, codeIPNotWhitelisted, message, fiber.Map{"client_ip": clientIP}, 0)
		}
	}

	_, _ = server.db.Exec("UPDATE access_tokens SET last_used_at = ? WHERE id = ?", time.Now(), user.TokenID)

	context.Locals("dev_user", user)
	return context.Next()
}

func getDevUser(context fiber.Ctx) devUserData {
	return context.Locals("dev_user").(devUserData)
}

func (server *Server) apiIPAllowed(userID int64, clientIP string, settings securitySettings) (bool, string) {
	if settings.WebPortalBypassEnabled && clientIP != "" && ipMatchesList(clientIP, settings.TrustedPortalIPs) {
		return true, ""
	}

	if !settings.IPWhitelistEnforced {
		return true, ""
	}

	approved, err := server.approvedWhitelistIPs(userID)
	if err != nil {
		return false, settings.BlockedMessage
	}

	if len(approved) == 0 {
		if settings.AllowAllWhenWhitelistEmpty {
			return true, ""
		}

		return false, "IP Not Whitelisted: Direct API access requires at least one approved public IP in your whitelist. Please add your IP in Developer Settings or via the IP Whitelist API."
	}

	if clientIP == "" || !ipMatchesAny(clientIP, approved) {
		return false, settings.BlockedMessage
	}

	return true, ""
}

func (server *Server) approvedWhitelistIPs(userID int64) ([]string, error) {
	rows, err := server.db.Query(
		"SELECT ip_address FROM ip_whitelist_requests WHERE user_id = ? AND status = 'approved'",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := []string{}
	for rows.Next() {
		var entry string
		if rows.Scan(&entry) == nil {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

func splitIPList(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})

	out := []string{}
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func ipMatchesList(clientIP, list string) bool {
	return ipMatchesAny(clientIP, splitIPList(list))
}

func ipMatchesAny(clientIP string, entries []string) bool {
	parsed := net.ParseIP(clientIP)
	if parsed == nil {
		return false
	}

	for _, entry := range entries {
		if strings.Contains(entry, "/") {
			_, network, err := net.ParseCIDR(entry)
			if err == nil && network.Contains(parsed) {
				return true
			}
			continue
		}

		if candidate := net.ParseIP(entry); candidate != nil && candidate.Equal(parsed) {
			return true
		}
	}

	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/me
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) extMe(context fiber.Ctx) error {
	u := getDevUser(context)

	return apiSuccess(context, codeRequestAccepted, "Profile retrieved successfully.", fiber.Map{
		"data": fiber.Map{
			"id":       u.ID,
			"name":     u.Name,
			"email":    u.Email,
			"phone":    u.Phone,
			"balance":  u.Balance,
			"approval": u.Status,
		},
	}, 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/balance
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) extBalance(context fiber.Ctx) error {
	u := getDevUser(context)

	var balance float64
	if err := server.db.QueryRow("SELECT balance FROM users WHERE id = ?", u.ID).Scan(&balance); err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	return apiSuccess(context, codeRequestAccepted, "Balance retrieved successfully", fiber.Map{
		"balance":  balance,
		"currency": "BDT",
	}, 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/senders
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) extSenders(context fiber.Ctx) error {
	u := getDevUser(context)

	rows, err := server.db.Query(`
		SELECT DISTINCT s.sender_id, s.type
		FROM senders s
		JOIN sender_user su ON su.sender_id = s.id
		WHERE su.user_id = ?
		  AND s.deleted_at IS NULL
		ORDER BY s.sender_id ASC
	`, u.ID)
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var senderID string
		var senderType *string
		if rows.Scan(&senderID, &senderType) != nil {
			continue
		}

		var category any
		if senderType != nil {
			normalized := strings.ToLower(strings.TrimSpace(*senderType))
			if normalized == "numeric" || strings.Contains(normalized, "non") {
				category = "non-masking"
			} else {
				category = "masking"
			}
		}

		items = append(items, fiber.Map{
			"sender_id": senderID,
			"type":      category,
		})
	}

	return apiSuccess(context, codeRequestAccepted, "Senders retrieved successfully.", fiber.Map{"data": items}, 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/messages  POST /api/v1/messages
// ─────────────────────────────────────────────────────────────────────────────

type sendMessageInput struct {
	SenderID   string
	NumbersRaw any
	Body       string
	Schedule   string
	TemplateID any
}

func readSendMessageInput(context fiber.Ctx) sendMessageInput {
	input := sendMessageInput{
		SenderID: context.Query("sender_id"),
		Body:     context.Query("body"),
		Schedule: context.Query("schedule"),
	}
	if input.Schedule == "" {
		input.Schedule = context.Query("scheduled_at")
	}
	if numbers := context.Query("numbers"); numbers != "" {
		input.NumbersRaw = numbers
	} else if phone := context.Query("phone"); phone != "" {
		input.NumbersRaw = phone
	} else if to := context.Query("to"); to != "" {
		input.NumbersRaw = to
	}
	if input.Body == "" {
		input.Body = context.Query("message")
	}

	if context.Method() == http.MethodPost {
		var req struct {
			SenderID    string `json:"sender_id"`
			Numbers     any    `json:"numbers"`
			Phone       any    `json:"phone"`
			To          any    `json:"to"`
			Body        string `json:"body"`
			Message     string `json:"message"`
			Schedule    string `json:"schedule"`
			ScheduledAt string `json:"scheduled_at"`
			TemplateID  any    `json:"template_id"`
		}

		if err := json.Unmarshal(context.Body(), &req); err == nil && req.SenderID != "" {
			if req.SenderID != "" {
				input.SenderID = req.SenderID
			}
			if req.Numbers != nil {
				input.NumbersRaw = req.Numbers
			} else if req.Phone != nil {
				input.NumbersRaw = req.Phone
			} else if req.To != nil {
				input.NumbersRaw = req.To
			}
			if req.Body != "" {
				input.Body = req.Body
			} else if req.Message != "" {
				input.Body = req.Message
			}
			if req.Schedule != "" {
				input.Schedule = req.Schedule
			} else if req.ScheduledAt != "" {
				input.Schedule = req.ScheduledAt
			}
			input.TemplateID = req.TemplateID
		} else {
			if value := context.FormValue("sender_id"); value != "" {
				input.SenderID = value
			}
			if value := context.FormValue("numbers"); value != "" {
				input.NumbersRaw = value
			} else if value := context.FormValue("phone"); value != "" {
				input.NumbersRaw = value
			} else if value := context.FormValue("to"); value != "" {
				input.NumbersRaw = value
			}
			if value := context.FormValue("body"); value != "" {
				input.Body = value
			} else if value := context.FormValue("message"); value != "" {
				input.Body = value
			}
			if value := context.FormValue("schedule"); value != "" {
				input.Schedule = value
			} else if value := context.FormValue("scheduled_at"); value != "" {
				input.Schedule = value
			}
			input.TemplateID = context.FormValue("template_id")
		}
	}

	return input
}

func normalizeRecipientNumbers(raw any) []string {
	items := []string{}
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			items = append(items, fmt.Sprint(item))
		}
	case []string:
		items = append(items, typed...)
	case string:
		items = strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || r == ';' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
		})
	}

	out := []string{}
	for _, item := range items {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func parseSchedule(raw string) (*time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	location, err := time.LoadLocation("Asia/Dhaka")
	if err != nil {
		location = time.Local
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		var parsed time.Time
		if strings.Contains(layout, "Z07:00") {
			parsed, err = time.Parse(layout, raw)
		} else {
			parsed, err = time.ParseInLocation(layout, raw, location)
		}
		if err == nil {
			if parsed.Before(time.Now().Add(-time.Minute)) {
				return nil, false
			}

			return &parsed, true
		}
	}

	return nil, false
}

func templateIDValue(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case string:
		id, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return id
	}

	return 0
}

func (server *Server) extSendMessage(context fiber.Ctx) error {
	u := getDevUser(context)
	input := readSendMessageInput(context)

	senderLabel := strings.TrimSpace(input.SenderID)
	if senderLabel == "" {
		return apiError(context, codeMissingParameter, "Missing Required Parameter: The sender_id parameter is required.", nil, 0)
	}

	if input.NumbersRaw == nil {
		return apiError(context, codeMissingParameter, "Missing Required Parameter: The numbers parameter is required.", nil, 0)
	}

	body := strings.TrimSpace(input.Body)
	if body == "" {
		return apiError(context, codeMissingParameter, "Missing Required Parameter: The body parameter is required.", nil, 0)
	}

	var senderDBID int64
	var senderType *string
	err := server.db.QueryRow(`
		SELECT s.id, s.type
		FROM senders s
		JOIN sender_user su ON su.sender_id = s.id
		WHERE s.sender_id = ?
		  AND su.user_id = ?
		  AND s.deleted_at IS NULL
		LIMIT 1
	`, senderLabel, u.ID).Scan(&senderDBID, &senderType)
	if err == sql.ErrNoRows {
		return apiError(context, codeInvalidSenderID, fmt.Sprintf("Invalid Sender ID: Sender %q is not available for this account.", senderLabel), nil, 0)
	}
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	numbers := normalizeRecipientNumbers(input.NumbersRaw)
	if len(numbers) == 0 {
		return apiError(context, codeInvalidMobileNumber, "Invalid Mobile Number: At least one valid recipient phone number is required.", nil, 0)
	}

	resolver, err := billing.ForUser(context.Context(), server.db, u.ID)
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	isMasking := false
	if senderType != nil {
		normalized := strings.ToLower(strings.TrimSpace(*senderType))
		isMasking = normalized != "numeric" && !strings.Contains(normalized, "non")
	}

	parts, _ := billing.CalculateParts(body)
	maxRate := 0.0
	for _, rate := range resolver.RatesForUser(isMasking) {
		if rate > maxRate {
			maxRate = rate
		}
	}

	estimatedCost := billing.RoundFloat(maxRate*float64(parts)*float64(len(numbers)), 4)
	var balance float64
	if err := server.db.QueryRow("SELECT balance FROM users WHERE id = ?", u.ID).Scan(&balance); err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}
	if len(resolver.RatesForUser(isMasking)) > 0 && balance < estimatedCost {
		return apiError(context, codeInsufficientBalance, "Insufficient Balance", fiber.Map{"balance": balance}, 0)
	}

	var scheduledAt *time.Time
	if strings.TrimSpace(input.Schedule) != "" {
		parsed, ok := parseSchedule(input.Schedule)
		if !ok {
			return apiError(context, codeInvalidScheduleTime, "Invalid Schedule Time: Unrecognized schedule date format or a time in the past.", nil, 0)
		}
		scheduledAt = parsed
	}

	requiresApproval := !u.SMSAutoApproved
	templateID := templateIDValue(input.TemplateID)

	if requiresApproval && templateID > 0 {
		var approval *string
		err := server.db.QueryRow(
			"SELECT approval FROM templates WHERE id = ? AND user_id = ? LIMIT 1",
			templateID, u.ID,
		).Scan(&approval)
		if err == nil && approval != nil && strings.EqualFold(strings.TrimSpace(*approval), "approved") {
			requiresApproval = false
		}
	}

	status := "processing"
	if scheduledAt != nil {
		status = "scheduled"
	} else if requiresApproval {
		status = "pending"
	}

	var approval any
	if requiresApproval {
		approval = "pending"
	}

	recipients := make([]any, len(numbers))
	for i, number := range numbers {
		recipients[i] = map[string]any{"phone": number, "values": map[string]string{}}
	}
	sourceConfig, _ := json.Marshal(map[string]any{
		"recipients": recipients,
		"ip":         context.IP(),
	})

	var templateValue any
	if templateID > 0 {
		templateValue = templateID
	}

	now := time.Now()
	result, err := server.db.Exec(`
		INSERT INTO batches (user_id, sender_id, template_id, body, source_type, source_ip, source_config, status, scheduled_at, approval, total_recipients, valid_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'api', ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`, u.ID, senderDBID, templateValue, body, context.IP(), string(sourceConfig), status, scheduledAt, approval, len(numbers), now, now)
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	batchID, _ := result.LastInsertId()

	if !requiresApproval && scheduledAt == nil {
		go func(bid int64) {
			_ = server.batchProcessor.ProcessBatch(stdctx.Background(), bid)
		}(batchID)
	}

	var scheduledValue any
	if scheduledAt != nil {
		scheduledValue = scheduledAt.Format(time.RFC3339)
	}

	return apiSuccess(context, codeSMSSubmitted, "SMS submitted successfully", fiber.Map{
		"message_id":   fmt.Sprintf("SMS%d", batchID),
		"batch_id":     batchID,
		"status":       status,
		"approval":     approval,
		"recipients":   len(numbers),
		"scheduled_at": scheduledValue,
	}, http.StatusOK)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /api/v1/batches/:batch
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) extShowBatch(context fiber.Ctx) error {
	u := getDevUser(context)
	batchParam := context.Params("batch")

	var id int64
	var senderID int64
	var status string
	var approval *string
	var totalRecipients, validCount, invalidCount int
	var createdAt time.Time

	err := server.db.QueryRow(`
		SELECT b.id, b.sender_id, b.status, b.approval, b.total_recipients, b.valid_count, b.invalid_count, b.created_at
		FROM batches b
		WHERE b.id = ? AND b.user_id = ?
	`, batchParam, u.ID).Scan(&id, &senderID, &status, &approval, &totalRecipients, &validCount, &invalidCount, &createdAt)
	if err == sql.ErrNoRows {
		return apiError(context, codeMissingParameter, "Batch not found.", nil, http.StatusNotFound)
	}
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	return apiSuccess(context, codeDeliveryReport, "Delivery Report Available", fiber.Map{
		"message_id":       fmt.Sprintf("SMS%d", id),
		"id":               id,
		"sender_id":        senderID,
		"status":           status,
		"approval":         approval,
		"total_recipients": totalRecipients,
		"valid_count":      validCount,
		"invalid_count":    invalidCount,
		"created_at":       createdAt.Format(time.RFC3339),
	}, 0)
}

// ─────────────────────────────────────────────────────────────────────────────
// IP whitelisting endpoints
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) extIPWhitelists(context fiber.Ctx) error {
	u := getDevUser(context)

	rows, err := server.db.Query(`
		SELECT id, ip_address, COALESCE(label, ''), status, admin_note, created_at, reviewed_at
		FROM ip_whitelist_requests
		WHERE user_id = ?
		ORDER BY id DESC
	`, u.ID)
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var ip, label, status string
		var adminNote *string
		var createdAt time.Time
		var reviewedAt *time.Time

		if rows.Scan(&id, &ip, &label, &status, &adminNote, &createdAt, &reviewedAt) != nil {
			continue
		}

		items = append(items, fiber.Map{
			"id":          id,
			"ip_address":  ip,
			"label":       label,
			"status":      status,
			"admin_note":  adminNote,
			"created_at":  createdAt.Format(time.RFC3339),
			"reviewed_at": apiTimePtr(reviewedAt),
		})
	}

	return apiSuccess(context, codeRequestAccepted, "Whitelisted IPs retrieved successfully.", fiber.Map{"data": items}, 0)
}

func (server *Server) extCreateIPWhitelist(context fiber.Ctx) error {
	u := getDevUser(context)

	ip := strings.TrimSpace(context.FormValue("ip_address"))
	label := strings.TrimSpace(context.FormValue("label"))

	var req struct {
		IPAddress string `json:"ip_address"`
		Label     string `json:"label"`
	}
	if err := json.Unmarshal(context.Body(), &req); err == nil {
		if req.IPAddress != "" {
			ip = strings.TrimSpace(req.IPAddress)
		}
		if req.Label != "" {
			label = strings.TrimSpace(req.Label)
		}
	}

	if ip == "" {
		return apiError(context, codeMissingParameter, "The ip_address field is required.", nil, 0)
	}

	if errMsg := validatePublicIPStr(ip); errMsg != "" {
		return apiError(context, codeInvalidRequestFormat, errMsg, nil, 0)
	}

	var existingID int
	var existingStatus string
	err := server.db.QueryRow(
		"SELECT id, status FROM ip_whitelist_requests WHERE user_id = ? AND ip_address = ? LIMIT 1",
		u.ID, ip,
	).Scan(&existingID, &existingStatus)
	if err == nil {
		return apiError(context, codeDuplicateRequest,
			fmt.Sprintf("IP address %s is already registered on your account with status [%s].", ip, existingStatus),
			fiber.Map{"data": fiber.Map{"id": existingID, "ip_address": ip, "status": existingStatus}},
			http.StatusConflict,
		)
	}

	settings := server.securitySettings()
	now := time.Now()
	status := "pending"
	var reviewedAt any
	if settings.AutoApproveAPIRequests {
		status = "approved"
		reviewedAt = now
	}

	result, err := server.db.Exec(`
		INSERT INTO ip_whitelist_requests (user_id, ip_address, label, status, reviewed_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, u.ID, ip, nullString(label), status, reviewedAt, now, now)
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	id, _ := result.LastInsertId()

	server.recordIPWhitelistAudit(u, id, "created", ip,
		nil,
		map[string]any{"ip_address": ip, "label": label, "status": status},
		map[bool]string{true: "Added via API (Auto-approved per security policy)", false: "Added via API (Pending review)"}[settings.AutoApproveAPIRequests],
		context.IP(),
	)

	message := "IP whitelist request submitted successfully. Awaiting administrator review."
	if settings.AutoApproveAPIRequests {
		message = "IP address whitelisted and approved successfully."
	}

	return apiSuccess(context, codeRequestAccepted, message, fiber.Map{
		"data": fiber.Map{
			"id":          id,
			"ip_address":  ip,
			"label":       label,
			"status":      status,
			"created_at":  now.Format(time.RFC3339),
			"reviewed_at": apiTimePtrValue(reviewedAt),
		},
	}, http.StatusCreated)
}

func (server *Server) extDeleteIPWhitelist(context fiber.Ctx) error {
	u := getDevUser(context)
	id := context.Params("id")

	var ipAddress, status string
	err := server.db.QueryRow(
		"SELECT ip_address, status FROM ip_whitelist_requests WHERE id = ? AND user_id = ?",
		id, u.ID,
	).Scan(&ipAddress, &status)
	if err == sql.ErrNoRows {
		return apiError(context, codeInvalidRequestFormat, "IP whitelist entry not found.", nil, http.StatusNotFound)
	}
	if err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	if _, err := server.db.Exec("DELETE FROM ip_whitelist_requests WHERE id = ? AND user_id = ?", id, u.ID); err != nil {
		return apiError(context, codeInternalServerError, "Internal Server Error", nil, 0)
	}

	server.recordIPWhitelistAudit(u, 0, "deleted", ipAddress,
		map[string]any{"ip_address": ipAddress, "status": status},
		nil,
		"Deleted by customer via API",
		context.IP(),
	)

	return apiSuccess(context, codeRequestAccepted,
		fmt.Sprintf("IP whitelist entry for %s removed successfully.", ipAddress), nil, 0)
}

func apiTimePtrValue(value any) any {
	if typed, ok := value.(time.Time); ok {
		return typed.Format(time.RFC3339)
	}

	return nil
}

func (server *Server) recordIPWhitelistAudit(u devUserData, requestID int64, action, ip string, oldValues, newValues map[string]any, note, sourceIP string) {
	oldJSON, _ := json.Marshal(oldValues)
	newJSON, _ := json.Marshal(newValues)

	var requestValue any
	if requestID > 0 {
		requestValue = requestID
	}
	var oldValue any
	if oldValues != nil {
		oldValue = string(oldJSON)
	}
	var newValue any
	if newValues != nil {
		newValue = string(newJSON)
	}

	_, _ = server.db.Exec(`
		INSERT INTO ip_whitelist_audit_logs
			(ip_whitelist_request_id, user_id, actor_id, actor_name, actor_type, action, ip_address, old_values, new_values, note, source_ip, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'customer', ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, requestValue, u.ID, u.ID, u.Name, action, ip, oldValue, newValue, nullString(note), nullString(sourceIP))
}

// ─────────────────────────────────────────────────────────────────────────────
// IP validation
// ─────────────────────────────────────────────────────────────────────────────

func validatePublicIPStr(value string) string {
	if strings.Contains(value, "/") {
		ip, network, err := net.ParseCIDR(value)
		if err != nil {
			return "Invalid CIDR format."
		}

		if isPrivateIP(ip) {
			return "Private or reserved IP ranges (e.g. 192.168.x.x, 10.x.x.x, 172.16.x.x) cannot be whitelisted. Only public internet-facing IP blocks are permitted."
		}

		ones, bits := network.Mask.Size()
		if bits == 32 && (ones < 16 || ones > 32) {
			return "For IPv4 CIDR blocks, the subnet mask must be between /16 and /32."
		}
		if bits == 128 && (ones < 48 || ones > 128) {
			return "For IPv6 CIDR blocks, the prefix length must be between /48 and /128."
		}

		return ""
	}

	parsed := net.ParseIP(value)
	if parsed == nil {
		return "The ip_address must be a valid IPv4 or IPv6 address."
	}
	if isPrivateIP(parsed) {
		return "Private or reserved IP addresses (e.g. 192.168.x.x, 10.x.x.x, 172.16.x.x, 127.0.0.1) cannot be whitelisted. Only public internet-facing IPs are permitted."
	}

	return ""
}

func isPrivateIP(ip net.IP) bool {
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"0.0.0.0/8",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"224.0.0.0/4",
		"240.0.0.0/4",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}

	return false
}
