package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/jwt"
	"flowwithlit/pkg/response"
)

// RefreshHandler — POST /auth/refresh
// Body: { "refresh_token": "..." } (or Authorization: Bearer <refresh>)
// Issues a new access + refresh pair when the refresh token is still valid.
func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	token := strings.TrimSpace(req.RefreshToken)
	if token == "" {
		auth := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
			token = strings.TrimSpace(auth[7:])
		}
	}
	if token == "" {
		response.Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	claims, err := jwt.ValidateToken(token)
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Prefer sessions table when this refresh token was recorded at login
	var sess models.Session
	hasSession := database.DB.Where("token = ?", token).First(&sess).Error == nil

	rawID, ok := claims["user_id"]
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Invalid refresh token claims")
		return
	}
	var userID uint
	switch v := rawID.(type) {
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	case int64:
		userID = uint(v)
	default:
		response.Error(w, http.StatusUnauthorized, "Invalid refresh token claims")
		return
	}

	if hasSession && sess.UserID != 0 && sess.UserID != userID {
		response.Error(w, http.StatusUnauthorized, "Session does not match token")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusUnauthorized, "User not found")
		return
	}

	accessToken, refreshToken, err := jwt.GenerateTokens(user.ID, user.Email)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Could not issue tokens")
		return
	}

	// Rotate session token when we know the old refresh token row
	if hasSession && sess.ID != 0 {
		database.DB.Model(&sess).Updates(map[string]interface{}{
			"token":       refreshToken,
			"last_active": time.Now(),
		})
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"message":      "Token refreshed",
	})
}
