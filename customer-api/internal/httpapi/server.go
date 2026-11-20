package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	"smswhiteinfo/customer-api/internal/auth"
	"smswhiteinfo/customer-api/internal/batch"
	"smswhiteinfo/customer-api/internal/config"
	"smswhiteinfo/customer-api/internal/dlr"
	"smswhiteinfo/customer-api/internal/email"
	"smswhiteinfo/customer-api/internal/kyc"
	"smswhiteinfo/customer-api/internal/queue"
	"smswhiteinfo/customer-api/internal/scheduler"
	"smswhiteinfo/customer-api/internal/worker"
)

type Server struct {
	config         config.Config
	db             *sql.DB
	tokens         auth.TokenManager
	batchProcessor *batch.BatchProcessor
	workerPool     *worker.SendWorkerPool
	dlrPoller      *dlr.DeliveryPoller
	scheduler      *scheduler.MasterScheduler
	kycService     *kyc.KycService
	mailer         *email.Mailer
	loginLimiter   *LoginLimiter
	rabbitClient   *queue.RabbitMQClient
	submitLockMu   sync.Mutex
	submitLocks    map[string]time.Time
}

func New(config config.Config, database *sql.DB) *Server {
	storageRoot := os.Getenv("STORAGE_PATH")
	if storageRoot == "" {
		storageRoot = os.Getenv("KYC_STORAGE_PATH")
	}
	if storageRoot == "" {
		storageRoot = "./storage/app/private"
	}

	appKey := os.Getenv("APP_KEY")

	batchProc := batch.NewBatchProcessor(database, storageRoot)
	kycSvc := kyc.NewKycService(database, storageRoot)
	mailer := email.NewMailerFromEnv()

	workerPool, _ := worker.NewSendWorkerPool(database, appKey, 4)
	dlrPoller, _ := dlr.NewDeliveryPoller(database, appKey)
	masterSched := scheduler.NewMasterScheduler(database, batchProc)

	var rabbitClient *queue.RabbitMQClient
	if config.RabbitMQURL != "" {
		rc, err := queue.NewRabbitMQClient(config.RabbitMQURL)
		if err != nil {
			log.Printf("[RABBITMQ] Warning: Failed to connect to RabbitMQ: %v", err)
		} else {
			log.Printf("[RABBITMQ] Successfully connected to RabbitMQ at %s", config.RabbitMQURL)
			rabbitClient = rc
			batchProc.SetPublisher(rc)
		}
	}

	return &Server{
		config:         config,
		db:             database,
		tokens:         auth.NewTokenManager(config.JWTSecret),
		batchProcessor: batchProc,
		workerPool:     workerPool,
		dlrPoller:      dlrPoller,
		scheduler:      masterSched,
		kycService:     kycSvc,
		mailer:         mailer,
		loginLimiter:   NewLoginLimiter(),
		rabbitClient:   rabbitClient,
		submitLocks:    make(map[string]time.Time),
	}
}

func (server *Server) StartBackground(ctx context.Context) {
	if server.rabbitClient != nil && server.workerPool != nil {
		err := server.rabbitClient.StartConsumer(ctx, func(msgID int64) error {
			return server.workerPool.ProcessMessageByID(ctx, msgID)
		})
		if err != nil {
			log.Printf("[RABBITMQ] Failed to start queue consumer: %v", err)
		} else {
			log.Println("[RABBITMQ] Worker queue consumer started listening on sms_dispatch_queue")
		}
	}
	if server.workerPool != nil {
		server.workerPool.Start(ctx)
	}
	if server.dlrPoller != nil {
		server.dlrPoller.Start(ctx)
	}
	if server.scheduler != nil {
		server.scheduler.Start(ctx)
	}
}

func (server *Server) StopBackground() {
	if server.rabbitClient != nil {
		server.rabbitClient.Close()
	}
	if server.workerPool != nil {
		server.workerPool.Stop()
	}
	if server.dlrPoller != nil {
		server.dlrPoller.Stop()
	}
	if server.scheduler != nil {
		server.scheduler.Stop()
	}
}

func (server *Server) App() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:    "SMS Whiteinfo Customer API",
		BodyLimit:  20 * 1024 * 1024, // 20MB for file uploads
		TrustProxy: true,
		TrustProxyConfig: fiber.TrustProxyConfig{
			Proxies:  []string{"127.0.0.1", "::1"},
			Loopback: true,
		},
		ProxyHeader:        fiber.HeaderXForwardedFor,
		EnableIPValidation: true,
	})

	// Normalize proxy headers: use X-Real-IP only when the TCP peer is loopback
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
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-Requested-With", "Accept", "X-XSRF-TOKEN"},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	}))

	app.Use(server.maintenanceGuard)

	app.Get("/health", func(context fiber.Ctx) error {
		if err := server.db.Ping(); err != nil {
			return context.Status(http.StatusServiceUnavailable).JSON(fiber.Map{
				"status":   "unavailable",
				"database": "down",
			})
		}

		var inFlight int64
		var lastLoop time.Time
		if server.workerPool != nil {
			inFlight, lastLoop = server.workerPool.Status()
		}

		return context.JSON(fiber.Map{
			"status":    "ok",
			"database":  "ok",
			"in_flight": inFlight,
			"last_loop": lastLoop.Format(time.RFC3339),
		})
	})

	// Public Webhooks & Callbacks
	app.All("/webhooks/sms/:gateway", server.handleSmsWebhook)
	app.Post("/webhooks/payment/:gateway", server.handlePaymentWebhook)
	app.Get("/payments/:gateway/callback/:deposit", server.handlePaymentCallback)

	// Phone Photo capture
	app.Post("/phone-photo/session", server.kycService.CreatePhonePhotoSession)
	app.Get("/phone-photo/:token", server.kycService.ShowCapturePage)
	app.Post("/phone-photo/:token/upload", server.kycService.UploadPhoto)
	app.Get("/phone-photo/:token/status", server.kycService.CheckStatus)

	// Private KYC & Report streaming
	app.Get("/kyc-files/:user/:field", server.handleKycFileStream)
	app.Get("/downloads/batches/:batch/failed-report/:filename", server.handleFailedReportDownload)

	api := app.Group("/api/v1/customer")

	// Public routes
	api.Post("/login", server.login)
	api.Post("/register", server.register)
	api.Post("/forgot-password", server.forgotPassword)
	api.Post("/reset-password", server.resetPassword)
	api.Post("/email-change/verify", server.verifyEmailChange)
	api.Get("/register/plan/:slug", server.registerPlan)

	// Signup verification codes (public: no account exists yet).
	api.Post("/otp/send", server.otpSend)
	api.Post("/otp/verify", server.otpVerify)

	// Protected routes
	protected := api.Group("", server.authenticate)
	protected.Post("/logout", server.logout)
	protected.Get("/me", server.me)
	protected.Put("/profile", server.updateProfile)
	protected.Post("/profile/email", server.requestEmailChange)
	protected.Put("/password", server.updatePassword)

	// Dashboard
	protected.Get("/dashboard/stats", server.dashboardStats)

	// Messages
	protected.Get("/messages", server.messages)
	protected.Get("/messages/export", server.exportMessages)
	protected.Get("/messages/:id", server.messageDetail)
	protected.Get("/recent-recipients", server.recentRecipients)
	protected.Post("/messages/send", server.sendMessage)
	protected.Post("/messages/schedule", server.scheduleMessage)
	protected.Post("/messages/bulk-send", server.bulkSend)
	protected.Post("/messages/upload-preview", server.uploadRecipientPreview)

	// Batches
	protected.Get("/batches", server.batches)
	protected.Get("/batches/export", server.exportBatches)
	protected.Get("/batches/:id", server.batchDetail)
	protected.Get("/batches/:id/failed-report", server.downloadBatchFailedReport)
	protected.Post("/batches/:id/pause", server.pauseScheduledMessage)
	protected.Post("/batches/:id/resume", server.resumeScheduledMessage)
	protected.Delete("/batches/:id", server.deleteScheduledMessage)

	// Scheduled Messages
	protected.Get("/scheduled-messages", server.scheduledMessages)
	protected.Post("/scheduled-messages/bulk-delete", server.bulkDeleteScheduledMessages)
	protected.Put("/scheduled-messages/:id", server.updateScheduledMessage)
	protected.Post("/scheduled-messages/:id/pause", server.pauseScheduledMessage)
	protected.Post("/scheduled-messages/:id/resume", server.resumeScheduledMessage)
	protected.Delete("/scheduled-messages/:id", server.deleteScheduledMessage)

	// Senders
	protected.Get("/senders", server.senders)
	protected.Post("/senders", server.createSender)

	// Contacts
	protected.Get("/contacts", server.contacts)
	protected.Get("/contacts/export", server.exportContacts)
	protected.Post("/contacts", server.createContact)
	protected.Post("/contacts/bulk-delete", server.bulkDeleteContacts)
	protected.Get("/contacts/:id/groups", server.contactGroups)
	protected.Put("/contacts/:id", server.updateContact)
	protected.Delete("/contacts/:id", server.deleteContact)

	// Groups
	protected.Get("/groups", server.groups)
	protected.Post("/groups", server.createGroup)
	protected.Post("/groups/bulk-delete", server.bulkDeleteGroups)
	protected.Get("/groups/:id", server.groupDetail)
	protected.Put("/groups/:id", server.updateGroup)
	protected.Delete("/groups/:id", server.deleteGroup)
	protected.Get("/groups/:id/contacts", server.groupContacts)
	protected.Get("/groups/:id/contacts/export", server.exportGroupContacts)
	protected.Post("/groups/:id/contacts", server.attachGroupContacts)
	protected.Post("/groups/:id/contacts/import", server.importGroupContacts)
	protected.Post("/groups/:id/contacts/bulk-detach", server.bulkDetachGroupContacts)
	protected.Delete("/groups/:id/contacts/:contact_id", server.detachGroupContact)

	// Templates
	protected.Get("/templates", server.templates)
	protected.Post("/templates", server.createTemplate)
	protected.Post("/templates/bulk-delete", server.bulkDeleteTemplates)
	protected.Get("/templates/:id", server.templateDetail)
	protected.Put("/templates/:id", server.updateTemplate)
	protected.Delete("/templates/:id", server.deleteTemplate)

	// Transactions
	protected.Get("/transactions", server.transactionsReport)

	// Deposits
	protected.Get("/deposits", server.deposits)
	protected.Post("/deposits", server.createDeposit)

	// KYC
	protected.Get("/kyc", server.kycShow)
	protected.Post("/kyc", server.kycSubmit)

	// Database notifications
	protected.Get("/2fa", server.twoFactorStatus)
	protected.Post("/2fa/setup", server.twoFactorSetup)
	protected.Post("/2fa/enable", server.twoFactorEnable)
	protected.Post("/2fa/disable", server.twoFactorDisable)

	protected.Get("/notifications", server.notifications)
	protected.Post("/notifications/read-all", server.markAllNotificationsRead)
	protected.Post("/notifications/:id/read", server.markNotificationRead)

	// IP Whitelists
	protected.Get("/ip-whitelists", server.devIPWhitelistsList)
	protected.Post("/ip-whitelists", server.devIPWhitelistCreate)
	protected.Put("/ip-whitelists/:id", server.devIPWhitelistUpdate)
	protected.Delete("/ip-whitelists/:id", server.devIPWhitelistDelete)

	// Pricing
	protected.Get("/pricing", server.pricingReport)

	// Developer / API tokens
	protected.Get("/tokens", server.devTokensList)
	protected.Post("/tokens", server.devTokenCreate)
	protected.Post("/tokens/bulk-delete", server.devTokensBulkDelete)
	protected.Put("/tokens/:id", server.devTokenUpdate)
	protected.Delete("/tokens/:id", server.devTokenDelete)

	// Global Search
	protected.Get("/global-search", server.globalSearch)

	// Analytics
	protected.Get("/analytics/messaging", server.messagingAnalytics)
	protected.Get("/analytics/messaging/export", server.exportMessagingAnalytics)
	protected.Get("/analytics/billing", server.analyticsBilling)

	// Live queue
	protected.Get("/live-queue", server.liveQueue)

	// External developer API (replaces Laravel routes/api.php)
	server.RegisterExternalAPI(app)

	return app
}

func (server *Server) authenticate(context fiber.Ctx) error {
	var rawToken string
	authorization := context.Get(fiber.HeaderAuthorization)
	if strings.HasPrefix(authorization, "Bearer ") {
		rawToken = strings.TrimPrefix(authorization, "Bearer ")
	} else if cookie := context.Cookies("customer_token"); cookie != "" {
		rawToken = cookie
	}

	if rawToken == "" {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}
	claims, err := server.tokens.Parse(rawToken)
	if err != nil {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	if server.isTokenRevoked(rawToken) {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	// Verify user still exists and is a customer
	var userType, userStatus string
	err = server.db.QueryRow(
		"SELECT type, status FROM users WHERE id = ? AND deleted_at IS NULL",
		claims.UserID,
	).Scan(&userType, &userStatus)
	if err != nil || userType != "customer" {
		return context.Status(http.StatusUnauthorized).JSON(fiber.Map{"message": "Unauthenticated."})
	}

	context.Locals("customer", claims)
	context.Locals("customer_token", rawToken)

	return context.Next()
}

func (server *Server) tokenCacheKey(token string) string {
	digest := sha256.Sum256([]byte(token))

	return server.config.CachePrefix + "customer_api_revoked_" + hex.EncodeToString(digest[:])
}

func (server *Server) isTokenRevoked(token string) bool {
	var exists bool
	err := server.db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM cache WHERE `key` = ? AND expiration >= ?)",
		server.tokenCacheKey(token), time.Now().Unix(),
	).Scan(&exists)

	return err == nil && exists
}

func customer(context fiber.Ctx) auth.Claims {
	return context.Locals("customer").(auth.Claims)
}

func pagination(context fiber.Ctx) (int, int) {
	page, _ := strconv.Atoi(context.Query("page", "1"))
	perPage, _ := strconv.Atoi(context.Query("per_page", "15"))

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

func apiTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}

	return apiTime(*value)
}

func pageMeta(total, perPage, page int) fiber.Map {
	lastPage := total / perPage
	if total%perPage != 0 {
		lastPage++
	}
	if lastPage < 1 {
		lastPage = 1
	}

	return fiber.Map{
		"total":        total,
		"per_page":     perPage,
		"current_page": page,
		"last_page":    lastPage,
	}
}

func databaseError(context fiber.Ctx, err error) error {
	// Log the real cause server-side; do not leak SQL error text to the client.
	log.Printf("database error on %s %s: %v", context.Method(), context.Path(), err)

	return context.Status(http.StatusInternalServerError).JSON(fiber.Map{
		"message": "A database error occurred.",
	})
}
