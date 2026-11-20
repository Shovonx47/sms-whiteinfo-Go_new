package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"smswhiteinfo/sp-api/internal/auth"
	"smswhiteinfo/sp-api/internal/config"
)

// Server is the SP API HTTP server.
type Server struct {
	config config.Config
	db     *sql.DB
	tokens auth.TokenManager
}

func New(cfg config.Config, database *sql.DB) *Server {
	return &Server{
		config: cfg,
		db:     database,
		tokens: auth.NewTokenManager(cfg.JWTSecret),
	}
}

func (server *Server) App() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:    "SMS Whiteinfo SP Reseller API",
		TrustProxy: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:  []string{"127.0.0.1", "::1"},
			Loopback: true,
		},
		ProxyHeader:        fiber.HeaderXForwardedFor,
		EnableIPValidation: true,
	})

	app.Use(func(c fiber.Ctx) error {
		if remoteAddr := c.RemoteAddr(); remoteAddr != nil {
			peerAddr := remoteAddr.String()
			host, _, err := net.SplitHostPort(peerAddr)
			if err != nil {
				host = peerAddr
			}
			peerIP := net.ParseIP(host)
			if (peerIP != nil && peerIP.IsLoopback()) || c.IsFromLocal() {
				if realIP := c.Get("X-Real-IP"); realIP != "" {
					c.Request().Header.Set(fiber.HeaderXForwardedFor, realIP)
				}
			}
		}
		return c.Next()
	})

	origins := []string{"http://localhost:5173", "http://127.0.0.1:5173"}
	for _, o := range strings.Split(server.config.FrontendOrigin, ",") {
		if tr := strings.TrimSpace(o); tr != "" {
			origins = append(origins, tr)
		}
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

	app.Use(server.maintenanceGuard)

	app.Get("/health", func(ctx fiber.Ctx) error {
		if err := server.db.Ping(); err != nil {
			return ctx.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"status": "unavailable"})
		}

		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1/sp")

	// Public
	api.Post("/auth/login", server.login)

	// Protected – require valid SP Bearer token
	protected := api.Group("", server.authenticate)
	protected.Post("/auth/logout", server.logout)
	protected.Get("/auth/me", server.me)

	// Dashboard stats
	protected.Get("/dashboard", server.dashboard)

	// Customers managed by this SP
	protected.Get("/customers", server.requirePermission("customers.view"), server.customers)
	protected.Get("/customers/export", server.requirePermission("customers.export"), server.exportCustomers)
	protected.Get("/customers/:id", server.requirePermission("customers.view"), server.customerDetail)
	protected.Post("/customers", server.requirePermission("customers.create"), server.createCustomer)
	protected.Put("/customers/:id", server.requirePermission("customers.edit"), server.updateCustomer)
	protected.Delete("/customers/:id", server.requirePermission("customers.delete"), server.deleteCustomer)
	protected.Post("/customers/:id/restore", server.requirePermission("customers.edit"), server.restoreCustomer)
	protected.Post("/customers/:id/fund", server.requirePermission("billing.manage"), server.fundCustomer)

	// KYC review (customers created by this SP)
	protected.Get("/kyc", server.requirePermission("customers.view"), server.kycList)
	protected.Post("/kyc/:id/verify", server.requirePermission("customers.edit"), server.kycVerify)
	protected.Post("/kyc/:id/reject", server.requirePermission("customers.edit"), server.kycReject)

	// Employees and roles
	protected.Get("/employees", server.requirePermission("employees.view"), server.employees)
	protected.Post("/employees", server.requirePermission("employees.create"), server.createEmployee)
	protected.Put("/employees/:id", server.requirePermission("employees.edit"), server.updateEmployee)
	protected.Delete("/employees/:id", server.requirePermission("employees.delete"), server.deleteEmployee)
	protected.Get("/roles", server.requirePermission("roles.view"), server.roles)
	protected.Post("/roles", server.requirePermission("roles.manage"), server.saveRole)
	protected.Put("/roles/:id", server.requirePermission("roles.manage"), server.saveRole)
	protected.Delete("/roles/:id", server.requirePermission("roles.manage"), server.deleteRole)

	// Sender IDs
	protected.Get("/senders", server.requirePermission("messaging.view"), server.spSenders)
	protected.Post("/senders", server.requirePermission("messaging.manage"), server.createSPSender)

	// Tariff plans
	protected.Get("/tariff-plans", server.requirePermission("billing.view"), server.spTariffPlans)
	protected.Get("/tariff-plans/:id", server.requirePermission("billing.view"), server.spTariffPlanDetail)
	protected.Post("/tariff-plans", server.requirePermission("billing.manage"), server.saveSPTariffPlan)
	protected.Put("/tariff-plans/:id", server.requirePermission("billing.manage"), server.saveSPTariffPlan)
	protected.Post("/tariff-plans/:id/archive", server.requirePermission("billing.manage"), server.archiveSPTariffPlan)
	protected.Get("/operators", server.requirePermission("billing.view"), server.spOperators)

	// Deposits + reports
	protected.Get("/deposits", server.requirePermission("billing.view"), server.spDeposits)
	protected.Get("/reports", server.requirePermission("reports.view"), server.spReports)

	// Profile
	protected.Put("/auth/profile", server.spUpdateProfile)
	protected.Post("/auth/change-password", server.spChangePassword)

	// Wallet / distribution ledger
	protected.Get("/wallet", server.requirePermission("billing.view"), server.wallet)

	// Billing – SP transaction ledger
	protected.Get("/billing", server.requirePermission("billing.view"), server.billing)

	// SMS traffic for customers under this SP
	protected.Get("/messages", server.requirePermission("messaging.view"), server.messages)
	protected.Get("/queue", server.requirePermission("messaging.view"), server.spLiveQueue)

	return app
}

// authenticate validates the Bearer JWT and ensures the user is of type "sp".
func (server *Server) authenticate(ctx fiber.Ctx) error {
	authorization := ctx.Get(fiber.HeaderAuthorization)
	if !strings.HasPrefix(authorization, "Bearer ") {
		return ctx.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	rawToken := strings.TrimPrefix(authorization, "Bearer ")
	claims, err := server.tokens.Parse(rawToken)
	if err != nil {
		return ctx.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	if server.isTokenRevoked(rawToken) {
		return ctx.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	// Verify the user still exists and is still of type "sp".
	// SP owners have sp_id = NULL and full access; employees inherit the owner's
	// data scope and are limited by their assigned role permissions.
	var userType string
	var ownerID, roleID sql.NullInt64
	err = server.db.QueryRow(
		"SELECT type, sp_id, sp_role_id FROM users WHERE id = ? AND deleted_at IS NULL",
		claims.SPID,
	).Scan(&userType, &ownerID, &roleID)
	if err != nil || userType != "sp" {
		return ctx.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	userID := claims.SPID
	isOwner := !ownerID.Valid
	if ownerID.Valid {
		claims.SPID = ownerID.Int64
	}

	permissions := map[string]bool{}
	if !isOwner && roleID.Valid {
		var payload []byte
		if server.db.QueryRow(
			"SELECT permissions FROM sp_roles WHERE id = ? AND scope = ? LIMIT 1",
			roleID.Int64, strconv.FormatInt(claims.SPID, 10),
		).Scan(&payload) == nil {
			var granted []string
			if json.Unmarshal(payload, &granted) == nil {
				for _, permission := range granted {
					permissions[permission] = true
				}
			}
		}
	}

	ctx.Locals("sp", claims)
	ctx.Locals("sp_token", rawToken)
	ctx.Locals("sp_user_id", userID)
	ctx.Locals("sp_is_owner", isOwner)
	ctx.Locals("sp_permissions", permissions)

	return ctx.Next()
}

func (server *Server) tokenCacheKey(token string) string {
	digest := sha256.Sum256([]byte(token))

	return server.config.CachePrefix + "sp_api_revoked_" + hex.EncodeToString(digest[:])
}

func (server *Server) isTokenRevoked(token string) bool {
	var exists bool
	err := server.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM cache WHERE `key` = ? AND expiration >= ?)",
		server.tokenCacheKey(token),
		time.Now().Unix(),
	).Scan(&exists)

	return err == nil && exists
}

// sp extracts the validated SP claims from the request context.
func sp(ctx fiber.Ctx) auth.Claims {
	return ctx.Locals("sp").(auth.Claims)
}

// requirePermission mirrors SpPermissionService: owners have every permission,
// employees only those granted by their role.
func (server *Server) requirePermission(permission string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		isOwner, _ := ctx.Locals("sp_is_owner").(bool)
		permissions, _ := ctx.Locals("sp_permissions").(map[string]bool)
		if isOwner || permissions[permission] {
			return ctx.Next()
		}

		return ctx.Status(http.StatusForbidden).JSON(fiber.Map{
			"message": "You do not have the [" + permission + "] permission.",
		})
	}
}

func pagination(ctx fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(ctx.Query("page", "1"))
	perPage, _ := strconv.Atoi(ctx.Query("per_page", "25"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 1
	}
	if perPage > 100 {
		perPage = 100
	}

	return page, perPage
}

func apiTime(value time.Time) *string {
	if value.IsZero() {
		return nil
	}

	formatted := value.Format(time.RFC3339)

	return &formatted
}
