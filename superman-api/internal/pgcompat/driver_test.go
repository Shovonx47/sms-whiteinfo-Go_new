package pgcompat

import (
	"strings"
	"testing"
)

func TestRebindSenderAndReportQueries(t *testing.T) {
	insertIgnore := Rebind("INSERT IGNORE INTO sender_user (sender_id, user_id) VALUES (?, ?)")
	if !strings.Contains(insertIgnore, "INSERT INTO sender_user (sender_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING") {
		t.Fatalf("unexpected Rebind for INSERT IGNORE: %q", insertIgnore)
	}

	backticks := Rebind("SELECT * FROM `senders` WHERE `is_enabled` = TRUE AND `id` = ?")
	if backticks != `SELECT * FROM "senders" WHERE "is_enabled" = TRUE AND "id" = $1` {
		t.Fatalf("unexpected Rebind for backticks: %q", backticks)
	}
}
