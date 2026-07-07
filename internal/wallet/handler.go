package wallet

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"gorm.io/gorm"
)

// GetWalletsHandler returns all wallets for the authenticated user
func GetWalletsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	env := envfilter.Parse(r)

	var wallets []models.Wallet
	if err := database.DB.Where("user_id = ?", userID).Find(&wallets).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch wallets")
		return
	}

	if len(wallets) == 0 {
		defaultWallets := []models.Wallet{
			{UserID: userID, Currency: "NGN", Balance: 0},
			{UserID: userID, Currency: "USDT", Balance: 0},
		}
		for _, w := range defaultWallets {
			database.DB.Create(&w)
		}
		wallets = defaultWallets
	}

	if env == "test" {
		sandbox := envfilter.SandboxBalances(userID)
		for i := range wallets {
			if bal, ok := sandbox[wallets[i].Currency]; ok {
				wallets[i].Balance = bal
			} else {
				wallets[i].Balance = 0
			}
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"env":     env,
		"wallets": wallets,
	})
}

// GetBalancesHandler returns a summarized balances map (used by many pages)
func GetBalancesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	env := envfilter.Parse(r)
	balances := envfilter.BalancesForEnv(userID, env)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"env":      env,
		"balances": balances,
	})
}

// SwapRequest payload
type SwapRequest struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	Amount       float64 `json:"amount"`
}

// SwapHandler performs fiat <-> crypto swap using internal transfer + rate
func SwapHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Amount must be greater than 0")
		return
	}

	rate := getSwapRate(req.FromCurrency, req.ToCurrency)
	if rate <= 0 {
		response.Error(w, http.StatusBadRequest, "Unsupported currency pair")
		return
	}

	toAmount := req.Amount * rate

	// Use a proper GORM transaction
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Debit from source wallet
		var fromWallet models.Wallet
		if err := tx.Where("user_id = ? AND currency = ?", userID, req.FromCurrency).First(&fromWallet).Error; err != nil {
			return errors.New("source wallet not found")
		}
		if fromWallet.Balance < req.Amount {
			return errors.New("insufficient balance")
		}

		fromWallet.Balance -= req.Amount
		if err := tx.Save(&fromWallet).Error; err != nil {
			return err
		}

		// Credit / create destination wallet
		var toWallet models.Wallet
		if err := tx.Where("user_id = ? AND currency = ?", userID, req.ToCurrency).First(&toWallet).Error; err != nil {
			toWallet = models.Wallet{
				UserID:   userID,
				Currency: req.ToCurrency,
				Balance:  0,
			}
			if err := tx.Create(&toWallet).Error; err != nil {
				return err
			}
		}
		toWallet.Balance += toAmount
		if err := tx.Save(&toWallet).Error; err != nil {
			return err
		}

		// Record ledger entries
		now := time.Now()
		ref := "SWAP-" + now.Format("20060102150405")

		if err := tx.Create(&models.Transaction{
			UserID:       userID,
			Reference:    ref + "-OUT",
			Amount:       req.Amount,
			BalanceAfter: fromWallet.Balance,
			Currency:     req.FromCurrency,
			Type:         "swap_out",
			Status:       "successful",
			Provider:     "internal",
			Description:  "Swap to " + req.ToCurrency,
		}).Error; err != nil {
			return err
		}

		if err := tx.Create(&models.Transaction{
			UserID:       userID,
			Reference:    ref + "-IN",
			Amount:       toAmount,
			BalanceAfter: toWallet.Balance,
			Currency:     req.ToCurrency,
			Type:         "swap_in",
			Status:       "successful",
			Provider:     "internal",
			Description:  "Swap from " + req.FromCurrency,
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":     "Swap completed",
		"from":        req.FromCurrency,
		"to":          req.ToCurrency,
		"from_amount": req.Amount,
		"to_amount":   toAmount,
		"rate":        rate,
	})
}

func getSwapRate(from, to string) float64 {
	return rates.GetRate(from, to)
}

// GetRatesHandler for frontend swap calculator
func GetRatesHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	rate := getSwapRate(from, to)
	if rate == 0 {
		rate = 1.0 // fallback
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"from": from,
		"to":   to,
		"rate": rate,
	})
}

// GetDepositDetailsHandler returns the user's default deposit instructions — a
// stable, persisted fiat account (see EnsureDefaultDepositAccount) and their
// default USDT crypto address (see EnsureDefaultCryptoAddress). Kept for backward
// compatibility with existing frontends; new frontends should prefer
// GetDepositAccountsHandler / GetCryptoAddressesHandler, which support more than
// one currency/asset per user.
func GetDepositDetailsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	account, err := EnsureDefaultDepositAccount(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not prepare your deposit account: "+err.Error())
		return
	}
	cryptoAddr, err := EnsureDefaultCryptoAddress(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not prepare your crypto address: "+err.Error())
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"fiat": map[string]interface{}{
			"currency":         account.Currency,
			"account_number":   account.AccountNumber,
			"bank_name":        account.BankName,
			"account_name":     account.AccountName,
			"payment_provider": account.Provider,
			"instructions":     "Send " + account.Currency + " to this account. Funds credit in seconds.",
		},
		"crypto": map[string]interface{}{
			"currency":     cryptoAddr.Asset,
			"network":      cryptoAddr.Network,
			"address":      cryptoAddr.Address,
			"instructions": "Send only " + cryptoAddr.Asset + " (" + cryptoAddr.Network + "). Other tokens will be lost.",
		},
	})
}
