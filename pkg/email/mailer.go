package email

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
)

// Config holds the SMTP configuration
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// GetConfig returns the SMTP configuration
func GetConfig() Config {
	portStr := os.Getenv("SMTP_PORT")
	port := 2525 // Default Mailtrap port
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	host := os.Getenv("SMTP_HOST")
	if host == "" {
		host = "sandbox.smtp.mailtrap.io"
	}

	username := os.Getenv("SMTP_USER")
	if username == "" {
		username = "00db2bcfc8dae7" // Mailtrap Username
	}

	password := os.Getenv("SMTP_PASS")
	if password == "" {
		password = "****99a0" // Replace ****99a0 with the real password from Mailtrap
	}

	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "noreply@flowwithlit.com"
	}

	return Config{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
	}
}

// SendEmail sends HTML via PHP mail/dispatch.php → PHPMailer (no Go SMTP unless EMAIL_ALLOW_GOMAIL_FALLBACK=true).
func SendEmail(to string, subject string, htmlBody string) error {
	phpErr := sendViaPHP(to, subject, "", htmlBody, nil)
	if phpErr == nil {
		return nil
	}
	if allowGomailFallback() {
		log.Printf("⚠️ PHP mail dispatch failed, falling back to gomail: %v", phpErr)
		return SendEmailGomail(to, subject, htmlBody)
	}
	return fmt.Errorf("php mail dispatch failed: %w", phpErr)
}

// SendEmailGomail sends directly through SMTP (fallback).
func SendEmailGomail(to string, subject string, htmlBody string) error {
	cfg := GetConfig()

	m := gomail.NewMessage()
	m.SetAddressHeader("From", cfg.From, "Flowwithlit")
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	d := gomail.NewDialer(cfg.Host, cfg.Port, cfg.Username, cfg.Password)

	if err := d.DialAndSend(m); err != nil {
		log.Printf("❌ Failed to send email to %s: %v", to, err)
		return err
	}

	log.Printf("📩 Successfully sent email to %s (Subject: %s)", to, subject)
	return nil
}

// SendPasswordResetOTP sends the 6-digit code for password recovery
func SendPasswordResetOTP(to string, firstName string, otp string) error {
	return sendTemplate(to, "Password Reset Code — Flowwithlit", "password-reset", map[string]interface{}{
		"first_name": firstName, "otp": otp, "expiry_minutes": "15",
	})
}

// SendEmailVerificationOTP sends the 6-digit code for account activation
func SendEmailVerificationOTP(to string, firstName string, otp string) error {
	return sendTemplate(to, "Verify Your Email — Flowwithlit", "verify-email", map[string]interface{}{
		"first_name": firstName, "otp": otp, "expiry_minutes": "15",
	})
}

// SendFlowTagReceived sends the classic FlowTag notification email
func SendFlowTagReceived(to string, senderName string, senderEmail string, amount float64, currency string, claimURL string, expiresIn string) error {
	subject := fmt.Sprintf("You have received a payment of %s %.2f", currency, amount)
	return sendTemplate(to, subject, "flowtag-received", map[string]interface{}{
		"sender_name": senderName, "sender_email": senderEmail,
		"amount": fmt.Sprintf("%.2f", amount), "currency": currency,
		"currency_symbol": currencySymbol(currency), "claim_url": claimURL, "expires_in": expiresIn,
	})
}

// SendSecureTransferReceived notifies recipient of escrow transfer (no access key in email).
func SendSecureTransferReceived(to string, senderName string, senderEmail string, amount float64, currency string, claimPageURL string, expiresIn string) error {
	subject := fmt.Sprintf("%s sent you %s %.2f via E-Transfer", senderName, currency, amount)
	return sendTemplate(to, subject, "secure-transfer-received", map[string]interface{}{
		"sender_name": senderName, "sender_email": senderEmail,
		"amount": fmt.Sprintf("%.2f", amount), "currency": currency,
		"currency_symbol": currencySymbol(currency), "claim_url": claimPageURL, "expires_in": expiresIn,
	})
}

// SendTemplateMail sends a named HTML template from email/template/ via PHPMailer.
func SendTemplateMail(to, subject, template string, vars map[string]interface{}) error {
	return sendTemplate(to, subject, template, vars)
}

// SendAdminInvite emails a new staff member their invite link (admin subdomain URL).
func SendAdminInvite(to, role, inviteLink string) error {
	return sendTemplate(to, "You've been invited to join Flowwithlit Admin", "admin-invite", map[string]interface{}{
		"recipient_email": to,
		"role":            role,
		"invite_link":     inviteLink,
		"expires_in":      "48 hours",
	})
}

// SendLoginAlert notifies user of a new login session.
func SendLoginAlert(to, firstName, ipAddress string) error {
	return sendTemplate(to, "New Login Alert — Flowwithlit", "login-alert", map[string]interface{}{
		"first_name":  firstName,
		"ip_address":  ipAddress,
		"login_time":  time.Now().Format("Jan 02, 2006 at 03:04 PM"),
		"location":    "Unknown",
		"device":      "Web Browser",
		"browser":     "Unknown",
	})
}

// SendTransferSent notifies sender of an internal wallet transfer.
func SendTransferSent(to, firstName, recipientEmail string, amount float64, currency string) error {
	return sendTemplate(to, "Transfer Sent", "transfer-sent", map[string]interface{}{
		"first_name": firstName, "recipient_email": recipientEmail,
		"amount": fmt.Sprintf("%.2f", amount), "currency": currency,
		"currency_symbol": currencySymbol(currency),
	})
}

// SendTransferReceived notifies recipient of an internal wallet transfer.
func SendTransferReceived(to, firstName, senderEmail string, amount float64, currency string) error {
	return sendTemplate(to, "Transfer Received", "transfer-received", map[string]interface{}{
		"first_name": firstName, "sender_email": senderEmail,
		"amount": fmt.Sprintf("%.2f", amount), "currency": currency,
		"currency_symbol": currencySymbol(currency),
	})
}

// SendPaymentReceiptCustomer emails the payer a receipt (Paystack-style proof of payment).
func SendPaymentReceiptCustomer(
	to, customerName, reference string,
	amount float64, currency, paymentMethod, paidAt string,
	merchantName, merchantSupportEmail string,
	isTest bool,
) error {
	subject := fmt.Sprintf("Payment receipt — %s %s%.2f", strings.ToUpper(currency), currencySymbol(currency), amount)
	if isTest {
		subject = "[TEST] " + subject
	}
	vars := map[string]interface{}{
		"customer_name":          customerName,
		"merchant_name":          merchantName,
		"merchant_support_email": merchantSupportEmail,
		"reference":              reference,
		"amount":                 fmt.Sprintf("%.2f", amount),
		"currency":               strings.ToUpper(currency),
		"currency_symbol":        currencySymbol(currency),
		"payment_method":         paymentMethod,
		"paid_at":                paidAt,
	}
	return sendTemplate(to, subject, "payment-receipt-customer", vars)
}

// SendPaymentReceivedMerchant notifies the merchant of a successful checkout payment.
func SendPaymentReceivedMerchant(
	to, firstName, customerEmail, reference string,
	checkoutAmount float64, checkoutCurrency string,
	settledAmount float64, settledCurrency string,
	isTest bool,
) error {
	subject := fmt.Sprintf("Payment received — %s %s%.2f", strings.ToUpper(settledCurrency), currencySymbol(settledCurrency), settledAmount)
	if isTest {
		subject = "[TEST] " + subject
	}
	return sendTemplate(to, subject, "payment-received-merchant", map[string]interface{}{
		"first_name":       firstName,
		"customer_email":   customerEmail,
		"reference":        reference,
		"checkout_currency": strings.ToUpper(checkoutCurrency),
		"settled_amount":   fmt.Sprintf("%.2f", settledAmount),
		"settled_currency": strings.ToUpper(settledCurrency),
		"currency_symbol":  currencySymbol(settledCurrency),
	})
}
