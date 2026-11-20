package httpapi

import (
	"database/sql"
	"strings"
	"sync"
)

// messageReportColumns records which gateway snapshot columns exist on the
// messages table. The columns are added by
// database/migrations/2026_11_19_add_sms_report_fields_to_messages.sql; the
// report still renders when that migration has not been applied yet.
type messageReportColumns struct {
	billMsisdn         bool
	smsType            bool
	channel            bool
	requestID          bool
	serverTxnID        bool
	serverResponseCode bool
	mnoTxnID           bool
	mnoResponseCode    bool
	mnoResponseMessage bool
}

var (
	messageReportSchemaOnce sync.Once
	messageReportSchemaSet  messageReportColumns
)

func messageReportSchema(server *Server) messageReportColumns {
	messageReportSchemaOnce.Do(func() {
		probe := func(column string) bool {
			var ignored sql.NullString
			err := server.db.QueryRow("SELECT " + column + " FROM messages WHERE 1 = 0").Scan(&ignored)

			return err == nil || err == sql.ErrNoRows
		}

		messageReportSchemaSet = messageReportColumns{
			billMsisdn:         probe("bill_msisdn"),
			smsType:            probe("sms_type"),
			channel:            probe("channel"),
			requestID:          probe("request_id"),
			serverTxnID:        probe("server_txn_id"),
			serverResponseCode: probe("server_response_code"),
			mnoTxnID:           probe("mno_txn_id"),
			mnoResponseCode:    probe("mno_response_code"),
			mnoResponseMessage: probe("mno_response_message"),
		}
	})

	return messageReportSchemaSet
}

// reportColumn builds a SELECT expression for an optional snapshot column.
func reportColumn(available bool, column string) string {
	if available {
		return "COALESCE(m." + column + ", '')"
	}

	return "''"
}

// balanceSnapshotSQL links a message to the ledger row that moved the wallet,
// either through its batch debit or through a per-message debit/refund.
func balanceSnapshotSQL(column string) string {
	return "(SELECT t." + column + " FROM transactions t WHERE t.user_id = m.user_id " +
		"AND ((t.transactionable_type IN ('message_batch', 'App\\\\Models\\\\Batch') AND t.transactionable_id = m.batch_id) " +
		"OR (t.transactionable_type IN ('message', 'App\\\\Models\\\\Message') AND t.transactionable_id = m.id)) " +
		"ORDER BY t.id DESC LIMIT 1)"
}

// operatorForRecipient maps a Bangladeshi mobile prefix to its operator, used
// when the message was queued without a resolved operator name.
func operatorForRecipient(phone string) string {
	cleaned := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(phone), "+"), "88")
	if len(cleaned) < 3 {
		return ""
	}

	switch cleaned[:3] {
	case "017", "013":
		return "Grameenphone"
	case "018":
		return "Robi"
	case "019", "014":
		return "Banglalink"
	case "015":
		return "Teletalk"
	case "016":
		return "Airtel"
	}

	return ""
}

// billMsisdnFromSender mirrors the send-time rule for numeric IPTSP CLIs: the
// billing MSISDN is the sender CLI normalised to 880 format.
func billMsisdnFromSender(senderID string) string {
	senderID = strings.TrimSpace(senderID)
	if senderID == "" {
		return ""
	}

	var digits strings.Builder
	for _, character := range senderID {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') {
			return ""
		}
		if character >= '0' && character <= '9' {
			digits.WriteRune(character)
		}
	}
	if digits.Len() == 0 {
		return ""
	}

	value := digits.String()
	if strings.HasPrefix(value, "0") {
		return "880" + value[1:]
	}
	if !strings.HasPrefix(value, "880") {
		return "880" + value
	}

	return value
}

// resolvedSmsType mirrors the sender rule used by the Laravel report: a message
// sent on its own is transactional (T), a queued batch is promotional (P).
func resolvedSmsType(hasBatch bool) string {
	if hasBatch {
		return "P"
	}

	return "T"
}
