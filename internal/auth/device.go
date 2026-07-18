package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/jwt"
	"flowwithlit/pkg/response"
)

// clientIP returns the best-effort client IP (honours first X-Forwarded-For hop).
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}

// deviceFingerprint hashes a stable browser signal so we can recognise "this device".
// Prefer X-Device-Id (set in browser localStorage, forwarded by PHP); then client UA.
func deviceFingerprint(r *http.Request) string {
	raw := strings.TrimSpace(r.Header.Get("X-Device-Id"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("X-Client-User-Agent"))
	}
	if raw == "" {
		raw = strings.TrimSpace(r.UserAgent())
	}
	if raw == "" {
		raw = "unknown"
	}
	sum := sha256.Sum256([]byte(strings.ToLower(raw)))
	return hex.EncodeToString(sum[:])
}

// requestUserAgent prefers the real browser UA forwarded by the PHP portal proxy.
func requestUserAgent(r *http.Request) string {
	if ua := strings.TrimSpace(r.Header.Get("X-Client-User-Agent")); ua != "" {
		return ua
	}
	return r.UserAgent()
}

// deviceLabel builds a short human-readable device name from the User-Agent.
func deviceLabel(ua string) string {
	uaLower := strings.ToLower(ua)
	browser := "Browser"
	switch {
	case strings.Contains(uaLower, "edg/"):
		browser = "Edge"
	case strings.Contains(uaLower, "chrome/") && !strings.Contains(uaLower, "edg/"):
		browser = "Chrome"
	case strings.Contains(uaLower, "firefox/"):
		browser = "Firefox"
	case strings.Contains(uaLower, "safari/") && !strings.Contains(uaLower, "chrome/"):
		browser = "Safari"
	}

	osName := "Device"
	switch {
	case strings.Contains(uaLower, "windows"):
		osName = "Windows"
	case strings.Contains(uaLower, "mac os") || strings.Contains(uaLower, "macintosh"):
		osName = "Mac"
	case strings.Contains(uaLower, "android"):
		osName = "Android"
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		osName = "iOS"
	case strings.Contains(uaLower, "linux"):
		osName = "Linux"
	}

	return browser + " on " + osName
}

func isKnownDevice(userID uint, fingerprint string) bool {
	if fingerprint == "" {
		return false
	}
	var trusted int64
	database.DB.Model(&models.TrustedDevice{}).
		Where("user_id = ? AND fingerprint = ?", userID, fingerprint).
		Count(&trusted)
	if trusted > 0 {
		return true
	}
	var sessions int64
	database.DB.Model(&models.Session{}).
		Where("user_id = ? AND fingerprint = ?", userID, fingerprint).
		Count(&sessions)
	return sessions > 0
}

func trustDevice(userID uint, fingerprint, label, ua, ip string) {
	if fingerprint == "" {
		return
	}
	var existing models.TrustedDevice
	err := database.DB.Where("user_id = ? AND fingerprint = ?", userID, fingerprint).First(&existing).Error
	if err != nil {
		database.DB.Create(&models.TrustedDevice{
			UserID:      userID,
			Fingerprint: fingerprint,
			Device:      label,
			UserAgent:   ua,
			IPAddress:   ip,
			LastSeenAt:  time.Now(),
		})
		return
	}
	database.DB.Model(&existing).Updates(map[string]interface{}{
		"device":       label,
		"user_agent":   ua,
		"ip_address":   ip,
		"last_seen_at": time.Now(),
	})
}

// startDeviceChallenge emails a one-time code and returns a temp token for verify-device.
func startDeviceChallenge(w http.ResponseWriter, r *http.Request, user models.User) {
	fp := deviceFingerprint(r)
	ip := clientIP(r)
	ua := requestUserAgent(r)
	label := deviceLabel(ua)
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)

	user.DeviceOTP = &otp
	user.DeviceOTPExpiry = &expiry
	user.DevicePendingFingerprint = fp
	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to start device verification")
		return
	}

	if err := email.SendDeviceVerifyOTP(user.Email, user.FirstName, otp, label, ip); err != nil {
		log.Printf("⚠️ device verify email failed for %s: %v", user.Email, err)
	}

	tempToken, _, err := jwt.GenerateTokens(user.ID, user.Email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create verification session")
		return
	}

	masked := maskEmail(user.Email)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"requires_device_verification": true,
		"temp_token":                  tempToken,
		"masked_email":                masked,
		"device":                      label,
		"message":                     "We sent a verification code to your email to confirm this new device.",
	})
}

func maskEmail(emailAddr string) string {
	parts := strings.Split(emailAddr, "@")
	if len(parts) != 2 {
		return "***"
	}
	local := parts[0]
	if len(local) <= 2 {
		return local[:1] + "***@" + parts[1]
	}
	return local[:2] + "***@" + parts[1]
}

// continueLoginAfterAuth runs after password (and optional TOTP). New devices require email OTP.
func continueLoginAfterAuth(w http.ResponseWriter, r *http.Request, user models.User) {
	fp := deviceFingerprint(r)
	if isKnownDevice(user.ID, fp) {
		finishLogin(w, r, user)
		return
	}
	startDeviceChallenge(w, r, user)
}

// VerifyDeviceRequest completes new-device email verification.
type VerifyDeviceRequest struct {
	TempToken string `json:"temp_token"`
	Code      string `json:"code"`
}

// VerifyDeviceHandler checks the email code for a new device and finishes login.
func VerifyDeviceHandler(w http.ResponseWriter, r *http.Request) {
	var req VerifyDeviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.TempToken == "" || req.Code == "" {
		response.Error(w, http.StatusBadRequest, "Verification code is required")
		return
	}

	claims, err := jwt.ValidateToken(req.TempToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired verification session. Please sign in again.")
		return
	}
	userID := uint(claims["user_id"].(float64))

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	fp := deviceFingerprint(r)
	if user.DevicePendingFingerprint != "" && user.DevicePendingFingerprint != fp {
		response.Error(w, http.StatusUnauthorized, "This code was issued for a different device. Please sign in again from this browser.")
		return
	}

	if user.DeviceOTP == nil || *user.DeviceOTP != req.Code {
		_ = email.SendSuspiciousActivity(
			user.Email, user.FirstName,
			"Failed new-device verification attempt",
			clientIP(r), time.Now(),
		)
		response.Error(w, http.StatusUnauthorized, "Invalid verification code")
		return
	}
	if user.DeviceOTPExpiry == nil || time.Now().After(*user.DeviceOTPExpiry) {
		response.Error(w, http.StatusUnauthorized, "Verification code has expired. Please sign in again.")
		return
	}

	// Clear challenge
	user.DeviceOTP = nil
	user.DeviceOTPExpiry = nil
	user.DevicePendingFingerprint = ""
	_ = database.DB.Save(&user).Error

	// Remember this device so next login skips email OTP
	ua := requestUserAgent(r)
	trustDevice(user.ID, fp, deviceLabel(ua), ua, clientIP(r))

	finishLogin(w, r, user)
}

// ResendDeviceCodeRequest re-sends the new-device email OTP.
type ResendDeviceCodeRequest struct {
	TempToken string `json:"temp_token"`
}

// ResendDeviceCodeHandler re-issues a device verification code.
func ResendDeviceCodeHandler(w http.ResponseWriter, r *http.Request) {
	var req ResendDeviceCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	claims, err := jwt.ValidateToken(req.TempToken)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired verification session")
		return
	}
	userID := uint(claims["user_id"].(float64))

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	fp := deviceFingerprint(r)
	otp := generateOTP()
	expiry := time.Now().Add(15 * time.Minute)
	user.DeviceOTP = &otp
	user.DeviceOTPExpiry = &expiry
	user.DevicePendingFingerprint = fp
	_ = database.DB.Save(&user).Error

	ua := requestUserAgent(r)
	label := deviceLabel(ua)
	if err := email.SendDeviceVerifyOTP(user.Email, user.FirstName, otp, label, clientIP(r)); err != nil {
		log.Printf("⚠️ device verify resend failed for %s: %v", user.Email, err)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":      "A new code has been sent to your email.",
		"masked_email": maskEmail(user.Email),
	})
}

// EndSessionRequest ends the current browser session (tab close / logout).
type EndSessionRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// EndSessionHandler revokes the session tied to the refresh token (or all matching user sessions if only bearer is present).
func EndSessionHandler(w http.ResponseWriter, r *http.Request) {
	var req EndSessionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Prefer explicit refresh token (PHP session holds it)
	if strings.TrimSpace(req.RefreshToken) != "" {
		res := database.DB.Where("token = ?", req.RefreshToken).Delete(&models.Session{})
		if res.Error != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to end session")
			return
		}
		response.Success(w, http.StatusOK, map[string]string{"message": "Session ended"})
		return
	}

	// Fallback: bearer access token → delete sessions for that user (tab close may only send bearer)
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if claims, err := jwt.ValidateToken(token); err == nil {
			if uid, ok := claims["user_id"].(float64); ok {
				// Only delete the most recent session to avoid wiping other devices
				var sess models.Session
				if err := database.DB.Where("user_id = ?", uint(uid)).Order("last_active desc").First(&sess).Error; err == nil {
					database.DB.Delete(&sess)
				}
				response.Success(w, http.StatusOK, map[string]string{"message": "Session ended"})
				return
			}
		}
	}

	// Always return OK so sendBeacon / logout never fail the UX
	response.Success(w, http.StatusOK, map[string]string{"message": "Session ended"})
}
