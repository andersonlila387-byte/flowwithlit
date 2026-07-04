package admin

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

var allowedUserQuickActions = map[string]bool{
	"kyc_reminder":       true,
	"email_verification": true,
	"security_checkup":   true,
	"account_activation": true,
}

type UserQuickActionRequest struct {
	UserID uint   `json:"user_id"`
	Action string `json:"action"`
}

func appFrontendURL() string {
	if u := os.Getenv("FRONTEND_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost/flowwithlit/app"
}

func generateUserOTP() string {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// SendUserQuickActionHandler sends a targeted automated email to a single user.
// POST /admin/users/quick-action  { "user_id": 1, "action": "kyc_reminder" }
func SendUserQuickActionHandler(w http.ResponseWriter, r *http.Request) {
	var req UserQuickActionRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	if req.UserID == 0 || !allowedUserQuickActions[action] {
		response.Error(w, http.StatusBadRequest, "user_id and a valid action are required")
		return
	}

	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if strings.TrimSpace(user.Email) == "" {
		response.Error(w, http.StatusBadRequest, "User has no email address on file")
		return
	}

	firstName := strings.TrimSpace(user.FirstName)
	if firstName == "" {
		firstName = "there"
	}

	baseURL := appFrontendURL()
	var subject string
	var template string
	var vars map[string]interface{}
	var skipReason string

	switch action {
	case "kyc_reminder":
		if user.KYCLevel >= 2 {
			skipReason = "User is already fully verified (Tier 2)"
			break
		}
		subject = "Complete Your Verification — Flowwithlit"
		template = "kyc-reminder"
		tierNote := "You have not started verification yet."
		if user.KYCLevel == 1 {
			tierNote = "You are on Tier 1 — complete Tier 2 to unlock higher limits and full merchant features."
		}
		vars = map[string]interface{}{
			"first_name": firstName, "kyc_url": baseURL + "/kyc.php",
			"tier_note": tierNote, "kyc_level": user.KYCLevel,
		}

	case "email_verification":
		if user.IsEmailVerified {
			skipReason = "User email is already verified"
			break
		}
		otp := generateUserOTP()
		expiry := time.Now().Add(15 * time.Minute)
		user.VerificationOTP = &otp
		user.VerificationOTPExpiry = &expiry
		if err := database.DB.Save(&user).Error; err != nil {
			response.Error(w, http.StatusInternalServerError, "Failed to generate verification code")
			return
		}
		subject = "Verify Your Email — Flowwithlit"
		template = "verify-email"
		vars = map[string]interface{}{"first_name": firstName, "otp": otp, "expiry_minutes": "15"}

	case "security_checkup":
		subject = "Secure Your Account — Flowwithlit"
		template = "security-checkup"
		vars = map[string]interface{}{"first_name": firstName, "settings_url": baseURL + "/settings.php"}

	case "account_activation":
		subject = "Activate Your Wallet — Flowwithlit"
		template = "account-activation"
		vars = map[string]interface{}{"first_name": firstName, "dashboard_url": baseURL + "/index.php"}
	}

	if skipReason != "" {
		response.Success(w, http.StatusOK, map[string]interface{}{
			"sent":    false,
			"action":  action,
			"message": skipReason,
		})
		return
	}

	if err := email.SendTemplateMail(user.Email, subject, template, vars); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to send email. Check SMTP settings.")
		return
	}

	adminIDVal, _ := r.Context().Value(AdminIDKey).(uint)
	var adminUser models.AdminUser
	database.DB.Select("email").Where("id = ?", adminIDVal).First(&adminUser)

	WriteAuditLog(
		adminIDVal,
		adminUser.Email,
		"user_quick_action_"+action,
		"user",
		strconv.FormatUint(uint64(user.ID), 10),
		fmt.Sprintf("Sent %s email to %s", action, user.Email),
		r.RemoteAddr,
	)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"sent":    true,
		"action":  action,
		"email":   user.Email,
		"message": "Email sent successfully to " + user.Email,
	})
}