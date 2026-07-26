package kyc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settlement"
	"flowwithlit/internal/settings"
	"flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func getActiveProvider() KYCProvider {
	switch settings.KYCProvider() {
	case "mock":
		return &MockProvider{}
	case "flutterwave":
		return &FlutterwaveProvider{}
	case "smileid":
		return NewSmileIDProvider()
	case "onepipe":
		return &OnePipeProvider{}
	default:
		// Prefer OnePipe for NG identity when configured; else Flutterwave hybrid
		if settings.OnePipeClient().Configured() {
			return &OnePipeProvider{}
		}
		return &FlutterwaveProvider{}
	}
}

// ----------------------------------------------------------------------------
// HANDLERS
// ----------------------------------------------------------------------------

func StatusHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"completed": user.KYCLevel > 0,
		"level":     user.KYCLevel,
		// Do not expose underlying KYC vendor to the client
	})
}

type ActivateBusinessRequest struct {
	BusinessName string `json:"business_name"`
	Industry     string `json:"industry"`
	SupportEmail string `json:"support_email"`
	Phone        string `json:"phone"`
	Address      string `json:"address"`
	CountryCode  string `json:"country_code"`
	BaseCurrency string `json:"base_currency"`
	PrimaryIDType string `json:"primary_id_type"`
	PrimaryIDVal  string `json:"primary_id_val"`
	BankCode              string `json:"bank_code"`
	AccountNumber         string `json:"account_number"`
	DefaultFiatCurrency   string `json:"default_fiat_currency"`
	DefaultCryptoCurrency string `json:"default_crypto_currency"`
}

func ActivateHandler(w http.ResponseWriter, r *http.Request) {
	var req ActivateBusinessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	if strings.TrimSpace(req.BusinessName) == "" {
		response.Error(w, http.StatusBadRequest, "Company name is required")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	provider := getActiveProvider()

	identityPayload := IdentityPayload{
		CountryCode:   req.CountryCode,
		PrimaryIDType: req.PrimaryIDType,
		PrimaryIDVal:  req.PrimaryIDVal,
		UserID:        fmt.Sprintf("%d", userID),
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Email:         user.Email,
		Phone:         firstNonEmpty(req.Phone, user.Phone),
	}
	if user.DateOfBirth != nil {
		identityPayload.DOB = user.DateOfBirth.Format("2006-01-02")
	}

	status, err := provider.VerifyIdentity(identityPayload)
	if status == "failed" || err != nil {
		errMsg := "Identity verification failed"
		if err != nil {
			errMsg = err.Error()
		}
		// Never leak vendor/rail names to the browser
		errMsg = sanitizePublicError(errMsg)
		response.Error(w, http.StatusBadRequest, errMsg)
		return
	}

	fiatCur := settlement.CurrencyForCountry(req.CountryCode)
	if v := strings.ToUpper(strings.TrimSpace(req.DefaultFiatCurrency)); v != "" {
		fiatCur = v
	} else if v := strings.ToUpper(strings.TrimSpace(req.BaseCurrency)); v != "" {
		fiatCur = v
	}

	cryptoCur := settlement.DefaultCrypto
	if v := strings.ToUpper(strings.TrimSpace(req.DefaultCryptoCurrency)); v != "" {
		cryptoCur = v
	} else if v := strings.ToUpper(strings.TrimSpace(user.DefaultCryptoCurrency)); v != "" {
		cryptoCur = v
	}

	profile := models.BusinessProfile{
		UserID:         user.ID,
		BusinessName:   req.BusinessName,
		Industry:       req.Industry,
		SupportEmail:   req.SupportEmail,
		Phone:          req.Phone,
		Address:        req.Address,
		CountryCode:    req.CountryCode,
		BaseCurrency:   fiatCur,
		PrimaryIDType:  req.PrimaryIDType,
		PrimaryIDValue: req.PrimaryIDVal,
		BankCode:       req.BankCode,
		AccountNumber:  req.AccountNumber,
	}

	if status == "approved" {
		profile.KYCStatus = "approved"
		user.KYCLevel = 1
	} else if status == "pending" {
		profile.KYCStatus = "pending"
		user.KYCLevel = 0
	}

	var existing models.BusinessProfile
	if err := database.DB.Where("user_id = ?", user.ID).First(&existing).Error; err == nil {
		profile.ID = existing.ID
	}

	if err := database.DB.Save(&profile).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save business profile")
		return
	}
	user.DefaultFiatCurrency = fiatCur
	user.DefaultCryptoCurrency = cryptoCur
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update user settlement preferences")
		return
	}
	if _, err := wallet.EnsureWallet(database.DB, user.ID, fiatCur); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to provision fiat wallet")
		return
	}
	if _, err := wallet.EnsureWallet(database.DB, user.ID, cryptoCur); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to provision crypto wallet")
		return
	}

	// When identity is auto-approved, provision deposit account immediately (server-side rail only).
	if status == "approved" {
		if _, err := wallet.EnsureDefaultDepositAccount(user.ID); err != nil {
			// Non-fatal: admin can re-approve / user can retry Add Fund
			_ = err
		}
	}

	if to := strings.TrimSpace(user.Email); to != "" {
		_ = email.SendBusinessActivated(
			to, user.FirstName, profile.BusinessName,
			profile.CountryCode, fiatCur, cryptoCur,
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":                 "Business activated successfully!",
		"default_fiat_currency":   fiatCur,
		"default_crypto_currency": cryptoCur,
		// Never return underlying rail / KYC vendor names to the browser
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func sanitizePublicError(msg string) string {
	lower := strings.ToLower(msg)
	for _, vendor := range []string{"onepipe", "flutterwave", "palmpay", "smile id", "smileid", "circle"} {
		if strings.Contains(lower, vendor) {
			return "Identity verification failed. Please check your details and try again."
		}
	}
	return msg
}

func GetProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err != nil {
		response.Success(w, http.StatusOK, map[string]interface{}{
			"profile": nil,
		})
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"profile": profile,
	})
}