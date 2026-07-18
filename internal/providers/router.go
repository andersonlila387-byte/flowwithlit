package providers

import "strings"

// Payment rails:
//   NGN bank/VA/payouts → OnePipe by default; PalmPay when Admin sets ngn_bank_provider=palmpay
//   All other fiat      → Flutterwave
//   Card payments       → always Flutterwave (OnePipe/PalmPay do not process cards here)

const (
	OnePipe     = "onepipe"
	Flutterwave = "flutterwave"
	PalmPay     = "palmpay"
)

// NormalizeNGNProvider returns a known NGN rail id. Unknown values fall back to OnePipe
// so a bad admin setting never takes live traffic offline.
func NormalizeNGNProvider(preferred string) string {
	switch strings.ToLower(strings.TrimSpace(preferred)) {
	case PalmPay:
		return PalmPay
	case OnePipe, "":
		return OnePipe
	default:
		return OnePipe
	}
}

// ForBankTransfer picks the bank-rail provider for a currency.
// preferredNGN is ignored for non-NGN (always Flutterwave). For NGN it selects
// OnePipe or PalmPay via NormalizeNGNProvider. Empty preferred keeps OnePipe.
func ForBankTransfer(currency, preferredNGN string) string {
	if strings.ToUpper(strings.TrimSpace(currency)) == "NGN" {
		return NormalizeNGNProvider(preferredNGN)
	}
	return Flutterwave
}

// ForCard always returns Flutterwave.
func ForCard() string {
	return Flutterwave
}

// ForPayout picks the payout provider for a currency.
func ForPayout(currency, preferredNGN string) string {
	return ForBankTransfer(currency, preferredNGN)
}

// UsesOnePipe reports whether NGN OnePipe rails apply for the given preference.
func UsesOnePipe(currency, preferredNGN string) bool {
	return ForBankTransfer(currency, preferredNGN) == OnePipe
}

// UsesPalmPay reports whether NGN PalmPay rails apply for the given preference.
func UsesPalmPay(currency, preferredNGN string) bool {
	return ForBankTransfer(currency, preferredNGN) == PalmPay
}
