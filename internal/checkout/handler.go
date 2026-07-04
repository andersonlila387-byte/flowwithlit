package checkout

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/developer"
	"flowwithlit/internal/bankrails"
	"flowwithlit/internal/models"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/settings"
	"flowwithlit/internal/rates"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

func genRef() string {
	b := make([]byte, 8)
	rand.Read(b)
	return strings.ToUpper(hex.EncodeToString(b))
}

func lookupCreds(key string) (*models.ApiCredentials, bool, error) {
	var creds models.ApiCredentials
	err := database.DB.Where("pub_key_test = ? OR pub_key_live = ?", key, key).First(&creds).Error
	if err != nil {
		return nil, false, err
	}
	isTest := creds.PubKeyTest == key
	return &creds, isTest, nil
}

// MerchantInfoHandler returns public merchant info so the checkout page can show the merchant name.
// GET /public/merchant-info?key=flw_pub_test_xxx
func MerchantInfoHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		response.Error(w, http.StatusBadRequest, "key is required")
		return
	}

	creds, isTest, err := lookupCreds(key)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid public key")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"merchant_name": merchantDisplayName(creds.UserID),
		"is_test":       isTest,
	})
}

// BankDetailsHandler returns NGN virtual account details for bank-transfer checkout.
// GET /public/bank-details?key=flw_pub_test_xxx&ref=optional&amount=500000&email=optional
func BankDetailsHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		response.Error(w, http.StatusBadRequest, "key is required")
		return
	}

	creds, isTest, err := lookupCreds(key)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid public key")
		return
	}

	var user models.User
	if err := database.DB.Select("first_name, last_name, email, phone").First(&user, creds.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Merchant not found")
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency")))
	if currency == "" {
		currency = "NGN"
	}

	rail, err := bankrails.Resolve(currency, user.FirstName, user.LastName, user.Email, user.Phone)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not generate virtual account")
		return
	}

	ref := r.URL.Query().Get("ref")
	amountKobo := parseAmountKobo(r.URL.Query().Get("amount"))
	customerEmail := r.URL.Query().Get("email")
	if customerEmail == "" {
		customerEmail = user.Email
	}

	if ref != "" && amountKobo > 0 {
		registerPendingBankTransfer(ref, creds.UserID, amountKobo, rail.Currency, customerEmail)
	}

	merchantName := merchantDisplayName(creds.UserID)
	providerLabel := rail.Provider
	if rail.Provider == providers.OnePipe {
		providerLabel = "OnePipe"
	} else {
		providerLabel = "Flutterwave"
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"currency":            rail.Currency,
		"bank_name":           rail.BankName,
		"account_number":      rail.AccountNumber,
		"account_name":        merchantName,
		"reference":           ref,
		"payment_provider":    rail.Provider,
		"provider_configured": rail.Configured,
		"instructions":        "Transfer the exact amount to this account. Payment is confirmed automatically once received via " + providerLabel + ".",
		"is_test":             isTest,
	})
}

// BankStatusHandler polls for incoming bank transfer (auto-confirms in test mode).
// GET /public/bank-status?key=flw_pub_test_xxx&ref=ORDER_xxx
func BankStatusHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	ref := r.URL.Query().Get("ref")
	if key == "" || ref == "" {
		response.Error(w, http.StatusBadRequest, "key and ref are required")
		return
	}

	creds, isTest, err := lookupCreds(key)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid public key")
		return
	}

	status, done := checkPendingBankTransfer(ref, creds, isTest)
	payload := map[string]interface{}{
		"status": status,
	}
	if done {
		payload["transaction_ref"] = ref
	}

	response.Success(w, http.StatusOK, payload)
}

// ChargeHandler processes a payment initiated from the hosted checkout page.
// POST /public/charge
func ChargeHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PublicKey   string                 `json:"public_key"`
		Amount      float64                `json:"amount"` // in kobo / lowest denomination
		Currency    string                 `json:"currency"`
		Email       string                 `json:"email"`
		Name        string                 `json:"name"`
		CardNumber  string                 `json:"card_number"`
		ExpiryMonth string                 `json:"expiry_month"`
		ExpiryYear  string                 `json:"expiry_year"`
		CVV         string                 `json:"cvv"`
		Ref         string                 `json:"ref"`
		Meta        map[string]interface{} `json:"meta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PublicKey == "" || req.Amount <= 0 || req.Email == "" {
		response.Error(w, http.StatusBadRequest, "public_key, amount, and email are required")
		return
	}

	creds, isTest, err := lookupCreds(req.PublicKey)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid public key")
		return
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}
	if req.Ref == "" {
		req.Ref = "FLW_TXN_" + genRef()
	}

	if isTest {
		// Test mode — simulate success, no real card processing
		amountMajor := req.Amount / 100
		if req.Meta == nil {
			req.Meta = map[string]interface{}{}
		}
		if strings.TrimSpace(req.Name) != "" {
			req.Meta["customer_name"] = strings.TrimSpace(req.Name)
		}
		if _, ok := req.Meta["payment_method"]; !ok {
			req.Meta["payment_method"] = "card"
		}
		if err := recordCheckoutPayment(
			creds.UserID, req.Ref, amountMajor, req.Currency, isTest, req.Email,
			"Checkout payment from "+req.Email, req.Meta,
		); err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to record payment")
			return
		}

		go developer.DispatchWebhook(creds.UserID, "charge.success", map[string]interface{}{
			"transaction_ref": req.Ref,
			"amount":          req.Amount,
			"currency":        req.Currency,
			"status":          "successful",
			"customer": map[string]interface{}{
				"email": req.Email,
				"name":  req.Name,
			},
			"meta":     req.Meta,
			"is_test":  true,
		})

		response.Success(w, http.StatusOK, map[string]interface{}{
			"status":          "successful",
			"transaction_ref": req.Ref,
			"amount":          req.Amount,
			"currency":        req.Currency,
			"message":         "Payment successful",
		})
		return
	}

	// Live card payments always use Flutterwave (NGN bank rails use OnePipe separately).
	fw := settings.FlutterwaveClient()
	amountMajor := req.Amount / 100
	card := map[string]string{
		"number":       req.CardNumber,
		"cvv":          req.CVV,
		"expiry_month": req.ExpiryMonth,
		"expiry_year":  req.ExpiryYear,
	}
	ok, providerRef, err := fw.ChargeCard(amountMajor, req.Currency, req.Email, req.Ref, card)
	if err != nil || !ok {
		msg := "Card payment failed"
		if err != nil {
			msg = err.Error()
		}
		response.Error(w, http.StatusBadRequest, msg)
		return
	}

	if req.Meta == nil {
		req.Meta = map[string]interface{}{}
	}
	req.Meta["payment_method"] = "card"
	req.Meta["payment_provider"] = providers.ForCard()
	req.Meta["provider_ref"] = providerRef

	if err := recordCheckoutPayment(
		creds.UserID, req.Ref, amountMajor, req.Currency, false, req.Email,
		"Checkout payment from "+req.Email, req.Meta,
	); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to record payment")
		return
	}

	go developer.DispatchWebhook(creds.UserID, "charge.success", map[string]interface{}{
		"transaction_ref": req.Ref,
		"amount":          req.Amount,
		"currency":        req.Currency,
		"status":          "successful",
		"customer": map[string]interface{}{
			"email": req.Email,
			"name":  req.Name,
		},
		"meta":     req.Meta,
		"is_test":  false,
	})

	response.Success(w, http.StatusOK, map[string]interface{}{
		"status":          "successful",
		"transaction_ref": req.Ref,
		"amount":          req.Amount,
		"currency":        req.Currency,
		"provider":        providers.ForCard(),
		"message":         "Payment successful",
	})
}

// PublicRatesHandler returns exchange rates from the rates service (admin-managed DB).
// GET /public/rates
func PublicRatesHandler(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, rates.GetAllRates())
}

// PublicCurrenciesHandler returns enabled fiat currencies for checkout display.
// GET /public/currencies
func PublicCurrenciesHandler(w http.ResponseWriter, r *http.Request) {
	list := rates.GetEnabledCurrencies()
	out := make([]map[string]interface{}, 0, len(list))
	for _, c := range list {
		out = append(out, map[string]interface{}{
			"code":                    c.Code,
			"name":                    c.Name,
			"symbol":                  c.Symbol,
			"decimals":                c.Decimals,
			"card_supported":          c.CardSupported,
			"bank_transfer_supported": c.BankTransferSupported,
			"is_base":                 c.IsBase,
		})
	}
	response.Success(w, http.StatusOK, out)
}

// PublicCryptoAssetsHandler returns enabled crypto assets + USD prices.
// GET /public/crypto-assets
func PublicCryptoAssetsHandler(w http.ResponseWriter, r *http.Request) {
	list := rates.GetEnabledCryptoAssets()
	out := make([]map[string]interface{}, 0, len(list))
	for _, a := range list {
		out = append(out, map[string]interface{}{
			"code":        a.Code,
			"name":        a.Name,
			"network":     a.Network,
			"network_tag": a.NetworkTag,
			"decimals":    a.Decimals,
			"icon_key":    a.IconKey,
			"color":       a.Color,
			"usd_price":   rates.CryptoUSDPrice(a.Code),
		})
	}
	response.Success(w, http.StatusOK, out)
}

// VerifyTransactionHandler lets a merchant's server verify a payment using their secret key.
// Authenticates via Bearer secret key (not JWT) — safe to call server-side only.
// GET /v1/transaction/verify/{ref}
func VerifyTransactionHandler(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		response.Error(w, http.StatusUnauthorized, "Authorization: Bearer <secret_key> header required")
		return
	}
	secKey := strings.TrimPrefix(authHeader, "Bearer ")

	var creds models.ApiCredentials
	if err := database.DB.Where("sec_key_test = ? OR sec_key_live = ?", secKey, secKey).First(&creds).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid secret key")
		return
	}

	ref := chi.URLParam(r, "ref")
	if ref == "" {
		response.Error(w, http.StatusBadRequest, "Transaction reference required")
		return
	}

	var txn models.Transaction
	if err := database.DB.Where("reference = ? AND user_id = ?", ref, creds.UserID).First(&txn).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Transaction not found")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"status":          txn.Status,
		"transaction_ref": txn.Reference,
		"amount":          txn.Amount * 100, // return in kobo for consistency
		"currency":        txn.Currency,
		"customer":        txn.Customer,
		"created_at":      txn.CreatedAt,
	})
}
