package checkout

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/developer"
	"flowwithlit/internal/models"
)

const bankTransferAutoConfirmAfter = 8 * time.Second

type pendingBankTransfer struct {
	UserID     uint
	AmountKobo float64
	Currency   string
	Email      string
	CreatedAt  time.Time
	Completed  bool
}

var (
	pendingBankMu        sync.Mutex
	pendingBankTransfers = make(map[string]*pendingBankTransfer)
)

func registerPendingBankTransfer(ref string, userID uint, amountKobo float64, currency, email string) {
	if ref == "" || amountKobo <= 0 {
		return
	}

	pendingBankMu.Lock()
	defer pendingBankMu.Unlock()

	pendingBankTransfers[ref] = &pendingBankTransfer{
		UserID:     userID,
		AmountKobo: amountKobo,
		Currency:   currency,
		Email:      email,
		CreatedAt:  time.Now(),
	}
}

// merchantDisplayName is shown to the paying customer on the checkout page
// ("Pay X"), so it must only ever be the merchant's registered business name
// — never the account owner's personal name or email, which would leak
// private info about who's behind the store.
func merchantDisplayName(userID uint) string {
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		if name := strings.TrimSpace(profile.BusinessName); name != "" {
			return name
		}
	}

	return "Merchant"
}

func parseAmountKobo(raw string) float64 {
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v <= 0 {
		return 0
	}
	return v
}

func bankTransferAlreadyPaid(ref string, userID uint) bool {
	var count int64
	database.DB.Model(&models.Transaction{}).
		Where("reference = ? AND user_id = ? AND status = ?", ref, userID, "successful").
		Count(&count)
	return count > 0
}

func finalizeTestBankPayment(creds *models.ApiCredentials, ref string, amountKobo float64, currency, customerEmail string) error {
	if bankTransferAlreadyPaid(ref, creds.UserID) {
		return nil
	}

	amountMajor := amountKobo / 100
	if currency == "" {
		currency = "NGN"
	}

	meta := map[string]interface{}{
		"payment_method": "bank",
		"auto_confirmed": true,
	}
	if err := recordCheckoutPayment(
		creds.UserID, ref, amountMajor, currency, true, customerEmail,
		"Bank transfer checkout from "+customerEmail, meta,
	); err != nil {
		return err
	}

	go developer.DispatchWebhook(creds.UserID, "charge.success", map[string]interface{}{
		"transaction_ref": ref,
		"amount":          amountKobo,
		"currency":        currency,
		"status":          "successful",
		"customer": map[string]interface{}{
			"email": customerEmail,
		},
		"meta": meta,
		"is_test": true,
	})

	return nil
}

func checkPendingBankTransfer(ref string, creds *models.ApiCredentials, isTest bool) (string, bool) {
	pendingBankMu.Lock()
	pending, ok := pendingBankTransfers[ref]
	if !ok {
		pendingBankMu.Unlock()
		return "pending", false
	}
	if pending.Completed || pending.UserID != creds.UserID {
		pendingBankMu.Unlock()
		if bankTransferAlreadyPaid(ref, creds.UserID) {
			return "successful", true
		}
		return "pending", false
	}

	ready := isTest && time.Since(pending.CreatedAt) >= bankTransferAutoConfirmAfter
	if ready {
		pending.Completed = true
	}
	amount := pending.AmountKobo
	currency := pending.Currency
	email := pending.Email
	pendingBankMu.Unlock()

	if bankTransferAlreadyPaid(ref, creds.UserID) {
		return "successful", true
	}

	if ready {
		if err := finalizeTestBankPayment(creds, ref, amount, currency, email); err != nil {
			return "pending", false
		}
		return "successful", true
	}

	return "pending", false
}