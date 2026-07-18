package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"

	"golang.org/x/crypto/bcrypt"
)

// BiometricLoginHandler — POST /auth/biometric/login
// Mobile: OS fingerprint/Face ID unlocks biometric_token from secure storage,
// then this endpoint issues normal JWT tokens (no password prompt).
func BiometricLoginHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID       string `json:"device_id"`
		BiometricToken string `json:"biometric_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		deviceID = strings.TrimSpace(r.Header.Get("X-Device-Id"))
	}
	token := strings.TrimSpace(req.BiometricToken)
	if deviceID == "" || token == "" {
		response.Error(w, http.StatusBadRequest, "device_id and biometric_token are required")
		return
	}

	var cred models.BiometricCredential
	// Find by device — token hash verified below
	if err := database.DB.Where("device_id = ?", deviceID).First(&cred).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "Biometric login is not set up on this device")
		return
	}
	if !cred.LoginEnabled {
		response.Error(w, http.StatusForbidden, "Biometric login is disabled for this device")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.TokenHash), []byte(token)) != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid biometric credential")
		return
	}

	var user models.User
	if err := database.DB.First(&user, cred.UserID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}
	if !user.IsEmailVerified {
		response.Error(w, http.StatusForbidden, "Please verify your email address to continue.")
		return
	}

	now := time.Now()
	database.DB.Model(&cred).Update("last_used_at", now)

	// Issue normal session (same as password login finish)
	finishLogin(w, r, user)
}
