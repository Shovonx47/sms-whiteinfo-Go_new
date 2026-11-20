package httpapi

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

var templateVariablePattern = regexp.MustCompile(`\{(\w+)\}`)

type adminTemplate struct {
	ID              int64    `json:"id"`
	UserID          int64    `json:"user_id"`
	Customer        string   `json:"customer"`
	Name            string   `json:"name"`
	Body            string   `json:"body"`
	Variables       []string `json:"variables"`
	Approval        string   `json:"approval"`
	ReviewedBy      *string  `json:"reviewed_by"`
	ApprovedAt      *string  `json:"approved_at"`
	RejectionReason *string  `json:"rejection_reason"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	DeletedAt       *string  `json:"deleted_at"`
}

type adminTemplatePayload struct {
	UserID    int64    `json:"user_id"`
	Name      string   `json:"name"`
	Body      string   `json:"body"`
	Variables []string `json:"variables"`
}

func extractTemplateVariables(body string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, match := range templateVariablePattern.FindAllStringSubmatch(body, -1) {
		if !seen[match[1]] {
			seen[match[1]] = true
			out = append(out, match[1])
		}
	}
	return out
}

func decodeTemplateVariables(raw sql.NullString) []string {
	out := []string{}
	if raw.Valid && strings.TrimSpace(raw.String) != "" {
		var parsed []any
		if json.Unmarshal([]byte(raw.String), &parsed) == nil {
			for _, item := range parsed {
				if text := strings.TrimSpace(toString(item)); text != "" {
					out = append(out, text)
				}
			}
		}
	}
	return out
}

func toString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return ""
	}
	raw, _ := json.Marshal(value)
	return string(raw)
}

const adminTemplateSelect = `
	SELECT t.id, t.user_id, COALESCE(u.name, ''), t.name, t.body, t.variables,
	       COALESCE(t.approval, 'approved'), rb.name, t.approved_at, t.rejection_reason,
	       t.created_at, t.updated_at, t.deleted_at
	FROM templates t
	LEFT JOIN users u ON u.id = t.user_id
	LEFT JOIN users rb ON rb.id = t.approved_by
`

func scanAdminTemplate(scanner interface{ Scan(dest ...any) error }) (adminTemplate, error) {
	var item adminTemplate
	var variables, reviewedBy, rejection sql.NullString
	var approvedAt, createdAt, updatedAt, deletedAt sql.NullTime
	if err := scanner.Scan(&item.ID, &item.UserID, &item.Customer, &item.Name, &item.Body, &variables,
		&item.Approval, &reviewedBy, &approvedAt, &rejection, &createdAt, &updatedAt, &deletedAt); err != nil {
		return item, err
	}
	item.Variables = decodeTemplateVariables(variables)
	if reviewedBy.Valid && reviewedBy.String != "" {
		item.ReviewedBy = &reviewedBy.String
	}
	if rejection.Valid && rejection.String != "" {
		item.RejectionReason = &rejection.String
	}
	if approvedAt.Valid {
		value := approvedAt.Time.Format(time.RFC3339)
		item.ApprovedAt = &value
	}
	if createdAt.Valid {
		item.CreatedAt = createdAt.Time.Format(time.RFC3339)
	}
	if updatedAt.Valid {
		item.UpdatedAt = updatedAt.Time.Format(time.RFC3339)
	}
	if deletedAt.Valid {
		value := deletedAt.Time.Format(time.RFC3339)
		item.DeletedAt = &value
	}
	return item, nil
}

// templates implements the Filament TemplatesTable: search, approval + trashed
// filters, sorting, pagination and export.
func (server *Server) templates(context fiber.Ctx) error {
	page, perPage := pagination(context)
	if context.Query("export") == "1" {
		page, perPage = 1, 20000
	}

	where := "WHERE t.deleted_at IS NULL"
	switch strings.ToLower(strings.TrimSpace(context.Query("trashed"))) {
	case "with":
		where = "WHERE 1 = 1"
	case "only":
		where = "WHERE t.deleted_at IS NOT NULL"
	}
	args := []any{}

	if search := strings.TrimSpace(context.Query("search")); search != "" {
		where += " AND (u.name LIKE ? OR t.name LIKE ? OR t.body LIKE ?)"
		term := "%" + search + "%"
		args = append(args, term, term, term)
	}
	if approval := strings.ToLower(strings.TrimSpace(context.Query("approval"))); approval != "" && approval != "all" {
		where += " AND COALESCE(t.approval, 'approved') = ?"
		args = append(args, approval)
	}

	var total int
	if err := server.db.QueryRow(`
		SELECT COUNT(*) FROM templates t LEFT JOIN users u ON u.id = t.user_id `+where, args...).Scan(&total); err != nil {
		return databaseError(context, err)
	}

	direction := "DESC"
	if strings.EqualFold(context.Query("dir"), "asc") {
		direction = "ASC"
	}
	orderBy := "t.created_at DESC, t.id DESC"
	switch context.Query("sort") {
	case "customer":
		orderBy = "u.name " + direction + ", t.id DESC"
	case "name":
		orderBy = "t.name " + direction + ", t.id DESC"
	case "approval":
		orderBy = "t.approval " + direction + ", t.id DESC"
	case "created_at":
		orderBy = "t.created_at " + direction + ", t.id DESC"
	}

	queryArgs := append(append([]any{}, args...), perPage, (page-1)*perPage)
	rows, err := server.db.Query(adminTemplateSelect+where+" ORDER BY "+orderBy+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := make([]adminTemplate, 0)
	for rows.Next() {
		item, err := scanAdminTemplate(rows)
		if err != nil {
			return databaseError(context, err)
		}
		items = append(items, item)
	}

	lastPage := int(math.Ceil(float64(total) / float64(perPage)))
	if lastPage < 1 {
		lastPage = 1
	}
	return context.JSON(fiber.Map{
		"data": items,
		"meta": fiber.Map{"page": page, "per_page": perPage, "total": total, "last_page": lastPage},
	})
}

func (server *Server) templateDetail(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	item, err := scanAdminTemplate(server.db.QueryRow(adminTemplateSelect+" WHERE t.id = ?", id))
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}
	return context.JSON(fiber.Map{"data": item})
}

// templateOptions feeds the "Customer" select (Filament relationship select).
func (server *Server) templateOptions(context fiber.Ctx) error {
	rows, err := server.db.Query(`
		SELECT id, name, email FROM users
		WHERE type = 'customer' AND deleted_at IS NULL
		ORDER BY name ASC
	`)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	type option struct {
		ID    int64  `json:"id"`
		Name  string `json:"name"`
		Email string `json:"email"`
	}
	customers := make([]option, 0)
	for rows.Next() {
		var item option
		if err := rows.Scan(&item.ID, &item.Name, &item.Email); err == nil {
			customers = append(customers, item)
		}
	}
	return context.JSON(fiber.Map{"data": fiber.Map{"customers": customers}})
}

func (server *Server) validateTemplatePayload(context fiber.Ctx) (adminTemplatePayload, string, bool) {
	var payload adminTemplatePayload
	if err := json.Unmarshal(context.Body(), &payload); err != nil {
		return payload, "Invalid request payload.", false
	}
	payload.Name = strings.TrimSpace(payload.Name)
	payload.Body = strings.TrimSpace(payload.Body)
	if payload.UserID < 1 {
		return payload, "The customer field is required.", false
	}
	if payload.Name == "" {
		return payload, "The template name field is required.", false
	}
	if len([]rune(payload.Name)) > 255 {
		return payload, "The template name must not be greater than 255 characters.", false
	}
	if payload.Body == "" {
		return payload, "The message body field is required.", false
	}
	var exists int64
	if err := server.db.QueryRow("SELECT id FROM users WHERE id = ? AND type = 'customer' AND deleted_at IS NULL", payload.UserID).Scan(&exists); err != nil {
		return payload, "The selected customer is invalid.", false
	}
	cleaned := make([]string, 0, len(payload.Variables))
	seen := map[string]bool{}
	for _, variable := range payload.Variables {
		variable = strings.TrimSpace(variable)
		if variable != "" && !seen[variable] {
			seen[variable] = true
			cleaned = append(cleaned, variable)
		}
	}
	if payload.Variables == nil {
		cleaned = extractTemplateVariables(payload.Body)
	}
	payload.Variables = cleaned
	return payload, "", true
}

func (server *Server) createTemplate(context fiber.Ctx) error {
	payload, message, ok := server.validateTemplatePayload(context)
	if !ok {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": message})
	}
	variables, _ := json.Marshal(payload.Variables)
	admin := superman(context)

	result, err := server.db.Exec(`
		INSERT INTO templates (user_id, name, body, variables, approval, approved_by, approved_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'approved', ?, NOW(), NOW(), NOW())
	`, payload.UserID, payload.Name, payload.Body, string(variables), admin.AdminID)
	if err != nil {
		return databaseError(context, err)
	}
	id, _ := result.LastInsertId()
	return context.Status(http.StatusCreated).JSON(fiber.Map{"message": "Template created.", "id": id})
}

func (server *Server) updateTemplate(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	payload, message, ok := server.validateTemplatePayload(context)
	if !ok {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": message})
	}
	variables, _ := json.Marshal(payload.Variables)

	result, err := server.db.Exec(`
		UPDATE templates SET user_id = ?, name = ?, body = ?, variables = ?, updated_at = NOW()
		WHERE id = ? AND deleted_at IS NULL
	`, payload.UserID, payload.Name, payload.Body, string(variables), id)
	if err != nil {
		return databaseError(context, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		var exists int64
		if server.db.QueryRow("SELECT id FROM templates WHERE id = ? AND deleted_at IS NULL", id).Scan(&exists) != nil {
			return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
		}
	}
	return context.JSON(fiber.Map{"message": "Template saved."})
}

func (server *Server) deleteTemplate(context fiber.Ctx) error {
	id, err := strconv.ParseInt(context.Params("id"), 10, 64)
	if err != nil || id < 1 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	result, err := server.db.Exec("UPDATE templates SET deleted_at = NOW() WHERE id = ? AND deleted_at IS NULL", id)
	if err != nil {
		return databaseError(context, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	return context.JSON(fiber.Map{"message": "Template deleted."})
}

// bulkTemplates implements Delete / Force delete / Restore bulk actions.
func (server *Server) bulkTemplates(context fiber.Ctx) error {
	var payload struct {
		IDs    []int64 `json:"ids"`
		Action string  `json:"action"`
	}
	if err := json.Unmarshal(context.Body(), &payload); err != nil || len(payload.IDs) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one template."})
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(payload.IDs)), ",")
	args := make([]any, 0, len(payload.IDs))
	for _, id := range payload.IDs {
		args = append(args, id)
	}

	var query, message string
	switch payload.Action {
	case "delete":
		query, message = "UPDATE templates SET deleted_at = NOW() WHERE deleted_at IS NULL AND id IN ("+placeholders+")", "Selected templates deleted."
	case "restore":
		query, message = "UPDATE templates SET deleted_at = NULL WHERE deleted_at IS NOT NULL AND id IN ("+placeholders+")", "Selected templates restored."
	case "force_delete":
		query, message = "DELETE FROM templates WHERE deleted_at IS NOT NULL AND id IN ("+placeholders+")", "Selected templates permanently deleted."
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
