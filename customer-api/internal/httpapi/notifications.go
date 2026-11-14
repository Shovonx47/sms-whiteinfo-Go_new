package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// Database notifications use the old Laravel notifications table so existing
// customer notifications remain readable after the Go migration.
const notificationNotifiableType = "App\\Models\\User"

type notificationItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Icon      string `json:"icon,omitempty"`
	URL       string `json:"url,omitempty"`
	ReadAt    any    `json:"read_at"`
	CreatedAt string `json:"created_at"`
}

func notificationStrings(data []byte) (title, body, status, icon, url string) {
	var payload map[string]any
	if json.Unmarshal(data, &payload) != nil {
		return "", "", "", "", ""
	}

	text := func(key string) string {
		if value, ok := payload[key].(string); ok {
			return value
		}

		return ""
	}

	title = text("title")
	body = text("body")
	status = text("status")
	if status == "" {
		status = text("color")
	}
	icon = text("icon")
	url = text("url")

	if url == "" {
		if actions, ok := payload["actions"].([]any); ok && len(actions) > 0 {
			if action, ok := actions[0].(map[string]any); ok {
				if actionURL, ok := action["url"].(string); ok {
					url = actionURL
				}
			}
		}
	}

	return title, body, status, icon, url
}

func (server *Server) notifications(context fiber.Ctx) error {
	claims := customer(context)

	rows, err := server.db.Query(`
		SELECT id, data, read_at, created_at
		FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ?
		ORDER BY created_at DESC
		LIMIT 30
	`, notificationNotifiableType, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}
	defer rows.Close()

	items := []notificationItem{}
	for rows.Next() {
		var id string
		var data []byte
		var readAt *time.Time
		var createdAt time.Time
		if rows.Scan(&id, &data, &readAt, &createdAt) != nil {
			continue
		}

		title, body, status, icon, url := notificationStrings(data)
		items = append(items, notificationItem{
			ID:        id,
			Title:     title,
			Body:      body,
			Status:    status,
			Icon:      icon,
			URL:       url,
			ReadAt:    apiTimePtr(readAt),
			CreatedAt: createdAt.Format(time.RFC3339),
		})
	}

	var unread int
	_ = server.db.QueryRow(`
		SELECT COUNT(*) FROM notifications
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, notificationNotifiableType, claims.UserID).Scan(&unread)

	return context.JSON(fiber.Map{
		"data":         items,
		"unread_count": unread,
	})
}

func (server *Server) markNotificationRead(context fiber.Ctx) error {
	claims := customer(context)
	id := context.Params("id")

	result, err := server.db.Exec(`
		UPDATE notifications SET read_at = NOW()
		WHERE id = ? AND notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, id, notificationNotifiableType, claims.UserID)
	if err != nil {
		return databaseError(context, err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		var exists int
		if server.db.QueryRow(`
			SELECT COUNT(*) FROM notifications
			WHERE id = ? AND notifiable_type = ? AND notifiable_id = ?
		`, id, notificationNotifiableType, claims.UserID).Scan(&exists); exists == 0 {
			return context.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Notification not found."})
		}
	}

	return context.JSON(fiber.Map{"message": "Notification marked as read."})
}

func (server *Server) markAllNotificationsRead(context fiber.Ctx) error {
	claims := customer(context)

	if _, err := server.db.Exec(`
		UPDATE notifications SET read_at = NOW()
		WHERE notifiable_type = ? AND notifiable_id = ? AND read_at IS NULL
	`, notificationNotifiableType, claims.UserID); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "All notifications marked as read."})
}

// notifyAdmins sends a database notification to every Superman admin.
func (server *Server) notifyAdmins(title, body, status, url string) {
	rows, err := server.db.Query("SELECT id FROM users WHERE type = 'superman' AND deleted_at IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			server.createNotification(id, title, body, status, "heroicon-o-identification", url)
		}
	}
}

// createNotification writes a Filament-compatible database notification.
func (server *Server) createNotification(userID int64, title, body, status, icon, url string) {
	if userID <= 0 || title == "" {
		return
	}

	payload := map[string]any{
		"title":  title,
		"body":   body,
		"status": status,
	}
	if icon != "" {
		payload["icon"] = icon
	}
	if url != "" {
		payload["actions"] = []map[string]any{{"label": "View", "url": url}}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return
	}

	now := time.Now()
	_, _ = server.db.Exec(`
		INSERT INTO notifications (id, type, notifiable_type, notifiable_id, data, read_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, ?, ?)
	`, uuid.NewString(), "App\\Notifications\\DashboardNotification", notificationNotifiableType, userID, string(encoded), now, now)
}
