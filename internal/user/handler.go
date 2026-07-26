package user

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"golang.org/x/crypto/bcrypt"
)

// GetMeHandler returns the currently authenticated user's profile.
// Same user record for web (business) and mobile — shared PIN and wallets.
func GetMeHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve userID from context (injected by the RequireAuth middleware)
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	hasTransactionPin := user.TransactionPin != ""

	// Mobile flags for this device (optional header X-Device-Id)
	deviceID := strings.TrimSpace(r.Header.Get("X-Device-Id"))
	biometricEnrolled := false
	biometricLogin := false
	biometricPay := false
	if deviceID != "" {
		var cred models.BiometricCredential
		if err := database.DB.Where("user_id = ? AND device_id = ?", user.ID, deviceID).First(&cred).Error; err == nil {
			biometricEnrolled = true
			biometricLogin = cred.LoginEnabled
			biometricPay = cred.PayEnabled
		}
	}
	var pushCount int64
	database.DB.Model(&models.PushDevice{}).Where("user_id = ? AND enabled = ?", user.ID, true).Count(&pushCount)

	// Live balances (same wallets as web dashboard)
	balances := envfilter.BalancesForEnv(userID, "live")

	// Business flag for mobile UI (merchant tools vs personal)
	hasBusiness := false
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		hasBusiness = strings.TrimSpace(profile.BusinessName) != "" &&
			!strings.EqualFold(profile.BusinessName, "Personal Account")
	}

	// Double check sensitive fields are cleared
	user.Password = ""
	user.TransactionPin = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"user":                 user,
		"has_transaction_pin":  hasTransactionPin,
		"balances":             balances,
		"has_business":         hasBusiness,
		"kyc_level":            user.KYCLevel,
		"biometric_enrolled":   biometricEnrolled,
		"biometric_login":      biometricLogin,
		"biometric_pay":        biometricPay,
		"push_devices":         pushCount,
	})
}

// MobileHomeHandler — GET /user/mobile/home
// One-shot payload for the mobile home screen: profile, PIN flag, balances, KYC, deposit account.
// Business users who set a 4-digit PIN on web use that same PIN on mobile; balances are shared.
func MobileHomeHandler(w http.ResponseWriter, r *http.Request) {
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

	hasPIN := user.TransactionPin != ""
	balances := envfilter.BalancesForEnv(userID, "live")

	// Ensure default wallets exist so mobile always has NGN/USDT rows
	var wallets []models.Wallet
	database.DB.Where("user_id = ?", userID).Find(&wallets)
	if len(wallets) == 0 {
		for _, cur := range []string{"NGN", "USDT"} {
			w := models.Wallet{UserID: userID, Currency: cur, Balance: 0}
			database.DB.Create(&w)
			wallets = append(wallets, w)
		}
		balances = envfilter.BalancesForEnv(userID, "live")
	}

	kycStatus := "none"
	hasBusiness := false
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		kycStatus = profile.KYCStatus
		hasBusiness = strings.TrimSpace(profile.BusinessName) != "" &&
			!strings.EqualFold(profile.BusinessName, "Personal Account")
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

	user.Password = ""
	user.TransactionPin = ""

	// Onboarding / tracking for a faster mobile experience (progress ring + next step)
	deviceID := strings.TrimSpace(r.Header.Get("X-Device-Id"))
	bioEnrolled := false
	if deviceID != "" {
		var cred models.BiometricCredential
		if err := database.DB.Where("user_id = ? AND device_id = ?", user.ID, deviceID).First(&cred).Error; err == nil {
			bioEnrolled = true
		}
	}
	hasDeposit := deposit != nil && strings.TrimSpace(fmt.Sprint(deposit["account_number"])) != ""
	kycOk := user.KYCLevel >= 1 || strings.EqualFold(kycStatus, "approved") || strings.EqualFold(kycStatus, "verified")

	type trackStep struct {
		ID       string `json:"id"`
		Label    string `json:"label"`
		Done     bool   `json:"done"`
		Optional bool   `json:"optional,omitempty"`
		Route    string `json:"route,omitempty"` // app deep-link hint
	}
	steps := []trackStep{
		{ID: "email", Label: "Verify email", Done: user.IsEmailVerified, Route: "verify_email"},
		{ID: "phone", Label: "Verify phone", Done: user.IsPhoneVerified, Route: "verify_phone"},
		{ID: "pin", Label: "Set transaction PIN", Done: hasPIN, Route: "settings_pin"},
		{ID: "kyc", Label: "Complete identity KYC", Done: kycOk, Route: "kyc"},
		{ID: "deposit", Label: "Get deposit account", Done: hasDeposit, Route: "add_funds"},
		{ID: "biometric", Label: "Enable fingerprint", Done: bioEnrolled, Optional: true, Route: "settings_biometric"},
	}
	requiredDone, requiredTotal := 0, 0
	var next *trackStep
	for i := range steps {
		if steps[i].Optional {
			continue
		}
		requiredTotal++
		if steps[i].Done {
			requiredDone++
		} else if next == nil {
			cp := steps[i]
			next = &cp
		}
	}
	percent := 0
	if requiredTotal > 0 {
		percent = (requiredDone * 100) / requiredTotal
	}
	nextAction := "You're all set"
	nextID := ""
	if next != nil {
		nextAction = next.Label
		nextID = next.ID
	}

	// Lightweight recent LIVE activity only (never sandbox/test checkout rows)
	var recent []models.Transaction
	database.DB.Where("user_id = ?", userID).
		Where("NOT (is_test = ? OR provider = ?)", true, "test").
		Order("created_at desc").Limit(8).Find(&recent)
	if recent == nil {
		recent = []models.Transaction{}
	}
	// Strip heavy fields for mobile payload
	activity := make([]map[string]interface{}, 0, len(recent))
	for _, t := range recent {
		activity = append(activity, map[string]interface{}{
			"id":          t.ID,
			"reference":   t.Reference,
			"amount":      t.Amount,
			"currency":    t.Currency,
			"type":        t.Type,
			"status":      t.Status,
			"description": t.Description,
			"created_at":  t.CreatedAt,
		})
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":                user.ID,
			"email":             user.Email,
			"first_name":        user.FirstName,
			"last_name":         user.LastName,
			"phone":             user.Phone,
			"flowtag_username":  user.FlowTagUsername,
			"account_type":      user.AccountType,
			"kyc_level":         user.KYCLevel,
			"is_email_verified": user.IsEmailVerified,
			"is_phone_verified": user.IsPhoneVerified,
			"default_fiat":      user.DefaultFiatCurrency,
			"default_crypto":    user.DefaultCryptoCurrency,
			"profile_image":     user.ProfileImage,
		},
		"has_transaction_pin": hasPIN,
		"balances":            balances,
		"wallets":             wallets,
		"kyc_status":          kycStatus,
		"has_business":        hasBusiness,
		"deposit_account":     deposit,
		// Same PIN as web Settings → Transaction PIN; same wallets as web dashboard
		"pin_note": "The 4-digit transaction PIN set on web is used for mobile transfers and payments.",
		// Progress tracking for cool, fast mobile onboarding UX
		"tracking": map[string]interface{}{
			"percent":     percent,
			"done":        requiredDone,
			"total":       requiredTotal,
			"complete":    requiredDone >= requiredTotal,
			"next_step":   nextID,
			"next_action": nextAction,
			"steps":       steps,
		},
		"recent_activity": activity,
		"quick_actions": []map[string]string{
			{"id": "transfer", "label": "Transfer", "route": "transfers"},
			{"id": "fund", "label": "Add money", "route": "add_funds"},
			{"id": "bills", "label": "Airtime", "route": "bills"},
			{"id": "swap", "label": "Swap", "route": "swap"},
			{"id": "family", "label": "Family", "route": "family"},
			{"id": "vaults", "label": "Vaults", "route": "vaults"},
		},
	})
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdatePasswordHandler updates the user's password
func UpdatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req UpdatePasswordRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if len(req.NewPassword) < 8 {
		response.Error(w, http.StatusBadRequest, "New password must be at least 8 characters")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if err := user.CheckPassword(req.CurrentPassword); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect current password")
		return
	}

	if err := user.HashPassword(req.NewPassword); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to hash new password")
		return
	}

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	_ = email.SendPasswordChanged(user.Email, user.FirstName, time.Now())

	response.Success(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

type SetupPINRequest struct {
	PIN        string `json:"pin"`
	ConfirmPIN string `json:"confirm_pin"`
	// CurrentPIN required when changing an existing PIN (not when setting the first time).
	CurrentPIN string `json:"current_pin"`
}

// SetupPINHandler sets or changes the transaction PIN.
// PIN is stored as bcrypt only (never plaintext). Never returned in API responses.
func SetupPINHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req SetupPINRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	pin := strings.TrimSpace(req.PIN)
	confirm := strings.TrimSpace(req.ConfirmPIN)
	if confirm == "" {
		// Backward compatible: if confirm omitted, treat as pin (mobile old clients)
		confirm = pin
	}

	if !isValidTransactionPIN(pin) {
		response.Error(w, http.StatusBadRequest, "PIN must be exactly 4 digits (0–9). Avoid sequences like 1234 if possible.")
		return
	}
	if pin != confirm {
		response.Error(w, http.StatusBadRequest, "PIN and confirmation do not match")
		return
	}
	// Reject trivial PINs
	if isWeakPIN(pin) {
		response.Error(w, http.StatusBadRequest, "That PIN is too easy to guess. Please choose a different 4-digit code.")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	alreadySet := strings.TrimSpace(user.TransactionPin) != ""
	if alreadySet {
		cur := strings.TrimSpace(req.CurrentPIN)
		if cur == "" {
			response.Error(w, http.StatusBadRequest, "Enter your current PIN to set a new one")
			return
		}
		dummy := models.User{Password: user.TransactionPin}
		if err := dummy.CheckPassword(cur); err != nil {
			response.Error(w, http.StatusUnauthorized, "Current PIN is incorrect")
			return
		}
		if cur == pin {
			response.Error(w, http.StatusBadRequest, "New PIN must be different from your current PIN")
			return
		}
	}

	// Hash PIN with bcrypt independently of login password (never touch Password column)
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not secure your PIN. Please try again.")
		return
	}

	if err := database.DB.Model(&user).Update("transaction_pin", string(hash)).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not save your PIN. Please try again.")
		return
	}

	msg := "Transaction PIN set successfully"
	if alreadySet {
		msg = "Transaction PIN updated successfully"
		_ = email.SendPasswordChanged(user.Email, user.FirstName, time.Now()) // reuse security alert email
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":             msg,
		"has_transaction_pin": true,
	})
}

func isValidTransactionPIN(pin string) bool {
	if len(pin) != 4 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func isWeakPIN(pin string) bool {
	// All same digit
	if pin[0] == pin[1] && pin[1] == pin[2] && pin[2] == pin[3] {
		return true
	}
	// Simple ascending / descending sequences
	weak := map[string]bool{
		"0123": true, "1234": true, "2345": true, "3456": true, "4567": true,
		"5678": true, "6789": true, "9876": true, "8765": true, "7654": true,
		"6543": true, "5432": true, "4321": true, "3210": true, "0000": true,
		"1111": true, "2222": true, "3333": true, "4444": true, "5555": true,
		"6666": true, "7777": true, "8888": true, "9999": true, "1122": true,
		"1212": true, "2580": true,
	}
	return weak[pin]
}

// VerifyPINHandler checks if the provided PIN is correct (bcrypt compare).
// Never logs or returns the PIN.
func VerifyPINHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req SetupPINRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	pin := strings.TrimSpace(req.PIN)
	if !isValidTransactionPIN(pin) {
		response.Error(w, http.StatusBadRequest, "PIN must be exactly 4 digits")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if strings.TrimSpace(user.TransactionPin) == "" {
		response.Error(w, http.StatusBadRequest, "You have not set a transaction PIN yet")
		return
	}

	dummy := models.User{Password: user.TransactionPin}
	if err := dummy.CheckPassword(pin); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect PIN")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "PIN verified successfully"})
}


type UpdateProfileRequest struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Phone     string  `json:"phone"`
	Bio       *string `json:"bio"` // optional; empty string clears
}

func UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req UpdateProfileRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updates := map[string]interface{}{}
	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}
	if req.Bio != nil {
		bio := strings.TrimSpace(*req.Bio)
		if len(bio) > 280 {
			response.Error(w, http.StatusBadRequest, "Bio must be 280 characters or less")
			return
		}
		updates["bio"] = bio
	}

	if len(updates) == 0 {
		response.Error(w, http.StatusBadRequest, "No fields to update")
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	var user models.User
	database.DB.First(&user, userID)
	user.Password = ""
	user.TransactionPin = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Profile updated successfully",
		"user":    user,
	})
}

// UpdateProfileImageHandler updates the user's profile image URL
func UpdateProfileImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req struct {
		ProfileImageURL string `json:"profile_image_url"`
		CoverImageURL   string `json:"cover_image_url"`
	}
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(req.ProfileImageURL) != "" {
		updates["profile_image"] = strings.TrimSpace(req.ProfileImageURL)
	}
	if strings.TrimSpace(req.CoverImageURL) != "" {
		updates["cover_image"] = strings.TrimSpace(req.CoverImageURL)
	}
	if len(updates) == 0 {
		response.Error(w, http.StatusBadRequest, "profile_image_url or cover_image_url is required")
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update profile image")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Profile image updated successfully",
	})
}
