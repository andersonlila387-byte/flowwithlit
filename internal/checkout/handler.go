package checkout

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"flowwithlit/internal/bankrails"
	"flowwithlit/internal/database"
	"flowwithlit/internal/developer"
	"flowwithlit/internal/models"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/rates"
	"flowwithlit/internal/settings"
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
// Also returns payment_methods so the UI can hide/lock rails the admin has not configured.
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
		"merchant_name":    merchantDisplayName(creds.UserID),
		"is_test":          isTest,
		"payment_methods":  checkoutPaymentMethods(isTest),
	})
}

// checkoutPaymentMethods reports which buyer rails are ready (no vendor names in user-facing reasons).
func checkoutPaymentMethods(isTest bool) map[string]interface{} {
	flwOK := settings.FlutterwaveClient().Configured()
	opOK := settings.OnePipeClient().Configured()
	ppOK := settings.PalmPayClient().Configured()
	circleOK := settings.CircleClient().Configured()
	bankRailOK := opOK || ppOK || flwOK

	// Card: test keys can charge in FLW test; live needs secret key.
	cardOK := isTest || flwOK
	cardReason := ""
	if !cardOK {
		cardReason = "Card payments are not available yet. The merchant is finishing setup."
	}

	// Bank: test always has simulated VA; live needs a bank rail key.
	bankOK := isTest || bankRailOK
	bankReason := ""
	if !bankOK {
		bankReason = "Bank transfer is not available yet. The merchant is finishing setup."
	}

	// Crypto: never show fake live addresses. Only open when Circle API key is set.
	// Test sim crypto is off unless Circle is configured — honest lock when not set up.
	cryptoOK := circleOK
	cryptoReason := ""
	if !cryptoOK {
		cryptoReason = "Cryptocurrency is not available yet. Not configured by the platform."
	}

	return map[string]interface{}{
		"card": map[string]interface{}{
			"available": cardOK,
			"reason":    cardReason,
			"mode":      ternaryStr(isTest, "test", "live"),
		},
		"bank": map[string]interface{}{
			"available": bankOK,
			"reason":    bankReason,
			"mode":      ternaryStr(isTest, "test_sim", "live"),
		},
		"crypto": map[string]interface{}{
			"available": cryptoOK,
			"reason":    cryptoReason,
			// Live deposits still require finished Circle address API; UI may show “setup” vs “processing”.
			"mode": ternaryStr(circleOK, "live_keys", "off"),
		},
	}
}

func ternaryStr(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// BankDetailsHandler returns NGN virtual account details for bank-transfer checkout.
// GET /public/bank-details?key=flw_pub_test_xxx&ref=optional&amount=500000&email=optional
//
// Test keys (flw_pub_test_*): simulated account only — no real bank rail, no real money.
// Live keys (flw_pub_live_*): real merchant deposit account / live rail.
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
	if err := database.DB.Select("id, first_name, last_name, email, phone").First(&user, creds.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Merchant not found")
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("currency")))
	if currency == "" {
		currency = "NGN"
	}

	ref := r.URL.Query().Get("ref")
	amountKobo := parseAmountKobo(r.URL.Query().Get("amount"))
	customerEmail := r.URL.Query().Get("email")
	if customerEmail == "" {
		customerEmail = user.Email
	}

	merchantName := merchantDisplayName(creds.UserID)

	// ── TEST MODE: never touch real bank rails or real accounts ──────────────
	if isTest {
		if ref != "" && amountKobo > 0 {
			registerPendingBankTransfer(ref, creds.UserID, amountKobo, currency, customerEmail)
		}
		// Clearly simulated — buyers must not send real money here
		testAcct := testSimulatedAccountNumber(ref, creds.UserID)
		response.Success(w, http.StatusOK, map[string]interface{}{
			"currency":       currency,
			"bank_name":      "Test Bank (Simulated)",
			"account_number": testAcct,
			"account_name":   merchantName + " (TEST)",
			"reference":      ref,
			"instructions":   "TEST MODE — do not send real money. Payment will auto-confirm in a few seconds for integration testing.",
			"is_test":        true,
			"simulated":      true,
		})
		return
	}

	// ── LIVE MODE: real deposit account / live NGN rail ─────────────────────
	var bankName, accountNumber, accountName string

	var dep models.DepositAccount
	if err := database.DB.Where("user_id = ? AND currency = ?", creds.UserID, currency).
		Order("is_default desc, id asc").First(&dep).Error; err == nil &&
		strings.TrimSpace(dep.AccountNumber) != "" &&
		!strings.HasPrefix(strings.TrimSpace(dep.AccountNumber), "000") {
		bankName = dep.BankName
		accountNumber = dep.AccountNumber
		accountName = dep.AccountName
		if accountName == "" {
			accountName = merchantName
		}
	} else {
		// Open / resolve live VA for this merchant (server-side only)
		rail, railErr := bankrails.ResolveWithOpts(currency, user.FirstName, user.LastName, user.Email, user.Phone, bankrails.CustomerOpts{
			UserID: fmt.Sprintf("%d", user.ID),
		})
		if railErr != nil {
			response.Error(w, http.StatusInternalServerError, "Could not generate virtual account for live checkout. Complete merchant deposit setup and try again.")
			return
		}
		bankName = rail.BankName
		accountNumber = rail.AccountNumber
		accountName = merchantName
		// Persist for future checkouts / Add Fund (best-effort, no overwrite of existing)
		if strings.TrimSpace(accountNumber) != "" {
			var existing models.DepositAccount
			if database.DB.Where("user_id = ? AND currency = ?", creds.UserID, currency).First(&existing).Error != nil {
				_ = database.DB.Create(&models.DepositAccount{
					UserID:        creds.UserID,
					Currency:      currency,
					AccountNumber: accountNumber,
					BankName:      bankName,
					AccountName:   accountName,
					Provider:      rail.Provider,
					IsDefault:     true,
				}).Error
			}
		}
	}

	if ref != "" && amountKobo > 0 {
		// Live: register pending so status can be polled; only real webhook/credit settles
		registerPendingBankTransfer(ref, creds.UserID, amountKobo, currency, customerEmail)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"currency":       currency,
		"bank_name":      bankName,
		"account_number": accountNumber,
		"account_name":   accountName,
		"reference":      ref,
		"instructions":   "Transfer the exact amount to this account. Payment is confirmed automatically once received.",
		"is_test":        false,
		"simulated":      false,
	})
}

// testSimulatedAccountNumber builds an obvious non-real NUBAN for test checkout.
// Format 000XXXXXXX so it can never be confused with a live collection account.
func testSimulatedAccountNumber(ref string, merchantID uint) string {
	h := 0
	for _, c := range ref + fmt.Sprintf("%d", merchantID) {
		h = (h*31 + int(c)) % 10000000
	}
	if h < 0 {
		h = -h
	}
	return fmt.Sprintf("000%07d", h)
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
		PublicKey    string                 `json:"public_key"`
		SessionToken string                 `json:"session_token"` // preferred: locks amount/currency/email server-side
		Amount       float64                `json:"amount"`        // in kobo / lowest denomination — ignored when session_token is set
		Currency     string                 `json:"currency"`
		Email        string                 `json:"email"`
		Name         string                 `json:"name"`
		CardNumber   string                 `json:"card_number"`
		ExpiryMonth  string                 `json:"expiry_month"`
		ExpiryYear   string                 `json:"expiry_year"`
		CVV          string                 `json:"cvv"`
		Ref          string                 `json:"ref"`
		Meta         map[string]interface{} `json:"meta"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.PublicKey == "" || req.Email == "" {
		response.Error(w, http.StatusBadRequest, "public_key and email are required")
		return
	}
	if req.SessionToken == "" && req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount is required")
		return
	}

	creds, isTest, err := lookupCreds(req.PublicKey)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid public key")
		return
	}

	// A session locks in amount/currency/email server-side when it was created, so we
	// never trust whatever the client sends for those fields — this is what stops a
	// buyer from editing the checkout URL/request to pay less than the merchant meant
	// to charge, or forging a fake "successful" transaction against a merchant's
	// public key (which is not secret — it's visible in every checkout link).
	var session *models.CheckoutSession
	if req.SessionToken != "" {
		session, err = claimCheckoutSession(req.SessionToken, creds, isTest)
		if err != nil {
			code := http.StatusBadRequest
			if err == errCheckoutSessionUsed {
				code = http.StatusConflict
			} else if err == errCheckoutSessionExpired {
				code = http.StatusGone
			}
			response.Error(w, code, "Checkout session is invalid, expired, or already used")
			return
		}
		req.Amount = session.Amount
		req.Currency = session.Currency
		req.Email = session.CustomerEmail
		if session.CustomerName != "" {
			req.Name = session.CustomerName
		}
		if session.MerchantRef != "" {
			req.Ref = session.MerchantRef
		}
		if session.Meta != "" {
			var sessionMeta map[string]interface{}
			if json.Unmarshal([]byte(session.Meta), &sessionMeta) == nil && len(sessionMeta) > 0 {
				if req.Meta == nil {
					req.Meta = map[string]interface{}{}
				}
				for k, v := range sessionMeta {
					req.Meta[k] = v
				}
			}
		}
	} else {
		log.Printf("⚠️ checkout: charge without session_token (client-trusted amount) public_key=%s ref=%s", req.PublicKey, req.Ref)
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
		txnID, err := recordCheckoutPayment(
			creds.UserID, req.Ref, amountMajor, req.Currency, isTest, req.Email,
			"Checkout payment from "+req.Email, req.Meta,
		)
		if err != nil {
			if session != nil {
				releaseCheckoutSession(session)
			}
			response.Error(w, http.StatusInternalServerError, "Failed to record payment")
			return
		}
		if session != nil {
			completeCheckoutSession(session, txnID)
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
	if !fw.Configured() {
		if session != nil {
			releaseCheckoutSession(session)
		}
		response.Error(w, http.StatusBadRequest, "Live card payments are not available right now. Please try another method or contact the merchant.")
		return
	}
	ok, providerRef, err := fw.ChargeCard(amountMajor, req.Currency, req.Email, req.Ref, card)
	if err != nil || !ok {
		if session != nil {
			releaseCheckoutSession(session)
		}
		msg := "Card payment failed. Check card details and try again."
		if err != nil {
			e := strings.ToLower(err.Error())
			if !strings.Contains(e, "flutterwave") && !strings.Contains(e, "configured") && !strings.Contains(e, "key-get") {
				msg = err.Error()
			}
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

	txnID, err := recordCheckoutPayment(
		creds.UserID, req.Ref, amountMajor, req.Currency, false, req.Email,
		"Checkout payment from "+req.Email, req.Meta,
	)
	if err != nil {
		if session != nil {
			releaseCheckoutSession(session)
		}
		response.Error(w, http.StatusInternalServerError, "Failed to record payment")
		return
	}
	if session != nil {
		completeCheckoutSession(session, txnID)
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

	// Never expose card processor brand to the browser
	response.Success(w, http.StatusOK, map[string]interface{}{
		"status":          "successful",
		"transaction_ref": req.Ref,
		"amount":          req.Amount,
		"currency":        req.Currency,
		"message":         "Payment successful",
		"is_test":         false,
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
