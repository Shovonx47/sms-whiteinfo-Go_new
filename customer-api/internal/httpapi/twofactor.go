package httpapi

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	qrcode "github.com/skip2/go-qrcode"
	"golang.org/x/crypto/bcrypt"

	"smswhiteinfo/customer-api/internal/crypto"
)

var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

func generateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base32NoPadding.EncodeToString(buf), nil
}

func hotpCode(secret string, counter uint64) (string, error) {
	secret = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	key, err := base32NoPadding.DecodeString(secret)
	if err != nil {
		return "", err
	}
	var counterBytes [8]byte
	binary.BigEndian.PutUint64(counterBytes[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(counterBytes[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])
	return fmt.Sprintf("%06d", value%1_000_000), nil
}

func validateTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	counter := now.Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		candidateCounter := counter + offset
		if candidateCounter < 0 {
			continue
		}
		candidate, err := hotpCode(secret, uint64(candidateCounter))
		if err == nil && subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func randomCodePart(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%0*d", length, time.Now().UnixNano())
	}
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

// Filament generates recovery codes as two 10-character strings separated by "-"
// and stores only bcrypt hashes of them (AppAuthentication::generateRecoveryCodes).
func generateRecoveryCodes() []string {
	codes := make([]string, 8)
	for i := range codes {
		codes[i] = randomCodePart(10) + "-" + randomCodePart(10)
	}
	return codes
}

func hashRecoveryCodes(codes []string) []string {
	hashed := make([]string, 0, len(codes))
	for _, code := range codes {
		h, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
		if err == nil {
			hashed = append(hashed, string(h))
		}
	}
	return hashed
}

func filentEncrypter() (*crypto.LaravelEncrypter, error) {
	return crypto.NewLaravelEncrypter(os.Getenv("APP_KEY"))
}

// decryptFilamentValue reads Filament's encrypted cast values while remaining
// backward compatible with plaintext values written by earlier Go builds.
func decryptFilamentValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	enc, err := filentEncrypter()
	if err != nil {
		return raw
	}
	plain, err := enc.DecryptString(raw)
	if err != nil {
		return raw
	}
	return plain
}

// phpSerializeStrings serializes a list exactly as PHP serialize() does; this
// keeps recovery codes readable by the old Laravel encrypted:array cast.
func phpSerializeStrings(values []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "a:%d:{", len(values))
	for i, value := range values {
		fmt.Fprintf(&b, "i:%d;s:%d:\"%s\";", i, len(value), value)
	}
	b.WriteString("}")
	return b.String()
}

func parseRecoveryCodes(raw string) []string {
	plain := decryptFilamentValue(raw)
	if plain == "" {
		return nil
	}
	var jsonCodes []string
	if json.Unmarshal([]byte(plain), &jsonCodes) == nil {
		return jsonCodes
	}
	parsed, err := crypto.ParsePhpOrJson([]byte(plain))
	if err != nil {
		return nil
	}
	keys := make([]int, 0, len(parsed))
	for key := range parsed {
		if index, err := strconv.Atoi(key); err == nil {
			keys = append(keys, index)
		}
	}
	sort.Ints(keys)
	codes := make([]string, 0, len(keys))
	for _, key := range keys {
		if code, ok := parsed[strconv.Itoa(key)].(string); ok {
			codes = append(codes, code)
		}
	}
	return codes
}

func consumeRecoveryCode(raw, supplied string) ([]string, bool) {
	codes := parseRecoveryCodes(raw)
	supplied = strings.TrimSpace(supplied)
	for i, code := range codes {
		if bcrypt.CompareHashAndPassword([]byte(code), []byte(supplied)) == nil {
			return append(codes[:i], codes[i+1:]...), true
		}
	}
	return codes, false
}

func encryptFilamentRecoveryCodes(hashed []string) (string, error) {
	enc, err := filentEncrypter()
	if err != nil {
		return "", err
	}
	return enc.Encrypt([]byte(phpSerializeStrings(hashed)))
}

func encryptFilamentSecret(secret string) (string, error) {
	enc, err := filentEncrypter()
	if err != nil {
		return "", err
	}
	return enc.Encrypt([]byte(secret))
}

func (server *Server) twoFactorStatus(context fiber.Ctx) error {
	claims := customer(context)
	var secret sql.NullString
	if err := server.db.QueryRow("SELECT app_authentication_secret FROM users WHERE id = ?", claims.UserID).Scan(&secret); err != nil {
		return databaseError(context, err)
	}
	return context.JSON(fiber.Map{"data": fiber.Map{
		"enabled": decryptFilamentValue(secret.String) != "",
	}})
}

func (server *Server) twoFactorSetup(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil || req.Password == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Your current password is required."})
	}

	var passwordHash string
	if err := server.db.QueryRow("SELECT password FROM users WHERE id = ?", claims.UserID).Scan(&passwordHash); err != nil {
		return databaseError(context, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"password": []string{"The provided password does not match your current password."}},
		})
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to generate a secret."})
	}

	issuer := "White INFO"
	label := issuer + ":" + claims.Email
	otpauth := "otpauth://totp/" + url.PathEscape(label) + "?secret=" + secret + "&issuer=" + url.QueryEscape(issuer) + "&digits=6&period=30"
	png, err := qrcode.Encode(otpauth, qrcode.Medium, 220)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to generate a QR code."})
	}

	return context.JSON(fiber.Map{"data": fiber.Map{
		"secret":      secret,
		"otpauth_url": otpauth,
		"qr_data_uri": "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	}})
}

func (server *Server) twoFactorEnable(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Secret string `json:"secret"`
		Code   string `json:"code"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}
	req.Secret = strings.TrimSpace(req.Secret)
	if req.Secret == "" || !validateTOTP(req.Secret, req.Code, time.Now()) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"code": []string{"The authenticator code is invalid."}},
		})
	}

	recoveryCodes := generateRecoveryCodes()
	encryptedSecret, err := encryptFilamentSecret(req.Secret)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to store the authenticator secret."})
	}
	encryptedRecovery, err := encryptFilamentRecoveryCodes(hashRecoveryCodes(recoveryCodes))
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to store recovery codes."})
	}
	if _, err := server.db.Exec(
		"UPDATE users SET app_authentication_secret = ?, app_authentication_recovery_codes = ?, updated_at = NOW() WHERE id = ?",
		encryptedSecret, encryptedRecovery, claims.UserID,
	); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{
		"message":        "Two-factor authentication enabled.",
		"recovery_codes": recoveryCodes,
	})
}

func (server *Server) twoFactorDisable(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Password string `json:"password"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	var passwordHash string
	var secret, recoveryRaw sql.NullString
	if err := server.db.QueryRow(
		"SELECT password, app_authentication_secret, COALESCE(app_authentication_recovery_codes, '') FROM users WHERE id = ?",
		claims.UserID,
	).Scan(&passwordHash, &secret, &recoveryRaw); err != nil {
		return databaseError(context, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"password": []string{"The provided password does not match your current password."}},
		})
	}
	if secret.String != "" && !validateTOTP(decryptFilamentValue(secret.String), req.Code, time.Now()) {
		if _, ok := consumeRecoveryCode(recoveryRaw.String, req.Code); !ok {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
				"errors": fiber.Map{"code": []string{"Enter a valid authenticator or recovery code."}},
			})
		}
	}

	if _, err := server.db.Exec(
		"UPDATE users SET app_authentication_secret = NULL, app_authentication_recovery_codes = NULL, updated_at = NOW() WHERE id = ?",
		claims.UserID,
	); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Two-factor authentication disabled."})
}

// verifyLoginTwoFactor checks the supplied authenticator or recovery code.
// It returns the remaining recovery codes when one was consumed.
func (server *Server) verifyLoginTwoFactor(userID int64, secret, recoveryRaw, code, recovery string) (bool, []string) {
	now := time.Now()
	if code != "" && validateTOTP(decryptFilamentValue(secret), code, now) {
		return true, nil
	}
	if recovery != "" {
		remaining, ok := consumeRecoveryCode(recoveryRaw, recovery)
		if ok {
			if encoded, err := encryptFilamentRecoveryCodes(remaining); err == nil {
				_, _ = server.db.Exec("UPDATE users SET app_authentication_recovery_codes = ? WHERE id = ?", encoded, userID)
			}
			return true, remaining
		}
	}
	return false, nil
}
