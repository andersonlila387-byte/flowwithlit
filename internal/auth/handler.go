package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/referral"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/jwt"
	"flowwithlit/pkg/push"
	"flowwithlit/pkg/response"

	"github.com/pquerna/otp/totp"
)

// RegisterRequest defines the expected JSON body for registration
type RegisterRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ReferralCode string `json:"referral_code"`
}

// RegisterHandler handles new user sign-ups
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Email == "" || req.Password == "" {
		response.Error(w, http.StatusBadRequest, "Email and Password are required")
		return
	}

	user := models.User{
		Email:     req.Email,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	}

	if err := user.HashPassword(req.Password); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// Generate a verification OTP instead of instantly logging them in
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)

	user.VerificationOTP = &otp
	user.VerificationOTPExpiry = &expiry

	username, err := database.GenerateFlowTagUsername(req.FirstName)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate FlowTag username")
		return
	}
	user.FlowTagUsername = username

	if err := database.DB.Create(&user).Error; err != nil {
		response.Error(w, http.StatusConflict, "Email already exists or failed to create user")
		return
	}

	if code, err := referral.EnsureUserCode(user.ID); err == nil {
		user.ReferralCode = code
	}
	if req.ReferralCode != "" {
		_ = referral.AttachReferrer(user.ID, req.ReferralCode)
	}

	// Send verification email immediately (no background queue — must not be skipped)
	if err := email.SendEmailVerificationOTP(user.Email, user.FirstName, otp); err != nil {
		log.Printf("⚠️ verification email failed for %s: %v", user.Email, err)
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message": "Registration successful. Please verify your email.",
		"user":    user,
	})
}

// LoginRequest defines the expected JSON body for login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginHandler authenticates a user and returns a JWT
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if err := user.CheckPassword(req.Password); err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	if !user.IsEmailVerified {
		response.Error(w, http.StatusForbidden, "Please verify your email address to continue.")
		return
	}

	if user.TwoFactorEnabled {
		// Generate a temporary token for 2FA validation
		tempToken, _, _ := jwt.GenerateTokens(user.ID, user.Email) // Reuse jwt for temp auth
		
		response.Success(w, http.StatusOK, map[string]interface{}{
			"requires_2fa": true,
			"temp_token":   tempToken,
		})
		return
	}

	// Password OK — known devices finish login; new devices get email OTP first.
	continueLoginAfterAuth(w, r, user)
}

// Login2FARequest defines the expected JSON body for 2FA login
type Login2FARequest struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

// Login2FAHandler verifies the TOTP code and finishes login
func Login2FAHandler(w http.ResponseWriter, r *http.Request) {
	var req Login2FARequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	// Validate Temp Token (which is just an access token for the first step)
	claims, err := jwt.ValidateToken(req.TempToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired temp token")
		return
	}

	userID := uint(claims["user_id"].(float64))
	
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	// Validate the 2FA Code
	valid := totp.Validate(req.Code, user.TwoFactorSecret)
	if !valid {
		ip := r.RemoteAddr
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			ip = forwardedFor
		}
		_ = email.SendSuspiciousActivity(
			user.Email, user.FirstName,
			"Failed two-factor authentication sign-in attempt",
			ip, time.Now(),
		)
		response.Error(w, http.StatusUnauthorized, "Invalid 2FA code")
		return
	}

	// After TOTP, still require email OTP on brand-new devices.
	continueLoginAfterAuth(w, r, user)
}

// finishLogin generates final tokens, records the session, and sends the response
func finishLogin(w http.ResponseWriter, r *http.Request, user models.User) {
	// Generate both JWT Tokens
	accessToken, refreshToken, err := jwt.GenerateTokens(user.ID, user.Email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	ip := clientIP(r)
	ua := requestUserAgent(r)
	fp := deviceFingerprint(r)
	label := deviceLabel(ua)

	session := models.Session{
		UserID:      user.ID,
		Token:       refreshToken, // Using refresh token to uniquely identify session
		IPAddress:   ip,
		UserAgent:   ua,
		Device:      label,
		Fingerprint: fp,
		LastActive:  time.Now(),
	}
	database.DB.Create(&session)

	// Keep trusted-device last-seen fresh for known devices
	if isKnownDevice(user.ID, fp) {
		trustDevice(user.ID, fp, label, ua, ip)
	}

	// Login alert — synchronous send via PHPMailer dispatch
	_ = email.SendLoginAlert(user.Email, user.FirstName, ip)

	// Mobile push (no-op if FCM not configured / no devices registered)
	_ = push.SendToUser(user.ID, "New sign-in", "Your Flowwithlit account was signed in from "+label, map[string]string{
		"type": "login",
		"ip":   ip,
	})
	activity.Info("auth", "login_ok", "User signed in via "+label, activity.UID(user.ID), "", ip)

	// Return strictly matching what frontend expects
	response.Success(w, http.StatusOK, map[string]interface{}{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user":         user,
	})
}
