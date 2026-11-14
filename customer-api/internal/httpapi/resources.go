package httpapi

import (
	stdctx "context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"smswhiteinfo/customer-api/internal/batch"
	"smswhiteinfo/customer-api/internal/crypto"
	"smswhiteinfo/customer-api/internal/sms"
	"smswhiteinfo/customer-api/internal/sms/drivers"
)

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1d ago"
	}
	return fmt.Sprintf("%dd ago", days)
}

// ─────────────────────────────────────────────────────────────────────────────
// Senders
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) senders(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{
		"(su.user_id = ? OR s.requested_by = ?)",
		"s.deleted_at IS NULL",
	}
	args := []any{claims.UserID, claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "s.sender_id LIKE ?")
		args = append(args, "%"+search+"%")
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow(`
		SELECT COUNT(DISTINCT s.id) FROM senders s
		LEFT JOIN sender_user su ON su.sender_id = s.id
		`+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT DISTINCT s.id, s.sender_id, s.type, s.approval_status, s.is_enabled, s.created_at
		FROM senders s
		LEFT JOIN sender_user su ON su.sender_id = s.id
		`+where+`
		ORDER BY s.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var senderID, senderType, approvalStatus string
		var isEnabled bool
		var createdAt time.Time

		if err := rows.Scan(&id, &senderID, &senderType, &approvalStatus, &isEnabled, &createdAt); err != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":              id,
			"sender_id":       senderID,
			"type":            senderType,
			"approval_status": approvalStatus,
			"status":          approvalStatus,
			"is_enabled":      isEnabled,
			"created_at":      createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) createSender(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		SenderID string `json:"sender_id"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	senderLabel := strings.ToUpper(strings.TrimSpace(req.SenderID))
	if senderLabel == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"sender_id": []string{"Sender ID is required."}},
		})
	}
	if req.Type != "masking" && req.Type != "non_masking" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"type": []string{"Type must be masking or non_masking."}},
		})
	}

	// Check existing
	var existingID int64
	_ = server.db.QueryRow(`
		SELECT s.id FROM senders s
		LEFT JOIN sender_user su ON su.sender_id = s.id
		WHERE s.sender_id = ? AND (su.user_id = ? OR s.requested_by = ?) AND s.deleted_at IS NULL
		LIMIT 1
	`, senderLabel, claims.UserID, claims.UserID).Scan(&existingID)
	if existingID > 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "You have already registered or requested this Sender ID.",
		})
	}

	now := time.Now()
	result, err := server.db.Exec(`
		INSERT INTO senders (sender_id, type, approval_status, requested_by, is_enabled, created_at, updated_at)
		VALUES (?, ?, 'pending', ?, 0, ?, ?)
	`, senderLabel, req.Type, claims.UserID, now, now)
	if err != nil {
		return databaseError(context, err)
	}

	senderID, _ := result.LastInsertId()

	_, _ = server.db.Exec(
		"INSERT IGNORE INTO sender_user (sender_id, user_id) VALUES (?, ?)",
		senderID, claims.UserID,
	)

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Sender ID submitted for approval.",
		"data": fiber.Map{
			"id":              senderID,
			"sender_id":       senderLabel,
			"type":            req.Type,
			"approval_status": "pending",
			"status":          "pending",
			"is_enabled":      false,
			"created_at":      now.Format(time.RFC3339),
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Contacts
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) contacts(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"c.user_id = ?", "c.deleted_at IS NULL"}
	args := []any{claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "(c.phone LIKE ? OR c.name LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	if groupID := context.Query("group_id"); groupID != "" {
		conditions = append(conditions, "EXISTS(SELECT 1 FROM contact_group cg WHERE cg.contact_id = c.id AND cg.group_id = ?)")
		args = append(args, groupID)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM contacts c "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT c.id, c.name, c.phone,
		       (SELECT g.name FROM groups g JOIN contact_group cg ON cg.group_id = g.id WHERE cg.contact_id = c.id LIMIT 1) as group_name,
		       (SELECT g.id FROM groups g JOIN contact_group cg ON cg.group_id = g.id WHERE cg.contact_id = c.id LIMIT 1) as group_id,
		       c.created_at
		FROM contacts c
		`+where+`
		ORDER BY c.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var name, phone string
		var groupName *string
		var groupID *int64
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &phone, &groupName, &groupID, &createdAt); err != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":         id,
			"name":       name,
			"phone":      phone,
			"group":      groupName,
			"group_id":   groupID,
			"created_at": createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) createContact(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Name    string `json:"name"`
		Phone   string `json:"phone"`
		GroupID *int64 `json:"group_id"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Phone = strings.TrimSpace(req.Phone)
	if req.Phone == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"phone": []string{"Phone number is required."}},
		})
	}

	now := time.Now()
	result, err := server.db.Exec(
		"INSERT INTO contacts (user_id, name, phone, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		claims.UserID, strings.TrimSpace(req.Name), req.Phone, now, now,
	)
	if err != nil {
		return databaseError(context, err)
	}

	contactID, _ := result.LastInsertId()

	var groupName *string
	if req.GroupID != nil {
		var gName string
		err := server.db.QueryRow(
			"SELECT name FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
			*req.GroupID, claims.UserID,
		).Scan(&gName)
		if err == nil {
			_, _ = server.db.Exec(
				"INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)",
				contactID, *req.GroupID,
			)
			groupName = &gName
		}
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Contact added successfully.",
		"data": fiber.Map{
			"id":         contactID,
			"name":       strings.TrimSpace(req.Name),
			"phone":      req.Phone,
			"group":      groupName,
			"created_at": now.Format(time.RFC3339),
		},
	})
}

func (server *Server) deleteContact(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(
		"DELETE FROM contacts WHERE id = ? AND user_id = ? AND deleted_at IS NULL",
		id, claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact not found."})
	}

	return context.JSON(fiber.Map{"message": "Contact deleted successfully."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Groups
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) groups(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"g.user_id = ?", "g.deleted_at IS NULL"}
	args := []any{claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "g.name LIKE ?")
		args = append(args, "%"+search+"%")
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM groups g "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT g.id, g.name, COALESCE(g.description, ''),
		       (SELECT COUNT(*) FROM contact_group cg WHERE cg.group_id = g.id) as contacts_count,
		       g.created_at
		FROM groups g
		`+where+`
		ORDER BY g.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, contactsCount int
		var name, description string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &description, &contactsCount, &createdAt); err != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":             id,
			"name":           name,
			"description":    description,
			"contacts_count": contactsCount,
			"created_at":     createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) createGroup(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		PhoneNumbers string `json:"_phone_numbers"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"name": []string{"Group name is required."}},
		})
	}

	now := time.Now()
	result, err := server.db.Exec(
		"INSERT INTO groups (user_id, name, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		claims.UserID, req.Name, strings.TrimSpace(req.Description), now, now,
	)
	if err != nil {
		return databaseError(context, err)
	}

	groupID, _ := result.LastInsertId()

	// If phone numbers provided during creation, add them
	if rawPhones := strings.TrimSpace(req.PhoneNumbers); rawPhones != "" {
		tokens := strings.FieldsFunc(rawPhones, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' '
		})
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			// Find or create contact
			var cid int64
			err := server.db.QueryRow("SELECT id FROM contacts WHERE user_id = ? AND phone = ? LIMIT 1", claims.UserID, token).Scan(&cid)
			if err == sql.ErrNoRows {
				res, err := server.db.Exec("INSERT INTO contacts (user_id, phone, created_at, updated_at) VALUES (?, ?, ?, ?)", claims.UserID, token, now, now)
				if err == nil {
					cid, _ = res.LastInsertId()
				}
			}
			if cid > 0 {
				_, _ = server.db.Exec("INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)", cid, groupID)
			}
		}
	}

	var count int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM contact_group WHERE group_id = ?", groupID).Scan(&count)

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Contact group created.",
		"data": fiber.Map{
			"id":             groupID,
			"name":           req.Name,
			"description":    strings.TrimSpace(req.Description),
			"contacts_count": count,
			"created_at":     now.Format(time.RFC3339),
		},
	})
}

func (server *Server) groupDetail(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	var group struct {
		ID            int64     `json:"id"`
		Name          string    `json:"name"`
		Description   string    `json:"description"`
		ContactsCount int       `json:"contacts_count"`
		CreatedAt     time.Time `json:"created_at"`
	}

	err := server.db.QueryRow(`
		SELECT g.id, g.name, COALESCE(g.description, ''),
		       (SELECT COUNT(*) FROM contact_group cg WHERE cg.group_id = g.id),
		       g.created_at
		FROM groups g
		WHERE g.id = ? AND g.user_id = ? AND g.deleted_at IS NULL
	`, id, claims.UserID).Scan(&group.ID, &group.Name, &group.Description, &group.ContactsCount, &group.CreatedAt)

	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"id":             group.ID,
			"name":           group.Name,
			"description":    group.Description,
			"contacts_count": group.ContactsCount,
			"created_at":     group.CreatedAt.Format(time.RFC3339),
		},
	})
}

func (server *Server) updateGroup(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"name": []string{"Group name is required."}},
		})
	}

	now := time.Now()
	res, err := server.db.Exec(`
		UPDATE groups
		SET name = ?, description = ?, updated_at = ?
		WHERE id = ? AND user_id = ? AND deleted_at IS NULL
	`, req.Name, strings.TrimSpace(req.Description), now, id, claims.UserID)

	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := res.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	return context.JSON(fiber.Map{
		"message": "Group updated successfully.",
		"data": fiber.Map{
			"id":          id,
			"name":        req.Name,
			"description": strings.TrimSpace(req.Description),
		},
	})
}

func (server *Server) attachGroupContacts(context fiber.Ctx) error {
	claims := customer(context)
	groupID := context.Params("id")

	// Verify group belongs to user
	var exists int
	err := server.db.QueryRow("SELECT COUNT(*) FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL", groupID, claims.UserID).Scan(&exists)
	if err != nil || exists == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	var req struct {
		PhoneNumbers string  `json:"phone_numbers"`
		ContactName  string  `json:"name"`
		ContactIDs   []int64 `json:"contact_ids"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	now := time.Now()
	addedCount := 0

	// 1. Existing contacts attached by ID
	for _, cid := range req.ContactIDs {
		// verify contact belongs to user
		var valid int
		_ = server.db.QueryRow("SELECT COUNT(*) FROM contacts WHERE id = ? AND user_id = ? AND deleted_at IS NULL", cid, claims.UserID).Scan(&valid)
		if valid > 0 {
			res, err := server.db.Exec("INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)", cid, groupID)
			if err == nil {
				if aff, _ := res.RowsAffected(); aff > 0 {
					addedCount++
				}
			}
		}
	}

	// 2. Raw phone numbers
	if raw := strings.TrimSpace(req.PhoneNumbers); raw != "" {
		tokens := strings.FieldsFunc(raw, func(r rune) bool {
			return r == '\n' || r == '\r' || r == ',' || r == ';' || r == ' '
		})
		for _, token := range tokens {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			var cid int64
			err := server.db.QueryRow("SELECT id FROM contacts WHERE user_id = ? AND phone = ? LIMIT 1", claims.UserID, token).Scan(&cid)
			if err == sql.ErrNoRows {
				res, err := server.db.Exec("INSERT INTO contacts (user_id, name, phone, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
					claims.UserID, strings.TrimSpace(req.ContactName), token, now, now)
				if err == nil {
					cid, _ = res.LastInsertId()
				}
			}
			if cid > 0 {
				res, err := server.db.Exec("INSERT IGNORE INTO contact_group (contact_id, group_id) VALUES (?, ?)", cid, groupID)
				if err == nil {
					if aff, _ := res.RowsAffected(); aff > 0 {
						addedCount++
					}
				}
			}
		}
	}

	return context.JSON(fiber.Map{
		"message": fmt.Sprintf("%d contact(s) attached to group.", addedCount),
		"added":   addedCount,
	})
}

func (server *Server) detachGroupContact(context fiber.Ctx) error {
	claims := customer(context)
	groupID := context.Params("id")
	contactID := context.Params("contact_id")

	// Verify group belongs to user
	var exists int
	err := server.db.QueryRow("SELECT COUNT(*) FROM groups WHERE id = ? AND user_id = ? AND deleted_at IS NULL", groupID, claims.UserID).Scan(&exists)
	if err != nil || exists == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	res, err := server.db.Exec("DELETE FROM contact_group WHERE group_id = ? AND contact_id = ?", groupID, contactID)
	if err != nil {
		return databaseError(context, err)
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Contact was not attached to group."})
	}

	return context.JSON(fiber.Map{"message": "Contact detached from group successfully."})
}

func (server *Server) deleteGroup(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(
		"DELETE FROM groups WHERE id = ? AND user_id = ?",
		id, claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Group not found."})
	}

	return context.JSON(fiber.Map{"message": "Contact group deleted."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Templates
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) templates(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"t.user_id = ?", "t.deleted_at IS NULL"}
	args := []any{claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "(t.name LIKE ? OR t.body LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM templates t "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT t.id, t.name, t.body, t.variables, t.created_at
		FROM templates t
		`+where+`
		ORDER BY t.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var name, body string
		var variablesJSON *string
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &body, &variablesJSON, &createdAt); err != nil {
			continue
		}

		variables := []string{}
		if variablesJSON != nil {
			_ = json.Unmarshal([]byte(*variablesJSON), &variables)
		}

		items = append(items, fiber.Map{
			"id":         id,
			"name":       name,
			"body":       body,
			"variables":  variables,
			"status":     "approved",
			"created_at": createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) createTemplate(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Name string `json:"name"`
		Body string `json:"body"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Body = strings.TrimSpace(req.Body)

	if req.Name == "" || req.Body == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name and body are required."})
	}

	variables := extractVariables(req.Body)
	variablesJSON, _ := json.Marshal(variables)

	now := time.Now()
	result, err := server.db.Exec(
		"INSERT INTO templates (user_id, name, body, variables, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		claims.UserID, req.Name, req.Body, string(variablesJSON), now, now,
	)
	if err != nil {
		return databaseError(context, err)
	}

	templateID, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Template created successfully.",
		"data": fiber.Map{
			"id":         templateID,
			"name":       req.Name,
			"body":       req.Body,
			"variables":  variables,
			"status":     "approved",
			"created_at": now.Format(time.RFC3339),
		},
	})
}

func (server *Server) deleteTemplate(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(
		"DELETE FROM templates WHERE id = ? AND user_id = ?",
		id, claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}

	return context.JSON(fiber.Map{"message": "Template deleted successfully."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Transactions
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) transactions(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"t.user_id = ?"}
	args := []any{claims.UserID}

	if dirFilter := context.Query("type"); dirFilter == "credit" || dirFilter == "debit" {
		conditions = append(conditions, "t.direction = ?")
		args = append(args, dirFilter)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM transactions t "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT t.id, t.direction, t.amount, t.balance_before, t.balance_after, t.description, t.transactionable_type, t.created_at
		FROM transactions t
		`+where+`
		ORDER BY t.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var direction string
		var amount, balanceBefore, balanceAfter float64
		var description, txType *string
		var createdAt time.Time

		if err := rows.Scan(&id, &direction, &amount, &balanceBefore, &balanceAfter, &description, &txType, &createdAt); err != nil {
			continue
		}

		desc := "Balance Adjustment"
		if description != nil && *description != "" {
			desc = *description
		} else if txType != nil {
			parts := strings.Split(*txType, "\\")
			desc = parts[len(parts)-1]
		}

		items = append(items, fiber.Map{
			"id":             id,
			"type":           direction,
			"amount":         fmt.Sprintf("%.2f", amount),
			"balance_before": fmt.Sprintf("%.2f", balanceBefore),
			"balance_after":  fmt.Sprintf("%.2f", balanceAfter),
			"description":    desc,
			"created_at":     createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Deposits
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) deposits(context fiber.Ctx) error {
	claims := customer(context)
	page, perPage := pagination(context)
	offset := (page - 1) * perPage

	conditions := []string{"d.user_id = ?"}
	args := []any{claims.UserID}

	if search := context.Query("search"); search != "" {
		conditions = append(conditions, "(d.transaction_id LIKE ? OR CAST(d.id AS CHAR) LIKE ?)")
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	where := "WHERE " + strings.Join(conditions, " AND ")
	countArgs := make([]any, len(args))
	copy(countArgs, args)

	var total int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM deposits d "+where, countArgs...).Scan(&total)

	args = append(args, perPage, offset)
	rows, err := server.db.Query(`
		SELECT d.id, d.transaction_id, d.amount, d.payment_method, d.status, d.created_at, d.paid_at
		FROM deposits d
		`+where+`
		ORDER BY d.id DESC
		LIMIT ? OFFSET ?
	`, args...)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var txID *string
		var amount float64
		var paymentMethod, status string
		var createdAt time.Time
		var paidAt *time.Time

		if err := rows.Scan(&id, &txID, &amount, &paymentMethod, &status, &createdAt, &paidAt); err != nil {
			continue
		}

		tid := fmt.Sprintf("DEP-%d", id)
		if txID != nil && *txID != "" {
			tid = *txID
		}

		items = append(items, fiber.Map{
			"id":             id,
			"transaction_id": tid,
			"amount":         fmt.Sprintf("%.2f", amount),
			"payment_method": paymentMethod,
			"status":         status,
			"created_at":     createdAt.Format(time.RFC3339),
			"paid_at":        apiTimePtr(paidAt),
		})
	}

	return context.JSON(fiber.Map{
		"data": items,
		"meta": pageMeta(total, perPage, page),
	})
}

func (server *Server) createDeposit(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Amount  float64 `json:"amount"`
		Gateway string  `json:"gateway"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	if req.Amount < 50 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Minimum recharge amount is ৳50."})
	}

	now := time.Now()
	method := "piprapay"
	if req.Gateway != "" {
		method = req.Gateway
	}

	res, err := server.db.Exec(`
		INSERT INTO deposits (user_id, amount, currency, payment_method, status, created_at, updated_at)
		VALUES (?, ?, 'BDT', ?, 'pending', ?, ?)
	`, claims.UserID, req.Amount, method, now, now)
	if err != nil {
		return databaseError(context, err)
	}

	depID, _ := res.LastInsertId()
	txID := fmt.Sprintf("DEP-%d", depID)
	_, _ = server.db.Exec("UPDATE deposits SET transaction_id = ? WHERE id = ?", txID, depID)

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Deposit initiated successfully.",
		"data": fiber.Map{
			"id":             depID,
			"transaction_id": txID,
			"amount":         fmt.Sprintf("%.2f", req.Amount),
			"status":         "pending",
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// KYC
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) kycShow(context fiber.Ctx) error {
	claims := customer(context)

	u, err := server.loadUser(claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	isResubmission := u.KycStatus == "resubmit_required"
	title := "KYC Verification Required"
	heading := "Your account requires KYC verification"
	if isResubmission {
		title = "KYC Resubmission Required"
		heading = "Please update and resubmit your KYC documents"
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"user":             u,
			"is_resubmission": isResubmission,
			"requested_fields": nil,
			"title":            title,
			"heading":          heading,
			"subheading":       "We require KYC verification to ensure the security of our platform. Please submit the necessary documents to complete your account verification.",
		},
	})
}

func (server *Server) kycSubmit(context fiber.Ctx) error {
	claims := customer(context)

	storageRoot := server.kycService.StorageRoot()

	nidNumber := strings.TrimSpace(context.FormValue("nid_number"))
	tradeLicenseNumber := strings.TrimSpace(context.FormValue("trade_license_number"))

	var frontNidPath, backNidPath, livePhotoPath, tradeLicensePath *string

	// Helper to save uploaded file
	saveFile := func(formField, subDir string) (*string, error) {
		fileHeader, err := context.FormFile(formField)
		if err != nil || fileHeader == nil {
			return nil, nil
		}
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		if ext == "" {
			ext = ".jpg"
		}
		targetDir := filepath.Join(storageRoot, subDir)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return nil, err
		}
		filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
		destPath := filepath.Join(targetDir, filename)
		if err := context.SaveFile(fileHeader, destPath); err != nil {
			return nil, err
		}
		relPath := fmt.Sprintf("%s/%s", subDir, filename)
		return &relPath, nil
	}

	var err error
	frontNidPath, err = saveFile("front_nid", "ids")
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save front NID file."})
	}
	backNidPath, err = saveFile("back_nid", "ids")
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save back NID file."})
	}
	tradeLicensePath, err = saveFile("trade_license_file", "trade_licenses")
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Failed to save trade license file."})
	}

	// Handle live_photo (can be multipart file or base64 data string)
	livePhotoHeader, _ := context.FormFile("live_photo")
	if livePhotoHeader != nil {
		targetDir := filepath.Join(storageRoot, "live_photos")
		_ = os.MkdirAll(targetDir, 0755)
		filename := fmt.Sprintf("live_%s.jpg", uuid.New().String())
		destPath := filepath.Join(targetDir, filename)
		if err := context.SaveFile(livePhotoHeader, destPath); err == nil {
			p := fmt.Sprintf("live_photos/%s", filename)
			livePhotoPath = &p
		}
	} else if liveData := context.FormValue("live_photo"); strings.HasPrefix(liveData, "data:image") {
		b64 := liveData
		if commaIdx := strings.Index(b64, ","); commaIdx != -1 {
			b64 = b64[commaIdx+1:]
		}
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err == nil && len(decoded) > 0 {
			targetDir := filepath.Join(storageRoot, "live_photos")
			_ = os.MkdirAll(targetDir, 0755)
			filename := fmt.Sprintf("live_%s.jpg", uuid.New().String())
			destPath := filepath.Join(targetDir, filename)
			if err := os.WriteFile(destPath, decoded, 0644); err == nil {
				p := fmt.Sprintf("live_photos/%s", filename)
				livePhotoPath = &p
			}
		}
	}

	_, err = server.db.Exec(`
		UPDATE users
		SET nid_number = COALESCE(NULLIF(?, ''), nid_number),
		    trade_license_number = COALESCE(NULLIF(?, ''), trade_license_number),
		    front_nid = COALESCE(?, front_nid),
		    back_nid = COALESCE(?, back_nid),
		    live_photo = COALESCE(?, live_photo),
		    trade_license_file = COALESCE(?, trade_license_file),
		    kyc_status = 'under_review',
		    kyc_submitted_at = NOW(),
		    kyc_rejected_reason = NULL,
		    kyc_resubmit_fields = NULL,
		    updated_at = NOW()
		WHERE id = ?
	`, nidNumber, tradeLicenseNumber, frontNidPath, backNidPath, livePhotoPath, tradeLicensePath, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	u, _ := server.loadUser(claims.UserID)

	return context.JSON(fiber.Map{
		"message": "KYC submitted successfully. Our team will review your documents shortly.",
		"data":    u,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// IP Whitelists
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) ipWhitelists(context fiber.Ctx) error {
	claims := customer(context)

	rows, err := server.db.Query(`
		SELECT id, ip_address, COALESCE(label, ''), status, admin_note, created_at, reviewed_at
		FROM ip_whitelist_requests
		WHERE user_id = ?
		ORDER BY id DESC
	`, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var ip, label, status string
		var adminNote *string
		var createdAt time.Time
		var reviewedAt *time.Time

		if err := rows.Scan(&id, &ip, &label, &status, &adminNote, &createdAt, &reviewedAt); err != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":          id,
			"ip_address":  ip,
			"label":       label,
			"description": label,
			"status":      status,
			"admin_note":  adminNote,
			"created_at":  createdAt.Format(time.RFC3339),
			"reviewed_at": apiTimePtr(reviewedAt),
		})
	}

	return context.JSON(fiber.Map{"data": items})
}

func (server *Server) createIpWhitelist(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		IPAddress   string `json:"ip_address"`
		Label       string `json:"label"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.IPAddress = strings.TrimSpace(req.IPAddress)
	if req.IPAddress == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"ip_address": []string{"IP address is required."}},
		})
	}

	label := strings.TrimSpace(req.Label)
	if label == "" {
		label = strings.TrimSpace(req.Description)
	}

	now := time.Now()
	result, err := server.db.Exec(
		"INSERT INTO ip_whitelist_requests (user_id, ip_address, label, status, created_at, updated_at) VALUES (?, ?, ?, 'pending', ?, ?)",
		claims.UserID, req.IPAddress, nullString(label), now, now,
	)
	if err != nil {
		return databaseError(context, err)
	}

	id, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "IP whitelist request submitted.",
		"data": fiber.Map{
			"id":          id,
			"ip_address":  req.IPAddress,
			"label":       label,
			"description": label,
			"status":      "pending",
			"created_at":  now.Format(time.RFC3339),
		},
	})
}

func (server *Server) deleteIpWhitelist(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(
		"DELETE FROM ip_whitelist_requests WHERE id = ? AND user_id = ?",
		id, claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "IP whitelist entry not found."})
	}

	return context.JSON(fiber.Map{"message": "IP whitelist entry removed."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Pricing
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) pricing(context fiber.Ctx) error {
	claims := customer(context)

	var tariffPlanID *int64
	_ = server.db.QueryRow("SELECT tariff_plan_id FROM tariff_plan_assignments WHERE user_id = ? AND is_active = 1 ORDER BY id DESC LIMIT 1", claims.UserID).Scan(&tariffPlanID)
	if tariffPlanID == nil {
		_ = server.db.QueryRow("SELECT tariff_plan_id FROM tariff_plan_assignments WHERE user_id = ? ORDER BY id DESC LIMIT 1", claims.UserID).Scan(&tariffPlanID)
	}

	var rows *sql.Rows
	var err error

	if tariffPlanID != nil {
		rows, err = server.db.Query(`
			SELECT o.id, o.name, COALESCE(c.name, 'Bangladesh') as country_name, o.codes,
			       COALESCE(tr.masking_price, o.masking_price, 0.4500) as masking_rate,
			       COALESCE(tr.non_masking_price, o.non_masking_price, 0.3500) as non_masking_rate
			FROM operators o
			LEFT JOIN countries c ON c.id = o.country_id
			LEFT JOIN tariff_rates tr ON tr.operator_id = o.id AND tr.tariff_plan_id = ?
			WHERE o.is_active = 1
			ORDER BY o.name ASC
		`, *tariffPlanID)
	} else {
		rows, err = server.db.Query(`
			SELECT o.id, o.name, COALESCE(c.name, 'Bangladesh') as country_name, o.codes,
			       COALESCE(o.masking_price, 0.4500) as masking_rate,
			       COALESCE(o.non_masking_price, 0.3500) as non_masking_rate
			FROM operators o
			LEFT JOIN countries c ON c.id = o.country_id
			WHERE o.is_active = 1
			ORDER BY o.name ASC
		`)
	}

	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int64
		var name, country string
		var codesRaw []byte
		var maskingRate, nonMaskingRate float64

		if err := rows.Scan(&id, &name, &country, &codesRaw, &maskingRate, &nonMaskingRate); err != nil {
			continue
		}

		var codes []string
		if len(codesRaw) > 0 {
			_ = json.Unmarshal(codesRaw, &codes)
		}
		if len(codes) == 0 {
			switch strings.ToLower(name) {
			case "grameenphone", "grameenphone (gp)", "gp":
				codes = []string{"017", "013"}
			case "robi", "robi axiata":
				codes = []string{"018"}
			case "banglalink":
				codes = []string{"019", "014"}
			case "teletalk":
				codes = []string{"015"}
			case "airtel":
				codes = []string{"016"}
			}
		}

		items = append(items, fiber.Map{
			"id":                id,
			"name":              name,
			"operator_name":     name,
			"country":           country,
			"codes":             codes,
			"masking_rate":      maskingRate,
			"non_masking_rate":  nonMaskingRate,
			"masking_price":     maskingRate,
			"non_masking_price": nonMaskingRate,
		})
	}

	return context.JSON(fiber.Map{"data": items})
}

// ─────────────────────────────────────────────────────────────────────────────
// Developer API Tokens (access_tokens)
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) apiTokens(context fiber.Ctx) error {
	claims := customer(context)

	rows, err := server.db.Query(`
		SELECT id, name, last_used_at, expires_at, created_at
		FROM access_tokens
		WHERE tokenable_type = 'App\\Models\\User' AND tokenable_id = ?
		ORDER BY id DESC
	`, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int
		var name string
		var lastUsedAt, expiresAt *time.Time
		var createdAt time.Time

		if err := rows.Scan(&id, &name, &lastUsedAt, &expiresAt, &createdAt); err != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id":           id,
			"name":         name,
			"last_used_at": apiTimePtr(lastUsedAt),
			"expires_at":   apiTimePtr(expiresAt),
			"created_at":   createdAt.Format(time.RFC3339),
		})
	}

	return context.JSON(fiber.Map{"data": items})
}

func (server *Server) createApiToken(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"name": []string{"Token name is required."}},
		})
	}

	// Generate cryptographically secure random token
	tokenBytes := make([]byte, 40)
	if _, err := rand.Read(tokenBytes); err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to generate token."})
	}
	plainToken := hex.EncodeToString(tokenBytes)

	// Hash for storage (SHA-256, Sanctum compatible)
	digest := sha256.Sum256([]byte(plainToken))
	hashedToken := hex.EncodeToString(digest[:])

	now := time.Now()
	result, err := server.db.Exec(`
		INSERT INTO access_tokens (tokenable_type, tokenable_id, name, token, abilities, created_at, updated_at)
		VALUES ('App\\Models\\User', ?, ?, ?, '["*"]', ?, ?)
	`, claims.UserID, req.Name, hashedToken, now, now)
	if err != nil {
		return databaseError(context, err)
	}

	tokenID, _ := result.LastInsertId()

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"token": fmt.Sprintf("%d|%s", tokenID, plainToken),
		"data": fiber.Map{
			"id":           tokenID,
			"name":         req.Name,
			"last_used_at": nil,
			"expires_at":   nil,
			"created_at":   now.Format(time.RFC3339),
		},
	})
}

func (server *Server) deleteApiToken(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(
		`DELETE FROM access_tokens WHERE id = ? AND tokenable_id = ? AND tokenable_type = 'App\\Models\\User'`,
		id, claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Token not found."})
	}

	return context.JSON(fiber.Map{"message": "Token revoked successfully."})
}

// ─────────────────────────────────────────────────────────────────────────────
// Analytics
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) analyticsMessaging(context fiber.Ctx) error {
	claims := customer(context)
	from, to := resolveDateRange(context)

	var total, successful, delivered, failed int
	var totalCost, totalParts, avgParts float64
	var unicodeCount int

	_ = server.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status IN ('sent','delivered') THEN 1 ELSE 0 END),0) as successful,
			COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END),0) as delivered,
			COALESCE(SUM(CASE WHEN status IN ('failed','dlr_failed') THEN 1 ELSE 0 END),0) as failed,
			COALESCE(SUM(CASE WHEN status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END),0) as total_cost,
			COALESCE(SUM(parts),0) as total_parts,
			COALESCE(AVG(parts),0) as avg_parts,
			COALESCE(SUM(is_unicode),0) as unicode_count
		FROM messages
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
	`, claims.UserID, from, to).Scan(&total, &successful, &delivered, &failed, &totalCost, &totalParts, &avgParts, &unicodeCount)

	var campTotal, campProcessing, campCompleted, campPending int
	_ = server.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'processing' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status = 'completed' THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END),0)
		FROM batches
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
	`, claims.UserID, from, to).Scan(&campTotal, &campProcessing, &campCompleted, &campPending)

	dailyRows, _ := server.db.Query(`
		SELECT DATE(created_at) as date_val,
		       COUNT(*) as total,
		       COALESCE(SUM(CASE WHEN status IN ('sent','delivered') THEN 1 ELSE 0 END),0) as successful,
		       COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END),0) as delivered,
		       COALESCE(SUM(CASE WHEN status IN ('failed','dlr_failed') THEN 1 ELSE 0 END),0) as failed,
		       COALESCE(SUM(CASE WHEN status NOT IN ('failed','dlr_failed') THEN cost ELSE 0 END),0) as cost
		FROM messages
		WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		GROUP BY date_val
		ORDER BY date_val ASC
	`, claims.UserID, from, to)

	daily := []fiber.Map{}
	if dailyRows != nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var dateVal string
			var dTotal, dSuccessful, dDelivered, dFailed int
			var dCost float64
			if err := dailyRows.Scan(&dateVal, &dTotal, &dSuccessful, &dDelivered, &dFailed, &dCost); err != nil {
				continue
			}
			t, _ := time.Parse("2006-01-02", dateVal)
			sr := 0.0
			if dTotal > 0 {
				sr = float64(dSuccessful) / float64(dTotal) * 100
			}
			daily = append(daily, fiber.Map{
				"date":         t.Format("Jan 02"),
				"total":        dTotal,
				"successful":   dSuccessful,
				"delivered":    dDelivered,
				"failed":       dFailed,
				"success_rate": roundFloat(sr, 1),
				"cost":         roundFloat(dCost, 2),
			})
		}
	}

	sr := 0.0
	if total > 0 {
		sr = float64(successful) / float64(total) * 100
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"period": fiber.Map{"from": from[:10], "to": to[:10]},
			"stats": fiber.Map{
				"total":        total,
				"successful":   successful,
				"delivered":    delivered,
				"failed":       failed,
				"success_rate": roundFloat(sr, 1),
				"total_cost":   roundFloat(totalCost, 2),
				"total_parts":  int(totalParts),
				"avg_parts":    roundFloat(avgParts, 1),
				"unicode":      unicodeCount,
			},
			"campaigns": fiber.Map{
				"total":      campTotal,
				"processing": campProcessing,
				"completed":  campCompleted,
				"pending":    campPending,
			},
			"daily": daily,
		},
	})
}

func (server *Server) analyticsBilling(context fiber.Ctx) error {
	claims := customer(context)
	from, to := resolveDateRange(context)

	var deposits, spent, pendingDeposits float64
	var txCount int

	_ = server.db.QueryRow(
		"SELECT COALESCE(SUM(amount),0) FROM deposits WHERE user_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?",
		claims.UserID, from, to,
	).Scan(&deposits)

	_ = server.db.QueryRow(
		"SELECT COALESCE(SUM(cost),0) FROM messages WHERE user_id = ? AND status != 'failed' AND created_at >= ? AND created_at <= ?",
		claims.UserID, from, to,
	).Scan(&spent)

	_ = server.db.QueryRow(
		"SELECT COALESCE(SUM(amount),0) FROM deposits WHERE user_id = ? AND status = 'pending'",
		claims.UserID,
	).Scan(&pendingDeposits)

	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM transactions WHERE user_id = ? AND created_at >= ? AND created_at <= ?",
		claims.UserID, from, to,
	).Scan(&txCount)

	// Deposit trend: completed deposits per day
	depositTrend := []fiber.Map{}
	dtRows, _ := server.db.Query(
		`SELECT DATE(created_at) as d, COALESCE(SUM(amount),0) FROM deposits
		 WHERE user_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?
		 GROUP BY d ORDER BY d ASC`,
		claims.UserID, from, to,
	)
	if dtRows != nil {
		defer dtRows.Close()
		for dtRows.Next() {
			var d string
			var amt float64
			if err := dtRows.Scan(&d, &amt); err != nil {
				continue
			}
			t, _ := time.Parse("2006-01-02", d)
			depositTrend = append(depositTrend, fiber.Map{"date": t.Format("Jan 02"), "amount": roundFloat(amt, 2)})
		}
	}

	// Spend trend: cost per day from messages
	spendTrend := []fiber.Map{}
	stRows, _ := server.db.Query(
		`SELECT DATE(created_at) as d, COALESCE(SUM(cost),0) FROM messages
		 WHERE user_id = ? AND status != 'failed' AND created_at >= ? AND created_at <= ?
		 GROUP BY d ORDER BY d ASC`,
		claims.UserID, from, to,
	)
	if stRows != nil {
		defer stRows.Close()
		for stRows.Next() {
			var d string
			var cost float64
			if err := stRows.Scan(&d, &cost); err != nil {
				continue
			}
			t, _ := time.Parse("2006-01-02", d)
			spendTrend = append(spendTrend, fiber.Map{"date": t.Format("Jan 02"), "cost": roundFloat(cost, 2)})
		}
	}

	// Deposits by payment method
	methods := []fiber.Map{}
	mRows, _ := server.db.Query(
		`SELECT COALESCE(payment_method,'Unknown') as method, COALESCE(SUM(amount),0) as total FROM deposits
		 WHERE user_id = ? AND status = 'completed' AND created_at >= ? AND created_at <= ?
		 GROUP BY method ORDER BY total DESC`,
		claims.UserID, from, to,
	)
	if mRows != nil {
		defer mRows.Close()
		for mRows.Next() {
			var method string
			var total float64
			if err := mRows.Scan(&method, &total); err != nil {
				continue
			}
			methods = append(methods, fiber.Map{"method": method, "amount": roundFloat(total, 2)})
		}
	}

	// Transaction volume: credits vs debits per day
	volume := []fiber.Map{}
	vRows, _ := server.db.Query(
		`SELECT DATE(created_at) as d,
		        COALESCE(SUM(CASE WHEN type = 'credit' THEN amount ELSE 0 END),0) as credits,
		        COALESCE(SUM(CASE WHEN type = 'debit' THEN amount ELSE 0 END),0) as debits
		 FROM transactions
		 WHERE user_id = ? AND created_at >= ? AND created_at <= ?
		 GROUP BY d ORDER BY d ASC`,
		claims.UserID, from, to,
	)
	if vRows != nil {
		defer vRows.Close()
		for vRows.Next() {
			var d string
			var credits, debits float64
			if err := vRows.Scan(&d, &credits, &debits); err != nil {
				continue
			}
			t, _ := time.Parse("2006-01-02", d)
			volume = append(volume, fiber.Map{"date": t.Format("Jan 02"), "credits": roundFloat(credits, 2), "debits": roundFloat(debits, 2)})
		}
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"period": fiber.Map{"from": from[:10], "to": to[:10]},
			"stats": fiber.Map{
				"deposits":         roundFloat(deposits, 2),
				"spent":            roundFloat(spent, 2),
				"pending_deposits": roundFloat(pendingDeposits, 2),
				"transactions":     txCount,
			},
			"deposit_trend": depositTrend,
			"spend_trend":   spendTrend,
			"methods":       methods,
			"volume":        volume,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Live Queue
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) liveQueue(context fiber.Ctx) error {
	claims := customer(context)
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	recentThreshold := time.Now().Add(-10 * time.Second)

	// In queue now
	var inQueue int
	_ = server.db.QueryRow(`
		SELECT COUNT(*) FROM messages
		WHERE user_id = ? AND status IN ('pending', 'sending', 'processing')
	`, claims.UserID).Scan(&inQueue)

	// Last 5 min aggregation
	var total5m, success5m int
	_ = server.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN status IN ('sent', 'delivered') THEN 1 ELSE 0 END), 0)
		FROM messages
		WHERE user_id = ? AND created_at >= ?
	`, claims.UserID, fiveMinAgo).Scan(&total5m, &success5m)

	msgsPerMin := 0.0
	if total5m > 0 {
		msgsPerMin = math.Round((float64(total5m)/5.0)*10) / 10
	}
	liveSuccessRate := 0.0
	if total5m > 0 {
		liveSuccessRate = math.Round((float64(success5m)/float64(total5m))*1000) / 10
	}

	// Active batches
	var activeBatchesCount int
	_ = server.db.QueryRow(`
		SELECT COUNT(*) FROM batches
		WHERE user_id = ? AND status IN ('pending', 'processing')
	`, claims.UserID).Scan(&activeBatchesCount)

	// Active batches details
	batchRows, _ := server.db.Query(`
		SELECT b.id, b.status, COALESCE(b.total_recipients, 0), COALESCE(b.valid_count, 0),
		       COALESCE(b.body, ''), COALESCE(s.sender_id, '—'), b.created_at
		FROM batches b
		LEFT JOIN senders s ON s.id = b.sender_id
		WHERE b.user_id = ? AND b.status IN ('pending', 'processing')
		ORDER BY b.id DESC
		LIMIT 10
	`, claims.UserID)

	batchesList := []fiber.Map{}
	if batchRows != nil {
		defer batchRows.Close()
		for batchRows.Next() {
			var bid int64
			var bstatus, bbody, bsender string
			var totalRecipients, validCount int
			var bcreatedAt time.Time
			if err := batchRows.Scan(&bid, &bstatus, &totalRecipients, &validCount, &bbody, &bsender, &bcreatedAt); err == nil {
				// Count processed messages
				var processed int
				_ = server.db.QueryRow("SELECT COUNT(*) FROM messages WHERE batch_id = ? AND status IN ('sent', 'delivered', 'failed', 'dlr_failed')", bid).Scan(&processed)
				percent := 0
				if totalRecipients > 0 {
					percent = (processed * 100) / totalRecipients
					if percent > 100 {
						percent = 100
					}
				}
				batchesList = append(batchesList, fiber.Map{
					"id":               bid,
					"status":           bstatus,
					"sender_id":        bsender,
					"body":             bbody,
					"total_recipients": totalRecipients,
					"processed":        processed,
					"percent":          percent,
					"created_at":       bcreatedAt.Format(time.RFC3339),
					"time_ago":         formatTimeAgo(bcreatedAt),
				})
			}
		}
	}

	// Active and recently dispatched messages
	msgRows, _ := server.db.Query(`
		SELECT m.id, m.`+"`to`"+`, m.body, m.status, COALESCE(m.cost, 0), m.batch_id,
		       COALESCE(s.sender_id, '—'), COALESCE(op.name, '—'), COALESCE(gw.name, '—'),
		       COALESCE(m.parts, 1), m.created_at, m.updated_at
		FROM messages m
		LEFT JOIN senders s ON s.id = m.sender_id
		LEFT JOIN operators op ON op.id = m.operator_id
		LEFT JOIN gateways gw ON gw.id = m.gateway_id
		WHERE m.user_id = ? AND (
			m.status IN ('pending', 'sending', 'processing')
			OR m.updated_at >= ?
		)
		ORDER BY m.id DESC
		LIMIT 60
	`, claims.UserID, recentThreshold)

	items := []fiber.Map{}
	if msgRows != nil {
		defer msgRows.Close()
		for msgRows.Next() {
			var id, parts int
			var to, body, status, senderName, opName, gwName string
			var cost float64
			var batchID sql.NullInt64
			var createdAt, updatedAt time.Time
			if err := msgRows.Scan(&id, &to, &body, &status, &cost, &batchID, &senderName, &opName, &gwName, &parts, &createdAt, &updatedAt); err != nil {
				continue
			}

			actionType := "Quick SMS"
			if batchID.Valid && batchID.Int64 > 0 {
				actionType = "Bulk SMS"
			}

			items = append(items, fiber.Map{
				"id":          id,
				"msg_id":      fmt.Sprintf("MSG-%06d", id),
				"to":          to,
				"body":        body,
				"status":      status,
				"cost":        cost,
				"parts":       parts,
				"sender_id":   senderName,
				"operator":    opName,
				"gateway":     gwName,
				"action_type": actionType,
				"is_bulk":     batchID.Valid && batchID.Int64 > 0,
				"timestamp":   createdAt.Format("15:04:05"),
				"created_at":  createdAt.Format(time.RFC3339),
				"time_ago":    formatTimeAgo(createdAt),
			})
		}
	}

	return context.JSON(fiber.Map{
		"stats": fiber.Map{
			"in_queue":       inQueue,
			"msgs_per_min":   msgsPerMin,
			"active_batches": activeBatchesCount,
			"live_success":   fmt.Sprintf("%.0f%%", liveSuccessRate),
			"success_count":  success5m,
			"total":          total5m,
		},
		"batches":  batchesList,
		"messages": items,
		"data": fiber.Map{
			"pending":    inQueue,
			"processing": activeBatchesCount,
			"messages":   items,
		},
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// OTP stubs
// ─────────────────────────────────────────────────────────────────────────────

func (server *Server) otpSend(context fiber.Ctx) error {
	var req struct {
		Identifier string `json:"identifier"`
		Type       string `json:"type"` // "sms" or "email"
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil || req.Identifier == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Identifier is required.",
		})
	}

	targetType := strings.ToLower(req.Type)
	if targetType != "email" {
		targetType = "sms"
	}

	identifier := strings.TrimSpace(req.Identifier)
	if targetType == "sms" {
		if norm, ok := batch.FormatByRegex(identifier); ok {
			identifier = norm
		}
	} else {
		identifier = strings.ToLower(identifier)
	}

	// Rate limit check: cooldown 60 seconds
	var lastSent sql.NullTime
	err := server.db.QueryRowContext(context.Context(), `
		SELECT last_sent_at FROM verification_otps 
		WHERE identifier = ? AND type = ?
	`, identifier, targetType).Scan(&lastSent)

	if err == nil && lastSent.Valid {
		if time.Since(lastSent.Time) < 60*time.Second {
			rem := int(60 - time.Since(lastSent.Time).Seconds())
			return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"success":          false,
				"message":          fmt.Sprintf("Please wait %d seconds before requesting another code.", rem),
				"cooldown_seconds": rem,
			})
		}
	}

	// Generate 6-digit OTP
	codeInt := time.Now().UnixNano()%900000 + 100000
	code := fmt.Sprintf("%06d", codeInt)

	hash, err := bcrypt.GenerateFromPassword([]byte(code), 10)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate OTP"})
	}

	expiresAt := time.Now().Add(10 * time.Minute)

	// Upsert into verification_otps
	_, err = server.db.ExecContext(context.Context(), `
		INSERT INTO verification_otps (identifier, type, otp_hash, attempts, last_sent_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 0, NOW(), ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE 
			otp_hash = VALUES(otp_hash),
			attempts = 0,
			last_sent_at = NOW(),
			expires_at = VALUES(expires_at),
			verified_at = NULL,
			updated_at = NOW()
	`, identifier, targetType, string(hash), expiresAt)
	if err != nil {
		return databaseError(context, err)
	}

	// Dispatch OTP
	if targetType == "sms" {
		var gwDriver, gwCreds string
		err := server.db.QueryRowContext(context.Context(), `
			SELECT driver, credentials 
			FROM gateways 
			WHERE is_enabled = 1 AND is_otp_enabled = 1 
			ORDER BY priority ASC, id ASC 
			LIMIT 1
		`).Scan(&gwDriver, &gwCreds)
		if err == nil && gwDriver != "" && gwCreds != "" {
			appKey := os.Getenv("APP_KEY")
			enc, eErr := crypto.NewLaravelEncrypter(appKey)
			if eErr == nil {
				creds, dErr := enc.DecryptArray(gwCreds)
				if dErr == nil {
					driver, drvErr := drivers.CreateDriver(gwDriver, creds)
					if drvErr == nil {
						smsReq := sms.SmsRequest{
							To:   identifier,
							Body: fmt.Sprintf("Your SMS-Panel verification code is: %s. Valid for 10 minutes.", code),
						}
						go func() {
							bgCtx, cancel := stdctx.WithTimeout(stdctx.Background(), 10*time.Second)
							defer cancel()
							_, _ = driver.Send(bgCtx, smsReq)
						}()
					}
				}
			}
		}
	} else if targetType == "email" {
		body := fmt.Sprintf("<p>Your SMS-Panel verification code is: <b>%s</b>.</p><p>This code will expire in 10 minutes.</p>", code)
		if server.mailer != nil {
			_ = server.mailer.SendMail(identifier, "SMS-WhiteInfo Verification Code", body)
		}
	}

	return context.JSON(fiber.Map{
		"success":          true,
		"message":          "Verification code sent.",
		"cooldown_seconds": 60,
	})
}

func (server *Server) otpVerify(context fiber.Ctx) error {
	var req struct {
		Identifier string `json:"identifier"`
		Type       string `json:"type"`
		Code       string `json:"code"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil || req.Identifier == "" || req.Code == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Identifier and code are required.",
		})
	}

	targetType := strings.ToLower(req.Type)
	if targetType != "email" {
		targetType = "sms"
	}

	identifier := strings.TrimSpace(req.Identifier)
	if targetType == "sms" {
		if norm, ok := batch.FormatByRegex(identifier); ok {
			identifier = norm
		}
	} else {
		identifier = strings.ToLower(identifier)
	}

	var id int64
	var otpHash string
	var attempts int
	var expiresAt time.Time

	err := server.db.QueryRowContext(context.Context(), `
		SELECT id, otp_hash, attempts, expires_at 
		FROM verification_otps 
		WHERE identifier = ? AND type = ? AND verified_at IS NULL
	`, identifier, targetType).Scan(&id, &otpHash, &attempts, &expiresAt)
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Invalid or expired verification code.",
		})
	}

	if time.Now().After(expiresAt) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Verification code has expired. Please request a new one.",
		})
	}

	if attempts >= 5 {
		return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"message": "Too many failed attempts. Please request a new code.",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(otpHash), []byte(req.Code)); err != nil {
		_, _ = server.db.ExecContext(context.Context(), "UPDATE verification_otps SET attempts = attempts + 1, updated_at = NOW() WHERE id = ?", id)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Invalid verification code.",
		})
	}

	_, _ = server.db.ExecContext(context.Context(), "UPDATE verification_otps SET verified_at = NOW(), updated_at = NOW() WHERE id = ?", id)

	return context.JSON(fiber.Map{
		"success": true,
		"message": "Verification successful.",
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Shared helpers
// ─────────────────────────────────────────────────────────────────────────────

func extractVariables(body string) []string {
	seen := map[string]bool{}
	var vars []string
	inBrace := false
	var current strings.Builder

	for _, ch := range body {
		switch {
		case ch == '{':
			inBrace = true
			current.Reset()
		case ch == '}' && inBrace:
			v := current.String()
			if v != "" && !seen[v] {
				seen[v] = true
				vars = append(vars, v)
			}
			inBrace = false
		case inBrace:
			current.WriteRune(ch)
		}
	}

	return vars
}

func resolveDateRange(context fiber.Ctx) (string, string) {
	dateFrom := context.Query("date_from")
	dateTo := context.Query("date_to")

	if dateFrom != "" && dateTo != "" {
		return dateFrom + " 00:00:00", dateTo + " 23:59:59"
	}

	now := time.Now()
	dateRange := context.Query("date_range", "last_30_days")

	switch dateRange {
	case "today":
		d := now.Format("2006-01-02")
		return d + " 00:00:00", d + " 23:59:59"
	case "yesterday":
		d := now.AddDate(0, 0, -1).Format("2006-01-02")
		return d + " 00:00:00", d + " 23:59:59"
	case "last_7_days":
		return now.AddDate(0, 0, -7).Format("2006-01-02") + " 00:00:00",
			now.Format("2006-01-02") + " 23:59:59"
	case "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		return start.Format("2006-01-02") + " 00:00:00", now.Format("2006-01-02") + " 23:59:59"
	default: // last_30_days
		return now.AddDate(0, 0, -30).Format("2006-01-02") + " 00:00:00",
			now.Format("2006-01-02") + " 23:59:59"
	}
}

// Ensure sql package is referenced (used in registerPlan).
var _ = sql.ErrNoRows
