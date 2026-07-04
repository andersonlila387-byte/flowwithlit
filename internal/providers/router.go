package providers

import "strings"

// Payment rails: NGN → OnePipe (bank/VA/payouts). All other fiat → Flutterwave.
// Card payments always use Flutterwave (OnePipe does not process cards).

const (
	OnePipe     = "onepipe"
	Flutterwave = "flutterwave"
)

// ForBankTransfer picks the bank-rail provider for a currency.
func ForBankTransfer(currency string) string {
	if strings.ToUpper(strings.TrimSpace(currency)) == "NGN" {
		return OnePipe
	}
	return Flutterwave
}

// ForCard always returns Flutterwave.
func ForCard() string {
	return Flutterwave
}

// ForPayout picks the payout provider for a currency.
func ForPayout(currency string) string {
	return ForBankTransfer(currency)
}

// UsesOnePipe reports whether NGN OnePipe rails apply.
func UsesOnePipe(currency string) bool {
	return ForBankTransfer(currency) == OnePipe
}