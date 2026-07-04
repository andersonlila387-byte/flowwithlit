package admin

import (
	"context"
	"encoding/json"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

// JWT key for admins (should Ideally be separate from standard JWT_SECRET, but we'll use a prefix or claim to distinguish)
func getAdminJWTSecret() []byte {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "fallback_super_secret_key_123!"
	}
	// Prefix it or just use the same secret, but the claim MUST explicitly say it's an admin
	return []byte(secret)
}

// AdminLoginRequest represents the payload for admin login
type AdminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AdminLogin handles the authentication of bank staff and admins
func AdminLogin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req AdminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid request payload"})
		return
	}

	var admin models.AdminUser
	if err := database.DB.Where("email = ?", req.Email).First(&admin).Error; err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid credentials"})
		return
	}

	if err := admin.CheckPassword(req.Password); err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid credentials"})
		return
	}

	if !admin.IsActive {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Your account has been deactivated. Contact a Super Admin."})
		return
	}

	// Create JWT token specifically for admins
	expirationTime := time.Now().Add(24 * time.Hour) // Admins get 24h tokens
	claims := &jwt.MapClaims{
		"admin_id": admin.ID,
		"role":     admin.Role,
		"is_admin": true, // Security flag to ensure standard users can't use their token here
		"exp":      jwt.NewNumericDate(expirationTime),
		"iat":      jwt.NewNumericDate(time.Now()),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(getAdminJWTSecret())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Could not generate token"})
		return
	}

	// Update last login
	now := time.Now()
	database.DB.Model(&admin).Update("last_login", &now)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  true,
		"message": "Login successful",
		"body": map[string]interface{}{
			"admin_token": tokenString,
			"role":        admin.Role,
			"email":       admin.Email,
		},
	})
}

// AdminRegisterStatus reports how many admin slots remain (max 3).
// GET /admin/auth/register-status
func AdminRegisterStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	const maxAdmins int64 = 3
	var adminCount int64
	if err := database.DB.Model(&models.AdminUser{}).Count(&adminCount).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Database error"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"data": map[string]interface{}{
			"count":   adminCount,
			"max":     maxAdmins,
			"allowed": adminCount < maxAdmins,
		},
	})
}

// AdminRegister seeds admin accounts (max 3 total).
// In production, this should be removed or highly restricted.
func AdminRegister(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid payload"})
		return
	}

	const maxAdmins = 3

	var adminCount int64
	if err := database.DB.Model(&models.AdminUser{}).Count(&adminCount).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Database error while checking admins"})
		return
	}
	if adminCount >= maxAdmins {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Maximum of 3 admin accounts reached. Registration is locked."})
		return
	}

	// Check if already exists (redundant now but safe)
	var existing models.AdminUser
	if err := database.DB.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Admin already exists"})
		return
	} else if err != gorm.ErrRecordNotFound {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Database error"})
		return
	}

	if req.Role == "" {
		req.Role = "SUPER_ADMIN"
	}

	newAdmin := models.AdminUser{
		Email: req.Email,
		Role:  req.Role,
	}
	if err := newAdmin.HashPassword(req.Password); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to hash password"})
		return
	}

	if err := database.DB.Create(&newAdmin).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to create admin"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  true,
		"message": "Admin user created successfully",
	})
}

type contextKey string
const AdminIDKey contextKey = "adminID"

// RequireAdminAuth is a middleware that checks for a valid admin JWT Bearer token
func RequireAdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Missing Authorization header"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid Authorization header format"})
			return
		}

		tokenString := parts[1]
		
		// Parse the token using the admin secret
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return getAdminJWTSecret(), nil
		})

		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid or expired admin token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid token payload"})
			return
		}
		
		// Ensure it's actually an admin token
		if isAdmin, ok := claims["is_admin"].(bool); !ok || !isAdmin {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Standard user tokens cannot access admin routes"})
			return
		}

		// JSON unmarshaling puts numbers into float64
		adminIDFloat, ok := claims["admin_id"].(float64)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid token admin ID"})
			return
		}

		// Add adminID to context
		ctx := context.WithValue(r.Context(), AdminIDKey, uint(adminIDFloat))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetAdminMeHandler returns the currently authenticated admin's details
func GetAdminMeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	adminID := r.Context().Value(AdminIDKey).(uint)

	var admin models.AdminUser
	if err := database.DB.Select("id, email, role, last_login, created_at").Where("id = ?", adminID).First(&admin).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Admin user not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"message": "Admin retrieved successfully",
		"body": map[string]interface{}{
			"admin": admin,
		},
	})
}

