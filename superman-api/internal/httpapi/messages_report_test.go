package httpapi

import (
	"strings"
	"testing"
)

func TestBillMsisdnFromSender(t *testing.T) {
	cases := map[string]string{
		"8809639228876": "8809639228876",
		"09639228876":   "8809639228876",
		"639228876":     "880639228876",
		"8809639228925": "8809639228925",
		"WHITEINFO":     "",
		"":              "",
	}

	for input, want := range cases {
		if got := billMsisdnFromSender(input); got != want {
			t.Fatalf("billMsisdnFromSender(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestOperatorForRecipient(t *testing.T) {
	cases := map[string]string{
		"+8801326221808": "Grameenphone",
		"8801826513091":  "Robi",
		"01925942222":    "Banglalink",
		"+8801599999999": "Teletalk",
		"+8801699999999": "Airtel",
		"":               "",
	}

	for input, want := range cases {
		if got := operatorForRecipient(input); got != want {
			t.Fatalf("operatorForRecipient(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestResolvedSmsType(t *testing.T) {
	if got := resolvedSmsType(true); got != "P" {
		t.Fatalf("batch sms type = %q, want P", got)
	}
	if got := resolvedSmsType(false); got != "T" {
		t.Fatalf("single sms type = %q, want T", got)
	}
}

func TestReportColumn(t *testing.T) {
	if got := reportColumn(true, "channel"); got != "COALESCE(m.channel, '')" {
		t.Fatalf("available column expression = %q", got)
	}
	if got := reportColumn(false, "channel"); got != "''" {
		t.Fatalf("missing column expression = %q", got)
	}
}

func TestBalanceSnapshotSQL(t *testing.T) {
	expression := balanceSnapshotSQL("balance_before")
	for _, fragment := range []string{"SELECT t.balance_before", "t.transactionable_id = m.batch_id", "t.transactionable_id = m.id", "ORDER BY t.id DESC LIMIT 1"} {
		if !strings.Contains(expression, fragment) {
			t.Fatalf("balance snapshot expression missing %q: %s", fragment, expression)
		}
	}
}
