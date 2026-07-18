package transfer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/settings"
	userPkg "flowwithlit/internal/user"
	walletPkg "flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/push"
	"flowwithlit/pkg/response"
)

// BankTransferRequest for external bank payout
type BankTransferRequest struct {
	BankCode      string  `json:"bank_code"`
	AccountNumber string  `json:"account_number"`
	AccountName   string  `json:"account_name"` // resolved name
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Description   string  `json:"description"`
	PIN           string  `json:"pin"`            // transaction PIN
	PaymentToken  string  `json:"payment_token"` // optional: short-lived biometric payment auth
}

// BankNameLookupRequest for name enquiry
type BankNameLookupRequest struct {
	BankCode      string `json:"bank_code"`
	AccountNumber string `json:"account_number"`
}

// BankNameLookupResponse
type BankNameLookupResponse struct {
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	BankCode      string `json:"bank_code"`
}

// BanksHandler returns real bank list (Flutterwave) for transfers.
func BanksHandler(w http.ResponseWriter, r *http.Request) {
	country := r.URL.Query().Get("country")
	if country == "" {
		country = "NG"
	}
	fw := settings.FlutterwaveClient()
	banks, err := fw.ListBanks(country)
	if err != nil {
		response.Error(w, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{
		"banks": banks,
	})
}

// LookupAccountHandler verifies account name via Flutterwave name enquiry.
func LookupAccountHandler(w http.ResponseWriter, r *http.Request) {
	var req BankNameLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	fw := settings.FlutterwaveClient()
	accountName, err := fw.ResolveBankAccount(req.BankCode, req.AccountNumber)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(w, http.StatusOK, BankNameLookupResponse{
		AccountName:   accountName,
		AccountNumber: req.AccountNumber,
		BankCode:      req.BankCode,
	})
}

// CreateBankTransferHandler initiates a bank payout
func CreateBankTransferHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req BankTransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Invalid amount")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	// Verify PIN or biometric payment_token (from POST /user/biometric/authorize)
	var pinUser models.User
	if err := database.DB.First(&pinUser, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	// Kids sub-accounts: no external bank cash-out (parent controls money movement)
	if pinUser.IsJunior() {
		response.Error(w, http.StatusForbidden, "Kids accounts cannot send bank transfers. Parent controls withdrawals.")
		return
	}
	if err := userPkg.VerifyDebitAuth(pinUser, req.PIN, req.PaymentToken); err != nil {
		activity.Warning("transfer", "auth_failed", err.Error(), activity.UID(userID), "", r.RemoteAddr)
		userPkg.WriteDebitAuthError(w, err)
		return
	}

	fee := 50.0 // Flat fee example
	ref := "TRF-" + time.Now().Format("20060102150405")

	bankName := req.AccountName
	if bankName == "" {
		bankName = "Bank (" + req.BankCode + ")"
	}
	accountMasked := maskBankAccount(req.AccountNumber)

	// Debit first, then live payout. On provider failure: refund + clear error (no silent mock).
	err := walletPkg.DebitWallet(userID, req.Amount, fee, req.Currency, "bank", ref, req.Description+" to "+req.AccountNumber)
	if err != nil {
		activity.Error("transfer", "debit_failed", err.Error(), activity.UID(userID), ref, r.RemoteAddr)
		_ = email.SendWithdrawalFailed(pinUser.Email, pinUser.FirstName, ref, err.Error(), req.Amount, req.Currency)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payoutProvider := providers.ForPayout(req.Currency, settings.NGNBankProvider())
	var providerRef string
	var payOK bool
	var payErr error
	switch payoutProvider {
	case providers.OnePipe:
		client := settings.OnePipeClient()
		payOK, providerRef, payErr = client.ProcessTransfer(req.Amount, req.BankCode, req.AccountNumber, req.Description)
	case providers.PalmPay:
		client := settings.PalmPayClient()
		payOK, providerRef, payErr = client.ProcessTransfer(req.Amount, req.BankCode, req.AccountNumber, req.Description)
	default:
		client := settings.FlutterwaveClient()
		payOK, providerRef, payErr = client.ProcessTransfer(req.Amount, req.Currency, req.BankCode, req.AccountNumber, req.Description)
	}
	if !payOK || payErr != nil {
		_ = walletPkg.FundWallet(userID, req.Amount+fee, req.Currency, "refund", ref+"-RFND", "Refund failed bank transfer: "+ref)
		msg := "Bank transfer provider failed"
		if payErr != nil {
			msg = payErr.Error()
		}
		activity.Error("transfer", "provider_failed", msg, activity.UID(userID), ref, r.RemoteAddr)
		_ = email.SendWithdrawalFailed(pinUser.Email, pinUser.FirstName, ref, msg, req.Amount, req.Currency)
		response.Error(w, http.StatusBadGateway, msg)
		return
	}
	if providerRef == "" {
		providerRef = ref
	}

	activity.Success("transfer", "bank_sent", "Bank transfer "+ref, activity.UID(userID), ref, r.RemoteAddr)

	_ = email.SendWithdrawalInitiated(
		pinUser.Email, pinUser.FirstName, bankName, accountMasked, ref,
		"1-3 business days", req.Amount, req.Currency,
	)
	_ = push.SendToUser(userID, "Transfer sent",
		fmt.Sprintf("You sent %s %.2f to %s (%s)", req.Currency, req.Amount, accountMasked, ref),
		map[string]string{"type": "transfer", "reference": ref},
	)

	txn := models.Transaction{
		UserID:            userID,
		Reference:         ref,
		Amount:            req.Amount,
		Fee:               fee,
		Currency:          req.Currency,
		Type:              "bank_transfer",
		Status:            "successful",
		Provider:          payoutProvider,
		ProviderReference: providerRef,
		Customer:          req.AccountNumber,
		Description:       req.Description,
	}
	database.DB.Create(&txn)

	_ = email.SendWithdrawalCompleted(
		pinUser.Email, pinUser.FirstName, bankName, accountMasked, ref,
		req.Amount, req.Currency, time.Now(),
	)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":      "Transfer initiated successfully",
		"reference":    ref,
		"amount":       req.Amount,
		"fee":          fee,
		"status":       "successful",
		"account":      req.AccountNumber,
		"account_name": req.AccountName,
	})
}

// BulkTransferHandler processes multiple bank payouts in one request
func BulkTransferHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Recipients []struct {
			BankCode      string  `json:"bank_code"`
			AccountNumber string  `json:"account_number"`
			AccountName   string  `json:"account_name"`
			Amount        float64 `json:"amount"`
			Description   string  `json:"description"`
		} `json:"recipients"`
		Currency string `json:"currency"`
		PIN      string `json:"pin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	if len(req.Recipients) == 0 {
		response.Error(w, http.StatusBadRequest, "At least one recipient is required")
		return
	}
	if req.PIN == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	var pinUser models.User
	if err := database.DB.First(&pinUser, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your Transaction PIN first in Settings")
		return
	}
	dummy := models.User{Password: pinUser.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	const fee = 50.0
	totalAmount := 0.0
	for _, r := range req.Recipients {
		totalAmount += r.Amount + fee
	}

	// Debit total, then live payouts. Any provider failure refunds that leg (no silent mock).
	batchRef := "BULK-" + time.Now().Format("20060102150405")
	if err := walletPkg.DebitWallet(userID, totalAmount, 0, req.Currency, "bulk_transfer", batchRef, "Bulk salary disbursement"); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	payoutProvider := providers.ForPayout(req.Currency, settings.NGNBankProvider())

	type Result struct {
		AccountNumber string  `json:"account_number"`
		AccountName   string  `json:"account_name"`
		Amount        float64 `json:"amount"`
		Reference     string  `json:"reference"`
		Status        string  `json:"status"`
		Error         string  `json:"error,omitempty"`
	}

	results := make([]Result, 0, len(req.Recipients))
	successCount := 0
	for i, rec := range req.Recipients {
		ref := fmt.Sprintf("%s-%d", batchRef, i+1)
		var payOK bool
		var pRef string
		var payErr error
		switch payoutProvider {
		case providers.OnePipe:
			payOK, pRef, payErr = settings.OnePipeClient().ProcessTransfer(rec.Amount, rec.BankCode, rec.AccountNumber, rec.Description)
		case providers.PalmPay:
			payOK, pRef, payErr = settings.PalmPayClient().ProcessTransfer(rec.Amount, rec.BankCode, rec.AccountNumber, rec.Description)
		default:
			payOK, pRef, payErr = settings.FlutterwaveClient().ProcessTransfer(rec.Amount, req.Currency, rec.BankCode, rec.AccountNumber, rec.Description)
		}
		status := "successful"
		errMsg := ""
		if !payOK || payErr != nil {
			status = "failed"
			if payErr != nil {
				errMsg = payErr.Error()
			} else {
				errMsg = "provider rejected transfer"
			}
			_ = walletPkg.FundWallet(userID, rec.Amount+fee, req.Currency, "refund", ref+"-RFND", "Refund failed bulk leg: "+ref)
		} else {
			successCount++
		}
		if pRef == "" {
			pRef = ref
		}
		txn := models.Transaction{
			UserID:            userID,
			Reference:         ref,
			Amount:            rec.Amount,
			Fee:               fee,
			Currency:          req.Currency,
			Type:              "bank_transfer",
			Status:            status,
			Provider:          payoutProvider,
			ProviderReference: pRef,
			Customer:          rec.AccountNumber,
			Description:       rec.Description,
		}
		database.DB.Create(&txn)
		results = append(results, Result{
			AccountNumber: rec.AccountNumber,
			AccountName:   rec.AccountName,
			Amount:        rec.Amount,
			Reference:     ref,
			Status:        status,
			Error:         errMsg,
		})
	}

	if successCount == 0 {
		response.Error(w, http.StatusBadGateway, "All bulk transfers failed — money refunded. Check provider keys (see key-get.md).")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":      "Bulk transfer processed",
		"batch_ref":    batchRef,
		"total_sent":   successCount,
		"total_failed": len(results) - successCount,
		"total_amount": totalAmount,
		"results":      results,
	})
}

// GetTransfersHandler returns user's transfer history
func GetTransfersHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var txns []models.Transaction
	database.DB.Where("user_id = ? AND type IN ?", userID, []string{"bank_transfer", "transfer_out", "transfer_in"}).
		Order("created_at desc").
		Limit(50).
		Find(&txns)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"transfers": txns,
	})
}

func maskBankAccount(acct string) string {
	acct = strings.TrimSpace(acct)
	if len(acct) <= 4 {
		return acct
	}
	return "****" + acct[len(acct)-4:]
}
