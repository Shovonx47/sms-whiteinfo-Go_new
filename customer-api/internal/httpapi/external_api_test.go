package httpapi

import (
	"testing"
	"time"
)

func TestNormalizeRecipientNumbers(t *testing.T) {
	got := normalizeRecipientNumbers("01700000000, 01800000000;01900000000\n01600000000")
	want := []string{"01700000000", "01800000000", "01900000000", "01600000000"}
	if len(got) != len(want) {
		t.Fatalf("expected %d numbers, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected numbers: %v", got)
		}
	}

	array := normalizeRecipientNumbers([]any{"01700000000", " 01800000000 ", ""})
	if len(array) != 2 || array[0] != "01700000000" || array[1] != "01800000000" {
		t.Fatalf("unexpected array normalization: %v", array)
	}
}

func TestParseSchedule(t *testing.T) {
	future := time.Now().Add(2 * time.Hour).Format("2006-01-02 15:04:05")
	parsed, ok := parseSchedule(future)
	if !ok || parsed == nil {
		t.Fatalf("expected future schedule to parse")
	}

	past := time.Now().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	if _, ok := parseSchedule(past); ok {
		t.Fatalf("expected past schedule to be rejected")
	}

	if _, ok := parseSchedule("not-a-date"); ok {
		t.Fatalf("expected invalid schedule to be rejected")
	}
}

func TestValidatePublicIPStr(t *testing.T) {
	cases := []struct {
		value string
		valid bool
	}{
		{"203.0.113.10", true},
		{"8.8.8.8", true},
		{"203.0.113.0/24", true},
		{"192.168.1.1", false},
		{"10.0.0.1", false},
		{"127.0.0.1", false},
		{"100.64.0.1", false},
		{"192.168.1.0/24", false},
		{"10.0.0.0/8", false},
		{"not-an-ip", false},
	}

	for _, tc := range cases {
		errMsg := validatePublicIPStr(tc.value)
		if tc.valid && errMsg != "" {
			t.Fatalf("expected %s valid, got %q", tc.value, errMsg)
		}
		if !tc.valid && errMsg == "" {
			t.Fatalf("expected %s invalid", tc.value)
		}
	}
}

func TestIPMatchesAny(t *testing.T) {
	entries := []string{"203.0.113.0/24", "8.8.8.8"}

	if !ipMatchesAny("203.0.113.55", entries) {
		t.Fatalf("expected CIDR match")
	}
	if !ipMatchesAny("8.8.8.8", entries) {
		t.Fatalf("expected exact match")
	}
	if ipMatchesAny("198.51.100.1", entries) {
		t.Fatalf("expected no match")
	}
}
