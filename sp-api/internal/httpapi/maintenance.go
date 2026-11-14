package httpapi

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// maintenanceGuard mirrors Laravel's down/up behaviour for the reseller API.
func (server *Server) maintenanceGuard(context fiber.Ctx) error {
	if context.Path() == "/health" {
		return context.Next()
	}

	var downPayload []byte
	err := server.db.QueryRow(
		"SELECT payload FROM settings WHERE `group` = 'system' AND name = 'maintenance_is_down' LIMIT 1",
	).Scan(&downPayload)
	if err != nil {
		return context.Next()
	}

	var isDown bool
	if json.Unmarshal(downPayload, &isDown) != nil || !isDown {
		return context.Next()
	}

	secret := "whiteinfo-admin-bypass"
	var secretPayload []byte
	if server.db.QueryRow(
		"SELECT payload FROM settings WHERE `group` = 'system' AND name = 'maintenance_secret' LIMIT 1",
	).Scan(&secretPayload) == nil {
		var stored string
		if json.Unmarshal(secretPayload, &stored) == nil && strings.TrimSpace(stored) != "" {
			secret = stored
		}
	}

	if context.Query("secret") == secret || context.Get("X-Maintenance-Secret") == secret {
		return context.Next()
	}

	notice := "System is currently undergoing scheduled platform upgrades."
	var noticePayload []byte
	if server.db.QueryRow(
		"SELECT payload FROM settings WHERE `group` = 'system' AND name = 'maintenance_notice' LIMIT 1",
	).Scan(&noticePayload) == nil {
		var stored string
		if json.Unmarshal(noticePayload, &stored) == nil && strings.TrimSpace(stored) != "" {
			notice = stored
		}
	}

	var exemptPayload []byte
	if server.db.QueryRow(
		"SELECT payload FROM settings WHERE `group` = 'system' AND name = 'maintenance_exempt_ips' LIMIT 1",
	).Scan(&exemptPayload) == nil {
		var exempt []string
		if json.Unmarshal(exemptPayload, &exempt) == nil && ipInList(context.IP(), exempt) {
			return context.Next()
		}
	}

	return context.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
		"status":  "error",
		"code":    5005,
		"message": notice,
	})
}

func ipInList(clientIP string, entries []string) bool {
	parsed := net.ParseIP(clientIP)
	if parsed == nil {
		return false
	}

	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
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
