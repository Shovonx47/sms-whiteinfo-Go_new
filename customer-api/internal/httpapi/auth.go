package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	Remember      bool   `json:"remember"`
	TwoFactorCode string `json:"two_factor_code"`
	RecoveryCode  string `json:"recovery_code"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	PlanSlug string `json:"plan_slug"`
}

type customerUser struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	Phone              *string `json:"phone"`
	Balance            string  `json:"balance"`
	Status             string  `json:"status"`
	KycStatus          string  `json:"kyc_status"`
	SpID               *int64  `json:"sp_id"`
	CreatedAt          string  `json:"created_at"`
	FrontNID           *string `json:"front_nid"`
	NidNumber          *string `json:"nid_number"`
	LivePhoto          *string `json:"live_photo"`
	TradeLicenseNumber *string `json:"trade_license_number"`
	TradeLicenseFile   *string `json:"trade_license_file"`
	CreatedByAdmin     bool    `json:"created_by_admin"`
	CreatedBySP        bool    `json:"created_by_sp"`
	SmsAutoApproved    bool    `json:"sms_auto_approved"`
	APIAccess          bool    `json:"api_access"`
	KycResubmitFields  *string `json:"kyc_resubmit_fields"`
	KycRejectedReason  *string `json:"kyc_rejected_reason"`
}

func (server *Server) login(context fiber.Ctx) error {
	request := loginRequest{}
	if err := json.Unmarshal(context.Body(), &request); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The supplied data is invalid."})
	}

	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	if request.Email == "" || request.Password == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "The supplied data is invalid.",
			"errors":  fiber.Map{"email": []string{"Email and password are required."}},
		})
	}

	throttleKey := context.IP() + "|" + request.Email
	if locked, retryAfter := server.loginLimiter.Check(throttleKey); locked {
		return context.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"message": fmt.Sprintf("Too many login attempts. Please try again in %d seconds.", retryAfter),
		})
	}

	var u customerUser
	var passwordHash, userType, status, kycStatus string
	var twoFactorSecret, recoveryCodes sql.NullString
	err := server.db.QueryRow(`
		SELECT id, name, email, phone, balance, type, status, kyc_status, sp_id, created_at, password,
		       COALESCE(app_authentication_secret, ''), COALESCE(app_authentication_recovery_codes, '')
		FROM users
		WHERE email = ? AND deleted_at IS NULL
		LIMIT 1
	`, request.Email).Scan(
		&u.ID, &u.Name, &u.Email, &u.Phone, &u.Balance,
		&userType, &status, &kycStatus, &u.SpID, &u.CreatedAt, &passwordHash,
		&twoFactorSecret, &recoveryCodes,
	)
	if err == sql.ErrNoRows {
		server.loginLimiter.RecordFailure(throttleKey)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "The supplied credentials are invalid.",
			"errors":  fiber.Map{"email": []string{"These credentials do not match our records."}},
		})
	}
	if err != nil {
		return databaseError(context, err)
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(request.Password)) != nil {
		server.loginLimiter.RecordFailure(throttleKey)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "The supplied credentials are invalid.",
			"errors":  fiber.Map{"email": []string{"These credentials do not match our records."}},
		})
	}

	// Check user type
	if userType != "customer" {
		server.loginLimiter.RecordFailure(throttleKey)
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "The supplied credentials are invalid.",
			"errors":  fiber.Map{"email": []string{"These credentials do not match our records."}},
		})
	}

	// Check account status
	if status == "banned" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"email": []string{"Your account has been banned due to a violation of our Terms & Conditions. Please contact the administrator for assistance."}},
		})
	}
	if status == "rejected" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"email": []string{"Your account application was rejected. Please contact the administrator for more information."}},
		})
	}
	if status == "pending" && (kycStatus == "pending" || kycStatus == "under_review") {
		// Accounts created by an admin are allowed to sign in while still pending KYC.
		var isAdminCreated bool
		_ = server.db.QueryRow("SELECT COALESCE(created_by_admin, false) FROM users WHERE id = ?", u.ID).Scan(&isAdminCreated)
		if !isAdminCreated {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
				"errors": fiber.Map{"email": []string{"Your account status is under review by the admin. Please contact the administrator for further assistance."}},
			})
		}
	}

	if strings.TrimSpace(twoFactorSecret.String) != "" {
		if request.TwoFactorCode == "" && request.RecoveryCode == "" {
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
				"message":             "Enter the 6-digit code from your authenticator app.",
				"two_factor_required": true,
			})
		}
		if ok, _ := server.verifyLoginTwoFactor(u.ID, twoFactorSecret.String, recoveryCodes.String, request.TwoFactorCode, request.RecoveryCode); !ok {
			server.loginLimiter.RecordFailure(throttleKey)
			return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
				"message":             "The authenticator code is invalid.",
				"two_factor_required": true,
				"errors":              fiber.Map{"two_factor_code": []string{"The authenticator code is invalid."}},
			})
		}
	}

	u.Status = status
	u.KycStatus = kycStatus

	_, _ = server.db.Exec("UPDATE users SET last_login_at = NOW() WHERE id = ?", u.ID)
	server.loginLimiter.Reset(throttleKey)

	ttl := server.config.TokenTTL
	if request.Remember {
		ttl = server.config.RememberTokenTTL
	}

	token, err := server.tokens.Issue(u.ID, u.Name, u.Email, time.Now().Add(ttl))
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to sign in."})
	}

	context.Cookie(&fiber.Cookie{
		Name:     "customer_token",
		Value:    token,
		Expires:  time.Now().Add(ttl),
		HTTPOnly: false,
		SameSite: "Lax",
		Path:     "/",
	})

	return context.JSON(fiber.Map{
		"token": token,
		"data": fiber.Map{
			"token": token,
			"user":  u,
		},
	})
}

func (server *Server) registerPlan(context fiber.Ctx) error {
	plan, err := server.resolveSignupPlan(context.Params("slug"))
	if err != nil {
		return databaseError(context, err)
	}
	if plan == nil {
		return context.Status(http.StatusGone).JSON(fiber.Map{"message": "This signup link is no longer active."})
	}

	return context.JSON(fiber.Map{
		"data": fiber.Map{
			"slug":     plan.CanonicalSlug,
			"name":     plan.Name,
			"is_alias": plan.IsAlias,
		},
	})
}

func (server *Server) logout(context fiber.Ctx) error {
	claims := customer(context)
	token, _ := context.Locals("customer_token").(string)
	if token != "" && claims.ExpiresAt != nil {
		_, err := server.db.Exec(`
			INSERT INTO cache (`+"`key`"+`, value, expiration)
			VALUES (?, 'b:1;', ?)
			ON DUPLICATE KEY UPDATE value = VALUES(value), expiration = VALUES(expiration)
		`, server.tokenCacheKey(token), claims.ExpiresAt.Unix())
		if err != nil {
			return databaseError(context, err)
		}
	}

	context.Cookie(&fiber.Cookie{
		Name:     "customer_token",
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		HTTPOnly: false,
		SameSite: "Lax",
		Path:     "/",
	})

	return context.SendStatus(http.StatusNoContent)
}

func (server *Server) me(context fiber.Ctx) error {
	claims := customer(context)

	u, err := server.loadUser(claims.UserID)
	if err != nil {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	return context.JSON(fiber.Map{"data": u})
}

func (server *Server) updateProfile(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"name": []string{"Name is required."}},
		})
	}

	_, err := server.db.Exec(
		"UPDATE users SET name = ?, phone = ?, updated_at = NOW() WHERE id = ?",
		req.Name, nullString(strings.TrimSpace(req.Phone)), claims.UserID,
	)
	if err != nil {
		return databaseError(context, err)
	}

	u, _ := server.loadUser(claims.UserID)

	return context.JSON(fiber.Map{"data": u})
}

func (server *Server) updatePassword(context fiber.Ctx) error {
	claims := customer(context)

	var req struct {
		CurrentPassword string `json:"current_password"`
		Password        string `json:"password"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	var currentHash string
	err := server.db.QueryRow("SELECT password FROM users WHERE id = ?", claims.UserID).Scan(&currentHash)
	if err != nil {
		return databaseError(context, err)
	}

	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(req.CurrentPassword)) != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"current_password": []string{"The provided password does not match your current password."}},
		})
	}

	if len(req.Password) < 8 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"errors": fiber.Map{"password": []string{"Password must be at least 8 characters."}},
		})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to update password."})
	}

	_, _ = server.db.Exec("UPDATE users SET password = ?, updated_at = NOW() WHERE id = ?", string(hash), claims.UserID)

	return context.JSON(fiber.Map{"message": "Password updated."})
}

func (server *Server) forgotPassword(context fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"email": []string{"Email address is required."}},
		})
	}

	var userID int64
	var userName string
	err := server.db.QueryRow("SELECT id, name FROM users WHERE email = ? AND type = 'customer' AND deleted_at IS NULL", req.Email).Scan(&userID, &userName)
	if err == nil {
		rawBytes := make([]byte, 32)
		_, _ = rand.Read(rawBytes)
		rawToken := hex.EncodeToString(rawBytes)

		hashToken := sha256.Sum256([]byte(rawToken))
		hashedHex := hex.EncodeToString(hashToken[:])

		_, _ = server.db.Exec(`
			INSERT INTO password_reset_tokens (email, token, created_at)
			VALUES (?, ?, NOW())
			ON DUPLICATE KEY UPDATE token = VALUES(token), created_at = NOW()
		`, req.Email, hashedHex)

		if server.mailer != nil {
			resetURL := fmt.Sprintf("%s/reset-password?token=%s&email=%s", server.config.FrontendOrigin, rawToken, req.Email)
			subject := "Reset Your Password"
			body := fmt.Sprintf("Hello %s,\n\nYou have requested a password reset. Click the link below to reset your password:\n\n%s\n\nThis link will expire in 60 minutes.\nIf you did not request this, please ignore this email.", userName, resetURL)
			_ = server.mailer.Send(req.Email, subject, body)
		}
	}

	return context.JSON(fiber.Map{
		"message": "If that email exists in our system, a password reset link has been sent.",
	})
}

func (server *Server) resetPassword(context fiber.Ctx) error {
	var req struct {
		Token                string `json:"token"`
		Email                string `json:"email"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Token == "" || req.Email == "" || len(req.Password) < 8 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"password": []string{"Password must be at least 8 characters and all fields are required."}},
		})
	}
	if req.PasswordConfirmation != "" && req.Password != req.PasswordConfirmation {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"password": []string{"Password confirmation does not match."}},
		})
	}

	var storedHash string
	err := server.db.QueryRow(`
		SELECT token FROM password_reset_tokens
		WHERE email = ? AND created_at >= DATE_SUB(NOW(), INTERVAL 60 MINUTE)
		LIMIT 1
	`, req.Email).Scan(&storedHash)
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Invalid or expired password reset token.",
			"errors":  fiber.Map{"email": []string{"This password reset token is invalid or has expired."}},
		})
	}

	computedHash := sha256.Sum256([]byte(req.Token))
	computedHex := hex.EncodeToString(computedHash[:])
	if storedHash != computedHex && storedHash != req.Token {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Invalid password reset token.",
			"errors":  fiber.Map{"email": []string{"This password reset token is invalid."}},
		})
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to reset password."})
	}

	_, _ = server.db.Exec("UPDATE users SET password = ?, updated_at = NOW() WHERE email = ? AND type = 'customer'", string(newHash), req.Email)
	_, _ = server.db.Exec("DELETE FROM password_reset_tokens WHERE email = ?", req.Email)

	return context.JSON(fiber.Map{
		"message": "Your password has been reset successfully. You can now sign in.",
	})
}

// loadUser fetches a full customer record by ID.
func (server *Server) loadUser(userID int64) (*customerUser, error) {
	u := &customerUser{}
	var createdAt time.Time
	err := server.db.QueryRow(`
		SELECT id, name, email, phone, balance, status, kyc_status, sp_id, created_at,
		       front_nid, nid_number, live_photo, trade_license_number, trade_license_file,
		       created_by_admin, created_by_sp, COALESCE(sms_auto_approved, 1), COALESCE(api_access, 0),
		       kyc_resubmit_fields, kyc_rejected_reason
		FROM users
		WHERE id = ? AND deleted_at IS NULL
	`, userID).Scan(
		&u.ID, &u.Name, &u.Email, &u.Phone,
		&u.Balance, &u.Status, &u.KycStatus, &u.SpID, &createdAt,
		&u.FrontNID, &u.NidNumber, &u.LivePhoto, &u.TradeLicenseNumber, &u.TradeLicenseFile,
		&u.CreatedByAdmin, &u.CreatedBySP, &u.SmsAutoApproved, &u.APIAccess,
		&u.KycResubmitFields, &u.KycRejectedReason,
	)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = createdAt.Format(time.RFC3339)

	return u, nil
}

// Helpers

func nullString(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	return re.MatchString(email)
}

// requestEmailChange implements Filament's emailChangeVerification flow: the
// signed-in customer confirms their password, and a one-hour token is mailed to
// the new address. The account email only changes after that token is used.
var emailChangePattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

func (server *Server) requestEmailChange(context fiber.Ctx) error {
	claims := customer(context)
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request."})
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || !emailChangePattern.MatchString(req.Email) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"email": []string{"Enter a valid email address."}},
		})
	}

	var currentEmail, passwordHash, name string
	if err := server.db.QueryRow("SELECT email, password, name FROM users WHERE id = ? AND deleted_at IS NULL", claims.UserID).Scan(&currentEmail, &passwordHash, &name); err != nil {
		return databaseError(context, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)) != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"password": []string{"The provided password is incorrect."}},
		})
	}
	if strings.EqualFold(currentEmail, req.Email) {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"email": []string{"The new email address must be different from your current email address."}},
		})
	}

	var taken int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id <> ? AND deleted_at IS NULL", req.Email, claims.UserID).Scan(&taken)
	if taken > 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "Validation failed.",
			"errors":  fiber.Map{"email": []string{"The email address has already been taken."}},
		})
	}

	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to start email change."})
	}
	rawToken := hex.EncodeToString(rawBytes)
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(hash[:])

	_, _ = server.db.Exec("DELETE FROM email_change_requests WHERE user_id = ? AND used_at IS NULL", claims.UserID)
	if _, err := server.db.Exec(`
		INSERT INTO email_change_requests (user_id, new_email, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, DATE_ADD(NOW(), INTERVAL 60 MINUTE), NOW())
	`, claims.UserID, req.Email, tokenHash); err != nil {
		return databaseError(context, err)
	}

	if server.mailer != nil {
		verifyURL := fmt.Sprintf("%s/email-change/verify?token=%s", server.config.FrontendOrigin, rawToken)
		body := fmt.Sprintf("Hello %s,\n\nConfirm your new email address for SMS Whiteinfo by opening this link:\n\n%s\n\nThis link expires in 60 minutes. If you did not request this change, you can ignore this email.", name, verifyURL)
		_ = server.mailer.Send(req.Email, "Confirm Your New Email Address", body)
	}

	return context.JSON(fiber.Map{
		"message": "A verification link has been sent to your new email address.",
	})
}

func (server *Server) verifyEmailChange(context fiber.Ctx) error {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(context.Body(), &req); err != nil || strings.TrimSpace(req.Token) == "" {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "This email verification link is invalid or has expired.",
		})
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(req.Token)))
	tokenHash := hex.EncodeToString(hash[:])

	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(context, err)
	}
	defer func() { _ = tx.Rollback() }()

	var id, userID int64
	var newEmail string
	err = tx.QueryRow(`
		SELECT id, user_id, new_email FROM email_change_requests
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, tokenHash).Scan(&id, &userID, &newEmail)
	if err == sql.ErrNoRows {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "This email verification link is invalid or has expired.",
		})
	}
	if err != nil {
		return databaseError(context, err)
	}

	var taken int
	_ = tx.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id <> ? AND deleted_at IS NULL", newEmail, userID).Scan(&taken)
	if taken > 0 {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
			"message": "That email address is no longer available. Please request a new verification link.",
		})
	}

	if _, err := tx.Exec("UPDATE users SET email = ?, email_verified_at = NOW(), updated_at = NOW() WHERE id = ?", newEmail, userID); err != nil {
		return databaseError(context, err)
	}
	if _, err := tx.Exec("UPDATE email_change_requests SET used_at = NOW() WHERE id = ?", id); err != nil {
		return databaseError(context, err)
	}
	if err := tx.Commit(); err != nil {
		return databaseError(context, err)
	}

	return context.JSON(fiber.Map{"message": "Your email address has been updated."})
}
