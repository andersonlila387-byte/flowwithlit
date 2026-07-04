package email

import (
	"os"
	"strings"
)

func AppFrontendURL() string {
	if u := strings.TrimSpace(os.Getenv("FRONTEND_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost/flowwithlit/app"
}

func CheckoutBaseURL() string {
	if u := strings.TrimSpace(os.Getenv("CHECKOUT_URL")); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://pay.flowwithlit.com"
}

func InvoicePayURL(invoiceNumber string) string {
	return CheckoutBaseURL() + "/invoice/" + invoiceNumber
}

func TicketURL() string {
	return AppFrontendURL() + "/incorporation-support.php"
}

func WithdrawURL() string {
	return AppFrontendURL() + "/transfers.php"
}

func KYCURL() string {
	return AppFrontendURL() + "/kyc.php"
}

func OnboardingURL() string {
	return AppFrontendURL() + "/onboarding.php"
}

func DashboardURL() string {
	return AppFrontendURL() + "/index.php"
}

func SettingsURL() string {
	return AppFrontendURL() + "/settings.php"
}

func TransactionsURL() string {
	return AppFrontendURL() + "/transactions.php"
}

func displayFirstName(name string) string {
	if n := strings.TrimSpace(name); n != "" {
		return n
	}
	return "there"
}

func maskAccount(acct string) string {
	acct = strings.TrimSpace(acct)
	if len(acct) <= 4 {
		return acct
	}
	return "****" + acct[len(acct)-4:]
}