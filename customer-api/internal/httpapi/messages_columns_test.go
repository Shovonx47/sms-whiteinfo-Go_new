package httpapi

import "testing"

func TestResolvedSMSTypeMatchesLegacyFallback(t *testing.T) {
	cases := []struct {
		name       string
		hasBatch   bool
		sourceType string
		total      int
		want       string
	}{
		{"quick message", false, "", 0, "Transactional"},
		{"single batch", true, "single", 1, "Transactional"},
		{"manual one recipient", true, "manual", 1, "Transactional"},
		{"manual bulk", true, "manual", 25, "Promotional"},
		{"group bulk", true, "group", 25, "Promotional"},
		{"upload bulk", true, "upload", 100, "Promotional"},
	}
	for _, tc := range cases {
		if got := resolvedSMSType(tc.hasBatch, tc.sourceType, tc.total); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestChannelFromOperator(t *testing.T) {
	cases := map[string]string{
		"Grameenphone": "CH-01",
		"Teletalk":     "CH-03",
		"Robi":         "CH-02",
		"Banglalink":   "CH-02",
		"Airtel":       "CH-02",
		"Other":        "",
	}
	for operator, want := range cases {
		if got := channelFromOperator(operator); got != want {
			t.Errorf("%s: got %q, want %q", operator, got, want)
		}
	}
}

func TestRequestIDFromGateway(t *testing.T) {
	cases := map[string]string{
		"":                "",
		"123456789":       "REQ-456789",
		"TXN-ABC-9876543": "REQ-876543",
		"gateway-ref":     "REQ-gateway-ref",
	}
	for input, want := range cases {
		if got := requestIDFromGateway(input); got != want {
			t.Errorf("%s: got %q, want %q", input, got, want)
		}
	}
}
