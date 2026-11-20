package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// senderOperators is the fixed set of mobile operators a masking sender has to
// be routed through. The key is what is stored in sender_operator_gateways.
var senderOperators = []struct {
	Key   string
	Label string
	Short string
	Codes string
}{
	{"gp", "Grameenphone", "GP", "013, 017"},
	{"bl", "Banglalink", "BL", "014, 019"},
	{"tt", "Teletalk", "TT", "015"},
	{"airtel", "Airtel", "Airtel", "016"},
	{"robi", "Robi", "Robi", "018"},
}

var (
	maskingSenderPattern = regexp.MustCompile(`^[A-Za-z0-9 ._-]{3,11}$`)
	numericSenderPattern = regexp.MustCompile(`^\+?[0-9]{4,20}$`)
	digitsOnlyPattern    = regexp.MustCompile(`^[0-9]+$`)
)

type senderRoute struct {
	OperatorKey   string `json:"operator_key"`
	OperatorLabel string `json:"operator_label"`
	OperatorShort string `json:"operator_short"`
	GatewayID     int64  `json:"gateway_id"`
	GatewayName   string `json:"gateway_name"`
}

// ensureSenderRouteTable creates the per-operator routing table. The DDL is
// deliberately limited to syntax that both MySQL and PostgreSQL accept.
func (server *Server) ensureSenderRouteTable() {
	if server.db == nil {
		return
	}
	_, _ = server.db.Exec(`
		CREATE TABLE IF NOT EXISTS sender_operator_gateways (
			sender_id BIGINT NOT NULL,
			operator_key VARCHAR(20) NOT NULL,
			gateway_id BIGINT NOT NULL,
			created_at TIMESTAMP NULL,
			updated_at TIMESTAMP NULL,
			PRIMARY KEY (sender_id, operator_key)
		)
	`)
}

func operatorLabel(key string) (string, string, bool) {
	for _, operator := range senderOperators {
		if operator.Key == key {
			return operator.Label, operator.Short, true
		}
	}

	return "", "", false
}

// senderRoutes loads the operator routes for a set of sender rows.
func (server *Server) senderRoutes(senderIDs []int64) (map[int64][]senderRoute, error) {
	result := make(map[int64][]senderRoute, len(senderIDs))
	group, bindings := placeholders(senderIDs)
	if group == "" {
		return result, nil
	}

	rows, err := server.db.Query(`
		SELECT r.sender_id, r.operator_key, r.gateway_id, COALESCE(g.name, '')
		FROM sender_operator_gateways r
		LEFT JOIN gateways g ON g.id = r.gateway_id
		WHERE r.sender_id IN `+group, bindings...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKey := map[int64]map[string]senderRoute{}
	for rows.Next() {
		var senderID int64
		var route senderRoute
		if err := rows.Scan(&senderID, &route.OperatorKey, &route.GatewayID, &route.GatewayName); err != nil {
			return nil, err
		}
		label, short, ok := operatorLabel(route.OperatorKey)
		if !ok {
			continue
		}
		route.OperatorLabel, route.OperatorShort = label, short
		if byKey[senderID] == nil {
			byKey[senderID] = map[string]senderRoute{}
		}
		byKey[senderID][route.OperatorKey] = route
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Keep a stable GP, BL, TT, Airtel, Robi order.
	for senderID, routes := range byKey {
		for _, operator := range senderOperators {
			if route, ok := routes[operator.Key]; ok {
				result[senderID] = append(result[senderID], route)
			}
		}
	}

	return result, nil
}

// saveSenderRoutes replaces the routes of one sender.
func (server *Server) saveSenderRoutes(senderID int64, routes map[string]int64) error {
	if _, err := server.db.Exec("DELETE FROM sender_operator_gateways WHERE sender_id = ?", senderID); err != nil {
		return err
	}
	for _, operator := range senderOperators {
		gatewayID := routes[operator.Key]
		if gatewayID <= 0 {
			continue
		}
		if _, err := server.db.Exec(`
			INSERT INTO sender_operator_gateways (sender_id, operator_key, gateway_id, created_at, updated_at)
			VALUES (?, ?, ?, NOW(), NOW())
		`, senderID, operator.Key, gatewayID); err != nil {
			return err
		}
	}

	return nil
}

// normalizeRoutes drops unknown operators and non-positive gateway ids.
func normalizeRoutes(input map[string]int64) map[string]int64 {
	clean := map[string]int64{}
	for key, gatewayID := range input {
		key = strings.ToLower(strings.TrimSpace(key))
		if _, _, ok := operatorLabel(key); ok && gatewayID > 0 {
			clean[key] = gatewayID
		}
	}

	return clean
}

func missingOperators(routes map[string]int64) []string {
	missing := []string{}
	for _, operator := range senderOperators {
		if routes[operator.Key] <= 0 {
			missing = append(missing, operator.Label)
		}
	}

	return missing
}

// validateSenderID checks a sender id against the rules of its type.
func validateSenderID(senderID, senderType string) string {
	if senderType == "numeric" {
		if !numericSenderPattern.MatchString(senderID) {
			return "A non-masking sender must be 4 to 20 digits."
		}
		return ""
	}
	if !maskingSenderPattern.MatchString(senderID) {
		return "A masking sender must be 3 to 11 letters, digits, spaces, dots, dashes or underscores."
	}
	if digitsOnlyPattern.MatchString(senderID) {
		return "A masking sender cannot be purely numeric."
	}

	return ""
}

func (server *Server) gatewayExistsByID(id int64) (bool, error) {
	var count int
	if err := server.db.QueryRow("SELECT COUNT(*) FROM gateways WHERE id = ? AND deleted_at IS NULL", id).Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

type senderWriteInput struct {
	SenderID  string
	Type      string
	GatewayID int64
	Routes    map[string]int64
	IsEnabled bool
	UserIDs   []int64
	// Overwrite lets an existing, active sender be updated in place (import).
	Overwrite bool
}

type senderWriteResult struct {
	ID      int64
	Created bool
	Updated bool
}

// writeSender creates, restores or (when Overwrite is set) updates one sender
// together with its operator routes and customer assignment.
func (server *Server) writeSender(input senderWriteInput) (senderWriteResult, string, error) {
	var result senderWriteResult

	if message := validateSenderID(input.SenderID, input.Type); message != "" {
		return result, message, nil
	}

	routes := input.Routes
	if input.Type == "numeric" {
		routes = map[string]int64{}
	} else {
		if missing := missingOperators(routes); len(missing) > 0 {
			return result, "Assign a gateway for every operator: " + strings.Join(missing, ", ") + ".", nil
		}
		if input.GatewayID <= 0 {
			input.GatewayID = routes["gp"]
		}
	}
	if input.GatewayID <= 0 {
		return result, "A gateway is required.", nil
	}

	var existingID int64
	var deletedAt sql.NullTime
	err := server.db.QueryRow(
		"SELECT id, deleted_at FROM senders WHERE sender_id = ? ORDER BY (CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END), id LIMIT 1",
		input.SenderID).Scan(&existingID, &deletedAt)
	switch {
	case err == sql.ErrNoRows:
		res, err := server.db.Exec(`
			INSERT INTO senders (gateway_id, sender_id, type, is_enabled, approval_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'approved', NOW(), NOW())
		`, input.GatewayID, input.SenderID, input.Type, input.IsEnabled)
		if err != nil {
			return result, "", err
		}
		result.ID, _ = res.LastInsertId()
		result.Created = true
	case err != nil:
		return result, "", err
	default:
		if !deletedAt.Valid && !input.Overwrite {
			return result, fmt.Sprintf("Sender ID %s already exists.", input.SenderID), nil
		}
		if _, err := server.db.Exec(`
			UPDATE senders SET gateway_id = ?, type = ?, is_enabled = ?, deleted_at = NULL, updated_at = NOW() WHERE id = ?
		`, input.GatewayID, input.Type, input.IsEnabled, existingID); err != nil {
			return result, "", err
		}
		result.ID = existingID
		result.Updated = true
	}

	if err := server.saveSenderRoutes(result.ID, routes); err != nil {
		return result, "", err
	}
	if len(input.UserIDs) > 0 {
		if err := server.attachSenderUsers(result.ID, input.UserIDs); err != nil {
			return result, "", err
		}
	}

	return result, "", nil
}

type senderCreatePayloadV2 struct {
	SenderID  string           `json:"sender_id"`
	SenderIDs []string         `json:"sender_ids"`
	GatewayID int64            `json:"gateway_id"`
	Type      string           `json:"type"`
	IsEnabled *bool            `json:"is_enabled"`
	UserIDs   []int64          `json:"user_ids"`
	Routes    map[string]int64 `json:"routes"`
}

func unprocessable(context fiber.Ctx, message string) error {
	return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": message})
}

func (server *Server) createSender(context fiber.Ctx) error {
	payload := senderCreatePayloadV2{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return unprocessable(context, "Invalid sender payload.")
	}

	senderType := normalizeSenderType(payload.Type)
	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}

	requested := []string{}
	for _, candidate := range append([]string{payload.SenderID}, payload.SenderIDs...) {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			requested = append(requested, trimmed)
		}
	}
	if len(requested) == 0 {
		return unprocessable(context, "A sender ID is required.")
	}

	routes := normalizeRoutes(payload.Routes)
	gatewayID := payload.GatewayID
	if senderType == "numeric" && gatewayID <= 0 {
		return unprocessable(context, "A gateway is required.")
	}
	if senderType != "numeric" && len(missingOperators(routes)) > 0 {
		// Legacy callers send a single gateway: use it for every operator.
		if gatewayID > 0 && len(routes) == 0 {
			for _, operator := range senderOperators {
				routes[operator.Key] = gatewayID
			}
		}
	}

	checked := map[int64]bool{}
	for _, id := range append([]int64{gatewayID}, mapValues(routes)...) {
		if id <= 0 || checked[id] {
			continue
		}
		exists, err := server.gatewayExistsByID(id)
		if err != nil {
			return databaseError(context, err)
		}
		if !exists {
			return unprocessable(context, "The selected gateway no longer exists.")
		}
		checked[id] = true
	}

	created, updated, lastID := 0, 0, int64(0)
	problems := []string{}
	for _, senderID := range requested {
		result, problem, err := server.writeSender(senderWriteInput{
			SenderID: senderID, Type: senderType, GatewayID: gatewayID, Routes: routes,
			IsEnabled: isEnabled, UserIDs: payload.UserIDs,
		})
		if err != nil {
			return databaseError(context, err)
		}
		if problem != "" {
			problems = append(problems, problem)
			continue
		}
		lastID = result.ID
		if result.Created {
			created++
		}
		if result.Updated {
			updated++
		}
	}

	if created+updated == 0 {
		return unprocessable(context, strings.Join(problems, " "))
	}

	message := fmt.Sprintf("%d sender ID(s) created.", created)
	if updated > 0 {
		message = fmt.Sprintf("%d sender ID(s) created, %d restored.", created, updated)
	}
	if len(problems) > 0 {
		message += " Skipped: " + strings.Join(problems, " ")
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{"message": message, "data": fiber.Map{
		"created": created, "updated": updated, "id": lastID,
	}})
}

func mapValues(values map[string]int64) []int64 {
	out := make([]int64, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}

	return out
}

type senderImportRow struct {
	SenderID string           `json:"sender_id"`
	Type     string           `json:"type"`
	Gateway  string           `json:"gateway"`
	Customer string           `json:"customer"`
	Routes   map[string]string `json:"routes"`
}

type senderImportPayload struct {
	Rows             []senderImportRow `json:"rows"`
	DefaultGatewayID int64             `json:"default_gateway_id"`
	DefaultType      string            `json:"default_type"`
	UserID           int64             `json:"user_id"`
}

type senderImportError struct {
	Row      int    `json:"row"`
	SenderID string `json:"sender_id"`
	Message  string `json:"message"`
}

// importSenders bulk-creates senders from rows parsed out of an uploaded CSV.
// A row can name its own gateway; otherwise the default gateway applies.
func (server *Server) importSenders(context fiber.Ctx) error {
	payload := senderImportPayload{}
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return unprocessable(context, "Invalid import payload.")
	}
	if len(payload.Rows) == 0 {
		return unprocessable(context, "The file has no sender rows.")
	}
	if len(payload.Rows) > 5000 {
		return unprocessable(context, "Import at most 5,000 senders at a time.")
	}

	gatewayRows, err := server.db.Query("SELECT id, name FROM gateways WHERE deleted_at IS NULL")
	if err != nil {
		return databaseError(context, err)
	}
	gatewayByName := map[string]int64{}
	gatewayExists := map[int64]bool{}
	for gatewayRows.Next() {
		var id int64
		var name string
		if err := gatewayRows.Scan(&id, &name); err == nil {
			gatewayByName[strings.ToLower(strings.TrimSpace(name))] = id
			gatewayExists[id] = true
		}
	}
	gatewayRows.Close()

	if payload.DefaultGatewayID > 0 && !gatewayExists[payload.DefaultGatewayID] {
		return unprocessable(context, "The default gateway no longer exists.")
	}

	userIDs := []int64{}
	if payload.UserID > 0 {
		var exists int
		if err := server.db.QueryRow(
			"SELECT COUNT(*) FROM users WHERE id = ? AND type = 'customer' AND deleted_at IS NULL",
			payload.UserID,
		).Scan(&exists); err != nil {
			return databaseError(context, err)
		}
		if exists == 0 {
			return unprocessable(context, "The selected customer account no longer exists.")
		}
		userIDs = append(userIDs, payload.UserID)
	}

	customerByLookup := map[string]int64{}
	needsCustomerLookup := false
	for _, row := range payload.Rows {
		if strings.TrimSpace(row.Customer) != "" {
			needsCustomerLookup = true
			break
		}
	}
	if needsCustomerLookup {
		cRows, cErr := server.db.Query("SELECT id, name, COALESCE(email, '') FROM users WHERE type = 'customer' AND deleted_at IS NULL")
		if cErr != nil {
			return databaseError(context, cErr)
		}
		for cRows.Next() {
			var cid int64
			var cname, cemail string
			if err := cRows.Scan(&cid, &cname, &cemail); err == nil {
				if key := strings.ToLower(strings.TrimSpace(cemail)); key != "" {
					customerByLookup[key] = cid
				}
				if key := strings.ToLower(strings.TrimSpace(cname)); key != "" {
					customerByLookup[key] = cid
				}
			}
		}
		cRows.Close()
	}

	resolveGateway := func(name string) (int64, bool) {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return 0, true
		}
		id, ok := gatewayByName[name]
		return id, ok
	}

	created, updated := 0, 0
	errorsOut := []senderImportError{}
	for index, row := range payload.Rows {
		line := index + 1
		senderID := strings.TrimSpace(row.SenderID)
		fail := func(message string) {
			errorsOut = append(errorsOut, senderImportError{Row: line, SenderID: senderID, Message: message})
		}
		if senderID == "" {
			fail("Sender ID is empty.")
			continue
		}

		typeValue := row.Type
		if strings.TrimSpace(typeValue) == "" {
			typeValue = payload.DefaultType
		}
		senderType := normalizeSenderType(typeValue)
		if senderType == "shortcode" {
			senderType = "alphanumeric"
		}

		gatewayID, ok := resolveGateway(row.Gateway)
		if !ok {
			fail(fmt.Sprintf("Unknown gateway %q.", row.Gateway))
			continue
		}
		if gatewayID == 0 {
			gatewayID = payload.DefaultGatewayID
		}

		routes := map[string]int64{}
		bad := false
		for _, operator := range senderOperators {
			if gatewayID > 0 {
				routes[operator.Key] = gatewayID
			}
			if override, has := row.Routes[operator.Key]; has && strings.TrimSpace(override) != "" {
				id, found := resolveGateway(override)
				if !found {
					fail(fmt.Sprintf("Unknown %s gateway %q.", operator.Label, override))
					bad = true
					break
				}
				routes[operator.Key] = id
			}
		}
		if bad {
			continue
		}

		if gatewayID == 0 && senderType != "numeric" && len(missingOperators(routes)) == 0 {
			gatewayID = routes["gp"]
		}
		if gatewayID == 0 {
			fail("No gateway in the file and no default gateway selected.")
			continue
		}

		rowUsers := userIDs
		if rawCust := strings.TrimSpace(row.Customer); rawCust != "" {
			cid, found := customerByLookup[strings.ToLower(rawCust)]
			if !found {
				fail(fmt.Sprintf("Unknown customer %q.", row.Customer))
				continue
			}
			rowUsers = []int64{cid}
		}

		result, problem, err := server.writeSender(senderWriteInput{
			SenderID: senderID, Type: senderType, GatewayID: gatewayID, Routes: routes,
			IsEnabled: true, UserIDs: rowUsers, Overwrite: true,
		})
		if err != nil {
			return databaseError(context, err)
		}
		if problem != "" {
			fail(problem)
			continue
		}
		if result.Created {
			created++
		}
		if result.Updated {
			updated++
		}
	}

	status := http.StatusOK
	message := fmt.Sprintf("Import finished: %d created, %d updated, %d failed.", created, updated, len(errorsOut))
	if created+updated == 0 {
		status = http.StatusUnprocessableEntity
		message = fmt.Sprintf("No senders were imported (%d row(s) failed).", len(errorsOut))
	}

	return context.Status(status).JSON(fiber.Map{"message": message, "data": fiber.Map{
		"created": created, "updated": updated, "failed": len(errorsOut), "errors": errorsOut,
	}})
}
