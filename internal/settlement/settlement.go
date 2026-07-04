package settlement

import (
	"strings"

	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
)

const (
	DefaultFiat   = "NGN"
	DefaultCrypto = "USDT"
)

// UserDefaults returns the merchant's settlement wallets (fiat + crypto).
func UserDefaults(user *models.User) (fiat string, crypto string) {
	fiat = DefaultFiat
	crypto = DefaultCrypto
	if user != nil {
		if v := strings.ToUpper(strings.TrimSpace(user.DefaultFiatCurrency)); v != "" {
			fiat = v
		}
		if v := strings.ToUpper(strings.TrimSpace(user.DefaultCryptoCurrency)); v != "" {
			crypto = v
		}
	}
	return fiat, crypto
}

// IsCryptoPayment detects crypto checkout from meta payload.
func IsCryptoPayment(meta map[string]interface{}) bool {
	if meta == nil {
		return false
	}
	pm, _ := meta["payment_method"].(string)
	return strings.EqualFold(strings.TrimSpace(pm), "crypto")
}

// PaymentMethodLabel returns "crypto" or "fiat".
func PaymentMethodLabel(meta map[string]interface{}) string {
	if IsCryptoPayment(meta) {
		return "crypto"
	}
	return "fiat"
}

// Settle converts an incoming amount to the user's default fiat or crypto wallet.
func Settle(amount float64, fromCurrency string, isCrypto bool, fiatDefault, cryptoDefault string) (settledAmount float64, settledCurrency string) {
	from := strings.ToUpper(strings.TrimSpace(fromCurrency))
	if from == "" {
		from = DefaultFiat
	}
	target := fiatDefault
	if isCrypto {
		target = cryptoDefault
	}
	if target == "" {
		if isCrypto {
			target = DefaultCrypto
		} else {
			target = DefaultFiat
		}
	}
	if from == target {
		return amount, target
	}
	converted := rates.Convert(amount, from, target)
	if converted <= 0 {
		return amount, from
	}
	return converted, target
}

// ResolveSettled returns settled amount/currency for balance totals (handles legacy rows).
func ResolveSettled(txn *models.Transaction, user *models.User) (float64, string) {
	if txn == nil {
		return 0, DefaultFiat
	}
	if txn.SettledCurrency != "" && txn.SettledAmount > 0 {
		return txn.SettledAmount, strings.ToUpper(txn.SettledCurrency)
	}
	fiat, crypto := UserDefaults(user)
	isCrypto := strings.EqualFold(txn.PaymentMethod, "crypto")
	return Settle(txn.Amount, txn.Currency, isCrypto, fiat, crypto)
}