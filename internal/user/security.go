package user

import (
	"encoding/json"
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/pquerna/otp/totp"
)

// Generate2FAHandler creates a new TOTP secret for the user and returns the setup URL (for QR code)
func Generate2FAHandler(w http.ResponseWriter, r *http.Request) {
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

	if user.TwoFactorEnabled {
		response.Error(w, http.StatusBadRequest, "2FA is already enabled")
		return
	}

	// Generate a new TOTP key for the user (using their email as the account name)
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Flowwithlit",
		AccountName: user.Email,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate 2FA secret")
		return
	}

	// Temporarily store the secret in the database (or we could just return it and only save on verify)
	user.TwoFactorSecret = key.Secret()
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save 2FA secret")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"secret":   key.Secret(),
		"url":      key.URL(), // the frontend will generate a QR code from this URL
	})
}

// Verify2FAHandler verifies the initial code and officially enables 2FA
func Verify2FAHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if user.TwoFactorSecret == "" {
		response.Error(w, http.StatusBadRequest, "2FA setup not initiated")
		return
	}

	// Validate the code
	valid := totp.Validate(req.Code, user.TwoFactorSecret)
	if !valid {
		response.Error(w, http.StatusUnauthorized, "Invalid 2FA code")
		return
	}

	// Enable 2FA
	user.TwoFactorEnabled = true
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to enable 2FA")
		return
	}

	_ = email.Send2FAEnabled(user.Email, user.FirstName, time.Now())

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Two-Factor Authentication successfully enabled",
	})
}

// Disable2FAHandler allows the user to disable 2FA by providing a valid code
func Disable2FAHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if !user.TwoFactorEnabled {
		response.Error(w, http.StatusBadRequest, "2FA is not enabled")
		return
	}

	valid := totp.Validate(req.Code, user.TwoFactorSecret)
	if !valid {
		response.Error(w, http.StatusUnauthorized, "Invalid 2FA code")
		return
	}

	user.TwoFactorEnabled = false
	user.TwoFactorSecret = ""
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to disable 2FA")
		return
	}

	_ = email.Send2FADisabled(user.Email, user.FirstName, time.Now())

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Two-Factor Authentication successfully disabled",
	})
}

// GetSessionsHandler retrieves all active sessions for the user
func GetSessionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var sessions []models.Session
	if err := database.DB.Where("user_id = ?", userID).Order("last_active desc").Find(&sessions).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch sessions")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"sessions": sessions,
	})
}

// RevokeSessionHandler deletes a specific session
func RevokeSessionHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	sessionID := r.URL.Query().Get("id")
	if sessionID == "" {
		response.Error(w, http.StatusBadRequest, "Session ID is required")
		return
	}

	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&models.Session{}).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to revoke session")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Session successfully revoked",
	})
}
