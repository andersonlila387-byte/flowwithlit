package kyc

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settlement"
	"flowwithlit/internal/wallet"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// MobileVerifyRequest is personal identity KYC for the mobile bank app
// (no business / CAC form — BVN or NIN + optional address).
type MobileVerifyRequest struct {
	PrimaryIDType string `json:"primary_id_type"` // BVN | NIN
	PrimaryIDVal  string `json:"primary_id_val"`
	CountryCode   string `json:"country_code"` // default NG
	Address       string `json:"address"`
	// ForceNewAccount re-opens a deposit account via the live rail (use after keys change).
	ForceNewAccount bool `json:"force_new_account"`
}

// MobileVerifyHandler — POST /kyc/mobile/verify
// Verifies BVN/NIN server-side, upgrades personal KYC tier, provisions NGN deposit VA.
func MobileVerifyHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req MobileVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	idType := strings.ToUpper(strings.TrimSpace(req.PrimaryIDType))
	idVal := strings.TrimSpace(req.PrimaryIDVal)
	if idType != "BVN" && idType != "NIN" {
		response.Error(w, http.StatusBadRequest, "primary_id_type must be BVN or NIN")
		return
	}
	if idVal == "" {
		response.Error(w, http.StatusBadRequest, "primary_id_val is required")
		return
	}
	if idType == "BVN" && len(idVal) != 11 {
		response.Error(w, http.StatusBadRequest, "BVN must be 11 digits")
		return
	}

	country := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if country == "" {
		country = "NG"
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if !user.IsPhoneVerified {
		response.Error(w, http.StatusForbidden, "Verify your phone number before identity KYC")
		return
	}

	provider := getActiveProvider()
	status, err := provider.VerifyIdentity(IdentityPayload{
		CountryCode:   country,
		PrimaryIDType: idType,
		PrimaryIDVal:  idVal,
		UserID:        fmt.Sprintf("%d", user.ID),
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Email:         user.Email,
		Phone:         user.Phone,
		DOB:           dobString(user),
	})
	if status == "failed" || err != nil {
		msg := "Identity verification failed"
		if err != nil {
			msg = sanitizePublicError(err.Error())
		}
		response.Error(w, http.StatusBadRequest, msg)
		return
	}

	fiatCur := settlement.CurrencyForCountry(country)
	if fiatCur == "" {
		fiatCur = "NGN"
	}
	cryptoCur := settlement.DefaultCrypto
	if user.DefaultCryptoCurrency != "" {
		cryptoCur = user.DefaultCryptoCurrency
	}

	// Personal KYC profile (not a full business activation).
	profile := models.BusinessProfile{
		UserID:         user.ID,
		BusinessName:   "Personal Account",
		Industry:       "Personal",
		SupportEmail:   user.Email,
		Phone:          user.Phone,
		Address:        strings.TrimSpace(req.Address),
		CountryCode:    country,
		BaseCurrency:   fiatCur,
		PrimaryIDType:  idType,
		PrimaryIDValue: idVal,
	}
	if status == "approved" {
		profile.KYCStatus = "approved"
		user.KYCLevel = 1
	} else {
		profile.KYCStatus = "pending"
		// keep existing tier if already higher
		if user.KYCLevel < 1 {
			user.KYCLevel = 0
		}
	}

	var existing models.BusinessProfile
	if err := database.DB.Where("user_id = ?", user.ID).First(&existing).Error; err == nil {
		profile.ID = existing.ID
		// Preserve real business name if merchant already activated on web
		if strings.TrimSpace(existing.BusinessName) != "" &&
			!strings.EqualFold(existing.BusinessName, "Personal Account") {
			profile.BusinessName = existing.BusinessName
			profile.Industry = existing.Industry
		}
	}
	if err := database.DB.Save(&profile).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save KYC profile")
		return
	}

	user.DefaultFiatCurrency = fiatCur
	user.DefaultCryptoCurrency = cryptoCur
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update account")
		return
	}

	_, _ = wallet.EnsureWallet(database.DB, user.ID, fiatCur)
	_, _ = wallet.EnsureWallet(database.DB, user.ID, cryptoCur)

	out := map[string]interface{}{
		"message":               messageForStatus(status),
		"kyc_status":            profile.KYCStatus,
		"kyc_level":             user.KYCLevel,
		"default_fiat_currency": fiatCur,
		"phone_verified":        user.IsPhoneVerified,
		"deposit_account":       nil,
	}

	if status == "approved" {
		var acct *models.DepositAccount
		var acctErr error
		if req.ForceNewAccount {
			acct, acctErr = wallet.ReprovisionDefaultDepositAccount(user.ID)
		} else {
			acct, acctErr = wallet.EnsureDefaultDepositAccount(user.ID)
		}
		if acctErr != nil {
			log.Printf("[KYC mobile] deposit account for user %d: %v", user.ID, acctErr)
			out["message"] = "Identity verified. Deposit account could not be created yet — try again from Fund Wallet."
			out["deposit_account_error"] = true
		} else if acct != nil {
			out["deposit_account"] = map[string]interface{}{
				"currency":       acct.Currency,
				"account_number": acct.AccountNumber,
				"bank_name":      acct.BankName,
				"account_name":   acct.AccountName,
				"is_default":     acct.IsDefault,
			}
		}
	}

	response.Success(w, http.StatusOK, out)
}

// MobileStatusHandler — GET /kyc/mobile/status
// Compact status for the mobile app home / KYC screens.
func MobileStatusHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	kycStatus := "none"
	idType := ""
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		kycStatus = profile.KYCStatus
		idType = profile.PrimaryIDType
	}

	var deposit map[string]interface{}
	var dep models.DepositAccount
	if err := database.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&dep).Error; err == nil {
		deposit = map[string]interface{}{
			"currency":       dep.Currency,
			"account_number": dep.AccountNumber,
			"bank_name":      dep.BankName,
			"account_name":   dep.AccountName,
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"kyc_level":         user.KYCLevel,
		"kyc_status":        kycStatus,
		"id_type":           idType,
		"is_phone_verified": user.IsPhoneVerified,
		"is_email_verified": user.IsEmailVerified,
		"first_name":        user.FirstName,
		"last_name":         user.LastName,
		"deposit_account":   deposit,
		"next_step":         mobileNextStep(user, kycStatus, deposit != nil),
	})
}

// MobileEnsureDepositHandler — POST /kyc/mobile/ensure-deposit
// Creates or re-provisions the default deposit VA after KYC is approved.
func MobileEnsureDepositHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Force bool `json:"force_new_account"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if user.KYCLevel < 1 {
		response.Error(w, http.StatusForbidden, "Complete identity verification before requesting a deposit account")
		return
	}

	var acct *models.DepositAccount
	var err error
	if req.Force {
		acct, err = wallet.ReprovisionDefaultDepositAccount(userID)
	} else {
		acct, err = wallet.EnsureDefaultDepositAccount(userID)
	}
	if err != nil {
		response.Error(w, http.StatusInternalServerError, publicDepositErrForKYC(err))
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Deposit account ready",
		"deposit_account": map[string]interface{}{
			"currency":       acct.Currency,
			"account_number": acct.AccountNumber,
			"bank_name":      acct.BankName,
			"account_name":   acct.AccountName,
			"is_default":     acct.IsDefault,
		},
	})
}

func dobString(user models.User) string {
	if user.DateOfBirth == nil {
		return ""
	}
	return user.DateOfBirth.Format("2006-01-02")
}

func messageForStatus(status string) string {
	switch status {
	case "approved":
		return "Identity verified successfully"
	case "pending":
		return "Identity submitted for review"
	default:
		return "Identity verification update received"
	}
}

func mobileNextStep(user models.User, kycStatus string, hasDeposit bool) string {
	if !user.IsEmailVerified {
		return "verify_email"
	}
	if !user.IsPhoneVerified {
		return "verify_phone"
	}
	if user.KYCLevel < 1 || kycStatus == "none" || kycStatus == "" {
		return "verify_identity" // BVN/NIN
	}
	if kycStatus == "pending" {
		return "await_review"
	}
	if kycStatus == "rejected" {
		return "resubmit_identity"
	}
	if !hasDeposit {
		return "ensure_deposit_account"
	}
	return "ready"
}

func publicDepositErrForKYC(err error) string {
	if err == nil {
		return "Could not create deposit account"
	}
	lower := strings.ToLower(err.Error())
	for _, v := range []string{"onepipe", "flutterwave", "palmpay", "circle", "not configured"} {
		if strings.Contains(lower, v) {
			return "Could not create deposit account right now. Please try again later."
		}
	}
	return "Could not create deposit account. Please try again later."
}
