package vault

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

// GetVaultsHandler lists user's vaults
func GetVaultsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var vaults []models.Vault
	database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&vaults)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"vaults": vaults,
	})
}

// CreateVaultRequest
type CreateVaultRequest struct {
	Name         string  `json:"name"`
	Currency     string  `json:"currency"`
	TargetAmount float64 `json:"target_amount"`
	APY          float64 `json:"apy"`
	LockDays     int     `json:"lock_days"` // number of days to lock
}

// CreateVaultHandler
func CreateVaultHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateVaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	if req.Name == "" {
		req.Name = "Savings Vault"
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}
	if req.APY == 0 {
		req.APY = 10.5
	}

	lockUntil := time.Now().AddDate(0, 0, req.LockDays)
	if req.LockDays == 0 {
		lockUntil = time.Now().AddDate(0, 3, 0) // default 3 months
	}

	vault := models.Vault{
		UserID:        userID,
		Name:          req.Name,
		Currency:      req.Currency,
		TargetAmount:  req.TargetAmount,
		CurrentAmount: 0,
		APY:           req.APY,
		LockUntil:     &lockUntil,
		Status:        "active",
	}

	if err := database.DB.Create(&vault).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create vault")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Vault created",
		"vault":   vault,
	})
}

// DepositToVaultRequest
type DepositToVaultRequest struct {
	Amount float64 `json:"amount"`
	PIN    string  `json:"pin"`
}

// DepositToVaultHandler
func DepositToVaultHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vaultIDStr := chi.URLParam(r, "id")
	vaultID, _ := strconv.Atoi(vaultIDStr)

	var req DepositToVaultRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Invalid amount")
		return
	}

	// Verify PIN
	if req.PIN == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required")
		return
	}
	var pinUser models.User
	database.DB.First(&pinUser, userID)
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN first in Settings")
		return
	}
	dummy := models.User{Password: pinUser.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect PIN")
		return
	}

	var vault models.Vault
	if err := database.DB.Where("id = ? AND user_id = ?", vaultID, userID).First(&vault).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Vault not found")
		return
	}

	// Debit user's main wallet for the currency
	var wallet models.Wallet
	if err := database.DB.Where("user_id = ? AND currency = ?", userID, vault.Currency).First(&wallet).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Wallet not found for this currency")
		return
	}
	if wallet.Balance < req.Amount {
		response.Error(w, http.StatusBadRequest, "Insufficient balance")
		return
	}

	wallet.Balance -= req.Amount
	database.DB.Save(&wallet)

	vault.CurrentAmount += req.Amount
	database.DB.Save(&vault)

	// Record transaction
	ref := "VAULT-" + time.Now().Format("20060102150405")
	database.DB.Create(&models.Transaction{
		UserID:       userID,
		Reference:    ref,
		Amount:       req.Amount,
		Currency:     vault.Currency,
		Type:         "vault_deposit",
		Status:       "successful",
		Provider:     "internal",
		Description:  "Deposit to vault: " + vault.Name,
		BalanceAfter: wallet.Balance,
	})

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Funds locked in vault",
		"vault":   vault,
	})
}
