package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

type VerifyEmailRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func VerifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if user.IsEmailVerified {
		response.Success(w, http.StatusOK, map[string]string{"message": "Email is already verified"})
		return
	}

	if user.VerificationOTP == nil || *user.VerificationOTP != req.Code {
		response.Error(w, http.StatusBadRequest, "Invalid or expired verification code")
		return
	}

	if user.VerificationOTPExpiry == nil || time.Now().After(*user.VerificationOTPExpiry) {
		response.Error(w, http.StatusBadRequest, "Verification code has expired. Please request a new one.")
		return
	}

	// Mark as verified
	user.IsEmailVerified = true
	user.VerificationOTP = nil
	user.VerificationOTPExpiry = nil

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to verify email")
		return
	}

	_ = email.SendNewUserWelcome(user.Email, user.FirstName)

	response.Success(w, http.StatusOK, map[string]string{"message": "Email verified successfully"})
}

type ResendVerificationRequest struct {
	Email string `json:"email"`
}

func ResendVerificationHandler(w http.ResponseWriter, r *http.Request) {
	var req ResendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Do not leak email existence
		response.Success(w, http.StatusOK, map[string]string{"message": "If the email exists, a new code will be sent."})
		return
	}

	if user.IsEmailVerified {
		response.Success(w, http.StatusOK, map[string]string{"message": "Email is already verified"})
		return
	}

	// Generate new OTP
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)

	user.VerificationOTP = &otp
	user.VerificationOTPExpiry = &expiry

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate code")
		return
	}

	_ = email.SendEmailVerificationOTP(user.Email, user.FirstName, otp)

	response.Success(w, http.StatusOK, map[string]string{"message": "A new verification code has been sent."})
}
