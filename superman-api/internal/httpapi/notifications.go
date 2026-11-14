package httpapi

import (
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
)

type notificationExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// notifyUser writes a Filament-compatible database notification. The admin
// actions in this service run inside transactions, so the caller passes either
// *sql.DB or *sql.Tx.
func notifyUser(execer notificationExecer, userID int64, title, body, status string) {
	if userID <= 0 || title == "" {
		return
	}

	payload, err := json.Marshal(map[string]any{
		"title":  title,
		"body":   body,
		"status": status,
	})
	if err != nil {
		return
	}

	_, _ = execer.Exec(`
		INSERT INTO notifications (id, type, notifiable_type, notifiable_id, data, read_at, created_at, updated_at)
		VALUES (?, ?, 'App\\Models\\User', ?, ?, NULL, NOW(), NOW())
	`, uuid.NewString(), "App\\Notifications\\DashboardNotification", userID, string(payload))
}
