package wallet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/bankrails"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
	"flowwithlit/internal/settings"
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

// ReprovisionDefaultDepositAccount deletes the current default fiat deposit account
// and creates a fresh one via the live bank rail. Use after keys/config change so
// users are not stuck with old mock numbers.
func ReprovisionDefaultDepositAccount(userID uint) (*models.DepositAccount, error) {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}

	currency := strings.ToUpper(strings.TrimSpace(user.DefaultFiatCurrency))
	countryCode := ""
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		if profile.BaseCurrency != "" {
			currency = strings.ToUpper(profile.BaseCurrency)
		}
		countryCode = profile.CountryCode
	}
	if currency == "" {
		currency = "NGN"
	}

	// Remove existing row for this currency so createDepositAccount hits the live rail.
	database.DB.Where("user_id = ? AND currency = ?", userID, currency).Delete(&models.DepositAccount{})

	return createDepositAccount(user, currency, countryCode, true)
}

func createDepositAccount(user models.User, currency, countryCode string, isDefault bool) (*models.DepositAccount, error) {
	// Idempotent: a currency the user already has just gets returned, never duplicated.
	var existing models.DepositAccount
	if err := database.DB.Where("user_id = ? AND currency = ?", user.ID, currency).First(&existing).Error; err == nil {
		return &existing, nil
	}

	opts := bankrails.CustomerOpts{
		UserID:  fmt.Sprintf("%d", user.ID),
		Address: "",
	}
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", user.ID).First(&profile).Error; err == nil {
		opts.Address = profile.Address
		if countryCode == "" {
			countryCode = profile.CountryCode
		}
	}
	if user.DateOfBirth != nil {
		opts.DOB = user.DateOfBirth.Format("2006-01-02")
	}

	rail, err := bankrails.ResolveWithOpts(currency, user.FirstName, user.LastName, user.Email, user.Phone, opts)
	if err != nil {
		return nil, err
	}
	// Persist the actual rail for internal settlement routing only (never show to end users).
	providerName := rail.Provider
	if providerName == "" {
		providerName = "internal"
	}

	accountName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	// Prefer registered business name only when it looks like a real business (not personal KYC placeholder).
	if bn := strings.TrimSpace(profile.BusinessName); bn != "" &&
		!strings.EqualFold(bn, "Personal Account") &&
		!strings.EqualFold(bn, accountName) &&
		strings.EqualFold(currency, "NGN") {
		accountName = bn
	}

	account := models.DepositAccount{
		UserID:        user.ID,
		Currency:      strings.ToUpper(currency),
		CountryCode:   countryCode,
		AccountNumber: rail.AccountNumber,
		BankName:      rail.BankName,
		AccountName:   accountName,
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
		response.Error(w, http.StatusInternalServerError, publicDepositErr(err))
		return
	}

	var accounts []models.DepositAccount
	database.DB.Where("user_id = ?", userID).Order("is_default desc, created_at asc").Find(&accounts)

	// Never expose payment-rail vendor names to the browser / mobile app.
	public := make([]map[string]interface{}, 0, len(accounts))
	for _, a := range accounts {
		public = append(public, map[string]interface{}{
			"id":             a.ID,
			"currency":       a.Currency,
			"country_code":   a.CountryCode,
			"account_number": a.AccountNumber,
			"bank_name":      a.BankName,
			"account_name":   a.AccountName,
			"is_default":     a.IsDefault,
			"created_at":     a.CreatedAt,
		})
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"accounts":    public,
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
		response.Success(w, http.StatusOK, map[string]interface{}{
			"account": publicDepositAccount(existing),
			"message": "You already have a deposit account in this currency.",
		})
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
		response.Error(w, http.StatusInternalServerError, publicDepositErr(err))
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"account": publicDepositAccount(*account),
		"message": "Deposit account created.",
	})
}

func publicDepositAccount(a models.DepositAccount) map[string]interface{} {
	return map[string]interface{}{
		"id":             a.ID,
		"currency":       a.Currency,
		"country_code":   a.CountryCode,
		"account_number": a.AccountNumber,
		"bank_name":      a.BankName,
		"account_name":   a.AccountName,
		"is_default":     a.IsDefault,
		"created_at":     a.CreatedAt,
	}
}

func publicDepositErr(err error) string {
	if err == nil {
		return "Could not prepare your deposit account"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	for _, v := range []string{"onepipe", "flutterwave", "palmpay", "circle", "not configured", "key-get"} {
		if strings.Contains(lower, v) {
			return "Could not prepare your deposit account right now. Please try again later or contact support."
		}
	}
	return "Could not prepare your deposit account: " + msg
}

// ── Crypto deposit addresses ───────────────────────────────────────────────────

var cryptoNetworkByAsset = map[string]string{
	"USDT": "TRC20",
	"USDC": "TRC20",
	"BTC":  "Bitcoin",
	"ETH":  "ERC20",
	"SOL":  "Solana",
}

// EnsureDefaultCryptoAddress creates the user's default USDT receiving address if
// they don't have one yet. Called at KYC-approval time and as a lazy fallback.
// No mock addresses — requires Circle keys (see key-get.md).
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
	if network == "" {
		network = asset
	}
	client := settings.CircleClient()
	address, err := client.GenerateWalletAddress(network)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("Circle returned empty address for %s (see key-get.md)", asset)
	}
	addr := models.CryptoDepositAddress{
		UserID:  userID,
		Asset:   asset,
		Network: network,
		Address: address,
	}
	if err := database.DB.Create(&addr).Error; err != nil {
		return nil, err
	}
	return &addr, nil
}

// GetCryptoAddressesHandler lists the user's persisted crypto receiving addresses.
// If Circle is not configured, returns empty addresses with available=false (no hard error).
func GetCryptoAddressesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	circleOK := settings.CircleClient().Configured()
	var addresses []models.CryptoDepositAddress
	database.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&addresses)
	if addresses == nil {
		addresses = []models.CryptoDepositAddress{}
	}

	if !circleOK {
		response.Success(w, http.StatusOK, map[string]interface{}{
			"addresses":        addresses,
			"available":        false,
			"configured":       false,
			"settlement_asset": "USDT",
			"message":          "Crypto deposits are not available yet. Platform crypto rail is not configured.",
			"note":             "Ask support or wait until crypto is enabled in platform settings.",
		})
		return
	}

	// Try to ensure default USDT address when keys exist (may still fail if Circle API incomplete)
	genErr := ""
	if len(addresses) == 0 {
		if _, err := EnsureDefaultCryptoAddress(userID); err != nil {
			genErr = publicCryptoErr(err)
		} else {
			database.DB.Where("user_id = ?", userID).Order("created_at asc").Find(&addresses)
			if addresses == nil {
				addresses = []models.CryptoDepositAddress{}
			}
		}
	}

	available := len(addresses) > 0
	msg := ""
	if !available {
		msg = genErr
		if msg == "" {
			msg = "Crypto addresses could not be prepared right now. Try again later."
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"addresses":        addresses,
		"available":        available,
		"configured":       true,
		"settlement_asset": "USDT",
		"message":          msg,
		"note":             "Funds received on any address other than USDT are converted to USDT on arrival. You can still withdraw as the original asset.",
	})
}

func publicCryptoErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	for _, v := range []string{"circle", "not configured", "key-get"} {
		if strings.Contains(lower, v) {
			return "Crypto deposits are not ready yet. The platform is still connecting the crypto rail."
		}
	}
	return "Could not prepare your crypto address right now. Please try again later."
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

	if !settings.CircleClient().Configured() {
		response.Error(w, http.StatusServiceUnavailable, "Crypto is not available yet. Platform crypto rail is not configured.")
		return
	}

	address, err := ensureCryptoAddress(userID, asset)
	if err != nil {
		response.Error(w, http.StatusServiceUnavailable, publicCryptoErr(err))
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

// WithdrawCryptoHandler converts asset amount to USDT ledger debit then calls Circle
// for on-chain send. No mock: missing keys or unfinished Circle API → refund + clear error.
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
	if err := DebitWallet(userID, usdtAmount, 0, "USDT", "circle", ref, "Crypto withdrawal ("+asset+")"); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	client := settings.CircleClient()
	okPay, pRef, payErr := client.ProcessWithdrawal(asset, strings.TrimSpace(req.DestinationAddress), req.Amount)
	if !okPay || payErr != nil {
		_ = FundWallet(userID, usdtAmount, "USDT", "refund", ref+"-RFND", "Refund failed crypto withdrawal: "+ref)
		msg := "Crypto withdrawal provider failed"
		if payErr != nil {
			msg = payErr.Error()
		}
		response.Error(w, http.StatusBadGateway, msg)
		return
	}
	if pRef == "" {
		pRef = ref
	}

	var usdtWallet models.Wallet
	database.DB.Where("user_id = ? AND currency = ?", userID, "USDT").First(&usdtWallet)

	database.DB.Create(&models.Transaction{
		UserID:            userID,
		Reference:         ref,
		Amount:            req.Amount,
		BalanceAfter:      usdtWallet.Balance,
		Currency:          asset,
		Type:              "crypto_withdrawal",
		Status:            "successful",
		Provider:          "circle",
		ProviderReference: pRef,
		Description:       "Withdrew " + strconv.FormatFloat(req.Amount, 'f', -1, 64) + " " + asset + " to " + req.DestinationAddress,
	})

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":         "Withdrawal initiated.",
		"reference":       ref,
		"asset":           asset,
		"amount":          req.Amount,
		"usdt_equivalent": usdtAmount,
	})
}
