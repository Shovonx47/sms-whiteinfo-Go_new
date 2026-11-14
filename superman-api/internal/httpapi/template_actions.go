package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

func (server *Server) approveTemplate(context fiber.Ctx) error {
	admin := superman(context)
	id := context.Params("id")

	var name string
	var userID int64
	err := server.db.QueryRow(
		"SELECT name, user_id FROM templates WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&name, &userID)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	result, err := server.db.Exec(`
		UPDATE templates
		SET approval = 'approved', approved_by = ?, approved_at = NOW(), rejection_reason = NULL, updated_at = NOW()
		WHERE id = ? AND COALESCE(approval, 'approved') = 'pending' AND deleted_at IS NULL
	`, admin.AdminID, id)
	if err != nil {
		return databaseError(context, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "This template is not awaiting approval.",
		})
	}

	notifyUser(server.db, userID, "Bulk SMS Template Approved",
		"Your template '"+name+"' has been approved. You can now use it to send campaigns without waiting for daily approvals.",
		"success")

	return context.JSON(fiber.Map{"message": "Template approved."})
}

func (server *Server) rejectTemplate(context fiber.Ctx) error {
	admin := superman(context)
	id := context.Params("id")

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(context.Body(), &payload); err != nil || strings.TrimSpace(payload.Reason) == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "A rejection reason is required.",
		})
	}
	reason := strings.TrimSpace(payload.Reason)

	var name string
	var userID int64
	err := server.db.QueryRow(
		"SELECT name, user_id FROM templates WHERE id = ? AND deleted_at IS NULL",
		id,
	).Scan(&name, &userID)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Template not found."})
	}
	if err != nil {
		return databaseError(context, err)
	}

	result, err := server.db.Exec(`
		UPDATE templates
		SET approval = 'rejected', approved_by = ?, approved_at = NOW(), rejection_reason = ?, updated_at = ?
		WHERE id = ? AND COALESCE(approval, 'approved') = 'pending' AND deleted_at IS NULL
	`, admin.AdminID, reason, time.Now(), id)
	if err != nil {
		return databaseError(context, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "This template is not awaiting approval.",
		})
	}

	notifyUser(server.db, userID, "Bulk SMS Template Rejected",
		"Your template '"+name+"' was rejected by the admin. Reason: "+reason,
		"danger")

	return context.JSON(fiber.Map{"message": "Template rejected."})
}
