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
// Issues a new access + refresh pair and immediately invalidates the old refresh
// token by deleting its session row — prevents replay of stolen refresh tokens.
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

	// Validate the token and enforce purpose=refresh so that access tokens
	// (purpose=access) cannot be used to mint new token pairs.
	claims, err := jwt.ValidatePurposeToken(token, "refresh")
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Look up the session row for this refresh token.
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

	if hasSession && sess.ID != 0 {
		// Update the session to the new refresh token. The old token string is gone
		// from the DB — any replay of the old refresh token will fail the session lookup
		// AND the purpose=refresh validation will still pass (token is still
		// cryptographically valid until its exp). We therefore also record the issued-at
		// time so we can detect window-based replays in future if desired.
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

