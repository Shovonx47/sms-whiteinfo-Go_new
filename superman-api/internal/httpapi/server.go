package httpapi

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"smswhiteinfo/superman-api/internal/auth"
	"smswhiteinfo/superman-api/internal/config"
)

type Server struct {
	config config.Config
	db     *sql.DB
	tokens auth.TokenManager
}

func New(config config.Config, database *sql.DB) *Server {
	server := &Server{
		config: config,
		db:     database,
		tokens: auth.NewTokenManager(config.JWTSecret),
	}
	server.ensureSenderRouteTable()

	return server
}

func (server *Server) App() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:    "SMS Whiteinfo Superman API",
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

	app.Get("/health", func(context fiber.Ctx) error {
		if err := server.db.Ping(); err != nil {
			return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{"status": "unavailable"})
		}

		return context.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api/v1/superman")
	api.Post("/auth/login", server.login)

	protected := api.Group("", server.authenticate)
	protected.Post("/auth/logout", server.logout)
	protected.Get("/auth/me", server.me)
	protected.Get("/dashboard", server.dashboard)
	protected.Get("/topbar-stats", server.topbarStats)
	protected.Get("/customers", server.customers)
	protected.Get("/transactions", server.transactions)
	protected.Get("/messages", server.messages)
	protected.Get("/campaigns", server.campaigns)
	protected.Post("/campaigns", server.createCampaign)
	protected.Get("/alerts/dashboard", server.alertsDashboard)
	protected.Get("/alerts/providers", server.alertProviders)
	protected.Post("/alerts/providers/:id/restore", server.restoreProvider)
	protected.Get("/dlr/live", server.dlrLive)
	protected.Get("/dlr/queue", server.dlrQueue)
	protected.Get("/dlr/retry", server.dlrRetry)
	protected.Get("/dlr/logs", server.dlrLogs)
	protected.Get("/dlr/check", server.dlrCheck)
	protected.Post("/dlr/:id/recheck", server.recheckDlr)
	protected.Post("/messages/:id/requeue", server.requeueMessage)
	protected.Get("/sms/logs", server.smsAuditLogs)
	protected.Get("/sms/inquiry", server.smsInquiry)
	protected.Get("/sms/senders", server.smsSenderOptions)
	protected.Post("/sms/send", server.adminSendSMS)
	protected.Get("/batches", server.batches)
	protected.Post("/batches/:id/approve", server.approveBatch)
	protected.Post("/batches/:id/reject", server.rejectBatch)
	protected.Post("/batches/:id/pause", server.pauseBatch)
	protected.Post("/batches/:id/resume", server.resumeBatch)
	protected.Delete("/batches/:id", server.deleteScheduledBatch)
	protected.Get("/gateways", server.gateways)
	protected.Post("/gateways", server.createGateway)
	protected.Post("/gateways/bulk", server.bulkGateways)
	protected.Get("/gateways/:id", server.gatewayDetail)
	protected.Put("/gateways/:id", server.updateGateway)
	protected.Delete("/gateways/:id", server.deleteGateway)
	protected.Get("/gateways/:id/dashboard", server.gatewayDashboard)
	protected.Get("/gateways/:id/operator-rates", server.gatewayOperatorRates)
	protected.Post("/gateways/:id/operator-rates", server.saveOperatorRate)
	protected.Put("/gateways/:id/operator-rates/:rateId", server.saveOperatorRate)
	protected.Delete("/gateways/:id/operator-rates/:rateId", server.deleteOperatorRate)
	protected.Get("/gateways/:id/senders", server.gatewaySenders)
	protected.Get("/settings/:group", server.settings)
	protected.Put("/settings/:group", server.updateSettings)
	protected.Get("/kyc/dashboard", server.kycDashboard)
	protected.Get("/kyc/requests", server.kycRequests)
	protected.Post("/kyc/requests/:id/approve", server.approveKYC)
	protected.Post("/kyc/requests/:id/reject", server.rejectKYC)
	protected.Get("/kyc/audit", server.kycAudit)
	protected.Get("/kyc/permissions", server.kycPermissions)
	protected.Put("/kyc/permissions/:id", server.updateKYCPermissions)
	protected.Get("/deposits", server.deposits)
	protected.Get("/deposits/finance-report", server.depositFinanceReport)
	protected.Get("/deposits/:id", server.depositDetail)
	protected.Put("/deposits/:id", server.updateDeposit)
	protected.Get("/ip-whitelist-requests", server.ipWhitelistRequests)
	protected.Post("/ip-whitelist-requests/:id/approve", server.approveIPWhitelist)
	protected.Post("/ip-whitelist-requests/:id/reject", server.rejectIPWhitelist)
	protected.Post("/approval-center/ip-whitelists/:id/approve", server.approveIPWhitelist)
	protected.Post("/approval-center/ip-whitelists/:id/reject", server.rejectIPWhitelist)
	protected.Get("/sms-logs", server.smsLogs)
	protected.Get("/queue-monitor", server.queueMonitor)
	protected.Post("/queue-actions/:action", server.queueAction)
	protected.Post("/customers", server.createCustomer)
	protected.Post("/customers/:id/impersonate", server.impersonateCustomer)
	protected.Post("/customers/:id/deposit", server.depositCustomer)
	protected.Put("/customers/:id/status", server.updateCustomerStatus)
	protected.Get("/customers/:id", server.customerDetail)
	protected.Put("/customers/:id", server.updateCustomer)
	protected.Post("/customers/:id/approve-kyc", server.approveKYC)
	protected.Post("/customers/:id/reject-kyc", server.rejectKYC)
	protected.Post("/customers/:id/resubmit-kyc", server.resubmitKYC)
	// S P (Resellers)
	protected.Get("/sps", server.sps)
	protected.Get("/sps/customers", server.spsCustomers)
	protected.Get("/sps/rates", server.spsRates)
	protected.Get("/sps/wallet", server.spsWallets)
	protected.Post("/sps/wallet/topup", server.spsTopup)
	protected.Get("/sps/white-label", server.spsWhiteLabel)
	protected.Post("/sps/white-label", server.spsWhiteLabel)
	protected.Get("/sps/custom-domain", server.spsCustomDomains)
	protected.Get("/sps/profit", server.spsProfit)
	protected.Get("/sps/senders", server.spsSenders)
	protected.Get("/sps/billing", server.spsBilling)
	protected.Get("/sps/kyc", server.spsKyc)
	protected.Get("/sps/statements", server.spsStatements)
	protected.Get("/sps/api", server.spsApi)
	protected.Get("/sps/audit", server.spsAudit)
	protected.Get("/sps/routing", server.spsRouting)
	protected.Get("/sps/webhooks", server.spsWebhooks)
	protected.Get("/sps/reports", server.spsReports)
	protected.Get("/sps/employees", server.spsEmployees)
	protected.Get("/sps/:id", server.spDetail)
	protected.Put("/sps/:id", server.updateSP)
	protected.Post("/sps/:id/deposit", server.depositSP)
	protected.Post("/sps/:id/impersonate", server.impersonateSP)
	protected.Put("/sps/:id/status", server.updateSPStatus)
	protected.Get("/senders", server.senders)
	protected.Post("/senders", server.createSender)
	protected.Post("/senders/import", server.importSenders)
	protected.Post("/senders/bulk-delete", server.deleteSenders)
	protected.Delete("/senders", server.deleteSenders)
	protected.Get("/senders/:id", server.senderDetail)
	protected.Put("/senders/:id", server.updateSender)
	protected.Delete("/senders/:id", server.deleteSender)
	protected.Post("/senders/:id/approve", server.approveSender)
	protected.Post("/senders/:id/reject", server.rejectSender)
	protected.Post("/senders/:id/assignment", server.assignSenderUsers)
	protected.Patch("/senders/:id/enabled", server.updateSenderEnabled)
	protected.Get("/sender-cli/options", server.senderCliOptions)
	protected.Get("/sender-cli/summary", server.senderAuditSummary)
	protected.Get("/sender-cli/ledger", server.senderAuditLedger)
	protected.Get("/sender-cli/dispatches", server.senderDispatches)
	protected.Get("/sender-cli/resolution", server.senderResolution)
	protected.Get("/tariff-plans", server.tariffPlans)
	protected.Get("/rates", server.rates)
	protected.Get("/countries", server.countries)
	protected.Get("/operators", server.operators)
	protected.Get("/approvals/overview", server.approvalsOverview)
	protected.Get("/kyc/requests/:id", server.kycDetail)
	protected.Post("/kyc/requests/:id/resubmit", server.resubmitKYC)

	// Billing & Financial endpoints
	protected.Get("/billing/dashboard-stats", server.billingDashboardStats)
	protected.Post("/billing/adjust-balance", server.adjustBalance)
	protected.Get("/billing/tariff-plans", server.billingTariffPlans)
	protected.Post("/billing/tariff-plans", server.createTariffPlan)
	protected.Get("/billing/tariff-plans/:id", server.billingTariffPlanDetail)
	protected.Put("/billing/tariff-plans/:id", server.updateTariffPlan)
	protected.Delete("/billing/tariff-plans/:id", server.deleteTariffPlan)
	protected.Get("/billing/assignments", server.billingAssignments)
	protected.Get("/billing/assignments/:user_id", server.billingCustomerAssignmentHistory)
	protected.Post("/billing/assignments", server.assignTariffPlan)
	protected.Get("/billing/rate-history", server.billingRateHistory)
	protected.Post("/deposits/:id/approve", server.approveDeposit)
	protected.Post("/deposits/:id/reject", server.rejectDeposit)
	protected.Get("/billing/invoices", server.billingInvoices)
	protected.Get("/billing/payments", server.billingPayments)
	protected.Get("/billing/usage-cdr", server.billingUsageCdr)
	protected.Get("/billing/reports", server.billingReports)

	// Analytics endpoints
	protected.Get("/analytics/cost-profit", server.analyticsCostProfit)
	protected.Get("/analytics/messaging", server.analyticsMessaging)
	protected.Get("/analytics/financial", server.analyticsFinancial)

	// Operational endpoints (Providers, Reports, Health, Routing, Audit)
	protected.Get("/providers/health", server.providerHealth)
	protected.Post("/providers/health/check-all", server.providerHealthCheckAll)
	protected.Get("/providers/api-logs", server.providerApiLogs)
	protected.Get("/providers/dlr-mapping", server.dlrMappings)
	protected.Get("/providers/error-mapping", server.errorMappings)
	protected.Get("/reports/executive", server.reportsExecutive)
	protected.Get("/templates", server.templates)
	protected.Get("/templates/options", server.templateOptions)
	protected.Post("/templates", server.createTemplate)
	protected.Post("/templates/bulk", server.bulkTemplates)
	protected.Get("/templates/:id", server.templateDetail)
	protected.Put("/templates/:id", server.updateTemplate)
	protected.Delete("/templates/:id", server.deleteTemplate)
	protected.Post("/templates/:id/approve", server.approveTemplate)
	protected.Post("/templates/:id/reject", server.rejectTemplate)
	protected.Get("/contacts", server.contacts)
	protected.Post("/contacts", server.createContact)
	protected.Post("/contacts/bulk", server.bulkContacts)
	protected.Put("/contacts/:id", server.updateContact)
	protected.Delete("/contacts/:id", server.deleteContact)
	protected.Get("/contact-groups", server.contactGroups)
	protected.Post("/contact-groups", server.createContactGroup)
	protected.Post("/contact-groups/bulk", server.bulkContactGroups)
	protected.Put("/contact-groups/:id", server.updateContactGroup)
	protected.Delete("/contact-groups/:id", server.deleteContactGroup)
	protected.Get("/contact-owners", server.contactOwners)
	protected.Get("/employees", server.adminEmployees)
	protected.Get("/roles", server.adminRoles)
	protected.Get("/monitoring/system-health", server.monitoringSystemHealth)
	protected.Get("/audit-logs/global", server.auditLogsGlobal)
	protected.Get("/routing/dashboard", server.routingDashboard)

	// System Settings
	protected.Get("/system-settings/providers", server.systemSettingsProviders)
	protected.Get("/system-settings/routing", server.systemSettingsRouting)
	protected.Get("/system-settings/queue", server.systemSettingsQueue)
	protected.Get("/system-settings/error-codes", server.systemSettingsErrorCodes)
	protected.Get("/system-settings/dlr", server.systemSettingsDlr)
	protected.Post("/system-settings/dlr", server.systemSettingsDlr)
	protected.Get("/system-settings/maintenance", server.systemSettingsMaintenance)
	protected.Post("/system-settings/maintenance", server.systemSettingsMaintenance)
	protected.Get("/notifications", server.adminNotifications)
	protected.Post("/notifications/read-all", server.markAllAdminNotificationsRead)
	protected.Post("/notifications/:id/read", server.markAdminNotificationRead)

	protected.Get("/system-settings/backups", server.systemSettingsBackups)
	protected.Post("/system-settings/backups", server.systemSettingsBackups)
	protected.Get("/system-settings/backups/download/:filename", server.downloadDatabaseBackup)

	// Security & Fraud Control
	protected.Get("/security/dashboard", server.securityDashboard)
	protected.Get("/security/ip-whitelist", server.securityIpWhitelist)
	protected.Get("/security/fraud-rules", server.securityFraudRules)
	protected.Post("/security/fraud-rules", server.securityFraudRules)
	protected.Get("/security/logs", server.securityLogs)

	// Monitoring / NOC
	protected.Get("/monitoring/live", server.monitoringLive)
	protected.Get("/monitoring/providers", server.monitoringProviders)
	protected.Get("/monitoring/queue-health", server.monitoringQueueHealth)
	protected.Get("/monitoring/api-health", server.monitoringApiHealth)
	protected.Get("/monitoring/tps", server.monitoringTps)

	// Routing
	protected.Get("/routing/rules", server.routingRules)
	protected.Get("/routing/routes", server.routingRoutes)
	protected.Get("/routing/health", server.routingHealth)
	protected.Get("/routing/failover", server.routingFailover)
	protected.Post("/routing/testing", server.routingSimulate)
	protected.Get("/routing/load-balancing", server.routingLoadBalancing)
	protected.Post("/routing/load-balancing", server.routingLoadBalancing)

	// Audit Logs
	protected.Get("/audit-logs/ip-whitelist", server.auditLogsIpWhitelist)
	protected.Get("/audit-logs/employees", server.auditLogsEmployees)
	protected.Get("/audit-logs/security", server.auditLogsSecurity)

	return app
}

func (server *Server) authenticate(context fiber.Ctx) error {
	authorization := context.Get(fiber.HeaderAuthorization)
	if !strings.HasPrefix(authorization, "Bearer ") {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	claims, err := server.tokens.Parse(strings.TrimPrefix(authorization, "Bearer "))
	if err != nil {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}
	rawToken := strings.TrimPrefix(authorization, "Bearer ")
	if server.isTokenRevoked(rawToken) {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	var userType string
	err = server.db.QueryRow("SELECT type FROM users WHERE id = ? AND deleted_at IS NULL", claims.AdminID).Scan(&userType)
	if err != nil || userType != "superman" {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	context.Locals("superman", claims)
	context.Locals("superman_token", rawToken)

	return context.Next()
}

func (server *Server) tokenCacheKey(token string) string {
	digest := sha256.Sum256([]byte(token))

	return server.config.CachePrefix + "superman_api_revoked_" + hex.EncodeToString(digest[:])
}

func (server *Server) isTokenRevoked(token string) bool {
	var exists bool
	err := server.db.QueryRow("SELECT EXISTS(SELECT 1 FROM cache WHERE `key` = ? AND expiration >= ?)", server.tokenCacheKey(token), time.Now().Unix()).Scan(&exists)

	return err == nil && exists
}

func superman(context fiber.Ctx) auth.Claims {
	return context.Locals("superman").(auth.Claims)
}

func pagination(context fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(context.Query("page", "1"))
	perPage, _ := strconv.Atoi(context.Query("per_page", "25"))

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
