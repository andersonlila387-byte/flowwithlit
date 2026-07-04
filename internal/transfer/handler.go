package transfer

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/settings"
	walletPkg "flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
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
	PIN           string  `json:"pin"` // In production: verify against user.TransactionPin
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

	// Verify Transaction PIN
	if req.PIN == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required")
		return
	}
	var pinUser models.User
	if err := database.DB.First(&pinUser, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN first in Settings")
		return
	}
	dummy := models.User{Password: pinUser.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	fee := 50.0 // Flat fee example
	ref := "TRF-" + time.Now().Format("20060102150405")

	bankName := req.AccountName
	if bankName == "" {
		bankName = "Bank (" + req.BankCode + ")"
	}
	accountMasked := maskBankAccount(req.AccountNumber)

	// Debit the user's wallet using the manager
	err := walletPkg.DebitWallet(userID, req.Amount, fee, req.Currency, "bank", ref, req.Description+" to "+req.AccountNumber)
	if err != nil {
		_ = email.SendWithdrawalFailed(pinUser.Email, pinUser.FirstName, ref, err.Error(), req.Amount, req.Currency)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	_ = email.SendWithdrawalInitiated(
		pinUser.Email, pinUser.FirstName, bankName, accountMasked, ref,
		"1-3 business days", req.Amount, req.Currency,
	)

	payoutProvider := providers.ForPayout(req.Currency)
	var providerRef string
	if payoutProvider == providers.OnePipe {
		client := settings.OnePipeClient()
		ok, pRef, _ := client.ProcessTransfer(req.Amount, req.BankCode, req.AccountNumber, req.Description)
		if ok {
			providerRef = pRef
		}
	} else {
		client := settings.FlutterwaveClient()
		ok, pRef, _ := client.ProcessTransfer(req.Amount, req.Currency, req.BankCode, req.AccountNumber, req.Description)
		if ok {
			providerRef = pRef
		}
	}
	if providerRef == "" {
		providerRef = ref
	}

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

	if txn.Status == "successful" {
		_ = email.SendWithdrawalCompleted(
			pinUser.Email, pinUser.FirstName, bankName, accountMasked, ref,
			req.Amount, req.Currency, time.Now(),
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":    "Transfer initiated successfully",
		"reference":  ref,
		"amount":     req.Amount,
		"fee":        fee,
		"status":     "successful",
		"account":    req.AccountNumber,
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

	// Debit total in one shot
	batchRef := "BULK-" + time.Now().Format("20060102150405")
	if err := walletPkg.DebitWallet(userID, totalAmount, 0, req.Currency, "bulk_transfer", batchRef, "Bulk salary disbursement"); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	type Result struct {
		AccountNumber string  `json:"account_number"`
		AccountName   string  `json:"account_name"`
		Amount        float64 `json:"amount"`
		Reference     string  `json:"reference"`
		Status        string  `json:"status"`
	}

	results := make([]Result, 0, len(req.Recipients))
	for _, rec := range req.Recipients {
		ref := "TRF-" + time.Now().Format("20060102150405.999999999")
		txn := models.Transaction{
			UserID:            userID,
			Reference:         ref,
			Amount:            rec.Amount,
			Fee:               fee,
			Currency:          req.Currency,
			Type:              "bank_transfer",
			Status:            "successful",
			Provider:          "onepipe",
			ProviderReference: ref,
			Customer:          rec.AccountNumber,
			Description:       rec.Description,
		}
		database.DB.Create(&txn)
		results = append(results, Result{
			AccountNumber: rec.AccountNumber,
			AccountName:   rec.AccountName,
			Amount:        rec.Amount,
			Reference:     ref,
			Status:        "successful",
		})
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":       "Bulk transfer processed successfully",
		"batch_ref":     batchRef,
		"total_sent":    len(results),
		"total_amount":  totalAmount,
		"results":       results,
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
