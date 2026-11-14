package httpapi

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"smswhiteinfo/customer-api/internal/batch"
)

// normalizeBDContactPhone validates and normalizes Bangladeshi mobile numbers into +8801XXXXXXXXX.
func normalizeBDContactPhone(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	for _, r := range trimmed {
		if unicode.IsLetter(r) {
			return "", false
		}
	}
	return batch.FormatByRegex(trimmed)
}

func firstRowLooksLikeSpreadsheetHeader(firstRow []string) bool {
	for _, cell := range firstRow {
		if _, ok := normalizeBDContactPhone(cell); ok {
			return false
		}
	}
	return true
}

func extractContactFromSpreadsheetRow(headers, row []string) (string, string, bool) {
	normalized := make(map[string]string, len(headers))
	for i, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		key = strings.NewReplacer("/", "_", "-", "_", " ", "_").Replace(key)
		val := ""
		if i < len(row) {
			val = strings.TrimSpace(row[i])
		}
		if key != "" && normalized[key] == "" {
			normalized[key] = val
		}
	}

	phoneKeys := []string{
		"phone", "phone_number", "phonenumber", "mobile", "mobile_number", "mobilenumber",
		"contact", "contact_number", "recipient", "number", "msisdn", "cell", "tel",
	}
	var formattedPhone string
	for _, key := range phoneKeys {
		if val := normalized[key]; val != "" {
			if phone, ok := normalizeBDContactPhone(val); ok {
				formattedPhone = phone
				break
			}
		}
	}
	if formattedPhone == "" {
		for _, cell := range row {
			if phone, ok := normalizeBDContactPhone(cell); ok {
				formattedPhone = phone
				break
			}
		}
	}
	if formattedPhone == "" {
		return "", "", false
	}

	nameKeys := []string{
		"name", "full_name", "fullname", "contact_name", "contactname",
		"customer_name", "client_name", "recipient_name",
	}
	var contactName string
	for _, key := range nameKeys {
		if val := normalized[key]; val != "" {
			contactName = val
			break
		}
	}

	return formattedPhone, contactName, true
}

// upsertUserContact reuses or restores an existing contact by (user_id, phone) or inserts a new one.
func upsertUserContact(db *sql.DB, userID int64, phone, name string, now time.Time) (int64, error) {
	name = strings.TrimSpace(name)

	var cid int64
	var existingName sql.NullString
	var deletedAt sql.NullTime
	err := db.QueryRow(
		"SELECT id, name, deleted_at FROM contacts WHERE user_id = ? AND phone = ? LIMIT 1",
		userID, phone,
	).Scan(&cid, &existingName, &deletedAt)

	if err == nil {
		needsRestore := deletedAt.Valid
		fillName := name != "" && strings.TrimSpace(existingName.String) == ""
		if needsRestore || fillName {
			newName := existingName.String
			if fillName || (needsRestore && name != "") {
				newName = name
			}
			var nullableName any
			if strings.TrimSpace(newName) != "" {
				nullableName = strings.TrimSpace(newName)
			}
			_, err = db.Exec(
				"UPDATE contacts SET name = ?, deleted_at = NULL, updated_at = ? WHERE id = ?",
				nullableName, now, cid,
			)
			if err != nil {
				return 0, err
			}
		}
		return cid, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	var nullableName any
	if name != "" {
		nullableName = name
	}
	res, err := db.Exec(
		"INSERT INTO contacts (user_id, name, phone, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		userID, nullableName, phone, now, now,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func importRawNumbersToGroup(server *Server, userID, groupID int64, rawNumbers, defaultName string, now time.Time) (validCount, invalidCount, attached int) {
	tokens := strings.FieldsFunc(rawNumbers, func(r rune) bool {
		return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' ' || r == '\t'
	})
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		phone, ok := normalizeBDContactPhone(token)
		if !ok {
			invalidCount++
			continue
		}
		cid, err := upsertUserContact(server.db, userID, phone, defaultName, now)
		if err != nil || cid <= 0 {
			invalidCount++
			continue
		}
		validCount++
		res, err := server.db.Exec("INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)", cid, groupID)
		if err == nil {
			if aff, _ := res.RowsAffected(); aff > 0 {
				attached++
			}
		}
	}
	return validCount, invalidCount, attached
}

func importSpreadsheetFileToGroup(server *Server, userID, groupID int64, filePath string, now time.Time) (validCount, invalidCount, attached int, err error) {
	rawRows, err := readSpreadsheetRows(filePath)
	if err != nil {
		return 0, 0, 0, err
	}

	firstRow := rawRows[0]
	hasHeader := firstRowLooksLikeSpreadsheetHeader(firstRow)
	headers := make([]string, len(firstRow))
	if hasHeader {
		for i, h := range firstRow {
			headers[i] = strings.TrimSpace(h)
		}
	} else {
		for i := range firstRow {
			headers[i] = fmt.Sprintf("Column %d", i+1)
		}
	}

	startIdx := 0
	if hasHeader {
		startIdx = 1
	}

	for i := startIdx; i < len(rawRows); i++ {
		phone, name, ok := extractContactFromSpreadsheetRow(headers, rawRows[i])
		if !ok {
			invalidCount++
			continue
		}
		cid, uErr := upsertUserContact(server.db, userID, phone, name, now)
		if uErr != nil || cid <= 0 {
			invalidCount++
			continue
		}
		validCount++
		res, aErr := server.db.Exec("INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)", cid, groupID)
		if aErr == nil {
			if aff, _ := res.RowsAffected(); aff > 0 {
				attached++
			}
		}
	}
	return validCount, invalidCount, attached, nil
}

func parseIDsRequest(body []byte) []int64 {
	var req struct {
		IDs        []int64 `json:"ids"`
		ContactIDs []int64 `json:"contact_ids"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}
	raw := req.IDs
	if len(raw) == 0 {
		raw = req.ContactIDs
	}
	seen := make(map[int64]bool, len(raw))
	out := make([]int64, 0, len(raw))
	for _, id := range raw {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func (server *Server) exportContacts(context fiber.Ctx) error {
	claims := customer(context)

	conditions := []string{"c.user_id = ?", "c.deleted_at IS NULL"}
	args := []any{claims.UserID}

	if search := strings.TrimSpace(context.Query("search")); search != "" {
		conditions = append(conditions, "(c.phone LIKE ? OR COALESCE(c.name, '') LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	if groupID := strings.TrimSpace(context.Query("group_id")); groupID != "" {
		conditions = append(conditions, "EXISTS(SELECT 1 FROM contact_group cg WHERE cg.contact_id = c.id AND cg.group_id = ?)")
		args = append(args, groupID)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	rows, err := server.db.Query(`
		SELECT c.id, COALESCE(u.name, ''), COALESCE(c.name, ''), c.phone, c.created_at, c.updated_at
		FROM contacts c
		LEFT JOIN users u ON u.id = c.user_id
		`+where+`
		ORDER BY c.id DESC
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"ID", "User Name", "Name", "Phone", "Created At", "Updated At"})

	for rows.Next() {
		var id int64
		var userName, name, phone string
		var createdAt, updatedAt sql.NullTime
		if scanErr := rows.Scan(&id, &userName, &name, &phone, &createdAt, &updatedAt); scanErr != nil {
			continue
		}
		cAt := ""
		if createdAt.Valid {
			cAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		uAt := ""
		if updatedAt.Valid {
			uAt = updatedAt.Time.Format("2006-01-02 15:04:05")
		}
		_ = writer.Write([]string{
			fmt.Sprintf("%d", id),
			userName,
			name,
			phone,
			cAt,
			uAt,
		})
	}
	writer.Flush()

	filename := fmt.Sprintf("contacts-%s.csv", time.Now().Format("2006-01-02"))
	context.Set("Content-Type", "text/csv; charset=utf-8")
	context.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return context.SendString(buf.String())
}

func (server *Server) bulkDeleteContacts(context fiber.Ctx) error {
	claims := customer(context)
	ids := parseIDsRequest(context.Body())
	if len(ids) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one contact to delete."})
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	pivotArgs := make([]any, 0, len(ids)+1)
	pivotArgs = append(pivotArgs, claims.UserID)
	for _, id := range ids {
		pivotArgs = append(pivotArgs, id)
	}
	_, _ = server.db.Exec(
		"DELETE cg FROM contact_group cg INNER JOIN contacts c ON c.id = cg.contact_id WHERE c.user_id = ? AND cg.contact_id IN ("+placeholders+")",
		pivotArgs...,
	)

	res, err := server.db.Exec(
		"DELETE FROM contacts WHERE user_id = ? AND id IN ("+placeholders+")",
		pivotArgs...,
	)
	if err != nil {
		return databaseError(context, err)
	}

	deleted, _ := res.RowsAffected()
	return context.JSON(fiber.Map{
		"message": fmt.Sprintf("%d contact(s) deleted successfully.", deleted),
		"deleted": deleted,
	})
}

func (server *Server) exportGroupContacts(context fiber.Ctx) error {
	claims := customer(context)
	groupID := context.Params("id")

	var groupName string
	if err := server.db.QueryRow(
		"SELECT name FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		groupID, claims.UserID,
	).Scan(&groupName); err != nil {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	conditions := []string{"cg.group_id = ?", "c.user_id = ?", "c.deleted_at IS NULL"}
	args := []any{groupID, claims.UserID}
	if search := strings.TrimSpace(context.Query("search")); search != "" {
		conditions = append(conditions, "(c.phone LIKE ? OR COALESCE(c.name, '') LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	rows, err := server.db.Query(`
		SELECT c.id, COALESCE(c.name, ''), c.phone, c.created_at, c.updated_at
		FROM contacts c
		JOIN contact_group cg ON cg.contact_id = c.id
		WHERE `+strings.Join(conditions, " AND ")+`
		ORDER BY COALESCE(c.name, '') ASC, c.id DESC
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"ID", "Name", "Phone", "Created At", "Updated At"})

	for rows.Next() {
		var id int64
		var name, phone string
		var createdAt, updatedAt sql.NullTime
		if scanErr := rows.Scan(&id, &name, &phone, &createdAt, &updatedAt); scanErr != nil {
			continue
		}
		cAt := ""
		if createdAt.Valid {
			cAt = createdAt.Time.Format("2006-01-02 15:04:05")
		}
		uAt := ""
		if updatedAt.Valid {
			uAt = updatedAt.Time.Format("2006-01-02 15:04:05")
		}
		_ = writer.Write([]string{fmt.Sprintf("%d", id), name, phone, cAt, uAt})
	}
	writer.Flush()

	safeSlug := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			return unicode.ToLower(r)
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, groupName)
	if safeSlug == "" {
		safeSlug = "group-" + groupID
	}
	filename := fmt.Sprintf("%s-contacts.csv", safeSlug)
	context.Set("Content-Type", "text/csv; charset=utf-8")
	context.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	return context.SendString(buf.String())
}

func (server *Server) bulkDetachGroupContacts(context fiber.Ctx) error {
	claims := customer(context)
	groupID := context.Params("id")

	var exists int
	if err := server.db.QueryRow(
		"SELECT COUNT(*) FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		groupID, claims.UserID,
	).Scan(&exists); err != nil || exists == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	ids := parseIDsRequest(context.Body())
	if len(ids) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one contact to remove."})
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+1)
	args = append(args, groupID)
	for _, id := range ids {
		args = append(args, id)
	}

	res, err := server.db.Exec(
		"DELETE FROM contact_group WHERE group_id = ? AND contact_id IN ("+placeholders+")",
		args...,
	)
	if err != nil {
		return databaseError(context, err)
	}

	detached, _ := res.RowsAffected()
	return context.JSON(fiber.Map{
		"message":  fmt.Sprintf("%d contact(s) removed from group.", detached),
		"detached": detached,
	})
}

func (server *Server) importGroupContacts(context fiber.Ctx) error {
	claims := customer(context)
	groupID := context.Params("id")

	var groupIDNum int64
	var groupName string
	if err := server.db.QueryRow(
		"SELECT id, name FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		groupID, claims.UserID,
	).Scan(&groupIDNum, &groupName); err != nil {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	fileHeader, err := context.FormFile("file")
	if err != nil || fileHeader == nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Please select a CSV or Excel file to upload.",
		})
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".csv" && ext != ".txt" && ext != ".xlsx" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Unsupported file format. Please upload a .csv, .txt, or .xlsx file.",
		})
	}

	tmpDir := filepath.Join(server.storageRoot(), "tmp", "group-imports")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return databaseError(context, err)
	}
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("%d-%s%s", claims.UserID, uuid.NewString(), ext))
	if err := context.SaveFile(fileHeader, tmpFile); err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save uploaded file."})
	}
	defer os.Remove(tmpFile)

	validCount, invalidCount, attached, err := importSpreadsheetFileToGroup(server, claims.UserID, groupIDNum, tmpFile, time.Now())
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": err.Error(),
		})
	}

	return context.JSON(fiber.Map{
		"message":       fmt.Sprintf("Successfully imported %d phone number(s) into group '%s'.", validCount, groupName),
		"valid_count":   validCount,
		"invalid_count": invalidCount,
		"attached":      attached,
	})
}

func (server *Server) bulkDeleteGroups(context fiber.Ctx) error {
	claims := customer(context)
	ids := parseIDsRequest(context.Body())
	if len(ids) == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select at least one group to delete."})
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+2)
	args = append(args, time.Now(), claims.UserID)
	for _, id := range ids {
		args = append(args, id)
	}

	res, err := server.db.Exec(
		"UPDATE `groups` SET deleted_at = ?, updated_at = NOW() WHERE user_id = ? AND id IN ("+placeholders+") AND deleted_at IS NULL",
		args...,
	)
	if err != nil {
		return databaseError(context, err)
	}

	deleted, _ := res.RowsAffected()
	return context.JSON(fiber.Map{
		"message": fmt.Sprintf("%d group(s) deleted successfully.", deleted),
		"deleted": deleted,
	})
}
