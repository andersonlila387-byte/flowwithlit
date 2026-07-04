package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type phpMailPayload struct {
	To       string                 `json:"to"`
	Subject  string                 `json:"subject"`
	Template string                 `json:"template,omitempty"`
	HTML     string                 `json:"html,omitempty"`
	Vars     map[string]interface{} `json:"vars,omitempty"`
}

type phpMailResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Attempts int    `json:"attempts"`
}

func phpMailURL() string {
	if u := strings.TrimSpace(os.Getenv("PHP_MAIL_URL")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("MAIL_DISPATCH_URL")); u != "" {
		return u
	}
	if u := strings.TrimSpace(os.Getenv("SITE_BASE_URL")); u != "" {
		return strings.TrimRight(u, "/") + "/mail/dispatch.php"
	}
	if u := strings.TrimSpace(os.Getenv("FRONTEND_URL")); u != "" {
		u = strings.TrimRight(u, "/")
		if strings.Contains(u, "://app.") {
			u = strings.Replace(u, "://app.", "://www.", 1)
			if strings.Contains(u, "://www.flowwithlit.com") {
				u = "https://flowwithlit.com"
			}
		}
		return u + "/mail/dispatch.php"
	}
	return "http://localhost/flowwithlit/mail/dispatch.php"
}

func phpMailSecret() string {
	if s := strings.TrimSpace(os.Getenv("MAIL_DISPATCH_SECRET")); s != "" {
		return s
	}
	return "flowwithlit-dev-mail-secret"
}

// MailDispatchSecret is the shared secret for PHP mail/dispatch.php and internal test routes.
func MailDispatchSecret() string {
	return phpMailSecret()
}

// MailDispatchURL is the PHP endpoint Go calls for all outbound mail.
func MailDispatchURL() string {
	return phpMailURL()
}

func allowGomailFallback() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("EMAIL_ALLOW_GOMAIL_FALLBACK")), "true")
}

func varStr(vars map[string]interface{}, key string) string {
	if vars == nil {
		return ""
	}
	v, ok := vars[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func currencySymbol(currency string) string {
	switch strings.ToUpper(currency) {
	case "NGN":
		return "₦"
	case "USD", "USDT":
		return "$"
	case "EUR":
		return "€"
	case "GBP":
		return "£"
	case "GHS":
		return "GH₵"
	default:
		return currency + " "
	}
}

func renderTemplateFallback(template string, vars map[string]interface{}) string {
	switch template {
	case "verify-email":
		return GetEmailVerificationEmail(varStr(vars, "first_name"), varStr(vars, "otp"))
	case "password-reset":
		return GetPasswordResetEmail(varStr(vars, "first_name"), varStr(vars, "otp"))
	case "login-alert":
		return GetLoginAlertEmail(varStr(vars, "first_name"), varStr(vars, "ip_address"))
	case "flowtag-received":
		amt := 0.0
		fmt.Sscanf(varStr(vars, "amount"), "%f", &amt)
		return GetFlowTagReceivedEmail(
			varStr(vars, "sender_name"), varStr(vars, "sender_email"),
			amt, varStr(vars, "currency"), varStr(vars, "claim_url"), varStr(vars, "expires_in"),
		)
	case "secure-transfer-received":
		amt := 0.0
		fmt.Sscanf(varStr(vars, "amount"), "%f", &amt)
		return GetSecureTransferReceivedEmail(
			varStr(vars, "sender_name"), varStr(vars, "sender_email"),
			amt, varStr(vars, "currency"), varStr(vars, "claim_url"), varStr(vars, "expires_in"),
		)
	case "admin-invite":
		return GetAdminInviteEmail(varStr(vars, "recipient_email"), varStr(vars, "role"), varStr(vars, "invite_link"))
	case "kyc-reminder":
		tier := 0
		fmt.Sscanf(varStr(vars, "kyc_level"), "%d", &tier)
		return GetKYCReminderEmail(varStr(vars, "first_name"), varStr(vars, "kyc_url"), tier)
	case "security-checkup":
		return GetSecurityCheckupEmail(varStr(vars, "first_name"), varStr(vars, "settings_url"))
	case "account-activation":
		return GetAccountActivationEmail(varStr(vars, "first_name"), varStr(vars, "dashboard_url"))
	case "broadcast":
		return BaseEmailTemplate(varStr(vars, "title"), fmt.Sprintf(
			`<h2 style="color:#0A192F;">%s</h2><p>%s</p><p><a href="%s">%s</a></p>`,
			varStr(vars, "title"), varStr(vars, "message"),
			varStr(vars, "cta_url"), varStr(vars, "cta_label"),
		))
	case "transfer-sent":
		return BaseEmailTemplate("Transfer Sent", fmt.Sprintf(
			`<h2 style="color:#0A192F;">Transfer Sent</h2><p>You sent <strong>%s%s</strong> to %s.</p>`,
			varStr(vars, "currency_symbol"), varStr(vars, "amount"), varStr(vars, "recipient_email"),
		))
	case "transfer-received":
		return BaseEmailTemplate("Transfer Received", fmt.Sprintf(
			`<h2 style="color:#0A192F;">Transfer Received</h2><p>You received <strong>%s%s</strong> from %s.</p>`,
			varStr(vars, "currency_symbol"), varStr(vars, "amount"), varStr(vars, "sender_email"),
		))
	case "payment-receipt-customer":
		return BaseEmailTemplate("Payment Receipt", fmt.Sprintf(
			`<h2 style="color:#0A192F;">Payment successful</h2><p>Thank you for your payment to <strong>%s</strong>.</p>`+
				`<p><strong>Amount:</strong> %s%s<br><strong>Reference:</strong> %s<br><strong>Date:</strong> %s</p>`,
			varStr(vars, "merchant_name"), varStr(vars, "currency_symbol"), varStr(vars, "amount"),
			varStr(vars, "reference"), varStr(vars, "paid_at"),
		))
	case "payment-received-merchant":
		return BaseEmailTemplate("Payment Received", fmt.Sprintf(
			`<h2 style="color:#0A192F;">New payment received</h2><p>Customer <strong>%s</strong> paid <strong>%s%s</strong> (settled: %s%s).</p>`+
				`<p><strong>Reference:</strong> %s</p>`,
			varStr(vars, "customer_email"), varStr(vars, "currency_symbol"), varStr(vars, "settled_amount"),
			varStr(vars, "currency_symbol"), varStr(vars, "settled_amount"), varStr(vars, "reference"),
		))
	default:
		return BaseEmailTemplate("Notification", "<p>Flowwithlit notification</p>")
	}
}

func sendViaPHP(to, subject, template, html string, vars map[string]interface{}) error {
	payload := phpMailPayload{To: to, Subject: subject, Template: template, HTML: html, Vars: vars}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, phpMailURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Mail-Secret", phpMailSecret())

	client := &http.Client{Timeout: 25 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("php mail dispatch unreachable: %w", err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var out phpMailResponse
	_ = json.Unmarshal(raw, &out)

	if resp.StatusCode >= 400 || !out.OK {
		msg := out.Error
		if msg == "" {
			msg = string(raw)
		}
		return fmt.Errorf("php mail dispatch failed (%d): %s", resp.StatusCode, msg)
	}

	log.Printf("📩 PHP mail sent to %s (Subject: %s, template: %s)", to, subject, template)
	return nil
}

func sendTemplate(to, subject, template string, vars map[string]interface{}) error {
	if vars == nil {
		vars = make(map[string]interface{})
	}
	for k, v := range companyTemplateVars() {
		if _, exists := vars[k]; !exists {
			vars[k] = v
		}
	}
	phpErr := sendViaPHP(to, subject, template, "", vars)
	if phpErr == nil {
		return nil
	}
	if allowGomailFallback() {
		log.Printf("⚠️ PHP template mail failed, falling back to gomail: %v", phpErr)
		return SendEmailGomail(to, subject, renderTemplateFallback(template, vars))
	}
	return fmt.Errorf("php mail dispatch failed (template %s): %w", template, phpErr)
}