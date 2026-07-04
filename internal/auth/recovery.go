package auth

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

// generateOTP generates a secure 6-digit numeric code
func generateOTP() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "123456" // fallback
	}
	return fmt.Sprintf("%06d", n.Int64())
}

type ForgotPasswordRequest struct {
	Email string `json:"email"`
}

func ForgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		// Do not leak existence of email. Just return success.
		response.Success(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link will be sent."})
		return
	}

	// Generate a 6-digit OTP
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)

	user.ResetOTP = &otp
	user.ResetOTPExpiry = &expiry

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to initiate recovery")
		return
	}

	_ = email.SendPasswordResetOTP(user.Email, user.FirstName, otp)

	response.Success(w, http.StatusOK, map[string]string{"message": "If the email exists, a reset link will be sent."})
}

type VerifyResetCodeRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func VerifyResetCodeHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyResetCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if user.ResetOTP == nil || *user.ResetOTP != req.Code {
		response.Error(w, http.StatusBadRequest, "Invalid or expired reset code")
		return
	}

	if user.ResetOTPExpiry == nil || time.Now().After(*user.ResetOTPExpiry) {
		response.Error(w, http.StatusBadRequest, "Reset code has expired")
		return
	}

	response.Success(w, http.StatusOK, map[string]bool{"valid": true})
}

type ResetPasswordRequest struct {
	Email    string `json:"email"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

func ResetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Verify the OTP
	if user.ResetOTP == nil || *user.ResetOTP != req.Code {
		response.Error(w, http.StatusBadRequest, "Invalid or expired reset code")
		return
	}

	// Verify expiry
	if user.ResetOTPExpiry == nil || time.Now().After(*user.ResetOTPExpiry) {
		response.Error(w, http.StatusBadRequest, "Reset code has expired")
		return
	}

	// Hash the new password
	if err := user.HashPassword(req.Password); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// Clear the OTP fields
	user.ResetOTP = nil
	user.ResetOTPExpiry = nil

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	_ = email.SendPasswordChanged(user.Email, user.FirstName, time.Now())

	response.Success(w, http.StatusOK, map[string]string{"message": "Password reset successful"})
}
