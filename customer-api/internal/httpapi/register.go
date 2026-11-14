package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"smswhiteinfo/customer-api/internal/batch"
	"smswhiteinfo/customer-api/internal/validation"
)

var signupSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type registerForm struct {
	Name                   string
	Email                  string
	Phone                  string
	Password               string
	PasswordConfirmation   string
	EmailVerificationToken string
	PhoneVerificationToken string
	NIDNumber              string
	BusinessName           string
	TradeLicenseNumber     string
	BusinessAddress        string
	PlanSlug               string
	Terms                  bool
	LivePhoto              string
	FrontNID               *multipart.FileHeader
	BackNID                *multipart.FileHeader
	TradeLicenseFile       *multipart.FileHeader
}

// registerJSON is kept for existing JSON clients and the security tests.
type registerJSON struct {
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	Phone                  string `json:"phone"`
	Password               string `json:"password"`
	PasswordConfirmation   string `json:"password_confirmation"`
	EmailVerificationToken string `json:"email_verification_token"`
	PhoneVerificationToken string `json:"phone_verification_token"`
	NIDNumber              string `json:"nid_number"`
	BusinessName           string `json:"business_name"`
	TradeLicenseNumber     string `json:"trade_license_number"`
	BusinessAddress        string `json:"business_address"`
	PlanSlug               string `json:"plan_slug"`
	Terms                  bool   `json:"terms"`
	LivePhoto              string `json:"live_photo"`
}

type signupPlan struct {
	ID            int64
	SPID          *int64
	Name          string
	CanonicalSlug string
	IsAlias       bool
}

func parseRegisterForm(context fiber.Ctx) (*registerForm, error) {
	contentType := strings.ToLower(context.Get(fiber.HeaderContentType))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		form, err := context.Request().MultipartForm()
		if err != nil {
			return nil, err
		}

		return &registerForm{
			Name:                   firstFormValue(form.Value, "name"),
			Email:                  firstFormValue(form.Value, "email"),
			Phone:                  firstFormValue(form.Value, "phone"),
			Password:               firstFormValue(form.Value, "password"),
			PasswordConfirmation:   firstFormValue(form.Value, "password_confirmation"),
			EmailVerificationToken: firstFormValue(form.Value, "email_verification_token"),
			PhoneVerificationToken: firstFormValue(form.Value, "phone_verification_token"),
			NIDNumber:              firstFormValue(form.Value, "nid_number"),
			BusinessName:           firstFormValue(form.Value, "business_name"),
			TradeLicenseNumber:     firstFormValue(form.Value, "trade_license_number"),
			BusinessAddress:        firstFormValue(form.Value, "business_address"),
			PlanSlug:               firstFormValue(form.Value, "plan_slug"),
			Terms:                  truthy(firstFormValue(form.Value, "terms")),
			LivePhoto:              firstFormValue(form.Value, "live_photo"),
			FrontNID:               firstFormFile(form.File, "front_nid"),
			BackNID:                firstFormFile(form.File, "back_nid"),
			TradeLicenseFile:       firstFormFile(form.File, "trade_license_file"),
		}, nil
	}

	var request registerJSON
	if err := json.Unmarshal(context.Body(), &request); err != nil {
		return nil, err
	}

	return &registerForm{
		Name:                   request.Name,
		Email:                  request.Email,
		Phone:                  request.Phone,
		Password:               request.Password,
		PasswordConfirmation:   request.PasswordConfirmation,
		EmailVerificationToken: request.EmailVerificationToken,
		PhoneVerificationToken: request.PhoneVerificationToken,
		NIDNumber:              request.NIDNumber,
		BusinessName:           request.BusinessName,
		TradeLicenseNumber:     request.TradeLicenseNumber,
		BusinessAddress:        request.BusinessAddress,
		PlanSlug:               request.PlanSlug,
		Terms:                  request.Terms,
		LivePhoto:              request.LivePhoto,
	}, nil
}

func firstFormValue(values map[string][]string, key string) string {
	if entries, ok := values[key]; ok && len(entries) > 0 {
		return entries[0]
	}

	return ""
}

func firstFormFile(files map[string][]*multipart.FileHeader, key string) *multipart.FileHeader {
	if entries, ok := files[key]; ok && len(entries) > 0 {
		return entries[0]
	}

	return nil
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "on", "yes":
		return true
	}

	return false
}

func registerValidationError(context fiber.Ctx, field, message string) error {
	return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
		"message": message,
		"errors":  fiber.Map{field: []string{message}},
	})
}

// phoneVariants mirrors Register::getPhoneVariants so a number cannot register
// twice in different formats (017..., +88017..., 88017...).
func phoneVariants(normalized string) []string {
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return nil
	}

	seen := map[string]bool{}
	variants := make([]string, 0, 3)
	add := func(value string) {
		if value != "" && !seen[value] {
			seen[value] = true
			variants = append(variants, value)
		}
	}

	add(normalized)
	digits := strings.TrimPrefix(normalized, "+")
	add(digits)
	if strings.HasPrefix(digits, "880") && len(digits) == 13 {
		add("0" + digits[3:])
	}

	return variants
}

func (server *Server) resolveSignupPlan(slug string) (*signupPlan, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" || len(slug) > 64 || !signupSlugPattern.MatchString(slug) {
		return nil, nil
	}

	var plan signupPlan
	var spID sql.NullInt64
	var status string

	err := server.db.QueryRow(
		"SELECT id, sp_id, name, slug, status FROM tariff_plans WHERE slug = ? LIMIT 1",
		slug,
	).Scan(&plan.ID, &spID, &plan.Name, &plan.CanonicalSlug, &status)
	if err == sql.ErrNoRows {
		err = server.db.QueryRow(`
			SELECT tp.id, tp.sp_id, tp.name, tp.slug, tp.status
			FROM signup_slug_aliases alias
			JOIN tariff_plans tp ON tp.id = alias.tariff_plan_id
			WHERE alias.slug = ?
			LIMIT 1
		`, slug).Scan(&plan.ID, &spID, &plan.Name, &plan.CanonicalSlug, &status)
		plan.IsAlias = true
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if status != "active" {
		return nil, nil
	}
	if spID.Valid {
		var resellerStatus string
		err := server.db.QueryRow(
			"SELECT status FROM users WHERE id = ? AND type = 'sp' AND deleted_at IS NULL",
			spID.Int64,
		).Scan(&resellerStatus)
		if err == sql.ErrNoRows || err != nil || resellerStatus != "approved" {
			return nil, nil
		}
		value := spID.Int64
		plan.SPID = &value
	}

	return &plan, nil
}

func (server *Server) register(context fiber.Ctx) error {
	// Old page uses rateLimit(5): five registration attempts per minute per IP.
	limiterKey := "register|" + context.IP()
	if locked, retryAfter := server.loginLimiter.Check(limiterKey); locked {
		return context.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"message": fmt.Sprintf("Too many registration attempts. Please try again in %d seconds.", retryAfter),
		})
	}
	server.loginLimiter.RecordFailure(limiterKey)

	request, err := parseRegisterForm(context)
	if err != nil {
		return context.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The supplied data is invalid."})
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.Phone = strings.TrimSpace(request.Phone)
	request.NIDNumber = strings.TrimSpace(request.NIDNumber)
	request.BusinessName = strings.TrimSpace(request.BusinessName)
	request.TradeLicenseNumber = strings.TrimSpace(request.TradeLicenseNumber)
	request.BusinessAddress = strings.TrimSpace(request.BusinessAddress)
	request.PlanSlug = strings.TrimSpace(request.PlanSlug)

	if request.Name == "" {
		return registerValidationError(context, "name", "Your full name is required. Please enter your first and last name to proceed.")
	}
	if len(request.Name) > 255 {
		return registerValidationError(context, "name", "The full name may not be greater than 255 characters.")
	}
	if request.Email == "" {
		return registerValidationError(context, "email", "An email address is required. Your OTP verification code will be sent to this email.")
	}
	if !isValidEmail(request.Email) {
		return registerValidationError(context, "email", "Please enter a valid email address (e.g. you@example.com).")
	}
	if validation.IsDisposable(request.Email) {
		return registerValidationError(context, "email", "The email domain is not allowed. Please use a permanent email address.")
	}
	if len(request.Password) < 8 {
		return registerValidationError(context, "password", "Password must be at least 8 characters.")
	}
	if request.Phone == "" {
		return registerValidationError(context, "phone", "A mobile number is required. Your SMS OTP verification code will be sent to this number.")
	}

	normalizedPhone, phoneOK := batch.FormatByRegex(request.Phone)
	if !phoneOK {
		return registerValidationError(context, "phone", "Please enter a valid Bangladesh mobile number.")
	}
	request.Phone = normalizedPhone

	var plan *signupPlan
	if request.PlanSlug != "" {
		plan, err = server.resolveSignupPlan(request.PlanSlug)
		if err != nil {
			return databaseError(context, err)
		}
		if plan == nil {
			return registerValidationError(context, "plan_slug", "This signup link is no longer active.")
		}
	}

	// Active duplicates.
	var existingID int64
	if err := server.db.QueryRow(
		"SELECT id FROM users WHERE email = ? AND deleted_at IS NULL LIMIT 1",
		request.Email,
	).Scan(&existingID); err == nil && existingID > 0 {
		return registerValidationError(context, "email", "This email address is already registered.")
	}

	variants := phoneVariants(request.Phone)
	marks := make([]string, len(variants))
	args := make([]any, 0, len(variants))
	for i, variant := range variants {
		marks[i] = "?"
		args = append(args, variant)
	}
	if len(marks) > 0 {
		existenceQuery := "SELECT id FROM users WHERE deleted_at IS NULL AND phone IN (" + strings.Join(marks, ",") + ") LIMIT 1"
		if err := server.db.QueryRow(existenceQuery, args...).Scan(&existingID); err == nil && existingID > 0 {
			return registerValidationError(context, "phone", "This mobile number is already registered.")
		}
	}

	if request.NIDNumber != "" {
		if err := server.db.QueryRow(
			"SELECT id FROM users WHERE nid_number = ? AND deleted_at IS NULL LIMIT 1",
			request.NIDNumber,
		).Scan(&existingID); err == nil && existingID > 0 {
			return registerValidationError(context, "nid_number", "This NID Number is already registered.")
		}
	}
	if len(request.NIDNumber) > 30 {
		return registerValidationError(context, "nid_number", "The NID number may not be greater than 30 characters.")
	}

	// Mandatory OTP proof, exactly like the old Register page.
	if !server.isOTPVerified(request.Email, "email", request.EmailVerificationToken) {
		return registerValidationError(context, "email", "Please verify your email address using the Send OTP button before creating an account.")
	}
	if !server.isOTPVerified(request.Phone, "sms", request.PhoneVerificationToken) {
		return registerValidationError(context, "phone", "Please verify your mobile number using the Send OTP button before creating an account.")
	}

	if request.PasswordConfirmation != "" && request.Password != request.PasswordConfirmation {
		return registerValidationError(context, "password", "The password and confirmation password do not match. Please re-enter them.")
	}
	if request.NIDNumber == "" {
		return registerValidationError(context, "nid_number", "Your National ID (NID) number is required for KYC identity verification. Please enter your 10, 13, or 17-digit NID number.")
	}
	if request.FrontNID == nil {
		return registerValidationError(context, "front_nid", "Please upload a clear photo of the FRONT side of your NID card. This is required for KYC verification.")
	}
	if request.BackNID == nil {
		return registerValidationError(context, "back_nid", "Please upload a clear photo of the BACK side of your NID card. Both sides are required for KYC verification.")
	}
	if strings.TrimSpace(request.LivePhoto) == "" {
		return registerValidationError(context, "live_photo", "A live selfie photo is required for identity verification. Please click \"Open Webcam\" or \"Use Phone Camera\" to capture your photo.")
	}
	if request.BusinessName == "" {
		return registerValidationError(context, "business_name", "Your business or company name is required. Please enter the registered name of your business.")
	}
	if len(request.BusinessName) > 255 {
		return registerValidationError(context, "business_name", "The business name may not be greater than 255 characters.")
	}
	if request.TradeLicenseNumber == "" {
		return registerValidationError(context, "trade_license_number", "Your trade license number is required. Please enter the official number printed on your trade license document.")
	}
	if len(request.TradeLicenseNumber) > 100 {
		return registerValidationError(context, "trade_license_number", "The trade license number may not be greater than 100 characters.")
	}
	if request.BusinessAddress == "" {
		return registerValidationError(context, "business_address", "Your business address is required. Please enter the full address of your business premises.")
	}
	if len(request.BusinessAddress) > 500 {
		return registerValidationError(context, "business_address", "The business address may not be greater than 500 characters.")
	}
	if request.TradeLicenseFile == nil {
		return registerValidationError(context, "trade_license_file", "Please upload your Trade License document. Accepted formats are PDF or image (JPG, PNG). Max size 5MB.")
	}
	if !request.Terms {
		return registerValidationError(context, "terms", "You must read and agree to the Terms & Conditions and Privacy Policy before creating your account.")
	}

	frontNIDPath, err := server.saveUploadedFile(request.FrontNID, "ids", maxNIDImageBytes, false)
	if err != nil {
		return registerFileError(context, "front_nid", err, "Please upload a clear JPG/PNG photo of the FRONT side of your NID card (max 4MB).")
	}
	backNIDPath, err := server.saveUploadedFile(request.BackNID, "ids", maxNIDImageBytes, false)
	if err != nil {
		server.removeStoredFiles(frontNIDPath)
		return registerFileError(context, "back_nid", err, "Please upload a clear JPG/PNG photo of the BACK side of your NID card (max 4MB).")
	}
	tradeLicensePath, err := server.saveUploadedFile(request.TradeLicenseFile, "trade_licenses", maxTradeLicenseBytes, true)
	if err != nil {
		server.removeStoredFiles(frontNIDPath, backNIDPath)
		return registerFileError(context, "trade_license_file", err, "Please upload a PDF or image of your Trade License document (max 5MB).")
	}
	livePhotoPath, err := server.saveLivePhoto(request.LivePhoto)
	if err != nil {
		server.removeStoredFiles(frontNIDPath, backNIDPath, tradeLicensePath)
		return registerFileError(context, "live_photo", err, "A valid live camera photo is required for identity verification.")
	}

	cleanup := true
	defer func() {
		if cleanup {
			server.removeStoredFiles(frontNIDPath, backNIDPath, tradeLicensePath, livePhotoPath)
		}
	}()

	// Force-delete soft-deleted accounts that clash with unique email/phone/NID,
	// exactly like the old Register page, so the same person can sign up again.
	purgeMarks := make([]string, len(variants))
	purgeArgs := make([]any, 0, len(variants)+2)
	purgeArgs = append(purgeArgs, request.Email)
	for i, variant := range variants {
		purgeMarks[i] = "?"
		purgeArgs = append(purgeArgs, variant)
	}
	purgeArgs = append(purgeArgs, request.NIDNumber)
	purgeQuery := "DELETE FROM users WHERE deleted_at IS NOT NULL AND (email = ? OR phone IN (" + strings.Join(purgeMarks, ",") + ") OR nid_number = ?)"
	if len(purgeMarks) == 0 {
		purgeQuery = "DELETE FROM users WHERE deleted_at IS NOT NULL AND (email = ? OR nid_number = ?)"
		purgeArgs = []any{request.Email, request.NIDNumber}
	}
	if _, err := server.db.Exec(purgeQuery, purgeArgs...); err != nil {
		return databaseError(context, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), 12)
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Unable to create account."})
	}

	now := time.Now()
	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(context, err)
	}
	defer tx.Rollback()

	var spID any
	if plan != nil && plan.SPID != nil {
		spID = *plan.SPID
	}

	result, err := tx.Exec(`
		INSERT INTO users (
			sp_id, name, email, phone, password, type, status, kyc_status,
			nid_number, front_nid, back_nid, live_photo,
			business_name, business_address, trade_license_number, trade_license_file,
			email_verified_at, phone_verified_at, kyc_submitted_at, trade_license_submitted,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'customer', 'pending', 'under_review',
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, 1,
			?, ?)
	`,
		spID, request.Name, request.Email, request.Phone, string(hash),
		request.NIDNumber, frontNIDPath, backNIDPath, livePhotoPath,
		request.BusinessName, request.BusinessAddress, request.TradeLicenseNumber, tradeLicensePath,
		now, now, now,
		now, now,
	)
	if err != nil {
		return databaseError(context, err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return databaseError(context, err)
	}

	if plan != nil {
		notes := "Auto-assigned via signup link: /register/" + plan.CanonicalSlug
		if _, err := tx.Exec(`
			INSERT INTO tariff_plan_assignments
				(user_id, tariff_plan_id, assigned_by, notes, status, effective_from, created_at, updated_at)
			VALUES (?, ?, NULL, ?, 'active', CURDATE(), NOW(), NOW())
		`, userID, plan.ID, notes); err != nil {
			return databaseError(context, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return databaseError(context, err)
	}
	cleanup = false

	ttl := server.config.TokenTTL
	token, err := server.tokens.Issue(userID, request.Name, request.Email, time.Now().Add(ttl))
	if err != nil {
		return context.Status(http.StatusInternalServerError).JSON(fiber.Map{"message": "Account created but unable to sign in."})
	}

	context.Cookie(&fiber.Cookie{
		Name:     "customer_token",
		Value:    token,
		Expires:  time.Now().Add(ttl),
		HTTPOnly: false,
		SameSite: "Lax",
		Path:     "/",
	})

	user := customerUser{
		ID:        userID,
		Name:      request.Name,
		Email:     request.Email,
		Phone:     &request.Phone,
		Balance:   "0.00",
		Status:    "pending",
		KycStatus: "under_review",
		CreatedAt: now.Format(time.RFC3339),
	}
	if plan != nil && plan.SPID != nil {
		user.SpID = plan.SPID
	}

	return context.Status(http.StatusCreated).JSON(fiber.Map{
		"message": "Account created. Your KYC is under review.",
		"token":   token,
		"data": fiber.Map{
			"token": token,
			"user":  user,
		},
	})
}

func registerFileError(context fiber.Ctx, field string, err error, message string) error {
	if err == errFileTooLarge {
		return registerValidationError(context, field, message)
	}

	return registerValidationError(context, field, message)
}
