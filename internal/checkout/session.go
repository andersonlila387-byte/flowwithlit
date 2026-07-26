package checkout

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

const checkoutSessionTTL = 30 * time.Minute

var (
	errCheckoutSessionInvalid = errors.New("invalid checkout session")
	errCheckoutSessionExpired = errors.New("checkout session has expired")
	errCheckoutSessionUsed    = errors.New("checkout session has already been used")
)

func genSessionToken() string {
	b := make([]byte, 20)
	rand.Read(b)
	return "cs_" + hex.EncodeToString(b)
}

func secretKeyFromAuthHeader(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}

type CreateCheckoutSessionRequest struct {
	Amount   float64                `json:"amount"` // kobo / minor units
	Currency string                 `json:"currency"`
	Email    string                 `json:"email"`
	Name     string                 `json:"name"`
	Ref      string                 `json:"ref"`
	Meta     map[string]interface{} `json:"meta"`
}

// CreateCheckoutSessionHandler lets a merchant's own server lock in the amount/currency/
// customer for a checkout attempt ahead of time, so the browser-facing checkout page and
// ChargeHandler never have to trust a client-supplied amount.
// POST /v1/checkout/sessions — Authorization: Bearer <secret_key>
func CreateCheckoutSessionHandler(w http.ResponseWriter, r *http.Request) {
	secKey := secretKeyFromAuthHeader(r)
	if secKey == "" {
		response.Error(w, http.StatusUnauthorized, "Authorization: Bearer <secret_key> header required")
		return
	}

	var creds models.ApiCredentials
	if err := database.DB.Where("sec_key_test = ? OR sec_key_live = ?", secKey, secKey).First(&creds).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid secret key")
		return
	}
	isTest := creds.SecKeyTest == secKey

	var req CreateCheckoutSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Amount <= 0 || strings.TrimSpace(req.Email) == "" {
		response.Error(w, http.StatusBadRequest, "amount and email are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	metaJSON := "{}"
	if req.Meta != nil {
		if b, err := json.Marshal(req.Meta); err == nil {
			metaJSON = string(b)
		}
	}

	session := models.CheckoutSession{
		Token:         genSessionToken(),
		UserID:        creds.UserID,
		IsTest:        isTest,
		CustomerEmail: strings.TrimSpace(req.Email),
		CustomerName:  strings.TrimSpace(req.Name),
		Amount:        req.Amount,
		Currency:      strings.ToUpper(strings.TrimSpace(req.Currency)),
		MerchantRef:   strings.TrimSpace(req.Ref),
		Meta:          metaJSON,
		Status:        "pending",
		ExpiresAt:     time.Now().Add(checkoutSessionTTL),
	}
	if err := database.DB.Create(&session).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create checkout session")
		return
	}

	pubKey := creds.PubKeyLive
	if isTest {
		pubKey = creds.PubKeyTest
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"token":        session.Token,
		"checkout_url": email.CheckoutBaseURL() + "/?session=" + session.Token + "&key=" + pubKey,
		"expires_at":   session.ExpiresAt,
	})
}

// GetCheckoutSessionHandler lets the hosted checkout page bootstrap the authoritative
// amount/currency/customer for a session, instead of trusting URL query params.
// GET /public/checkout-session/{token}
func GetCheckoutSessionHandler(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		response.Error(w, http.StatusBadRequest, "token is required")
		return
	}

	var session models.CheckoutSession
	if err := database.DB.Where("token = ?", token).First(&session).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Checkout session not found")
		return
	}
	if session.Status != "pending" {
		response.Error(w, http.StatusGone, "This checkout session has already been used")
		return
	}
	if time.Now().After(session.ExpiresAt) {
		response.Error(w, http.StatusGone, "This checkout session has expired")
		return
	}

	var creds models.ApiCredentials
	pubKey := ""
	if err := database.DB.Where("user_id = ?", session.UserID).First(&creds).Error; err == nil {
		if session.IsTest {
			pubKey = creds.PubKeyTest
		} else {
			pubKey = creds.PubKeyLive
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"merchant_name": merchantDisplayName(session.UserID),
		"public_key":    pubKey,
		"is_test":       session.IsTest,
		"amount":        session.Amount,
		"currency":      session.Currency,
		"email":         session.CustomerEmail,
		"name":          session.CustomerName,
		"ref":           session.MerchantRef,
	})
}

// claimCheckoutSession validates a session token against the public key resolved by
// lookupCreds, atomically marks it "processing" so it can't be used twice at once, and
// returns it. Callers must release it via completeCheckoutSession or releaseCheckoutSession.
func claimCheckoutSession(token string, creds *models.ApiCredentials, isTest bool) (*models.CheckoutSession, error) {
	var session models.CheckoutSession
	if err := database.DB.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, errCheckoutSessionInvalid
	}
	if session.UserID != creds.UserID || session.IsTest != isTest {
		return nil, errCheckoutSessionInvalid
	}
	if time.Now().After(session.ExpiresAt) {
		return nil, errCheckoutSessionExpired
	}

	result := database.DB.Model(&models.CheckoutSession{}).
		Where("id = ? AND status = ?", session.ID, "pending").
		Update("status", "processing")
	if result.Error != nil || result.RowsAffected != 1 {
		return nil, errCheckoutSessionUsed
	}

	return &session, nil
}

func completeCheckoutSession(session *models.CheckoutSession, transactionID uint) {
	now := time.Now()
	database.DB.Model(&models.CheckoutSession{}).Where("id = ?", session.ID).Updates(map[string]interface{}{
		"status":         "completed",
		"completed_at":   &now,
		"transaction_id": transactionID,
	})
}

// releaseCheckoutSession puts a claimed session back to "pending" after a failed charge
// attempt so the customer can retry without needing a brand-new session.
func releaseCheckoutSession(session *models.CheckoutSession) {
	database.DB.Model(&models.CheckoutSession{}).
		Where("id = ? AND status = ?", session.ID, "processing").
		Update("status", "pending")
}
