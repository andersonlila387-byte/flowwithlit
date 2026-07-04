package email

import (
	"fmt"
	"strings"
	"time"
)

func formatTimestamp(t time.Time) string {
	return t.Format("Jan 02, 2006 at 03:04 PM")
}

func amountStr(amount float64) string {
	return fmt.Sprintf("%.2f", amount)
}

// SendNewUserWelcome welcomes a user after email verification.
func SendNewUserWelcome(to, firstName string) error {
	return sendTemplate(to, "Welcome to Flowwithlit", "newuser", map[string]interface{}{
		"first_name":     displayFirstName(firstName),
		"dashboard_url":  DashboardURL(),
		"onboarding_url": OnboardingURL(),
	})
}

// SendPasswordChanged notifies the user their password was updated.
func SendPasswordChanged(to, firstName string, changedAt time.Time) error {
	return sendTemplate(to, "Password Changed — Flowwithlit", "password-changed", map[string]interface{}{
		"first_name":    displayFirstName(firstName),
		"changed_at":    formatTimestamp(changedAt),
		"settings_url":  SettingsURL(),
	})
}

// Send2FAEnabled confirms two-factor authentication is active.
func Send2FAEnabled(to, firstName string, enabledAt time.Time) error {
	return sendTemplate(to, "Two-Factor Authentication Enabled", "2fa-enabled", map[string]interface{}{
		"first_name":  displayFirstName(firstName),
		"method":      "Authenticator app",
		"enabled_at":  formatTimestamp(enabledAt),
		"settings_url": SettingsURL(),
	})
}

// Send2FADisabled confirms two-factor authentication was turned off.
func Send2FADisabled(to, firstName string, disabledAt time.Time) error {
	return sendTemplate(to, "Two-Factor Authentication Disabled", "2fa-disabled", map[string]interface{}{
		"first_name":   displayFirstName(firstName),
		"disabled_at":  formatTimestamp(disabledAt),
		"settings_url": SettingsURL(),
	})
}

// SendSuspiciousActivity alerts the user about unusual account activity.
func SendSuspiciousActivity(to, firstName, summary, ipAddress string, detectedAt time.Time) error {
	return sendTemplate(to, "Suspicious Activity Detected — Flowwithlit", "suspicious-activity", map[string]interface{}{
		"first_name":        displayFirstName(firstName),
		"activity_summary":  summary,
		"ip_address":        ipAddress,
		"detected_at":       formatTimestamp(detectedAt),
		"settings_url":      SettingsURL(),
	})
}

// SendTicketCreated confirms a support ticket was opened.
func SendTicketCreated(to, firstName, ticketRef, subject, category, priority, status string) error {
	return sendTemplate(to, "Support Ticket Received — "+ticketRef, "ticket-created", map[string]interface{}{
		"first_name": displayFirstName(firstName),
		"ticket_ref": ticketRef,
		"subject":    subject,
		"category":   category,
		"priority":   priority,
		"status":     status,
		"ticket_url": TicketURL(),
	})
}

// SendTicketReply notifies the user of an admin reply on their ticket.
func SendTicketReply(to, firstName, ticketRef, subject, adminReply, status string) error {
	return sendTemplate(to, "Reply on Ticket "+ticketRef, "ticket-reply", map[string]interface{}{
		"first_name":  displayFirstName(firstName),
		"ticket_ref":  ticketRef,
		"subject":     subject,
		"admin_reply": adminReply,
		"status":      status,
		"ticket_url":  TicketURL(),
	})
}

// SendKYCApproved notifies the merchant their KYC was approved.
func SendKYCApproved(to, firstName string) error {
	return sendTemplate(to, "KYC Approved — Flowwithlit", "kyc-approved", map[string]interface{}{
		"first_name":    displayFirstName(firstName),
		"dashboard_url": DashboardURL(),
	})
}

// SendKYCRejected notifies the merchant their KYC was rejected.
func SendKYCRejected(to, firstName, reason string) error {
	return sendTemplate(to, "KYC Update Required — Flowwithlit", "kyc-rejected", map[string]interface{}{
		"first_name": displayFirstName(firstName),
		"reason":     reason,
		"kyc_url":    KYCURL(),
	})
}

// SendKYCNeedsInfo asks the merchant for more KYC information.
func SendKYCNeedsInfo(to, firstName, message string) error {
	return sendTemplate(to, "KYC: More Information Needed", "kyc-needs-info", map[string]interface{}{
		"first_name": displayFirstName(firstName),
		"message":    message,
		"kyc_url":    KYCURL(),
	})
}

// SendBusinessActivated confirms business profile activation (Tier 1).
func SendBusinessActivated(to, firstName, businessName, countryCode, fiatCurrency, cryptoCurrency string) error {
	return sendTemplate(to, "Business Activated — Flowwithlit", "business-activated", map[string]interface{}{
		"first_name":      displayFirstName(firstName),
		"business_name":   businessName,
		"country_code":    countryCode,
		"fiat_currency":   strings.ToUpper(fiatCurrency),
		"crypto_currency": strings.ToUpper(cryptoCurrency),
		"onboarding_url":  OnboardingURL(),
		"dashboard_url":   DashboardURL(),
	})
}

// SendInvoiceSent emails the customer an invoice.
func SendInvoiceSent(to, customerName, merchantName, invoiceNumber, description, dueDate string, amount float64, currency string) error {
	return sendTemplate(to, "Invoice "+invoiceNumber+" from "+merchantName, "invoice-sent", map[string]interface{}{
		"customer_name":  customerName,
		"merchant_name":  merchantName,
		"invoice_number": invoiceNumber,
		"description":    description,
		"due_date":       dueDate,
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"pay_url":        InvoicePayURL(invoiceNumber),
	})
}

// SendInvoiceReminder reminds the customer to pay an overdue invoice.
func SendInvoiceReminder(to, customerName, invoiceNumber, dueDate string, daysOverdue int, amount float64, currency string) error {
	return sendTemplate(to, "Invoice Reminder — "+invoiceNumber, "invoice-reminder", map[string]interface{}{
		"customer_name":  customerName,
		"invoice_number": invoiceNumber,
		"due_date":       dueDate,
		"days_overdue":   fmt.Sprintf("%d", daysOverdue),
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"pay_url":        InvoicePayURL(invoiceNumber),
	})
}

// SendInvoicePaid notifies the merchant that an invoice was paid.
func SendInvoicePaid(to, firstName, invoiceNumber, customerEmail, reference string, amount float64, currency string, paidAt time.Time) error {
	return sendTemplate(to, "Invoice Paid — "+invoiceNumber, "invoice-paid", map[string]interface{}{
		"first_name":     displayFirstName(firstName),
		"invoice_number": invoiceNumber,
		"customer_email": customerEmail,
		"reference":      reference,
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"paid_at":        formatTimestamp(paidAt),
	})
}

// SendWithdrawalInitiated confirms a bank withdrawal is processing.
func SendWithdrawalInitiated(to, firstName, bankName, accountMasked, reference, eta string, amount float64, currency string) error {
	return sendTemplate(to, "Withdrawal Initiated — Flowwithlit", "withdrawal-initiated", map[string]interface{}{
		"first_name":     displayFirstName(firstName),
		"bank_name":      bankName,
		"account_masked": accountMasked,
		"reference":      reference,
		"eta":            eta,
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
	})
}

// SendWithdrawalCompleted confirms a bank withdrawal completed.
func SendWithdrawalCompleted(to, firstName, bankName, accountMasked, reference string, amount float64, currency string, completedAt time.Time) error {
	return sendTemplate(to, "Withdrawal Completed — Flowwithlit", "withdrawal-completed", map[string]interface{}{
		"first_name":     displayFirstName(firstName),
		"bank_name":      bankName,
		"account_masked": accountMasked,
		"reference":      reference,
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"completed_at":   formatTimestamp(completedAt),
	})
}

// SendWithdrawalFailed notifies the user a withdrawal could not be completed.
func SendWithdrawalFailed(to, firstName, reference, failureReason string, amount float64, currency string) error {
	return sendTemplate(to, "Withdrawal Failed — Flowwithlit", "withdrawal-failed", map[string]interface{}{
		"first_name":     displayFirstName(firstName),
		"reference":      reference,
		"failure_reason": failureReason,
		"amount":         amountStr(amount),
		"currency":       strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"withdraw_url":   WithdrawURL(),
	})
}

// SendFlowTagPaymentCompleted notifies the sender their FlowTag was claimed.
func SendFlowTagPaymentCompleted(to, firstName, payerEmail, reference string, amount float64, currency string, paidAt time.Time) error {
	return sendTemplate(to, "FlowTag Payment Completed", "flowtag-payment-completed", map[string]interface{}{
		"first_name":       displayFirstName(firstName),
		"payer_email":      payerEmail,
		"reference":        reference,
		"amount":           amountStr(amount),
		"currency":         strings.ToUpper(currency),
		"currency_symbol":  currencySymbol(currency),
		"paid_at":          formatTimestamp(paidAt),
		"transactions_url": TransactionsURL(),
	})
}

// SendBroadcast emails a platform announcement to a user.
func SendBroadcast(to, title, message, ctaURL, ctaLabel string) error {
	if strings.TrimSpace(ctaURL) == "" {
		ctaURL = DashboardURL()
	}
	if strings.TrimSpace(ctaLabel) == "" {
		ctaLabel = "Open Dashboard"
	}
	subject := strings.TrimSpace(title)
	if subject == "" {
		subject = "Announcement — Flowwithlit"
	}
	return sendTemplate(to, subject, "broadcast", map[string]interface{}{
		"title":     title,
		"message":   message,
		"cta_url":   ctaURL,
		"cta_label": ctaLabel,
	})
}

// SendSecureTransferClaimed notifies the sender their E-Transfer was claimed.
func SendSecureTransferClaimed(to, firstName, recipientEmail, reference string, amount float64, currency string, claimedAt time.Time) error {
	return sendTemplate(to, "E-Transfer Claimed", "secure-transfer-claimed", map[string]interface{}{
		"first_name":      displayFirstName(firstName),
		"recipient_email": recipientEmail,
		"reference":       reference,
		"amount":          amountStr(amount),
		"currency":        strings.ToUpper(currency),
		"currency_symbol": currencySymbol(currency),
		"claimed_at":      formatTimestamp(claimedAt),
	})
}