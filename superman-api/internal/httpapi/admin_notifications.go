package httpapi

import (
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v3"
)

const adminNotifiableType = "App\\Models\\User"

type adminNotificationItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	ReadAt    any    `json:"read_at"`
	CreatedAt string `json:"created_at"`
}

func (server *Server) adminNotifications(context fiber.Ctx) error {
	admin := superman(context)

	rows, err := server.db.Query(`
		SELECT id, data, read_at, created_at
		FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ?
		ORDER BY created_at DESC
		LIMIT 30
	`, adminNotifiableType, admin.AdminID)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []adminNotificationItem{}
	for rows.Next() {
		var id string
		var data []byte
		var readAt *time.Time
		var createdAt time.Time
		if rows.Scan(&id, &data, &readAt, &createdAt) != nil {
			continue
		}

		var payload map[string]any
		_ = json.Unmarshal(data, &payload)
		text := func(key string) string {
			if value, ok := payload[key].(string); ok {
				return value
			}
			return ""
		}

		status := text("status")
		if status == "" {
			status = text("color")
		}
		url := text("url")
		if url == "" {
			if actions, ok := payload["actions"].([]any); ok && len(actions) > 0 {
				if action, ok := actions[0].(map[string]any); ok {
					if actionURL, ok := action["url"].(string); ok {
						url = actionURL
					}
				}
			}
		}

		var readAtValue any
		if readAt != nil {
			readAtValue = readAt.Format(time.RFC3339)
		}

		items = append(items, adminNotificationItem{
			ID:        id,
			Title:     text("title"),
			Body:      text("body"),
			Status:    status,
			URL:       url,
			ReadAt:    readAtValue,
			CreatedAt: createdAt.Format(time.RFC3339),
		})
	}

	var unread int
	_ = server.db.QueryRow(`
		SELECT COUNT(*) FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, adminNotifiableType, admin.AdminID).Scan(&unread)

	return context.JSON(fiber.Map{"data": items, "unread_count": unread})
}

func (server *Server) markAdminNotificationRead(context fiber.Ctx) error {
	admin := superman(context)

	if _, err := server.db.Exec(`
		UPDATE notifications SET read_at = NOW()
		WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, context.Params("id"), adminNotifiableType, admin.AdminID); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Notification marked as read."})
}

func (server *Server) markAllAdminNotificationsRead(context fiber.Ctx) error {
	admin := superman(context)

	if _, err := server.db.Exec(`
		UPDATE notifications SET read_at = NOW()
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, adminNotifiableType, admin.AdminID); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "All notifications marked as read."})
}
