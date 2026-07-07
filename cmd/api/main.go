package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"flowwithlit/internal/admin"
	"flowwithlit/internal/auth"
	"flowwithlit/internal/card"
	"flowwithlit/internal/chatbot"
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

	// Basic CORS settings
	// flowwithlitExtensionOrigin is the browser extension's fixed origin, derived from the
	// public key pinned in extension/manifest.json — chrome-extension:// origins don't match
	// the http/https wildcard patterns below, so it needs an explicit AllowOriginFunc check.
	// go-chi/cors ignores AllowedOrigins entirely once AllowOriginFunc is set, so the
	// http/https wildcard behavior that used to live in AllowedOrigins is reproduced below.
	const flowwithlitExtensionOrigin = "chrome-extension://oobmbnjnkfllnaonmhdkpladmacbnaih"
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			if origin == flowwithlitExtensionOrigin {
				return true
			}
			return strings.HasPrefix(origin, "https://") || strings.HasPrefix(origin, "http://")
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
		r.Post("/register", auth.RegisterHandler)
		r.Post("/login", auth.LoginHandler)
		r.Post("/login-2fa", auth.Login2FAHandler)
		r.Post("/forgot-password", auth.ForgotPasswordHandler)
		r.Post("/verify-reset-code", auth.VerifyResetCodeHandler)
		r.Post("/reset-password", auth.ResetPasswordHandler)
		r.Post("/verify-email", auth.VerifyEmailHandler)
		r.Post("/resend-verification", auth.ResendVerificationHandler)
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
		r.Post("/2fa/verify", user.Verify2FAHandler)
		r.Post("/2fa/disable", user.Disable2FAHandler)
		r.Get("/sessions", user.GetSessionsHandler)
		r.Delete("/sessions/revoke", user.RevokeSessionHandler)
		r.Get("/notifications", user.GetNotificationsHandler)
		r.Get("/notifications/pending-broadcast", user.GetPendingBroadcastModalHandler)
		r.Post("/notifications/dismiss-broadcast", user.DismissBroadcastModalHandler)
		r.Post("/notifications/read", user.MarkNotificationsReadHandler)
		r.Put("/notification-preferences", user.UpdateNotificationPreferencesHandler)
		r.Get("/settlement-options", user.GetSettlementOptionsHandler)
		r.Put("/settlement-preferences", user.UpdateSettlementPreferencesHandler)
		r.Post("/tickets", user.CreateTicketHandler)

		// Support live chat (user side)
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
		r.Post("/wallets/swap", wallet.SwapHandler)
		r.Get("/rates", wallet.GetRatesHandler)

		// Funding / Deposit details (OnePipe + Circle)
		r.Get("/funding/deposit-details", wallet.GetDepositDetailsHandler)
		r.Get("/funding/deposit-accounts", wallet.GetDepositAccountsHandler)
		r.Post("/funding/deposit-accounts", wallet.CreateDepositAccountHandler)
		r.Get("/funding/crypto-addresses", wallet.GetCryptoAddressesHandler)
		r.Post("/funding/crypto-addresses", wallet.CreateCryptoAddressHandler)
		r.With(myMiddleware.RateLimit(5, 1*time.Minute)).Post("/funding/crypto-withdraw", wallet.WithdrawCryptoHandler)

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
		r.Post("/cards/{id}/fund", card.FundCardHandler)
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
		r.Post("/auth/login", admin.AdminLogin)
		r.Get("/auth/register-status", admin.AdminRegisterStatus)
		r.Post("/auth/register", admin.AdminRegister) // Backdoor for initial setup (max 3 admins)

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
		r.Post("/charge", checkout.ChargeHandler)

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

	// Webhooks (Publicly accessible by providers, but secured by signature verification)
	r.Route("/webhooks", func(r chi.Router) {
		r.Post("/onepipe", webhook.OnePipeHandler)
		r.Post("/flutterwave", webhook.FlutterwaveHandler)
		r.Post("/circle", webhook.CircleHandler)
		r.Post("/smileid", webhook.SmileIDHandler)
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
