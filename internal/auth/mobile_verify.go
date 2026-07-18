package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// Mobile verification is NOT the same as web:
// - Web: email OTP + optional TOTP + session cookie
// - Mobile: phone SMS OTP + email OTP + device_id binding + biometric + push token
// SMS is soft-mode: if SMS provider is not configured, OTP is logged (dev) / emailed as fallback.

// SendPhoneOTPHandler — POST /auth/mobile/send-phone-otp
// Body: { "phone": "+234..." } OR authenticated user updates own phone.
func SendPhoneOTPHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Email string `json:"email"` // optional: identify user before login
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	phone := normalizePhone(req.Phone)
	if phone == "" {
		response.Error(w, http.StatusBadRequest, "phone is required")
		return
	}

	otp := generateOTP()
	expiry := time.Now().Add(10 * time.Minute)

	// Prefer authenticated user
	if uid, ok := r.Context().Value(middleware.UserIDKey).(uint); ok && uid > 0 {
		var user models.User
		if err := database.DB.First(&user, uid).Error; err == nil {
			user.Phone = phone
			user.PhoneOTP = &otp
			user.PhoneOTPExpiry = &expiry
			database.DB.Save(&user)
			dispatchPhoneOTP(user, phone, otp)
			response.Success(w, http.StatusOK, map[string]interface{}{
				"message":        "OTP sent for phone verification",
				"phone_masked":   maskPhone(phone),
				"expires_in_sec": 600,
				"channel":        "sms_or_fallback",
			})
			return
		}
	}

	// Unauthenticated: find by email+phone registration step
	emailAddr := strings.TrimSpace(strings.ToLower(req.Email))
	var user models.User
	q := database.DB.Where("phone = ?", phone)
	if emailAddr != "" {
		q = database.DB.Where("email = ?", emailAddr)
	}
	if err := q.First(&user).Error; err != nil {
		// Do not leak existence
		response.Success(w, http.StatusOK, map[string]interface{}{
			"message":      "If the account exists, an OTP was sent",
			"phone_masked": maskPhone(phone),
		})
		return
	}
	user.Phone = phone
	user.PhoneOTP = &otp
	user.PhoneOTPExpiry = &expiry
	database.DB.Save(&user)
	dispatchPhoneOTP(user, phone, otp)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":        "OTP sent for phone verification",
		"phone_masked":   maskPhone(phone),
		"expires_in_sec": 600,
	})
}

// VerifyPhoneOTPHandler — POST /auth/mobile/verify-phone-otp
func VerifyPhoneOTPHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		response.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	var user models.User
	if uid, ok := r.Context().Value(middleware.UserIDKey).(uint); ok && uid > 0 {
		if err := database.DB.First(&user, uid).Error; err != nil {
			response.Error(w, http.StatusNotFound, "User not found")
			return
		}
	} else {
		phone := normalizePhone(req.Phone)
		emailAddr := strings.TrimSpace(strings.ToLower(req.Email))
		q := database.DB
		if emailAddr != "" {
			q = q.Where("email = ?", emailAddr)
		} else if phone != "" {
			q = q.Where("phone = ?", phone)
		} else {
			response.Error(w, http.StatusBadRequest, "email or phone required")
			return
		}
		if err := q.First(&user).Error; err != nil {
			response.Error(w, http.StatusUnauthorized, "Invalid code")
			return
		}
	}

	if user.PhoneOTP == nil || *user.PhoneOTP != code {
		activity.Warning("auth", "phone_otp_fail", "Invalid phone OTP", &user.ID, "", r.RemoteAddr)
		response.Error(w, http.StatusUnauthorized, "Invalid verification code")
		return
	}
	if user.PhoneOTPExpiry == nil || time.Now().After(*user.PhoneOTPExpiry) {
		response.Error(w, http.StatusUnauthorized, "Code expired — request a new one")
		return
	}

	user.IsPhoneVerified = true
	user.PhoneOTP = nil
	user.PhoneOTPExpiry = nil
	if p := normalizePhone(req.Phone); p != "" {
		user.Phone = p
	}
	database.DB.Save(&user)
	activity.Success("auth", "phone_verified", "Phone verified on mobile", &user.ID, "", r.RemoteAddr)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":           "Phone verified successfully",
		"is_phone_verified": true,
		"phone_masked":      maskPhone(user.Phone),
	})
}

// MobileRegisterHandler — POST /auth/mobile/register
// Mobile-first signup: collects phone + DOB + device_id; blocks under-18 from adult account.
func MobileRegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email        string `json:"email"`
		Password     string `json:"password"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name"`
		Phone        string `json:"phone"`
		DateOfBirth  string `json:"date_of_birth"` // YYYY-MM-DD required on mobile
		ReferralCode string `json:"referral_code"`
		DeviceID     string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	if req.Email == "" || req.Password == "" || req.FirstName == "" {
		response.Error(w, http.StatusBadRequest, "email, password and first_name are required")
		return
	}
	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.DateOfBirth))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "date_of_birth is required on mobile (YYYY-MM-DD)")
		return
	}
	age := time.Now().Year() - dob.Year()
	if time.Now().YearDay() < dob.YearDay() {
		age--
	}
	if age < 18 {
		response.Error(w, http.StatusForbidden,
			"You must be 18+ to open a full account. Under-18 accounts are created by a parent in Family → Kids (in the child's name).")
		return
	}

	// Reuse standard register path fields
	user := models.User{
		Email:           strings.TrimSpace(strings.ToLower(req.Email)),
		FirstName:       strings.TrimSpace(req.FirstName),
		LastName:        strings.TrimSpace(req.LastName),
		Phone:           normalizePhone(req.Phone),
		AccountType:     "USER",
		DateOfBirth:     &dob,
		IsPhoneVerified: false,
	}
	if err := user.HashPassword(req.Password); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to process password")
		return
	}
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)
	user.VerificationOTP = &otp
	user.VerificationOTPExpiry = &expiry

	username, err := database.GenerateFlowTagUsername(req.FirstName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate username")
		return
	}
	user.FlowTagUsername = username

	if err := database.DB.Create(&user).Error; err != nil {
		response.Error(w, http.StatusConflict, "Email already exists or failed to create user")
		return
	}

	if err := email.SendEmailVerificationOTP(user.Email, user.FirstName, otp); err != nil {
		log.Printf("mobile register email OTP failed: %v", err)
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message": "Account created. Next: verify email OTP, then verify phone OTP (mobile flow).",
		"next_steps": []string{
			"POST /auth/verify-email { email, code }",
			"POST /auth/mobile/send-phone-otp { phone }",
			"POST /auth/mobile/verify-phone-otp { phone, code }",
			"POST /auth/login — then register push + optional biometric",
		},
		"user_id": user.ID,
		"email":   user.Email,
	})
}

func normalizePhone(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, " ", "")
	p = strings.ReplaceAll(p, "-", "")
	return p
}

func maskPhone(p string) string {
	if len(p) < 6 {
		return "***"
	}
	return p[:3] + "****" + p[len(p)-2:]
}

func dispatchPhoneOTP(user models.User, phone, otp string) {
	// Soft SMS: no paid SMS gateway required for dev.
	// Prefer email fallback so OTP always reaches the user without extra cost.
	log.Printf("[MobileOTP] phone=%s user=%d otp=%s (configure SMS provider for live SMS)", phone, user.ID, otp)
	if user.Email != "" {
		_ = email.SendEmailVerificationOTP(user.Email, user.FirstName, otp)
	}
}
