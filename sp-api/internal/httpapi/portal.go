package httpapi

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

// spActor returns the owning SP id, the signed-in user's id and owner flag.
func spActor(ctx fiber.Ctx) (int64, int64, bool) {
	owner := sp(ctx).SPID
	userID, _ := ctx.Locals("sp_user_id").(int64)
	isOwner, _ := ctx.Locals("sp_is_owner").(bool)

	return owner, userID, isOwner
}

func spNotify(server *Server, userIDs []int64, title, body, status, url string) {
	payload, _ := json.Marshal(map[string]any{
		"title":  title,
		"body":   body,
		"status": status,
		"url":    url,
		"color":  status,
		"icon":   "heroicon-o-bell",
	})
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		_, _ = server.db.Exec(`
			INSERT INTO notifications (id, type, notifiable_type, notifiable_id, data, created_at, updated_at)
			VALUES (UUID(), 'App\\Notifications\\SystemNotification', 'App\\Models\\User', ?, ?, NOW(), NOW())
		`, userID, string(payload))
	}
}

func (server *Server) spCustomerIDs(owner int64, ids []int64) []int64 {
	result := []int64{}
	for _, id := range ids {
		var exists int
		if server.db.QueryRow(
			"SELECT COUNT(*) FROM users WHERE id = ? AND sp_id = ? AND type = 'customer' AND deleted_at IS NULL",
			id, owner,
		).Scan(&exists) == nil && exists == 1 {
			result = append(result, id)
		}
	}

	return result
}

// ─── Customers ───────────────────────────────────────────────────────────────

type spCustomerInput struct {
	Name       string  `json:"name"`
	Email      string  `json:"email"`
	Phone      string  `json:"phone"`
	Password   string  `json:"password"`
	Status     string  `json:"status"`
	TariffPlan *int64  `json:"tariff_plan_id"`
	Balance    float64 `json:"balance"`
}

func (server *Server) assignTariffPlan(tx *sql.Tx, owner, userID int64, planID *int64) error {
	if planID == nil || *planID == 0 {
		_, err := tx.Exec(
			"UPDATE tariff_plan_assignments SET status = 'superseded', updated_at = NOW() WHERE user_id = ? AND status = 'active'",
			userID,
		)
		return err
	}

	var exists int
	if err := tx.QueryRow(
		"SELECT COUNT(*) FROM tariff_plans WHERE id = ? AND sp_id = ?",
		*planID, owner,
	).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("selected tariff plan is not available")
	}

	if _, err := tx.Exec(
		"UPDATE tariff_plan_assignments SET status = 'superseded', updated_at = NOW() WHERE user_id = ? AND status = 'active'",
		userID,
	); err != nil {
		return err
	}

	_, err := tx.Exec(`
		INSERT INTO tariff_plan_assignments (user_id, tariff_plan_id, assigned_by, effective_from, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, CURDATE(), 'active', 'Assigned via SP panel', NOW(), NOW())
	`, userID, *planID, owner)

	return err
}

func (server *Server) customerDetail(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var item spCustomer
	var createdAt time.Time
	err := server.db.QueryRow(`
		SELECT u.id, u.name, u.email, COALESCE(u.phone, ''), u.balance,
		       COALESCE(u.kyc_status, 'pending'), u.status, u.created_at,
		       COALESCE(p.name, ''), COALESCE(u.nid_number, ''), COALESCE(u.front_nid, ''),
		       COALESCE(u.back_nid, ''), COALESCE(u.live_photo, ''),
		       COALESCE(u.business_name, ''), COALESCE(u.trade_license_number, ''),
		       COALESCE(u.trade_license_file, ''), COALESCE(u.kyc_rejected_reason, ''),
		       u.kyc_resubmit_fields, u.deleted_at
		FROM users u
		LEFT JOIN tariff_plan_assignments a ON a.user_id = u.id AND a.status = 'active'
		LEFT JOIN tariff_plans p ON p.id = a.tariff_plan_id
		WHERE u.id = ? AND u.sp_id = ? AND u.type = 'customer'
		LIMIT 1
	`, id, owner).Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.Balance,
		&item.KYCStatus, &item.Status, &createdAt, &item.PlanName, &item.NIDNumber,
		&item.FrontNID, &item.BackNID, &item.LivePhoto, &item.BusinessName,
		&item.TradeLicenseNumber, &item.TradeLicenseFile, &item.KYCRejectedReason,
		&item.KYCResubmitFields, &item.DeletedAt)
	if err == sql.ErrNoRows {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Customer not found."})
	}
	if err != nil {
		return databaseError(ctx, err)
	}
	item.CreatedAt = createdAt.Format("2006-01-02 15:04")

	return ctx.JSON(fiber.Map{"data": item})
}

func (server *Server) createCustomer(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)

	var input spCustomerInput
	if err := json.Unmarshal(ctx.Body(), &input); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Phone = strings.TrimSpace(input.Phone)
	if input.Name == "" || input.Email == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name and email are required."})
	}
	if len(input.Password) < 8 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters."})
	}
	if input.Status == "" {
		input.Status = "pending"
	}

	var existing int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND deleted_at IS NULL", input.Email).Scan(&existing)
	if existing > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The email has already been taken."})
	}
	if input.Phone != "" {
		_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ? AND deleted_at IS NULL", input.Phone).Scan(&existing)
		if existing > 0 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The phone has already been taken."})
		}
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return databaseError(ctx, err)
	}

	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(ctx, err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO users (sp_id, name, email, phone, password, type, status, kyc_status, balance, created_by_admin, created_by_sp, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, 'customer', ?, 'pending', ?, 0, 1, NOW(), NOW())
	`, owner, input.Name, input.Email, input.Phone, string(hash), input.Status, input.Balance)
	if err != nil {
		return databaseError(ctx, err)
	}
	customerID, _ := result.LastInsertId()

	if err := server.assignTariffPlan(tx, owner, customerID, input.TariffPlan); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": err.Error()})
	}
	if err := tx.Commit(); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.Status(http.StatusCreated).JSON(fiber.Map{"data": fiber.Map{"id": customerID}, "message": "Customer created."})
}

func (server *Server) updateCustomer(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var input spCustomerInput
	if err := json.Unmarshal(ctx.Body(), &input); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}

	var exists int
	if server.db.QueryRow(
		"SELECT COUNT(*) FROM users WHERE id = ? AND sp_id = ? AND type = 'customer' AND deleted_at IS NULL",
		id, owner,
	).Scan(&exists); exists == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Customer not found."})
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Name == "" || input.Email == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name and email are required."})
	}

	var duplicate int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id <> ? AND deleted_at IS NULL", input.Email, id).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The email has already been taken."})
	}
	if input.Phone != "" {
		_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ? AND id <> ? AND deleted_at IS NULL", input.Phone, id).Scan(&duplicate)
		if duplicate > 0 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The phone has already been taken."})
		}
	}

	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(ctx, err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE users SET name = ?, email = ?, phone = NULLIF(?, ''), status = ?, updated_at = NOW()
		WHERE id = ? AND sp_id = ?
	`, input.Name, input.Email, input.Phone, input.Status, id, owner); err != nil {
		return databaseError(ctx, err)
	}
	if input.Password != "" {
		if len(input.Password) < 8 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters."})
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
		if _, err := tx.Exec("UPDATE users SET password = ? WHERE id = ?", string(hash), id); err != nil {
			return databaseError(ctx, err)
		}
	}
	if err := server.assignTariffPlan(tx, owner, id, input.TariffPlan); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": err.Error()})
	}
	if err := tx.Commit(); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"message": "Customer updated."})
}

func (server *Server) deleteCustomer(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	result, err := server.db.Exec(
		"UPDATE users SET deleted_at = NOW() WHERE id = ? AND sp_id = ? AND type = 'customer' AND deleted_at IS NULL",
		id, owner,
	)
	if err != nil {
		return databaseError(ctx, err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Customer not found."})
	}

	return ctx.JSON(fiber.Map{"message": "Customer deleted."})
}

func (server *Server) restoreCustomer(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	if _, err := server.db.Exec(
		"UPDATE users SET deleted_at = NULL WHERE id = ? AND sp_id = ? AND type = 'customer'",
		id, owner,
	); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"message": "Customer restored."})
}

func (server *Server) exportCustomers(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	rows, err := server.db.Query(`
		SELECT name, email, COALESCE(phone, ''), balance, status, COALESCE(kyc_status, 'pending'), created_at
		FROM users WHERE sp_id = ? AND type = 'customer' AND deleted_at IS NULL ORDER BY id
	`, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	var builder strings.Builder
	builder.WriteString("Name,Email,Phone,Balance,Status,KYC Status,Created At\n")
	for rows.Next() {
		var name, email, phone, status, kyc string
		var balance float64
		var createdAt time.Time
		if rows.Scan(&name, &email, &phone, &balance, &status, &kyc, &createdAt) != nil {
			continue
		}
		builder.WriteString(fmt.Sprintf("\"%s\",\"%s\",\"%s\",%.4f,\"%s\",\"%s\",\"%s\"\n",
			strings.ReplaceAll(name, "\"", "\"\""), strings.ReplaceAll(email, "\"", "\"\""),
			strings.ReplaceAll(phone, "\"", "\"\""), balance, status, kyc, createdAt.Format("2006-01-02 15:04")))
	}

	ctx.Set("Content-Type", "text/csv; charset=utf-8")
	ctx.Set("Content-Disposition", "attachment; filename=customers.csv")

	return ctx.SendString(builder.String())
}

// ─── KYC ─────────────────────────────────────────────────────────────────────

type spKYCItem struct {
	ID                 int64   `json:"id"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	Phone              string  `json:"phone"`
	NIDNumber          string  `json:"nid_number"`
	FrontNID           string  `json:"front_nid"`
	BackNID            string  `json:"back_nid"`
	LivePhoto          string  `json:"live_photo"`
	BusinessName       string  `json:"business_name"`
	TradeLicenseNumber string  `json:"trade_license_number"`
	TradeLicenseFile   string  `json:"trade_license_file"`
	KYCStatus          string  `json:"kyc_status"`
	SubmittedAt        *string `json:"kyc_submitted_at"`
	SPVerifiedAt       *string `json:"kyc_sp_verified_at"`
	RejectedReason     string  `json:"kyc_rejected_reason"`
	ResubmitFields     any     `json:"kyc_resubmit_fields"`
}

func (server *Server) kycList(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	status := ctx.Query("status", "awaiting_sp_verification")

	args := []any{owner}
	where := "u.sp_id = ? AND u.type = 'customer' AND u.created_by_sp = 1 AND u.deleted_at IS NULL"
	if status != "all" {
		where += " AND u.kyc_status = ?"
		args = append(args, status)
	}

	rows, err := server.db.Query(`
		SELECT u.id, u.name, u.email, COALESCE(u.phone, ''), COALESCE(u.nid_number, ''),
		       COALESCE(u.front_nid, ''), COALESCE(u.back_nid, ''), COALESCE(u.live_photo, ''),
		       COALESCE(u.business_name, ''), COALESCE(u.trade_license_number, ''),
		       COALESCE(u.trade_license_file, ''), COALESCE(u.kyc_status, 'pending'),
		       u.kyc_submitted_at, u.kyc_sp_verified_at, COALESCE(u.kyc_rejected_reason, ''),
		       u.kyc_resubmit_fields
		FROM users u WHERE `+where+`
		ORDER BY (u.kyc_status = 'awaiting_sp_verification') DESC, u.kyc_submitted_at DESC, u.id DESC
	`, args...)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []spKYCItem{}
	for rows.Next() {
		var item spKYCItem
		var submitted, verified *time.Time
		var fields []byte
		if rows.Scan(&item.ID, &item.Name, &item.Email, &item.Phone, &item.NIDNumber,
			&item.FrontNID, &item.BackNID, &item.LivePhoto, &item.BusinessName,
			&item.TradeLicenseNumber, &item.TradeLicenseFile, &item.KYCStatus,
			&submitted, &verified, &item.RejectedReason, &fields) != nil {
			continue
		}
		item.SubmittedAt = apiTimePtrValue(submitted)
		item.SPVerifiedAt = apiTimePtrValue(verified)
		if len(fields) > 0 {
			_ = json.Unmarshal(fields, &item.ResubmitFields)
		}
		items = append(items, item)
	}

	return ctx.JSON(fiber.Map{"data": items})
}

func apiTimePtrValue(value *time.Time) *string {
	if value == nil {
		return nil
	}

	formatted := value.Format(time.RFC3339)

	return &formatted
}

func (server *Server) kycVerify(ctx fiber.Ctx) error {
	owner, userID, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var name, status string
	err := server.db.QueryRow(`
		SELECT name, COALESCE(kyc_status, 'pending') FROM users
		WHERE id = ? AND sp_id = ? AND type = 'customer' AND created_by_sp = 1 AND deleted_at IS NULL
	`, id, owner).Scan(&name, &status)
	if err == sql.ErrNoRows {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Customer not found."})
	}
	if err != nil {
		return databaseError(ctx, err)
	}
	if status != "awaiting_sp_verification" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "This KYC is not awaiting SP verification."})
	}

	if _, err := server.db.Exec(`
		UPDATE users SET kyc_status = 'under_review', kyc_sp_verified_by = ?, kyc_sp_verified_at = NOW(),
		       kyc_rejected_reason = NULL, kyc_resubmit_fields = NULL, updated_at = NOW()
		WHERE id = ? AND sp_id = ?
	`, userID, id, owner); err != nil {
		return databaseError(ctx, err)
	}

	spNotify(server, []int64{id}, "KYC Verified by Your Reseller",
		"Your KYC documents were verified by your reseller and forwarded to the admin for final approval.",
		"success", "/customer/kyc")

	var adminIDs []int64
	adminRows, _ := server.db.Query("SELECT id FROM users WHERE type = 'superman' AND deleted_at IS NULL")
	if adminRows != nil {
		defer adminRows.Close()
		for adminRows.Next() {
			var adminID int64
			if adminRows.Scan(&adminID) == nil {
				adminIDs = append(adminIDs, adminID)
			}
		}
	}
	spNotify(server, adminIDs, "KYC Awaiting Your Approval",
		fmt.Sprintf("Reseller verified the KYC of customer %s. It is ready for your review.", name),
		"warning", "/superman/kyc")

	return ctx.JSON(fiber.Map{"message": "KYC verified and forwarded to the admin."})
}

func (server *Server) kycReject(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var payload struct {
		Fields []string `json:"fields"`
		Reason string   `json:"reason"`
	}
	if err := json.Unmarshal(ctx.Body(), &payload); err != nil || len(payload.Fields) == 0 || strings.TrimSpace(payload.Reason) == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Select the fields to correct and provide a reason."})
	}

	var name, status string
	err := server.db.QueryRow(`
		SELECT name, COALESCE(kyc_status, 'pending') FROM users
		WHERE id = ? AND sp_id = ? AND type = 'customer' AND created_by_sp = 1 AND deleted_at IS NULL
	`, id, owner).Scan(&name, &status)
	if err == sql.ErrNoRows {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Customer not found."})
	}
	if err != nil {
		return databaseError(ctx, err)
	}
	if status != "awaiting_sp_verification" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "This KYC is not awaiting SP verification."})
	}

	fields, _ := json.Marshal(payload.Fields)
	if _, err := server.db.Exec(`
		UPDATE users SET kyc_status = 'resubmit_required', kyc_rejected_reason = ?, kyc_resubmit_fields = ?,
		       kyc_sp_verified_by = NULL, kyc_sp_verified_at = NULL, updated_at = NOW()
		WHERE id = ? AND sp_id = ?
	`, strings.TrimSpace(payload.Reason), string(fields), id, owner); err != nil {
		return databaseError(ctx, err)
	}

	spNotify(server, []int64{id}, "KYC Resubmission Required",
		"Your reseller found a problem with your KYC. Please resubmit it: "+strings.TrimSpace(payload.Reason),
		"warning", "/customer/kyc")

	return ctx.JSON(fiber.Map{"message": "KYC sent back for resubmission."})
}

// ─── Employees ───────────────────────────────────────────────────────────────

type spEmployeeInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Password string `json:"password"`
	Status   string `json:"status"`
	RoleID   *int64 `json:"sp_role_id"`
}

func (server *Server) employees(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	rows, err := server.db.Query(`
		SELECT u.id, u.name, u.email, COALESCE(u.phone, ''), COALESCE(u.sp_role_id, 0),
		       COALESCE(r.name, ''), u.status, u.created_at
		FROM users u LEFT JOIN sp_roles r ON r.id = u.sp_role_id
		WHERE u.sp_id = ? AND u.type = 'sp' AND u.deleted_at IS NULL
		ORDER BY u.created_at DESC
	`, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, roleID int64
		var name, email, phone, roleName, status string
		var createdAt time.Time
		if rows.Scan(&id, &name, &email, &phone, &roleID, &roleName, &status, &createdAt) != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id": id, "name": name, "email": email, "phone": phone,
			"sp_role_id": roleID, "role": roleName, "status": status,
			"created_at": createdAt.Format("2006-01-02 15:04"),
		})
	}

	return ctx.JSON(fiber.Map{"data": items})
}

func (server *Server) createEmployee(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)

	var input spEmployeeInput
	if err := json.Unmarshal(ctx.Body(), &input); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Name == "" || input.Email == "" || len(input.Password) < 8 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name, email and a password of at least 8 characters are required."})
	}
	if input.Status == "" {
		input.Status = "approved"
	}

	var duplicate int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND deleted_at IS NULL", input.Email).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The email has already been taken."})
	}
	if input.Phone != "" {
		_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE phone = ? AND deleted_at IS NULL", input.Phone).Scan(&duplicate)
		if duplicate > 0 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The phone has already been taken."})
		}
	}

	var roleID any
	if input.RoleID != nil && *input.RoleID > 0 {
		var exists int
		_ = server.db.QueryRow("SELECT COUNT(*) FROM sp_roles WHERE id = ? AND scope = ?", *input.RoleID, strconv.FormatInt(owner, 10)).Scan(&exists)
		if exists == 0 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Selected role is not available."})
		}
		roleID = *input.RoleID
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	result, err := server.db.Exec(`
		INSERT INTO users (sp_id, sp_role_id, name, email, phone, password, type, status, kyc_status, balance, created_by_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, 'sp', ?, 'pending', 0, 0, NOW(), NOW())
	`, owner, roleID, input.Name, input.Email, input.Phone, string(hash), input.Status)
	if err != nil {
		return databaseError(ctx, err)
	}
	id, _ := result.LastInsertId()

	return ctx.Status(http.StatusCreated).JSON(fiber.Map{"data": fiber.Map{"id": id}, "message": "Employee created."})
}

func (server *Server) updateEmployee(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var input spEmployeeInput
	if err := json.Unmarshal(ctx.Body(), &input); err != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Invalid request body."})
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Name == "" || input.Email == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name and email are required."})
	}

	var duplicate int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE email = ? AND id <> ? AND deleted_at IS NULL", input.Email, id).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The email has already been taken."})
	}

	var roleID any
	if input.RoleID != nil && *input.RoleID > 0 {
		var exists int
		_ = server.db.QueryRow("SELECT COUNT(*) FROM sp_roles WHERE id = ? AND scope = ?", *input.RoleID, strconv.FormatInt(owner, 10)).Scan(&exists)
		if exists == 0 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Selected role is not available."})
		}
		roleID = *input.RoleID
	}

	result, err := server.db.Exec(`
		UPDATE users SET name = ?, email = ?, phone = NULLIF(?, ''), sp_role_id = ?, status = ?, updated_at = NOW()
		WHERE id = ? AND sp_id = ? AND type = 'sp' AND deleted_at IS NULL
	`, input.Name, input.Email, input.Phone, roleID, input.Status, id, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Employee not found."})
	}

	if input.Password != "" {
		if len(input.Password) < 8 {
			return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Password must be at least 8 characters."})
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
		_, _ = server.db.Exec("UPDATE users SET password = ? WHERE id = ? AND sp_id = ?", string(hash), id, owner)
	}

	return ctx.JSON(fiber.Map{"message": "Employee updated."})
}

func (server *Server) deleteEmployee(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	result, err := server.db.Exec(
		"UPDATE users SET deleted_at = NOW() WHERE id = ? AND sp_id = ? AND type = 'sp' AND deleted_at IS NULL",
		id, owner,
	)
	if err != nil {
		return databaseError(ctx, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Employee not found."})
	}

	return ctx.JSON(fiber.Map{"message": "Employee deleted."})
}

// ─── Roles ───────────────────────────────────────────────────────────────────

func (server *Server) roles(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	rows, err := server.db.Query(`
		SELECT r.id, r.name, COALESCE(r.description, ''), r.permissions,
		       (SELECT COUNT(*) FROM users u WHERE u.sp_role_id = r.id AND u.deleted_at IS NULL), r.created_at
		FROM sp_roles r WHERE r.scope = ? ORDER BY r.created_at DESC
	`, strconv.FormatInt(owner, 10))
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, users int
		var name, description string
		var permissions []byte
		var createdAt time.Time
		if rows.Scan(&id, &name, &description, &permissions, &users, &createdAt) != nil {
			continue
		}
		var granted []string
		_ = json.Unmarshal(permissions, &granted)
		items = append(items, fiber.Map{
			"id": id, "name": name, "description": description,
			"permissions": granted, "users_count": users,
			"created_at": createdAt.Format("2006-01-02 15:04"),
		})
	}

	return ctx.JSON(fiber.Map{"data": items})
}

func (server *Server) saveRole(ctx fiber.Ctx) error {
	owner, userID, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var input struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(ctx.Body(), &input); err != nil || strings.TrimSpace(input.Name) == "" || len(input.Permissions) == 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Role name and at least one permission are required."})
	}
	permissions, _ := json.Marshal(input.Permissions)
	scope := strconv.FormatInt(owner, 10)

	var duplicate int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM sp_roles WHERE scope = ? AND name = ? AND id <> ?", scope, input.Name, id).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "A role with this name already exists."})
	}

	if id == 0 {
		result, err := server.db.Exec(`
			INSERT INTO sp_roles (scope, name, description, permissions, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		`, scope, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), string(permissions), userID)
		if err != nil {
			return databaseError(ctx, err)
		}
		newID, _ := result.LastInsertId()
		return ctx.Status(http.StatusCreated).JSON(fiber.Map{"data": fiber.Map{"id": newID}, "message": "Role created."})
	}

	result, err := server.db.Exec(`
		UPDATE sp_roles SET name = ?, description = ?, permissions = ?, updated_at = NOW()
		WHERE id = ? AND scope = ?
	`, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description), string(permissions), id, scope)
	if err != nil {
		return databaseError(ctx, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Role not found."})
	}

	return ctx.JSON(fiber.Map{"message": "Role updated."})
}

func (server *Server) deleteRole(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var assigned int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE sp_role_id = ? AND deleted_at IS NULL", id).Scan(&assigned)
	if assigned > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "This role is assigned to employees and cannot be deleted."})
	}

	result, err := server.db.Exec("DELETE FROM sp_roles WHERE id = ? AND scope = ?", id, strconv.FormatInt(owner, 10))
	if err != nil {
		return databaseError(ctx, err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Role not found."})
	}

	return ctx.JSON(fiber.Map{"message": "Role deleted."})
}

// ─── Senders ─────────────────────────────────────────────────────────────────

func (server *Server) spSenders(ctx fiber.Ctx) error {
	owner, userID, isOwner := spActor(ctx)
	actor := userID
	if isOwner {
		actor = owner
	}

	rows, err := server.db.Query(`
		SELECT s.id, s.sender_id, s.type, s.is_enabled, COALESCE(s.approval_status, 'pending'),
		       COALESCE(s.rejection_reason, ''), COALESCE(g.name, ''), s.created_at,
		       (SELECT COUNT(*) FROM sender_user su JOIN users u ON u.id = su.user_id
		        WHERE su.sender_id = s.id AND u.sp_id = ? AND u.type = 'customer') AS customers_count
		FROM senders s
		LEFT JOIN gateways g ON g.id = s.gateway_id
		WHERE s.deleted_at IS NULL AND (s.requested_by = ? OR EXISTS (
			SELECT 1 FROM sender_user su WHERE su.sender_id = s.id AND su.user_id = ?
		))
		ORDER BY s.created_at DESC
	`, owner, actor, actor)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id, customers int
		var senderID, senderType, approval, rejection, gateway string
		var enabled bool
		var createdAt time.Time
		if rows.Scan(&id, &senderID, &senderType, &enabled, &approval, &rejection, &gateway, &createdAt, &customers) != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id": id, "sender_id": senderID, "type": senderType, "is_enabled": enabled,
			"approval_status": approval, "rejection_reason": rejection, "gateway": gateway,
			"customers_count": customers, "created_at": createdAt.Format("2006-01-02 15:04"),
		})
	}

	return ctx.JSON(fiber.Map{"data": items})
}

func (server *Server) createSPSender(ctx fiber.Ctx) error {
	owner, userID, isOwner := spActor(ctx)
	actor := userID
	if isOwner {
		actor = owner
	}

	var input struct {
		SenderID string `json:"sender_id"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(ctx.Body(), &input); err != nil || strings.TrimSpace(input.SenderID) == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Sender ID is required."})
	}
	input.SenderID = strings.TrimSpace(input.SenderID)
	if input.Type != "masking" && input.Type != "non_masking" {
		input.Type = "non_masking"
	}

	var duplicate int
	_ = server.db.QueryRow("SELECT COUNT(*) FROM senders WHERE sender_id = ? AND deleted_at IS NULL", input.SenderID).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "This Sender ID already exists."})
	}

	result, err := server.db.Exec(`
		INSERT INTO senders (sender_id, type, is_enabled, approval_status, requested_by, created_at, updated_at)
		VALUES (?, ?, 0, 'pending', ?, NOW(), NOW())
	`, input.SenderID, input.Type, actor)
	if err != nil {
		return databaseError(ctx, err)
	}
	id, _ := result.LastInsertId()

	return ctx.Status(http.StatusCreated).JSON(fiber.Map{
		"data":    fiber.Map{"id": id},
		"message": "Sender ID submitted for Superman approval.",
	})
}

// ─── Tariff plans ────────────────────────────────────────────────────────────

func (server *Server) spTariffPlans(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	rows, err := server.db.Query(`
		SELECT p.id, p.name, p.slug, p.type, p.status, p.currency, COALESCE(p.description, ''),
		       p.created_at,
		       (SELECT COUNT(*) FROM tariff_plan_assignments a WHERE a.tariff_plan_id = p.id AND a.status = 'active')
		FROM tariff_plans p WHERE p.sp_id = ? ORDER BY p.created_at DESC
	`, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	plans := []fiber.Map{}
	for rows.Next() {
		var id, customers int
		var name, slug, planType, status, currency, description string
		var createdAt time.Time
		if rows.Scan(&id, &name, &slug, &planType, &status, &currency, &description, &createdAt, &customers) != nil {
			continue
		}
		plans = append(plans, fiber.Map{
			"id": id, "name": name, "slug": slug, "type": planType, "status": status,
			"currency": currency, "description": description, "customers_count": customers,
			"created_at": createdAt.Format("2006-01-02 15:04"),
		})
	}

	return ctx.JSON(fiber.Map{"data": plans})
}

func (server *Server) spTariffPlanDetail(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var plan fiber.Map
	var planID int64
	var name, slug, planType, status, currency, description string
	err := server.db.QueryRow(`
		SELECT id, name, slug, type, status, currency, COALESCE(description, '')
		FROM tariff_plans WHERE id = ? AND sp_id = ?
	`, id, owner).Scan(&planID, &name, &slug, &planType, &status, &currency, &description)
	if err == sql.ErrNoRows {
		return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Tariff plan not found."})
	}
	if err != nil {
		return databaseError(ctx, err)
	}
	plan = fiber.Map{"id": planID, "name": name, "slug": slug, "type": planType, "status": status, "currency": currency, "description": description}

	rows, err := server.db.Query(`
		SELECT r.operator_id, COALESCE(o.name, ''), r.masking_price, r.non_masking_price
		FROM tariff_rates r LEFT JOIN operators o ON o.id = r.operator_id
		WHERE r.tariff_plan_id = ? ORDER BY o.name
	`, id)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	rates := []fiber.Map{}
	for rows.Next() {
		var operatorID int64
		var operatorName string
		var masking, nonMasking float64
		if rows.Scan(&operatorID, &operatorName, &masking, &nonMasking) == nil {
			rates = append(rates, fiber.Map{
				"operator_id": operatorID, "operator": operatorName,
				"masking_price": masking, "non_masking_price": nonMasking,
			})
		}
	}
	plan["rates"] = rates

	return ctx.JSON(fiber.Map{"data": plan})
}

func (server *Server) saveSPTariffPlan(ctx fiber.Ctx) error {
	owner, userID, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	var input struct {
		Name        string `json:"name"`
		Slug        string `json:"slug"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		Currency    string `json:"currency"`
		Description string `json:"description"`
		Rates       []struct {
			OperatorID      int64   `json:"operator_id"`
			MaskingPrice    float64 `json:"masking_price"`
			NonMaskingPrice float64 `json:"non_masking_price"`
		} `json:"rates"`
	}
	if err := json.Unmarshal(ctx.Body(), &input); err != nil || strings.TrimSpace(input.Name) == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Plan name is required."})
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.Type == "" {
		input.Type = "standard"
	}
	if input.Currency == "" {
		input.Currency = "BDT"
	}
	if strings.TrimSpace(input.Slug) == "" {
		input.Slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(input.Name), " ", "-"))
	}

	tx, err := server.db.Begin()
	if err != nil {
		return databaseError(ctx, err)
	}
	defer tx.Rollback()

	var duplicate int
	_ = tx.QueryRow("SELECT COUNT(*) FROM tariff_plans WHERE slug = ? AND id <> ?", input.Slug, id).Scan(&duplicate)
	if duplicate > 0 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "The plan slug has already been taken."})
	}

	if id == 0 {
		result, err := tx.Exec(`
			INSERT INTO tariff_plans (name, slug, type, status, currency, description, created_by, sp_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
		`, input.Name, input.Slug, input.Type, input.Status, input.Currency, input.Description, userID, owner)
		if err != nil {
			return databaseError(ctx, err)
		}
		id, _ = result.LastInsertId()
	} else {
		result, err := tx.Exec(`
			UPDATE tariff_plans SET name = ?, slug = ?, type = ?, status = ?, currency = ?, description = ?, updated_at = NOW()
			WHERE id = ? AND sp_id = ?
		`, input.Name, input.Slug, input.Type, input.Status, input.Currency, input.Description, id, owner)
		if err != nil {
			return databaseError(ctx, err)
		}
		if affected, _ := result.RowsAffected(); affected == 0 {
			return ctx.Status(http.StatusNotFound).JSON(fiber.Map{"message": "Tariff plan not found."})
		}
	}

	for _, rate := range input.Rates {
		if rate.OperatorID == 0 {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO tariff_rates (tariff_plan_id, operator_id, masking_price, non_masking_price, cost, effective_from, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, NOW(), NOW(), NOW())
			ON DUPLICATE KEY UPDATE masking_price = VALUES(masking_price), non_masking_price = VALUES(non_masking_price), updated_at = NOW()
		`, id, rate.OperatorID, rate.MaskingPrice, rate.NonMaskingPrice); err != nil {
			return databaseError(ctx, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"data": fiber.Map{"id": id}, "message": "Tariff plan saved."})
}

func (server *Server) archiveSPTariffPlan(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	id, _ := strconv.ParseInt(ctx.Params("id"), 10, 64)

	if _, err := server.db.Exec(
		"UPDATE tariff_plans SET status = 'archived', updated_at = NOW() WHERE id = ? AND sp_id = ?",
		id, owner,
	); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"message": "Plan archived."})
}

func (server *Server) spOperators(ctx fiber.Ctx) error {
	rows, err := server.db.Query("SELECT id, name FROM operators ORDER BY name")
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int64
		var name string
		if rows.Scan(&id, &name) == nil {
			items = append(items, fiber.Map{"id": id, "name": name})
		}
	}

	return ctx.JSON(fiber.Map{"data": items})
}

// ─── Deposits and reports ────────────────────────────────────────────────────

func (server *Server) spDeposits(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	page, perPage := pagination(ctx)

	var total int
	_ = server.db.QueryRow(`
		SELECT COUNT(*) FROM deposits d INNER JOIN users u ON u.id = d.user_id
		WHERE u.sp_id = ? AND u.type = 'customer'
	`, owner).Scan(&total)

	rows, err := server.db.Query(`
		SELECT d.id, u.name, u.email, d.amount, d.currency, COALESCE(d.payment_method, ''),
		       COALESCE(d.transaction_id, ''), d.status, d.paid_at, d.created_at
		FROM deposits d INNER JOIN users u ON u.id = d.user_id
		WHERE u.sp_id = ? AND u.type = 'customer'
		ORDER BY d.created_at DESC LIMIT ? OFFSET ?
	`, owner, perPage, (page-1)*perPage)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()

	items := []fiber.Map{}
	for rows.Next() {
		var id int64
		var customerName, email, currency, method, transactionID, status string
		var amount float64
		var paidAt *time.Time
		var createdAt time.Time
		if rows.Scan(&id, &customerName, &email, &amount, &currency, &method, &transactionID, &status, &paidAt, &createdAt) != nil {
			continue
		}
		items = append(items, fiber.Map{
			"id": id, "customer": customerName, "email": email, "amount": amount,
			"currency": currency, "payment_method": method, "transaction_id": transactionID,
			"status": status, "paid_at": apiTimePtrValue(paidAt), "created_at": createdAt.Format("2006-01-02 15:04"),
		})
	}

	return ctx.JSON(fiber.Map{"data": items, "meta": listMeta{Page: page, PerPage: perPage, Total: total}})
}

func (server *Server) spReports(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)
	from := ctx.Query("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := ctx.Query("to", time.Now().Format("2006-01-02"))

	stats := fiber.Map{
		"balance": 0.0, "total_sent": 0, "total_delivered": 0, "total_failed": 0,
		"customer_spend": 0.0, "distributed": 0.0, "customers": 0,
	}
	var balance, cost, distributed float64
	var totalSent, delivered, failed, customers int
	_ = server.db.QueryRow("SELECT balance FROM users WHERE id = ? AND type = 'sp'", owner).Scan(&balance)
	_ = server.db.QueryRow("SELECT COUNT(*) FROM users WHERE sp_id = ? AND type = 'customer' AND deleted_at IS NULL", owner).Scan(&customers)
	_ = server.db.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN m.status IN ('delivered','sent') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status IN ('failed','dlr_failed') THEN 1 ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN m.status != 'failed' THEN m.cost ELSE 0 END), 0)
		FROM messages m INNER JOIN users u ON u.id = m.user_id
		WHERE u.sp_id = ? AND u.type = 'customer' AND DATE(m.created_at) BETWEEN ? AND ?
	`, owner, from, to).Scan(&totalSent, &delivered, &failed, &cost)
	_ = server.db.QueryRow(`
		SELECT COALESCE(SUM(d.amount), 0) FROM sp_distributions d
		WHERE d.sp_id = ? AND DATE(d.created_at) BETWEEN ? AND ?
	`, owner, from, to).Scan(&distributed)
	stats["balance"] = balance
	stats["total_sent"] = totalSent
	stats["total_delivered"] = delivered
	stats["total_failed"] = failed
	stats["customer_spend"] = cost
	stats["distributed"] = distributed
	stats["customers"] = customers

	daily := []fiber.Map{}
	rows, err := server.db.Query(`
		SELECT DATE(m.created_at) AS day, COUNT(*) AS total,
		       COALESCE(SUM(CASE WHEN m.status IN ('delivered','sent') THEN 1 ELSE 0 END), 0) AS delivered,
		       COALESCE(SUM(m.cost), 0) AS cost
		FROM messages m INNER JOIN users u ON u.id = m.user_id
		WHERE u.sp_id = ? AND u.type = 'customer' AND DATE(m.created_at) BETWEEN ? AND ?
		GROUP BY DATE(m.created_at) ORDER BY day
	`, owner, from, to)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var day time.Time
		var total, ok int
		var dayCost float64
		if rows.Scan(&day, &total, &ok, &dayCost) == nil {
			daily = append(daily, fiber.Map{"date": day.Format("2006-01-02"), "total": total, "delivered": ok, "cost": dayCost})
		}
	}

	topCustomers := []fiber.Map{}
	customerRows, err := server.db.Query(`
		SELECT u.id, u.name, COUNT(m.id) AS total, COALESCE(SUM(m.cost), 0) AS cost
		FROM users u LEFT JOIN messages m ON m.user_id = u.id AND DATE(m.created_at) BETWEEN ? AND ?
		WHERE u.sp_id = ? AND u.type = 'customer' AND u.deleted_at IS NULL
		GROUP BY u.id, u.name ORDER BY total DESC LIMIT 10
	`, from, to, owner)
	if err == nil {
		defer customerRows.Close()
		for customerRows.Next() {
			var id int64
			var name string
			var total int
			var customerCost float64
			if customerRows.Scan(&id, &name, &total, &customerCost) == nil {
				topCustomers = append(topCustomers, fiber.Map{"id": id, "name": name, "total": total, "cost": customerCost})
			}
		}
	}

	return ctx.JSON(fiber.Map{
		"data": fiber.Map{
			"from": from, "to": to, "stats": stats,
			"daily": daily, "top_customers": topCustomers,
		},
	})
}

// ─── Live queue ──────────────────────────────────────────────────────────────

func (server *Server) spLiveQueue(ctx fiber.Ctx) error {
	owner, _, _ := spActor(ctx)

	counts := fiber.Map{"queued": 0, "processing": 0, "sent": 0, "failed": 0}
	rows, err := server.db.Query(`
		SELECT m.status, COUNT(*) FROM messages m
		INNER JOIN users u ON u.id = m.user_id
		WHERE u.sp_id = ? AND u.type = 'customer' AND u.deleted_at IS NULL
		  AND m.status IN ('queued', 'pending', 'processing', 'sent', 'failed', 'dlr_failed')
		GROUP BY m.status
	`, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if rows.Scan(&status, &count) != nil {
			continue
		}
		switch status {
		case "queued", "pending":
			counts["queued"] = counts["queued"].(int) + count
		case "processing":
			counts["processing"] = count
		case "sent":
			counts["sent"] = count
		case "failed", "dlr_failed":
			counts["failed"] = counts["failed"].(int) + count
		}
	}

	recent, err := server.db.Query(`
		SELECT m.id, u.name, m.to, COALESCE(m.status, ''), m.created_at
		FROM messages m INNER JOIN users u ON u.id = m.user_id
		WHERE u.sp_id = ? AND u.type = 'customer' AND u.deleted_at IS NULL
		  AND m.status IN ('queued', 'pending', 'processing')
		ORDER BY m.created_at ASC LIMIT 50
	`, owner)
	if err != nil {
		return databaseError(ctx, err)
	}
	defer recent.Close()

	items := []fiber.Map{}
	for recent.Next() {
		var id int64
		var customer, recipient, status string
		var createdAt time.Time
		if recent.Scan(&id, &customer, &recipient, &status, &createdAt) == nil {
			items = append(items, fiber.Map{
				"id": id, "customer": customer, "recipient": recipient,
				"status": status, "created_at": createdAt.Format("2006-01-02 15:04:05"),
			})
		}
	}

	return ctx.JSON(fiber.Map{"data": fiber.Map{"counts": counts, "queue": items}})
}

// ─── Profile / password ──────────────────────────────────────────────────────

func (server *Server) spUpdateProfile(ctx fiber.Ctx) error {
	_, userID, _ := spActor(ctx)

	var input struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(ctx.Body(), &input); err != nil || strings.TrimSpace(input.Name) == "" {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Name is required."})
	}

	if _, err := server.db.Exec(
		"UPDATE users SET name = ?, phone = NULLIF(?, ''), updated_at = NOW() WHERE id = ?",
		strings.TrimSpace(input.Name), strings.TrimSpace(input.Phone), userID,
	); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"message": "Profile updated."})
}

func (server *Server) spChangePassword(ctx fiber.Ctx) error {
	_, userID, _ := spActor(ctx)

	var input struct {
		CurrentPassword string `json:"current_password"`
		Password        string `json:"password"`
	}
	if err := json.Unmarshal(ctx.Body(), &input); err != nil || len(input.Password) < 8 {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "New password must be at least 8 characters."})
	}

	var hash string
	if err := server.db.QueryRow("SELECT password FROM users WHERE id = ?", userID).Scan(&hash); err != nil {
		return databaseError(ctx, err)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)) != nil {
		return ctx.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{"message": "Current password is incorrect."})
	}

	newHash, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if _, err := server.db.Exec("UPDATE users SET password = ?, updated_at = NOW() WHERE id = ?", string(newHash), userID); err != nil {
		return databaseError(ctx, err)
	}

	return ctx.JSON(fiber.Map{"message": "Password changed."})
}
