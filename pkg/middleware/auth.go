package middleware

import (
	"context"
	"net/http"
	"strings"

	"flowwithlit/pkg/jwt"
	"flowwithlit/pkg/response"
)

type contextKey string

const UserIDKey contextKey = "userID"

// RequireAuth is a middleware that checks for a valid JWT Bearer token
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			response.Error(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Error(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}

		tokenString := parts[1]
		claims, err := jwt.ValidateToken(tokenString)
		if err != nil {
			response.Error(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// JSON unmarshaling puts numbers into float64
		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			response.Error(w, http.StatusUnauthorized, "Invalid token payload")
			return
		}

		// Add userID to context
		ctx := context.WithValue(r.Context(), UserIDKey, uint(userIDFloat))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
