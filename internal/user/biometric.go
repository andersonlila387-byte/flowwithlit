package user

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/jwt"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"golang.org/x/crypto/bcrypt"
)

func hashSecret(secret string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
	return string(b), err
}

func checkSecret(hash, secret string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

func newBiometricToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func clientDeviceID(r *http.Request, bodyDevice string) string {
	if d := strings.TrimSpace(bodyDevice); d != "" {
		return d
	}
	return strings.TrimSpace(r.Header.Get("X-Device-Id"))
}

// BiometricStatusHandler — GET /user/biometric/status
// Returns whether this device has biometric login/pay enrolled.
func BiometricStatusHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	deviceID := clientDeviceID(r, r.URL.Query().Get("device_id"))
	if deviceID == "" {
		response.Error(w, http.StatusBadRequest, "device_id is required (query or X-Device-Id header)")
		return
	}

	var cred models.BiometricCredential
	err := database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&cred).Error
	if err != nil {
		response.Success(w, http.StatusOK, map[string]interface{}{
			"enrolled":      false,
			"login_enabled": false,
			"pay_enabled":   false,
			"device_id":     deviceID,
		})
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{
		"enrolled":      true,
		"login_enabled": cred.LoginEnabled,
		"pay_enabled":   cred.PayEnabled,
		"device_id":     deviceID,
		"device_label":  cred.DeviceLabel,
		"platform":      cred.Platform,
		"last_used_at":  cred.LastUsedAt,
	})
}

// BiometricEnableHandler — POST /user/biometric/enable
// Requires PIN. Returns biometric_token ONCE — store in Keychain/Keystore; never log it.
// Mobile flow: user enables Face ID/fingerprint in settings → OS biometric prompt → then call this.
func BiometricEnableHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		PIN         string `json:"pin"`
		DeviceID    string `json:"device_id"`
		DeviceLabel string `json:"device_label"`
		Platform    string `json:"platform"` // ios | android
		LoginEnabled *bool `json:"login_enabled"`
		PayEnabled   *bool `json:"pay_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	deviceID := clientDeviceID(r, req.DeviceID)
	if deviceID == "" {
		response.Error(w, http.StatusBadRequest, "device_id is required")
		return
	}
	if strings.TrimSpace(req.PIN) == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required to enable biometrics")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if user.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Set a transaction PIN before enabling biometrics")
		return
	}
	dummy := models.User{Password: user.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	plain, err := newBiometricToken()
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create biometric credential")
		return
	}
	hash, err := hashSecret(plain)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to secure biometric credential")
		return
	}

	loginOn := true
	payOn := true
	if req.LoginEnabled != nil {
		loginOn = *req.LoginEnabled
	}
	if req.PayEnabled != nil {
		payOn = *req.PayEnabled
	}
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		platform = "unknown"
	}

	// Replace existing credential for this device
	database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&models.BiometricCredential{})

	cred := models.BiometricCredential{
		UserID:       userID,
		DeviceID:     deviceID,
		TokenHash:    hash,
		DeviceLabel:  strings.TrimSpace(req.DeviceLabel),
		Platform:     platform,
		LoginEnabled: loginOn,
		PayEnabled:   payOn,
	}
	if err := database.DB.Create(&cred).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save biometric credential")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":         "Biometric credential enabled for this device",
		"biometric_token": plain, // show once — mobile stores under OS biometrics
		"device_id":       deviceID,
		"login_enabled":   loginOn,
		"pay_enabled":     payOn,
	})
}

// BiometricDisableHandler — POST /user/biometric/disable
func BiometricDisableHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	deviceID := clientDeviceID(r, req.DeviceID)
	if deviceID == "" {
		response.Error(w, http.StatusBadRequest, "device_id is required")
		return
	}
	database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).Delete(&models.BiometricCredential{})
	response.Success(w, http.StatusOK, map[string]string{"message": "Biometric login/payment disabled for this device"})
}

// BiometricAuthorizeHandler — POST /user/biometric/authorize
// After OS fingerprint/Face ID unlocks the stored biometric_token, exchange it for a short payment_token.
// Use payment_token instead of PIN on transfer/swap/withdraw for ~90 seconds.
func BiometricAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		DeviceID       string `json:"device_id"`
		BiometricToken string `json:"biometric_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	deviceID := clientDeviceID(r, req.DeviceID)
	if deviceID == "" || strings.TrimSpace(req.BiometricToken) == "" {
		response.Error(w, http.StatusBadRequest, "device_id and biometric_token are required")
		return
	}

	var cred models.BiometricCredential
	if err := database.DB.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&cred).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "Biometric payment is not enabled on this device")
		return
	}
	if !cred.PayEnabled {
		response.Error(w, http.StatusForbidden, "Biometric payment is disabled for this credential")
		return
	}
	if !checkSecret(cred.TokenHash, req.BiometricToken) {
		response.Error(w, http.StatusUnauthorized, "Invalid biometric credential")
		return
	}

	now := time.Now()
	database.DB.Model(&cred).Update("last_used_at", now)

	paymentToken, ttl, err := jwt.GeneratePaymentToken(userID, deviceID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create payment authorization")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"payment_token": paymentToken,
		"expires_in":    ttl,
		"message":       "Payment authorized — pass payment_token instead of pin on the next debit call",
	})
}

// VerifyDebitAuth accepts either a transaction PIN or a short-lived payment_token from biometrics.
func VerifyDebitAuth(user models.User, pin, paymentToken string) error {
	if strings.TrimSpace(paymentToken) != "" {
		_, err := jwt.ValidatePaymentToken(paymentToken, user.ID)
		if err != nil {
			return err
		}
		return nil
	}
	if strings.TrimSpace(pin) == "" {
		return errPINRequired
	}
	if user.TransactionPin == "" {
		return errPINNotSet
	}
	dummy := models.User{Password: user.TransactionPin}
	if err := dummy.CheckPassword(pin); err != nil {
		return errBadPIN
	}
	return nil
}

var (
	errPINRequired = &debitAuthError{msg: "Transaction PIN or payment_token is required", code: http.StatusBadRequest}
	errPINNotSet   = &debitAuthError{msg: "Please set up your 4-digit Transaction PIN first in Settings", code: http.StatusBadRequest}
	errBadPIN      = &debitAuthError{msg: "Incorrect Transaction PIN", code: http.StatusUnauthorized}
)

type debitAuthError struct {
	msg  string
	code int
}

func (e *debitAuthError) Error() string { return e.msg }
func (e *debitAuthError) Status() int   { return e.code }

// WriteDebitAuthError maps VerifyDebitAuth errors to HTTP responses.
func WriteDebitAuthError(w http.ResponseWriter, err error) {
	if e, ok := err.(*debitAuthError); ok {
		response.Error(w, e.code, e.msg)
		return
	}
	response.Error(w, http.StatusUnauthorized, "Payment authorization failed")
}
