package httpapi

import (
	"strings"
	"testing"
	"time"
)

func TestHOTPCodesMatchRFC6238(t *testing.T) {
	// RFC 6238 SHA-1 test secret (ASCII "12345678901234567890").
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cases := []struct {
		unix int64
		code string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, tc := range cases {
		got, err := hotpCode(secret, uint64(tc.unix/30))
		if err != nil {
			t.Fatalf("hotpCode: %v", err)
		}
		if got != tc.code {
			t.Errorf("counter %d: expected %s, got %s", tc.unix/30, tc.code, got)
		}
	}
}

func TestValidateTOTPWindow(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	now := time.Unix(1111111111, 0)
	current, _ := hotpCode(secret, uint64(now.Unix()/30))
	if !validateTOTP(secret, current, now) {
		t.Fatal("current code should validate")
	}
	previous, _ := hotpCode(secret, uint64(now.Unix()/30-1))
	if !validateTOTP(secret, previous, now) {
		t.Fatal("previous step code should validate within the allowed window")
	}
	old, _ := hotpCode(secret, uint64(now.Unix()/30-10))
	if validateTOTP(secret, old, now) {
		t.Fatal("codes outside the window must be rejected")
	}
}

func TestRecoveryCodesAreHashedAndSingleUse(t *testing.T) {
	t.Setenv("APP_KEY", "base64:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	codes := []string{"aaaa-bbbb", "cccc-dddd"}
	hashed := hashRecoveryCodes(codes)
	if len(hashed) != 2 || hashed[0] == codes[0] {
		t.Fatalf("recovery codes must be stored hashed, got %v", hashed)
	}
	raw, err := encryptFilamentRecoveryCodes(hashed)
	if err != nil {
		t.Fatalf("encrypt recovery codes: %v", err)
	}
	remaining, ok := consumeRecoveryCode(raw, "aaaa-bbbb")
	if !ok || len(remaining) != 1 {
		t.Fatalf("expected first recovery code to be consumed, got %v (ok=%v)", remaining, ok)
	}
	if _, ok := consumeRecoveryCode(raw, "aaaa-bbbb"); !ok {
		t.Fatal("stored value must still contain the second code")
	}
	reencrypted, err := encryptFilamentRecoveryCodes(remaining)
	if err != nil {
		t.Fatalf("re-encrypt: %v", err)
	}
	if _, ok := consumeRecoveryCode(reencrypted, "aaaa-bbbb"); ok {
		t.Fatal("already consumed recovery code must not validate")
	}
	if _, ok := consumeRecoveryCode(reencrypted, "cccc-dddd"); !ok {
		t.Fatal("second recovery code should still validate")
	}
}

func TestFilamentEncryptedSecretCompatibility(t *testing.T) {
	t.Setenv("APP_KEY", "base64:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")
	enc, err := filentEncrypter()
	if err != nil {
		t.Fatalf("encrypter: %v", err)
	}
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	stored, err := enc.Encrypt([]byte(secret))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if got := decryptFilamentValue(stored); got != secret {
		t.Fatalf("expected %s, got %s", secret, got)
	}
}

func TestPHPRecoveryCodeSerialization(t *testing.T) {
	values := []string{"one", "two"}
	got := phpSerializeStrings(values)
	want := `a:2:{i:0;s:3:"one";i:1;s:3:"two";}`
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
	parsed := parseRecoveryCodes(got)
	if len(parsed) != 2 || parsed[0] != "one" || parsed[1] != "two" {
		t.Fatalf("unexpected parsed codes: %v", parsed)
	}
}

func TestGenerateTOTPSecretIsBase32(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil || len(secret) < 16 {
		t.Fatalf("unexpected secret %q: %v", secret, err)
	}
	if strings.ContainsAny(secret, "0189") {
		t.Fatalf("base32 secret contains invalid characters: %q", secret)
	}
}
