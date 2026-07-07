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
	default:
		// Default to Flutterwave since it has manual fallback, or Smile ID
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
		"provider":  settings.KYCProvider(),
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

	provider := getActiveProvider()

	identityPayload := IdentityPayload{
		CountryCode:   req.CountryCode,
		PrimaryIDType: req.PrimaryIDType,
		PrimaryIDVal:  req.PrimaryIDVal,
		UserID:        fmt.Sprintf("%d", userID),
	}

	status, err := provider.VerifyIdentity(identityPayload)
	if status == "failed" || err != nil {
		errMsg := "Identity verification failed"
		if err != nil {
			errMsg = err.Error()
		}
		response.Error(w, http.StatusBadRequest, errMsg)
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
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

	if to := strings.TrimSpace(user.Email); to != "" {
		_ = email.SendBusinessActivated(
			to, user.FirstName, profile.BusinessName,
			profile.CountryCode, fiatCur, cryptoCur,
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":                 "Business activated successfully!",
		"provider_used":           provider.Name(),
		"default_fiat_currency":   fiatCur,
		"default_crypto_currency": cryptoCur,
	})
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