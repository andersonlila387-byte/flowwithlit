package wallet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/bankrails"
	"flowwithlit/internal/database"
	"flowwithlit/internal/integration/circle"
	"flowwithlit/internal/models"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/rates"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

const maxDepositAccountsPerUser = 4

// ── Fiat deposit accounts ──────────────────────────────────────────────────────

// EnsureDefaultDepositAccount creates the user's default (own-name) fiat deposit
// account if they don't have one yet, using their BusinessProfile's currency/country
// when available, else the user's own default fiat currency, else NGN. Called both
// at KYC-approval time and as a lazy fallback so nothing is ever left unset.
func EnsureDefaultDepositAccount(userID uint) (*models.DepositAccount, error) {
	var existing models.DepositAccount
	if err := database.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&existing).Error; err == nil {
		return &existing, nil
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}

	var profile models.BusinessProfile
	currency := strings.ToUpper(strings.TrimSpace(user.DefaultFiatCurrency))
	countryCode := ""
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		if profile.BaseCurrency != "" {
			currency = strings.ToUpper(profile.BaseCurrency)
		}
		countryCode = profile.CountryCode
	}
	if currency == "" {
		currency = "NGN"
	}

	return createDepositAccount(user, currency, countryCode, true)
}

func createDepositAccount(user models.User, currency, countryCode string, isDefault bool) (*models.DepositAccount, error) {
	// Idempotent: a currency the user already has just gets returned, never duplicated.
	var existing models.DepositAccount
	if err := database.DB.Where("user_id = ? AND currency = ?", user.ID, currency).First(&existing).Error; err == nil {
		return &existing, nil
	}

	rail, err := bankrails.Resolve(currency, user.FirstName, user.LastName, user.Email, user.Phone)
	if err != nil {
		return nil, err
	}
	providerName := "flutterwave"
	if rail.Provider == providers.OnePipe {
		providerName = "onepipe"
	}

	account := models.DepositAccount{
		UserID:        user.ID,
		Currency:      strings.ToUpper(currency),
		CountryCode:   countryCode,
		AccountNumber: rail.AccountNumber,
		BankName:      rail.BankName,
		AccountName:   strings.TrimSpace(user.FirstName + " " + user.LastName),
		Provider:      providerName,
		IsDefault:     isDefault,
	}
	if err := database.DB.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

// GetDepositAccountsHandler lists the user's persisted fiat deposit accounts
// (lazy-creating the default one if none exist yet) plus their crypto addresses.
func GetDepositAccountsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if _, err := EnsureDefaultDepositAccount(userID); err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not prepare your deposit account: "+err.Error())
		return
	}

	var accounts []models.DepositAccount
	database.DB.Where("user_id = ?", userID).Order("is_default desc, created_at asc").Find(&accounts)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"accounts":    accounts,
		"max_allowed": maxDepositAccountsPerUser,
	})
}

type createDepositAccountRequest struct {
	Currency string `json:"currency"`
}

// CreateDepositAccountHandler adds a deposit account in another currency, capped
// at maxDepositAccountsPerUser. Requesting a currency the user already has just
// returns the existing one (doesn't count twice, doesn't error).
func CreateDepositAccountHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req createDepositAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		response.Error(w, http.StatusBadRequest, "currency is required")
		return
	}

	var currencyRow models.Currency
	if err := database.DB.Where("code = ? AND is_enabled = ?", currency, true).First(&currencyRow).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Unsupported or disabled currency: "+currency)
		return
	}

	var existing models.DepositAccount
	if err := database.DB.Where("user_id = ? AND currency = ?", userID, currency).First(&existing).Error; err == nil {
		response.Success(w, http.StatusOK, map[string]interface{}{"account": existing, "message": "You already have a deposit account in this currency."})
		return
	}

	var count int64
	database.DB.Model(&models.DepositAccount{}).Where("user_id = ?", userID).Count(&count)
	if count >= maxDepositAccountsPerUser {
		response.Error(w, http.StatusBadRequest, "You've reached the limit of 4 deposit accounts.")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	account, err := createDepositAccount(user, currency, "", false)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not generate account: "+err.Error())
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{"account": account, "message": "Deposit account created."})
}

// ── Crypto deposit addresses ───────────────────────────────────────────────────

var cryptoNetworkByAsset = map[string]string{
	"USDT": "TRC20",
	"USDC": "TRC20",
	"BTC":  "Bitcoin",
	"ETH":  "ERC20",
	"SOL":  "Solana",
}

func mockCryptoAddress(asset string) string {
	switch strings.ToUpper(asset) {
	case "BTC":
		return "bc1q" + randomHex(20)
	case "ETH", "USDC":
		return "0x" + randomHex(20)
	case "SOL":
		return randomBase58Like(32)
	default: // USDT (TRC20) and anything else — reuse the existing Circle mock generator
		addr, _ := circle.NewClient("", "").GenerateWalletAddress("TRC20")
		return addr
	}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomBase58Like(n int) string {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	b := make([]byte, n)
	_, _ = rand.Read(b)
	out := make([]byte, n)
	for i, v := range b {
		out[i] = alphabet[int(v)%len(alphabet)]
	}
	return string(out)
}

// EnsureDefaultCryptoAddress creates the user's default USDT receiving address if
// they don't have one yet. Called at KYC-approval time and as a lazy fallback.
func EnsureDefaultCryptoAddress(userID uint) (*models.CryptoDepositAddress, error) {
	return ensureCryptoAddress(userID, "USDT")
}

func ensureCryptoAddress(userID uint, asset string) (*models.CryptoDepositAddress, error) {
	asset = strings.ToUpper(asset)
	var existing models.CryptoDepositAddress
	if err := database.DB.Where("user_id = ? AND asset = ?", userID, asset).First(&existing).Error; err == nil {
		return &existing, nil
	}

	network := cryptoNetworkByAsset[asset]
	addr := models.CryptoDepositAddress{
		UserID:  userID,
		Asset:   asset,
		Network: network,
		Address: mockCryptoAddress(asset),
	}
	if err := database.DB.Create(&addr).Error; err != nil {
		return nil, err
	}
	return &addr, nil
}

// GetCryptoAddressesHandler lists the user's persisted crypto receiving addresses
// (lazy-creating the default USDT one if none exist yet).
func GetCryptoAddressesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	if _, err := EnsureDefaultCryptoAddress(userID); err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not prepare your crypto address: "+err.Error())
		return
	}

	var addresses []models.CryptoDepositAddress
	database.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&addresses)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"addresses":        addresses,
		"settlement_asset": "USDT",
		"note":             "Funds received on any address other than USDT are converted to USDT on arrival. You can still withdraw as the original asset.",
	})
}

type createCryptoAddressRequest struct {
	Asset string `json:"asset"`
}

// CreateCryptoAddressHandler generates a receiving address for a crypto asset other
// than the default USDT (e.g. BTC) — idempotent per asset, no count cap (only fiat
// deposit accounts are capped at 4 per the product decision).
func CreateCryptoAddressHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req createCryptoAddressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	asset := strings.ToUpper(strings.TrimSpace(req.Asset))
	if asset == "" {
		response.Error(w, http.StatusBadRequest, "asset is required")
		return
	}

	var assetRow models.CryptoAsset
	if err := database.DB.Where("code = ? AND is_enabled = ?", asset, true).First(&assetRow).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Unsupported or disabled asset: "+asset)
		return
	}

	address, err := ensureCryptoAddress(userID, asset)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not generate address: "+err.Error())
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{"address": address})
}

// ── Crypto withdrawal (settles from the USDT ledger balance) ──────────────────

type withdrawCryptoRequest struct {
	Asset              string  `json:"asset"`
	Amount             float64 `json:"amount"` // in Asset terms, e.g. 0.01 BTC
	DestinationAddress string  `json:"destination_address"`
	PIN                string  `json:"pin"`
}

// WithdrawCryptoHandler lets a user withdraw as a non-settlement asset (e.g. BTC)
// even though their resting balance is tracked in USDT: it converts the requested
// asset amount to its USDT equivalent, debits that from the real USDT wallet, and
// mocks the actual on-chain send — same "real ledger movement, mocked external
// transfer" pattern as bank transfers elsewhere in this codebase.
func WithdrawCryptoHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req withdrawCryptoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	asset := strings.ToUpper(strings.TrimSpace(req.Asset))
	if asset == "" || req.Amount <= 0 || strings.TrimSpace(req.DestinationAddress) == "" {
		response.Error(w, http.StatusBadRequest, "asset, amount, and destination_address are required")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if user.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN first in Settings")
		return
	}
	dummy := models.User{Password: user.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect PIN")
		return
	}

	usdtAmount := req.Amount
	if asset != "USDT" {
		usdtAmount = rates.Convert(req.Amount, asset, "USDT")
		if usdtAmount <= 0 {
			response.Error(w, http.StatusBadRequest, "Could not price this asset right now — try again shortly")
			return
		}
	}

	ref := "CWD-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := DebitWallet(userID, usdtAmount, 0, "USDT", "internal", ref, "Crypto withdrawal ("+asset+")"); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	var usdtWallet models.Wallet
	database.DB.Where("user_id = ? AND currency = ?", userID, "USDT").First(&usdtWallet)

	database.DB.Create(&models.Transaction{
		UserID:       userID,
		Reference:    ref,
		Amount:       req.Amount,
		BalanceAfter: usdtWallet.Balance,
		Currency:     asset,
		Type:         "crypto_withdrawal",
		Status:       "successful",
		Provider:     "internal",
		Description:  "Withdrew " + strconv.FormatFloat(req.Amount, 'f', -1, 64) + " " + asset + " to " + req.DestinationAddress,
	})

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":         "Withdrawal initiated.",
		"reference":       ref,
		"asset":           asset,
		"amount":          req.Amount,
		"usdt_equivalent": usdtAmount,
	})
}
