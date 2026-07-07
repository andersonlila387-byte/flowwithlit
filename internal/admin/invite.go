package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	emailPkg "flowwithlit/pkg/email"
	"fmt"
	"net/http"
	"os"
	"time"
)

func generateInviteToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func SendInviteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	adminID := r.Context().Value(AdminIDKey).(uint)

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" || req.Role == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "email and role are required"})
		return
	}

	var count int64
	database.DB.Model(&models.AdminUser{}).Where("email = ?", req.Email).Count(&count)
	if count > 0 {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "An admin with this email already exists"})
		return
	}

	// Expire any existing pending invites for this email
	database.DB.Model(&models.AdminInvite{}).
		Where("email = ? AND status = 'pending'", req.Email).
		Update("status", "expired")

	token := generateInviteToken()
	invite := models.AdminInvite{
		Email:       req.Email,
		Role:        req.Role,
		Token:       token,
		Status:      "pending",
		InvitedByID: adminID,
		ExpiresAt:   time.Now().Add(48 * time.Hour),
	}

	if err := database.DB.Create(&invite).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to create invite"})
		return
	}

	baseURL := os.Getenv("ADMIN_BASE_URL")
	if baseURL == "" {
		baseURL = "https://admin.flowwithlit.com"
	}
	inviteLink := fmt.Sprintf("%s/accept-invite?token=%s", baseURL, token)

	if err := emailPkg.SendAdminInvite(req.Email, req.Role, inviteLink); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invite created but email failed to send"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  true,
		"message": "Invite sent successfully",
		"body":    map[string]interface{}{"invite_link": inviteLink},
	})
}

func ListInvitesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var invites []models.AdminInvite
	database.DB.Order("created_at desc").Find(&invites)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"invites": invites},
	})
}

// ValidateInviteTokenHandler is public — returns invite info if token is valid
func ValidateInviteTokenHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "token is required"})
		return
	}

	var invite models.AdminInvite
	if err := database.DB.Where("token = ? AND status = 'pending'", token).First(&invite).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid or expired invite link"})
		return
	}

	if time.Now().After(invite.ExpiresAt) {
		database.DB.Model(&invite).Update("status", "expired")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "This invite link has expired"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"email": invite.Email, "role": invite.Role},
	})
}

// AcceptInviteHandler is public — creates admin account from a valid invite token
func AcceptInviteHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "token and password are required"})
		return
	}

	var invite models.AdminInvite
	if err := database.DB.Where("token = ? AND status = 'pending'", req.Token).First(&invite).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid or expired invite link"})
		return
	}

	if time.Now().After(invite.ExpiresAt) {
		database.DB.Model(&invite).Update("status", "expired")
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "This invite link has expired"})
		return
	}

	newAdmin := models.AdminUser{
		Email:    invite.Email,
		Role:     invite.Role,
		IsActive: true,
	}
	if err := newAdmin.HashPassword(req.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to process password"})
		return
	}

	if err := database.DB.Create(&newAdmin).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to create admin account"})
		return
	}

	database.DB.Model(&invite).Update("status", "accepted")

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  true,
		"message": "Account created successfully. You can now log in.",
	})
}
