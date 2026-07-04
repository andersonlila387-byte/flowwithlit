package email

import (
	"fmt"
	"strings"
	"time"
)

// TestEmailTemplate describes a sendable email template for the test harness.
type TestEmailTemplate struct {
	ID      string                 `json:"id"`
	Label   string                 `json:"label"`
	Subject string                 `json:"subject"`
	Vars    map[string]interface{} `json:"vars"`
}

func testNow() time.Time {
	return time.Now()
}

func testTimestamp() string {
	return formatTimestamp(testNow())
}

func baseSampleVars() map[string]interface{} {
	return map[string]interface{}{
		"first_name":      "Alex",
		"customer_name":   "Alex Morgan",
		"otp":             "482910",
		"expiry_minutes":  "15",
		"reference":       "TXN-TEST-20260703120000",
		"amount":          "15000.00",
		"currency":        "NGN",
		"currency_symbol": "₦",
		"dashboard_url":   DashboardURL(),
		"settings_url":    SettingsURL(),
		"onboarding_url":  OnboardingURL(),
		"kyc_url":         KYCURL(),
		"transactions_url": TransactionsURL(),
		"ticket_url":      TicketURL(),
		"withdraw_url":    WithdrawURL(),
		"claim_url":       AppFrontendURL() + "/claim-transfer.php",
		"pay_url":         InvoicePayURL("INV-TEST-001"),
		"invite_link":     AppFrontendURL() + "/admin/accept-invite.php?token=test-token",
	}
}

// TestEmailCatalog returns every HTML template with sample placeholder data.
func TestEmailCatalog() []TestEmailTemplate {
	now := testTimestamp()
	base := baseSampleVars()

	catalog := []TestEmailTemplate{
		{"verify-email", "Verify Email", "Verify Your Email — Flowwithlit", pick(base, "first_name", "otp", "expiry_minutes")},
		{"password-reset", "Password Reset", "Password Reset Code — Flowwithlit", pick(base, "first_name", "otp", "expiry_minutes")},
		{"login-alert", "Login Alert", "New Login Alert — Flowwithlit", merge(base, map[string]interface{}{
			"ip_address": "102.89.44.12", "login_time": now, "location": "Lagos, NG", "device": "Windows PC", "browser": "Chrome",
		})},
		{"newuser", "New User Welcome", "Welcome to Flowwithlit", pick(base, "first_name", "dashboard_url", "onboarding_url")},
		{"password-changed", "Password Changed", "Password Changed — Flowwithlit", merge(base, map[string]interface{}{"changed_at": now})},
		{"2fa-enabled", "2FA Enabled", "Two-Factor Authentication Enabled", merge(base, map[string]interface{}{"method": "Authenticator app", "enabled_at": now})},
		{"2fa-disabled", "2FA Disabled", "Two-Factor Authentication Disabled", merge(base, map[string]interface{}{"disabled_at": now})},
		{"suspicious-activity", "Suspicious Activity", "Suspicious Activity Detected — Flowwithlit", merge(base, map[string]interface{}{
			"activity_summary": "Failed two-factor authentication during login", "ip_address": "102.89.44.12", "detected_at": now,
		})},
		{"ticket-created", "Ticket Created", "Support Ticket Received — TKT-TEST-001", merge(base, map[string]interface{}{
			"ticket_ref": "TKT-TEST-001", "subject": "Cannot withdraw to my bank", "category": "Payments", "priority": "High", "status": "Open",
		})},
		{"ticket-reply", "Ticket Reply", "Reply on Ticket TKT-TEST-001", merge(base, map[string]interface{}{
			"ticket_ref": "TKT-TEST-001", "subject": "Cannot withdraw to my bank",
			"admin_reply": "We have escalated your withdrawal. You should see funds within 24 hours.", "status": "In Progress",
		})},
		{"kyc-approved", "KYC Approved", "KYC Approved — Flowwithlit", pick(base, "first_name", "dashboard_url")},
		{"kyc-rejected", "KYC Rejected", "KYC Update Required — Flowwithlit", merge(base, map[string]interface{}{
			"reason": "The uploaded ID document was blurry. Please re-upload a clear photo.",
		})},
		{"kyc-needs-info", "KYC Needs Info", "KYC: More Information Needed", merge(base, map[string]interface{}{
			"message": "Please upload a recent utility bill showing your business address.",
		})},
		{"kyc-reminder", "KYC Reminder", "Complete Your Verification — Flowwithlit", merge(base, map[string]interface{}{
			"tier_note": "You are on Tier 1 — complete Tier 2 to unlock higher limits.", "kyc_level": 1,
		})},
		{"business-activated", "Business Activated", "Business Activated — Flowwithlit", merge(base, map[string]interface{}{
			"business_name": "Acme Stores Ltd", "country_code": "NG", "fiat_currency": "NGN", "crypto_currency": "USDT",
		})},
		{"invoice-sent", "Invoice Sent", "Invoice INV-TEST-001 from Acme Stores", merge(base, map[string]interface{}{
			"merchant_name": "Acme Stores", "invoice_number": "INV-TEST-001", "description": "Website design — March 2026", "due_date": "Jul 15, 2026",
		})},
		{"invoice-reminder", "Invoice Reminder", "Invoice Reminder — INV-TEST-001", merge(base, map[string]interface{}{
			"invoice_number": "INV-TEST-001", "due_date": "Jul 15, 2026", "days_overdue": "3",
		})},
		{"invoice-paid", "Invoice Paid", "Invoice Paid — INV-TEST-001", merge(base, map[string]interface{}{
			"invoice_number": "INV-TEST-001", "customer_email": "customer@example.com", "paid_at": now,
		})},
		{"withdrawal-initiated", "Withdrawal Initiated", "Withdrawal Initiated — Flowwithlit", merge(base, map[string]interface{}{
			"bank_name": "GTBank", "account_masked": "****7890", "eta": "Within 24 hours",
		})},
		{"withdrawal-completed", "Withdrawal Completed", "Withdrawal Completed — Flowwithlit", merge(base, map[string]interface{}{
			"bank_name": "GTBank", "account_masked": "****7890", "completed_at": now,
		})},
		{"withdrawal-failed", "Withdrawal Failed", "Withdrawal Failed — Flowwithlit", merge(base, map[string]interface{}{
			"failure_reason": "Bank account name mismatch. Please verify your account details.",
		})},
		{"flowtag-received", "FlowTag Received", "You have received a payment of NGN 15000.00", merge(base, map[string]interface{}{
			"sender_name": "Jordan Lee", "sender_email": "jordan@example.com", "expires_in": "72 hours",
		})},
		{"flowtag-payment-completed", "FlowTag Payment Completed", "FlowTag Payment Completed", merge(base, map[string]interface{}{
			"payer_email": "payer@example.com", "paid_at": now,
		})},
		{"secure-transfer-received", "E-Transfer Received", "Jordan Lee sent you NGN 15000.00 via E-Transfer", merge(base, map[string]interface{}{
			"sender_name": "Jordan Lee", "sender_email": "jordan@example.com", "expires_in": "72 hours",
		})},
		{"secure-transfer-claimed", "E-Transfer Claimed", "E-Transfer Claimed", merge(base, map[string]interface{}{
			"recipient_email": "recipient@example.com", "claimed_at": now,
		})},
		{"transfer-sent", "Transfer Sent", "Transfer Sent", merge(base, map[string]interface{}{"recipient_email": "recipient@example.com"})},
		{"transfer-received", "Transfer Received", "Transfer Received", merge(base, map[string]interface{}{"sender_email": "sender@example.com"})},
		{"payment-receipt-customer", "Payment Receipt (Customer)", "Payment receipt — NGN ₦15000.00", merge(base, map[string]interface{}{
			"merchant_name": "Acme Stores", "merchant_support_email": "support@acmestores.com",
			"payment_method": "Bank transfer", "paid_at": now,
		})},
		{"payment-received-merchant", "Payment Received (Merchant)", "Payment received — NGN ₦15000.00", merge(base, map[string]interface{}{
			"customer_email": "customer@example.com", "checkout_currency": "NGN", "settled_amount": "15000.00", "settled_currency": "NGN",
		})},
		{"admin-invite", "Admin Invite", "You've been invited to join Flowwithlit Admin", merge(base, map[string]interface{}{
			"recipient_email": "staff@example.com", "role": "Support Agent", "expires_in": "48 hours",
		})},
		{"security-checkup", "Security Checkup", "Secure Your Account — Flowwithlit", pick(base, "first_name", "settings_url")},
		{"account-activation", "Account Activation", "Activate Your Wallet — Flowwithlit", pick(base, "first_name", "dashboard_url")},
		{"broadcast", "Broadcast Announcement", "Platform Update — Flowwithlit", map[string]interface{}{
			"title":     "Scheduled maintenance this weekend",
			"message":   "Flowwithlit will undergo brief maintenance on Saturday 2:00–4:00 AM WAT. Wallet balances and pending transfers are unaffected.",
			"cta_url":   DashboardURL(),
			"cta_label": "Open Dashboard",
		}},
	}

	return catalog
}

// SampleForTemplate returns subject and vars for a template id, or false if unknown.
func SampleForTemplate(template string) (subject string, vars map[string]interface{}, ok bool) {
	template = strings.TrimSpace(template)
	for _, item := range TestEmailCatalog() {
		if item.ID == template {
			return item.Subject, cloneVars(item.Vars), true
		}
	}
	return "", nil, false
}

func pick(base map[string]interface{}, keys ...string) map[string]interface{} {
	out := make(map[string]interface{}, len(keys))
	for _, k := range keys {
		if v, exists := base[k]; exists {
			out[k] = v
		}
	}
	return out
}

func merge(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	out := cloneVars(base)
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func cloneVars(src map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// TestEmailSubject prefixes test sends for easy inbox filtering.
func TestEmailSubject(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = "Flowwithlit Email Test"
	}
	if strings.HasPrefix(strings.ToUpper(subject), "[TEST]") {
		return subject
	}
	return fmt.Sprintf("[TEST] %s", subject)
}