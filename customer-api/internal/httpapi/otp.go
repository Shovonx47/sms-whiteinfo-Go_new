package httpapi

import (
	"bytes"
	stdctx "context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"smswhiteinfo/customer-api/internal/batch"
	"smswhiteinfo/customer-api/internal/crypto"
	"smswhiteinfo/customer-api/internal/sms"
	"smswhiteinfo/customer-api/internal/sms/drivers"
)

const (
	otpExpiryMinutes    = 10
	otpCooldownSeconds  = 60
	otpMaxAttempts      = 5
	otpMaxHourlyResends = 5
)

// otpRequest accepts both the SPA payload (channel/email/phone/otp) and the
// existing JSON payload (identifier/type/code).
type otpRequest struct {
	Identifier string `json:"identifier"`
	Type       string `json:"type"`
	Channel    string `json:"channel"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
	Code       string `json:"code"`
	Otp        string `json:"otp"`
}

func (request otpRequest) target() (identifier string, targetType string) {
	targetType = strings.ToLower(strings.TrimSpace(request.Type))
	if targetType == "" {
		targetType = strings.ToLower(strings.TrimSpace(request.Channel))
	}
	if targetType != "email" {
		targetType = "sms"
	}

	identifier = request.Identifier
	if identifier == "" && targetType == "email" {
		identifier = request.Email
	}
	if identifier == "" && targetType == "sms" {
		identifier = request.Phone
	}

	return identifier, targetType
}

func (request otpRequest) code() string {
	if request.Code != "" {
		return request.Code
	}

	return request.Otp
}

func normalizeOtpIdentifier(identifier, targetType string) (string, bool) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", false
	}

	if targetType == "email" {
		identifier = strings.ToLower(identifier)
		if !isValidEmail(identifier) {
			return "", false
		}

		return identifier, true
	}

	normalized, ok := batch.FormatByRegex(identifier)
	if !ok {
		return "", false
	}

	return normalized, true
}

// hasValidSession reports whether the request carries a valid customer token.
// OTP sending stays available to signed-in users (used for email/phone changes),
// while anonymous signup OTPs enforce the "already registered" rules from the old flow.
func (server *Server) hasValidSession(context fiber.Ctx) bool {
	rawToken := ""
	if authorization := context.Get(fiber.HeaderAuthorization); strings.HasPrefix(authorization, "Bearer ") {
		rawToken = strings.TrimPrefix(authorization, "Bearer ")
	} else if cookie := context.Cookies("customer_token"); cookie != "" {
		rawToken = cookie
	}
	if rawToken == "" {
		return false
	}

	_, err := server.tokens.Parse(rawToken)

	return err == nil
}

func (server *Server) identifierIsRegistered(identifier, targetType string) bool {
	var count int
	if targetType == "email" {
		_ = server.db.QueryRow(
			"SELECT COUNT(*) FROM users WHERE email = ? AND deleted_at IS NULL",
			identifier,
		).Scan(&count)

		return count > 0
	}

	variants := phoneVariants(identifier)
	marks := make([]string, len(variants))
	args := make([]any, 0, len(variants))
	for i, variant := range variants {
		marks[i] = "?"
		args = append(args, variant)
	}
	query := "SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND phone IN (" + strings.Join(marks, ",") + ")"
	_ = server.db.QueryRow(query, args...).Scan(&count)

	return count > 0
}

func (server *Server) otpSend(context fiber.Ctx) error {
	var request otpRequest
	if err := json.Unmarshal(context.Body(), &request); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Identifier is required.",
		})
	}

	identifier, targetType := request.target()
	normalized, ok := normalizeOtpIdentifier(identifier, targetType)
	if !ok {
		message := "Invalid mobile number. Please enter a valid Bangladesh mobile number."
		if targetType == "email" {
			message = "Invalid email address provided."
		}

		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": message,
		})
	}
	identifier = normalized

	if !server.hasValidSession(context) && server.identifierIsRegistered(identifier, targetType) {
		message := "This mobile number is already registered."
		if targetType == "email" {
			message = "This email address is already registered."
		}

		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": message,
		})
	}

	// 60-second cooldown, based on the most recent code for this identifier.
	var lastSent sql.NullTime
	err := server.db.QueryRow(
		"SELECT last_sent_at FROM verification_otps WHERE identifier = ? AND type = ? ORDER BY id DESC LIMIT 1",
		identifier, targetType,
	).Scan(&lastSent)
	if err == nil && lastSent.Valid {
		if elapsed := time.Since(lastSent.Time); elapsed < otpCooldownSeconds*time.Second {
			remaining := int((otpCooldownSeconds*time.Second - elapsed).Seconds()) + 1
			if remaining < 1 {
				remaining = 1
			}

			return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
				"success":          false,
				"message":          fmt.Sprintf("Please wait %d seconds before requesting a new code.", remaining),
				"cooldown_seconds": remaining,
			})
		}
	}

	// Maximum five resends per identifier per hour (old OtpVerificationService).
	var hourlyCount int
	_ = server.db.QueryRow(
		"SELECT COUNT(*) FROM verification_otps WHERE identifier = ? AND type = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 1 HOUR)",
		identifier, targetType,
	).Scan(&hourlyCount)
	if hourlyCount >= otpMaxHourlyResends {
		return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
			"success":          false,
			"message":          "Too many requests. Please try again in an hour.",
			"cooldown_seconds": 3600,
		})
	}

	code, err := generateOTP()
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate OTP.",
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), 12)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate OTP.",
		})
	}

	expiresAt := time.Now().Add(otpExpiryMinutes * time.Minute)
	_, err = server.db.Exec(`
		INSERT INTO verification_otps (identifier, type, otp_hash, attempts, last_sent_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, 0, NOW(), ?, NOW(), NOW())
	`, identifier, targetType, string(hash), expiresAt)
	if err != nil {
		return databaseError(context, err)
	}

	if targetType == "sms" {
		server.dispatchSignupSMSOTP(identifier, code)
	} else if server.mailer != nil {
		body := fmt.Sprintf("<p>Your SMS-Panel verification code is: <b>%s</b>.</p><p>This code will expire in %d minutes.</p>", code, otpExpiryMinutes)
		_ = server.mailer.SendMail(identifier, "SMS-WhiteInfo Verification Code", body)
	}

	message := "Verification code sent to your mobile number via SMS."
	if targetType == "email" {
		message = "Verification code sent to your email."
	}

	return context.JSON(fiber.Map{
		"success":          true,
		"message":          message,
		"cooldown_seconds": otpCooldownSeconds,
	})
}

func (server *Server) otpVerify(context fiber.Ctx) error {
	var request otpRequest
	if err := json.Unmarshal(context.Body(), &request); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Identifier and code are required.",
		})
	}

	identifier, targetType := request.target()
	normalized, ok := normalizeOtpIdentifier(identifier, targetType)
	if !ok {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Invalid identifier.",
		})
	}
	identifier = normalized

	code := strings.TrimSpace(request.code())
	if code == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Identifier and code are required.",
		})
	}

	var id int64
	var otpHash string
	var attempts int
	var expiresAt time.Time
	err := server.db.QueryRow(`
		SELECT id, otp_hash, attempts, expires_at
		FROM verification_otps
		WHERE identifier = ? AND type = ?
		ORDER BY id DESC
		LIMIT 1
	`, identifier, targetType).Scan(&id, &otpHash, &attempts, &expiresAt)
	if err == sql.ErrNoRows {
		subject := "phone number"
		if targetType == "email" {
			subject = "email"
		}

		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "No verification code requested for this " + subject + ".",
		})
	}
	if err != nil {
		return databaseError(context, err)
	}

	if time.Now().After(expiresAt) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": "Verification code has expired. Please request a new code.",
		})
	}

	if attempts >= otpMaxAttempts {
		return context.Status(http.StatusTooManyRequests).JSON(fiber.Map{
			"success": false,
			"message": "Too many failed attempts. Please request a new verification code.",
		})
	}

	if bcrypt.CompareHashAndPassword([]byte(otpHash), []byte(code)) != nil {
		_, _ = server.db.Exec("UPDATE verification_otps SET attempts = attempts + 1, updated_at = NOW() WHERE id = ?", id)
		remaining := otpMaxAttempts - (attempts + 1)
		message := "Too many failed attempts. Please request a new verification code."
		if remaining > 0 {
			message = fmt.Sprintf("Incorrect code. %d attempts remaining.", remaining)
		}

		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"success": false,
			"message": message,
		})
	}

	token, err := randomToken(60)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Unable to verify code.",
		})
	}

	if _, err := server.db.Exec(
		"UPDATE verification_otps SET verified_at = NOW(), token = ?, updated_at = NOW() WHERE id = ?",
		token, id,
	); err != nil {
		return databaseError(context, err)
	}

	label := "Sms"
	if targetType == "email" {
		label = "Email"
	}

	return context.JSON(fiber.Map{
		"success": true,
		"message": label + " verified successfully.",
		"token":   token,
	})
}

// isOTPVerified mirrors OtpVerificationService::isVerified(): the proof token must
// belong to a row verified in the last two hours.
func (server *Server) isOTPVerified(identifier, targetType, token string) bool {
	if token == "" {
		return false
	}

	normalized, ok := normalizeOtpIdentifier(identifier, targetType)
	if !ok {
		return false
	}

	var exists int
	err := server.db.QueryRow(`
		SELECT COUNT(*) FROM verification_otps
		WHERE identifier = ? AND type = ? AND token = ? AND verified_at IS NOT NULL
		  AND verified_at >= DATE_SUB(NOW(), INTERVAL 2 HOUR)
	`, normalized, targetType, token).Scan(&exists)

	return err == nil && exists > 0
}

func (server *Server) dispatchSignupSMSOTP(phone, code string) {
	// Old flow tries the configured signup/register HTTP gateways first.
	if server.dispatchViaSignupGateways(phone, code) {
		return
	}

	var gwDriver, gwCreds string
	err := server.db.QueryRow(`
		SELECT driver, credentials
		FROM gateways
		WHERE is_enabled = 1 AND is_otp_enabled = 1
		ORDER BY priority ASC, id ASC
		LIMIT 1
	`).Scan(&gwDriver, &gwCreds)
	if err != nil || gwDriver == "" || gwCreds == "" {
		return
	}

	enc, err := crypto.NewLaravelEncrypter(os.Getenv("APP_KEY"))
	if err != nil {
		return
	}
	creds, err := enc.DecryptArray(gwCreds)
	if err != nil {
		return
	}
	driver, err := drivers.CreateDriver(gwDriver, creds)
	if err != nil {
		return
	}

	message := sms.SmsRequest{
		To:   phone,
		Body: fmt.Sprintf("Your SMS WhiteInfo verification code is: %s. Valid for %d minutes.", code, otpExpiryMinutes),
	}
	go func() {
		bgCtx, cancel := stdctx.WithTimeout(stdctx.Background(), 10*time.Second)
		defer cancel()
		_, _ = driver.Send(bgCtx, message)
	}()
}

func (server *Server) dispatchViaSignupGateways(phone, code string) bool {
	rows, err := server.db.Query(`
		SELECT endpoint_url, http_method, bearer_token, sender_id, body_template, phone_format, timeout_seconds
		FROM signup_register_gateways
		WHERE is_active = 1
		ORDER BY priority ASC, id ASC
	`)
	if err != nil {
		return false
	}
	defer rows.Close()

	enc, encErr := crypto.NewLaravelEncrypter(os.Getenv("APP_KEY"))

	for rows.Next() {
		var endpoint, method, tokenCipher, senderID, bodyTemplate, phoneFormat string
		var timeoutSeconds int
		if rows.Scan(&endpoint, &method, &tokenCipher, &senderID, &bodyTemplate, &phoneFormat, &timeoutSeconds) != nil {
			continue
		}

		bearerToken := ""
		if encErr == nil && tokenCipher != "" {
			if decrypted, decErr := enc.DecryptString(tokenCipher); decErr == nil {
				bearerToken = decrypted
			}
		}

		body := strings.ReplaceAll(bodyTemplate, "{otp}", code)
		body = strings.ReplaceAll(body, "{minutes}", fmt.Sprintf("%d", otpExpiryMinutes))
		numbers := formatSignupGatewayPhone(phone, phoneFormat)

		payload := map[string]string{
			"sender_id": senderID,
			"numbers":   numbers,
			"body":      body,
		}

		timeout := time.Duration(timeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 10 * time.Second
		}
		client := &http.Client{Timeout: timeout}

		var request *http.Request
		var reqErr error
		if strings.EqualFold(method, "GET") {
			parsed, parseErr := url.Parse(endpoint)
			if parseErr != nil {
				continue
			}
			query := parsed.Query()
			for key, value := range payload {
				query.Set(key, value)
			}
			parsed.RawQuery = query.Encode()
			request, reqErr = http.NewRequest(http.MethodGet, parsed.String(), nil)
		} else {
			encoded, marshalErr := json.Marshal(payload)
			if marshalErr != nil {
				continue
			}
			request, reqErr = http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(encoded))
			if request != nil {
				request.Header.Set("Content-Type", "application/json")
			}
		}
		if reqErr != nil || request == nil {
			continue
		}

		request.Header.Set("Accept", "application/json")
		if bearerToken != "" {
			request.Header.Set("Authorization", "Bearer "+bearerToken)
		}

		response, sendErr := client.Do(request)
		if sendErr != nil {
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return true
		}
	}

	return false
}

func formatSignupGatewayPhone(phone, format string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}

		return -1
	}, phone)

	if strings.EqualFold(format, "e164") {
		if strings.HasPrefix(phone, "+") {
			return phone
		}

		return "+" + digits
	}

	if strings.HasPrefix(digits, "880") && len(digits) == 13 {
		return digits[2:]
	}

	return digits
}

func generateOTP() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", value.Int64()+100000), nil
}

func randomToken(length int) (string, error) {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	result := make([]byte, length)
	for i := range result {
		index, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		result[i] = alphabet[index.Int64()]
	}

	return string(result), nil
}
