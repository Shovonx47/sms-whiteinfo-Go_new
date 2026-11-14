package httpapi

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseTokenExpiry(t *testing.T) {
	// Default (nothing specified) -> ~30 days
	exp, ok := parseTokenExpiry(nil, nil)
	if !ok || exp == nil {
		t.Fatalf("expected default 30-day expiry")
	}
	diffDays := int(time.Until(*exp).Hours() / 24)
	if diffDays < 29 || diffDays > 31 {
		t.Fatalf("expected ~30 days, got %d", diffDays)
	}

	// "never" -> nil expiry, valid
	exp, ok = parseTokenExpiry(json.RawMessage(`"never"`), nil)
	if !ok || exp != nil {
		t.Fatalf("expected nil expiry for 'never', got %v (ok=%v)", exp, ok)
	}

	// "7" or numeric 90 -> valid future date
	exp, ok = parseTokenExpiry(json.RawMessage(`90`), nil)
	if !ok || exp == nil {
		t.Fatalf("expected 90-day expiry")
	}
	diffDays = int(time.Until(*exp).Hours() / 24)
	if diffDays < 89 || diffDays > 91 {
		t.Fatalf("expected ~90 days, got %d", diffDays)
	}

	// Invalid preset
	if _, ok := parseTokenExpiry(json.RawMessage(`"bogus"`), nil); ok {
		t.Fatalf("expected invalid preset to be rejected")
	}
}
