package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"flowwithlit/internal/admin"
	"flowwithlit/internal/auth"
	"flowwithlit/internal/bills"
	"flowwithlit/internal/card"
	"flowwithlit/internal/chatbot"
	"flowwithlit/internal/family"
	"flowwithlit/internal/checkout"
	"flowwithlit/internal/commerce"
	"flowwithlit/internal/company"
	"flowwithlit/internal/dashboard"
	"flowwithlit/internal/database"
	"flowwithlit/internal/developer"
	"flowwithlit/internal/dispute"
	"flowwithlit/internal/kyc"
	"flowwithlit/internal/payroll"
	"flowwithlit/internal/referral"
	"flowwithlit/internal/secure"
	"flowwithlit/internal/support"
	"flowwithlit/internal/transaction"
	"flowwithlit/internal/transfer"
	"flowwithlit/internal/user"
	"flowwithlit/internal/vault"
	"flowwithlit/internal/wallet"
	"flowwithlit/internal/webhook"
	myMiddleware "flowwithlit/pkg/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found or error loading it, using system environment variables")
	}

	fmt.Println("Payment Gateway Engine Starting...")

	// Initialize database connection
	database.Connect()

	// Initialize Router
	r := chi.NewRouter()

	// CORS: locked to our own first-party origins + the browser extension, instead
	// of the previous "any http/https origin" rule. Every PHP page in this project
	// (app/, admin/, checkout/, demo/, the inline.js merchant embed) talks to this
	// API server-side via cURL (see includes/api_client.php) — the browser never
	// calls api.flowwithlit.com directly except from the extension. So the old
	// wildcard bought no real functionality, just let any website's JS script
	// requests straight against api.flowwithlit.com (with credentials) on behalf
	// of whoever had it open.
	const flowwithlitExtensionOrigin = "chrome-extension://oobmbnjnkfllnaonmhdkpladmacbnaih"
	isProdEnv := strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production")

	firstPartyOrigins := map[string]bool{
		"https://flowwithlit.com":          true,
		"https://app.flowwithlit.com":      true,
		"https://auth.flowwithlit.com":     true,
		"https://admin.flowwithlit.com":    true,
		"https://support.flowwithlit.com":  true,
		"https://checkout.flowwithlit.com": true,
		"https://pay.flowwithlit.com":      true,
		"https://demo.flowwithlit.com":     true,
	}

	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			if origin == flowwithlitExtensionOrigin || firstPartyOrigins[origin] {
				return true
			}
			// Local/ngrok dev only — never relaxed when ENVIRONMENT=production.
			if !isProdEnv && (strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1")) {
				return true
			}
			return false
		},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Add basic middleware (logging, etc)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// A simple test route you can visit in your browser
	r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong! Flowwithlit Backend is LIVE! 🚀"))
	})

	// Auth Routes
	r.Route("/auth", func(r chi.Router) {
		// Credential-guessing / spam surfaces — capped per IP so brute-forcing a
		// password, a 2FA code, or a password-reset code isn't just a for-loop away.
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/register", auth.RegisterHandler)
		// Mobile-first register (DOB + age gate) + phone OTP (not the same as web email-only)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/mobile/register", auth.MobileRegisterHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/mobile/send-phone-otp", auth.SendPhoneOTPHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/mobile/verify-phone-otp", auth.VerifyPhoneOTPHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/login", auth.LoginHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/login-2fa", auth.Login2FAHandler)
		// New-device email OTP (after password / 2FA) — tight rate limits to stop code guessing
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/verify-device", auth.VerifyDeviceHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/resend-device-code", auth.ResendDeviceCodeHandler)
		// Tab close / logout — ends the browser session server-side
		r.With(myMiddleware.RateLimit(30, 1*time.Minute)).Post("/end-session", auth.EndSessionHandler)
		// Mobile biometric login (fingerprint / Face ID unlocks device-held token)
		r.With(myMiddleware.RateLimit(15, 1*time.Minute)).Post("/biometric/login", auth.BiometricLoginHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/forgot-password", auth.ForgotPasswordHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/verify-reset-code", auth.VerifyResetCodeHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/reset-password", auth.ResetPasswordHandler)
		r.Post("/verify-email", auth.VerifyEmailHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/resend-verification", auth.ResendVerificationHandler)
		r.Post("/refresh", auth.RefreshHandler)
	})

	// User Routes (Protected)
	r.Route("/user", func(r chi.Router) {
		r.Use(myMiddleware.RequireAuth)
		r.Get("/me", user.GetMeHandler)
		r.Put("/profile", user.UpdateProfileHandler)
		r.Put("/password", user.UpdatePasswordHandler)
		r.Put("/profile-image", user.UpdateProfileImageHandler)
		r.Post("/pin/setup", user.SetupPINHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/pin/verify", user.VerifyPINHandler)
		r.Get("/2fa/generate", user.Generate2FAHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/2fa/verify", user.Verify2FAHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/2fa/disable", user.Disable2FAHandler)
		r.Get("/sessions", user.GetSessionsHandler)
		r.Delete("/sessions/revoke", user.RevokeSessionHandler)

		// Mobile biometric (fingerprint / Face ID) — enroll + payment authorize
		r.Get("/biometric/status", user.BiometricStatusHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/biometric/enable", user.BiometricEnableHandler)
		r.Post("/biometric/disable", user.BiometricDisableHandler)
		r.With(myMiddleware.RateLimit(20, 1*time.Minute)).Post("/biometric/authorize", user.BiometricAuthorizeHandler)

		// Mobile push notification device tokens (FCM / APNs token storage)
		r.Post("/push/register", user.RegisterPushHandler)
		r.Post("/push/unregister", user.UnregisterPushHandler)
		r.Get("/push/status", user.PushStatusHandler)

		// Family / kids sub-accounts (child's name; parent KYC + spend controls)
		r.Get("/family/juniors", family.ListJuniorsHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/family/juniors", family.CreateJuniorHandler)
		r.Put("/family/juniors/{id}", family.UpdateJuniorHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/family/juniors/{id}/fund", family.FundJuniorHandler)

		// Phone OTP also available when already logged in (mobile settings)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/mobile/send-phone-otp", auth.SendPhoneOTPHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/mobile/verify-phone-otp", auth.VerifyPhoneOTPHandler)

		r.Get("/notifications", user.GetNotificationsHandler)
		r.Get("/notifications/pending-broadcast", user.GetPendingBroadcastModalHandler)
		r.Post("/notifications/dismiss-broadcast", user.DismissBroadcastModalHandler)
		r.Post("/notifications/read", user.MarkNotificationsReadHandler)
		r.Put("/notification-preferences", user.UpdateNotificationPreferencesHandler)
		r.Get("/settlement-options", user.GetSettlementOptionsHandler)
		r.Put("/settlement-preferences", user.UpdateSettlementPreferencesHandler)
		r.Post("/tickets", user.CreateTicketHandler)

		// Support live chat (user side)
		r.Get("/support/chat/sessions", support.GetUserSessionsHandler)
		r.Post("/support/chat/start", support.StartChatHandler)
		r.Post("/support/chat/message", support.SendMessageHandler)
		r.Get("/support/chat/messages/{ref}", support.GetUserMessagesHandler)
		r.Post("/support/chat/escalate", support.EscalateHandler)

		// Commerce / Payment Gateway routes
		r.Route("/commerce", func(r chi.Router) {
			r.Post("/payment-links", commerce.CreatePaymentLinkHandler)
			r.Get("/payment-links", commerce.GetPaymentLinksHandler)
			r.Post("/payment-links/toggle", commerce.TogglePaymentLinkHandler)
			r.Post("/invoices", commerce.CreateInvoiceHandler)
			r.Get("/invoices", commerce.GetInvoicesHandler)
			r.Post("/invoices/remind", commerce.RemindInvoiceHandler)
			r.Post("/invoices/mark-paid", commerce.MarkInvoicePaidHandler)
			r.Get("/customers", commerce.GetCustomersHandler)
		})

		// User disputes
		r.Get("/disputes", dispute.GetUserDisputesHandler)
		r.Post("/disputes", dispute.CreateDisputeHandler)

		// Wallets & Swap (Phase 1-2)
		r.Get("/wallets", wallet.GetWalletsHandler)
		r.Get("/wallets/balances", wallet.GetBalancesHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/wallets/swap", wallet.SwapHandler)
		r.Get("/rates", wallet.GetRatesHandler)

		// Funding / Deposit details (OnePipe + Circle)
		r.Get("/funding/deposit-details", wallet.GetDepositDetailsHandler)
		r.Get("/funding/deposit-accounts", wallet.GetDepositAccountsHandler)
		r.Post("/funding/deposit-accounts", wallet.CreateDepositAccountHandler)
		r.Get("/funding/crypto-addresses", wallet.GetCryptoAddressesHandler)
		r.Post("/funding/crypto-addresses", wallet.CreateCryptoAddressHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/funding/crypto-withdraw", wallet.WithdrawCryptoHandler)

		// Bills / everyday bank services (airtime, data, electricity, cable)
		// No mock: requires VTU and/or Flutterwave keys (see key-get.md).
		r.Get("/bills/categories", bills.CategoriesHandler)
		r.Get("/bills/products", bills.ProductsHandler)
		r.Get("/bills/history", bills.HistoryHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/bills/purchase", bills.PurchaseHandler)

		// Transfers (bank + history)
		r.Get("/transfers/banks", transfer.BanksHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/transfers/bank", transfer.CreateBankTransferHandler)
		r.Post("/transfers/lookup", transfer.LookupAccountHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/transfers/bulk", transfer.BulkTransferHandler)
		r.Get("/transfers", transfer.GetTransfersHandler)

		// Payroll roster (Phase 2 — CRUD only, no settings/scheduler yet)
		r.Route("/payroll/employees", func(r chi.Router) {
			r.Get("/", payroll.GetEmployeesHandler)
			r.Post("/", payroll.CreateEmployeeHandler)
			r.Put("/{id}", payroll.UpdateEmployeeHandler)
			r.Delete("/{id}", payroll.DeleteEmployeeHandler)
		})

		// Virtual Cards (Phase 3)
		r.Get("/cards", card.GetCardsHandler)
		r.Post("/cards", card.CreateCardHandler)
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/cards/{id}/fund", card.FundCardHandler)
		r.Post("/cards/{id}/freeze", card.FreezeCardHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Get("/cards/{id}/reveal", card.RevealCardHandler)

		// Vaults (Phase 3)
		r.Get("/vaults", vault.GetVaultsHandler)
		r.Post("/vaults", vault.CreateVaultHandler)
		r.Post("/vaults/{id}/deposit", vault.DepositToVaultHandler)

		// Developer API credentials & webhook logs
		r.Get("/developer/credentials", developer.GetCredentialsHandler)
		r.Post("/developer/keys/rotate", developer.RotateKeysHandler)
		r.Post("/developer/webhooks", developer.UpdateWebhooksHandler)
		r.Get("/developer/webhook-logs", developer.GetWebhookLogsHandler)
		r.Post("/developer/webhook-logs/{id}/retry", developer.RetryWebhookHandler)

		// Secure / Escrow transfers
		r.Post("/secure-transfer/create", secure.CreateSecureTransferHandler)
		r.Post("/secure-transfer/lookup", secure.LookupSecureTransferHandler)
		r.Post("/secure-transfer/claim", secure.ClaimSecureTransferHandler)
	})

	// Dashboard Routes (Protected)
	r.Route("/dashboard", func(r chi.Router) {
		r.Use(myMiddleware.RequireAuth)
		r.Get("/metrics", dashboard.MetricsHandler)
		r.Get("/transactions", dashboard.TransactionsHandler)
	})

	// Refer & earn (user + mobile app)
	r.Route("/referral", func(r chi.Router) {
		r.Use(myMiddleware.RequireAuth)
		r.Get("/me", referral.MeHandler)
		r.Get("/earnings", referral.EarningsHandler)
	})

	// KYC and Onboarding endpoints
	r.Route("/kyc", func(r chi.Router) {
		r.Use(myMiddleware.RequireAuth)
		r.Get("/status", kyc.StatusHandler)
		r.Get("/profile", kyc.GetProfileHandler)
		r.Post("/activate", kyc.ActivateHandler)
	})

	// FlowTag
	r.With(myMiddleware.RequireAuth).Post("/flowtag/send", transaction.SendFlowTagHandler)
	r.With(myMiddleware.RequireAuth).Post("/flowtag/claim", transaction.ClaimFlowTagHandler)
	r.With(myMiddleware.RequireAuth).Get("/flowtag/history", transaction.GetFlowTagHistoryHandler)
	r.With(myMiddleware.RequireAuth).Get("/flowtag/lookup", transaction.LookupRecipientHandler)
	r.With(myMiddleware.RequireAuth).Post("/flowtag/request", transaction.RequestFlowTagPaymentHandler)
	r.With(myMiddleware.RequireAuth).Get("/flowtag/requests", transaction.GetFlowTagRequestsHandler)
	r.With(myMiddleware.RequireAuth).Post("/flowtag/request/pay", transaction.PayFlowTagRequestHandler)
	r.With(myMiddleware.RequireAuth).Post("/flowtag/request/decline", transaction.DeclineFlowTagRequestHandler)

	// Admin endpoints (Settings & Auth)
	r.Route("/admin", func(r chi.Router) {
		r.With(myMiddleware.RateLimit(10, 1*time.Minute)).Post("/auth/login", admin.AdminLogin)
		r.Get("/auth/register-status", admin.AdminRegisterStatus)
		// Initial-setup route, also gated by ADMIN_SETUP_SECRET (see AdminRegister) —
		// rate limited too since it's still reachable pre-auth on a live URL.
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/auth/register", admin.AdminRegister)

		r.With(admin.RequireAdminAuth).Get("/me", admin.GetAdminMeHandler)
		r.With(admin.RequireAdminAuth).Get("/users", admin.GetUsersHandler)
		r.With(admin.RequireAdminAuth).Post("/users/quick-action", admin.SendUserQuickActionHandler)
		r.With(admin.RequireAdminAuth).Get("/merchants", admin.GetMerchantsHandler)
		r.With(admin.RequireAdminAuth).Get("/transactions", admin.GetTransactionsHandler)
		r.With(admin.RequireAdminAuth).Get("/kyc", admin.GetKYCApprovalsHandler)
		r.With(admin.RequireAdminAuth).Post("/kyc/approve", admin.ApproveKYCHandler)
		r.With(admin.RequireAdminAuth).Post("/kyc/reject", admin.RejectKYCHandler)
		r.With(admin.RequireAdminAuth).Post("/kyc/request-info", admin.RequestKYCInfoHandler)

		r.With(admin.RequireAdminAuth).Get("/test-email/templates", admin.ListTestEmailTemplatesHandler)
		r.With(admin.RequireAdminAuth).Post("/test-email/send", admin.SendTestEmailHandler)

		r.With(admin.RequireAdminAuth).Get("/settings", admin.GetSettingsHandler)
		r.With(admin.RequireAdminAuth).Post("/settings", admin.UpdateSettingHandler)
		r.With(admin.RequireAdminAuth).Get("/company", company.AdminGetHandler)
		r.With(admin.RequireAdminAuth).Put("/company", company.AdminUpdateHandler)

		r.With(admin.RequireAdminAuth).Get("/currencies", admin.GetCurrenciesHandler)
		r.With(admin.RequireAdminAuth).Get("/exchange-rates", admin.GetExchangeRatesHandler)
		r.With(admin.RequireAdminAuth).Put("/exchange-rates", admin.UpdateExchangeRatesHandler)
		r.With(admin.RequireAdminAuth).Post("/currencies/toggle", admin.ToggleCurrencyHandler)
		r.With(admin.RequireAdminAuth).Get("/exchange-rates/preview", admin.GetRatePreviewHandler)

		r.With(admin.RequireAdminAuth).Get("/dashboard", admin.GetDashboardStatsHandler)
		r.With(admin.RequireAdminAuth).Get("/referrals", admin.GetReferralsHandler)
		r.With(admin.RequireAdminAuth).Get("/referrals/stats", admin.GetReferralStatsHandler)

		r.With(admin.RequireAdminAuth).Get("/disputes", admin.GetDisputesHandler)
		r.With(admin.RequireAdminAuth).Post("/disputes/update", admin.UpdateDisputeHandler)

		r.With(admin.RequireAdminAuth).Get("/webhook-logs", admin.GetWebhookLogsHandler)
		// Unified ops monitor — activity trail + health counters
		r.With(admin.RequireAdminAuth).Get("/activity", admin.GetActivityHandler)

		r.With(admin.RequireAdminAuth).Get("/tickets", admin.GetTicketsHandler)
		r.With(admin.RequireAdminAuth).Post("/tickets/reply", admin.ReplyTicketHandler)

		r.With(admin.RequireAdminAuth).Get("/audit-logs", admin.GetAuditLogsHandler)

		r.With(admin.RequireAdminAuth).Get("/broadcasts", admin.GetBroadcastsHandler)
		r.With(admin.RequireAdminAuth).Post("/broadcasts/send", admin.SendBroadcastHandler)

		// Admin invites
		r.Get("/auth/invite-info", admin.ValidateInviteTokenHandler)
		r.Post("/auth/accept-invite", admin.AcceptInviteHandler)
		r.With(admin.RequireAdminAuth).Get("/invites", admin.ListInvitesHandler)
		r.With(admin.RequireAdminAuth).Post("/invites/send", admin.SendInviteHandler)

		// Staff management
		r.With(admin.RequireAdminAuth).Get("/staff", admin.GetStaffHandler)
		r.With(admin.RequireAdminAuth).Post("/staff/update", admin.UpdateStaffHandler)

		// Support chat — agent side
		r.With(admin.RequireAdminAuth).Get("/support/sessions", support.GetSessionsHandler)
		r.With(admin.RequireAdminAuth).Get("/support/messages/{ref}", support.GetSessionMessagesHandler)
		r.With(admin.RequireAdminAuth).Post("/support/claim/{ref}", support.ClaimSessionHandler)
		r.With(admin.RequireAdminAuth).Post("/support/close/{ref}", support.CloseSessionHandler)
		r.With(admin.RequireAdminAuth).Post("/support/agent-message", support.AgentMessageHandler)
		r.With(admin.RequireAdminAuth).Post("/support/chatbot/suggest", chatbot.SuggestReplyHandler)
	})

	// Public checkout endpoints (no auth — authenticated by public_key)
	r.Route("/public", func(r chi.Router) {
		r.Get("/company", company.PublicHandler)
		r.Get("/merchant-info", checkout.MerchantInfoHandler)
		r.Get("/bank-details", checkout.BankDetailsHandler)
		r.Get("/bank-status", checkout.BankStatusHandler)
		r.Get("/rates", checkout.PublicRatesHandler)
		r.Get("/currencies", checkout.PublicCurrenciesHandler)
		r.Get("/crypto-assets", checkout.PublicCryptoAssetsHandler)
		// Unauthenticated (public_key only) and takes raw card details — rate limit
		// per IP so it can't be used for card-testing / brute-forcing card numbers.
		r.With(myMiddleware.RateLimit(15, 1*time.Minute)).Post("/charge", checkout.ChargeHandler)

		// Guest support chat (marketing site, no login required). Rate-limited since
		// it's the only unauthenticated route that calls the paid Anthropic API.
		r.Route("/support/chat", func(r chi.Router) {
			r.Use(myMiddleware.RateLimit(20, 10*time.Minute))
			r.Post("/start", support.StartGuestChatHandler)
			r.Post("/message", support.SendGuestMessageHandler)
			r.Get("/messages/{ref}", support.GetGuestMessagesHandler)
			r.Post("/escalate", support.EscalateGuestHandler)
		})
	})

	// Transaction verification (authenticated by secret key, not JWT)
	r.Get("/v1/transaction/verify/{ref}", checkout.VerifyTransactionHandler)

	// Internal mail test harness (email-test.php) — same secret as mail/dispatch.php
	r.Route("/internal/mail", func(r chi.Router) {
		r.Use(myMiddleware.RequireMailDispatchSecret)
		r.Get("/templates", admin.ListTestEmailTemplatesHandler)
		r.Post("/send-test", admin.SendTestEmailHandler)
	})

	// Webhooks (publicly reachable by providers). Soft signature verify when secrets
	// are set in Admin; otherwise accept + log so live deposits never break.
	// PalmPay route is reserved for the future NGN switch — OnePipe stays default.
	r.Route("/webhooks", func(r chi.Router) {
		r.Use(myMiddleware.RateLimit(120, 1*time.Minute))
		r.Post("/onepipe", webhook.OnePipeHandler)
		r.Post("/flutterwave", webhook.FlutterwaveHandler)
		r.Post("/circle", webhook.CircleHandler)
		r.Post("/smileid", webhook.SmileIDHandler)
		r.Post("/palmpay", webhook.PalmPayHandler)
	})

	// Start the server (Render/Railway/Fly set PORT)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	if port[0] != ':' {
		port = ":" + port
	}
	fmt.Printf("Server is ready to accept connections on http://localhost%s\n", port)

	err := http.ListenAndServe(port, r)
	if err != nil {
		fmt.Printf("Error starting server: %s\n", err)
	}
}
