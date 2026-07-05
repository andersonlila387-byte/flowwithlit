package jwt

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// secretKey signs all user auth tokens. Must be set via JWT_SECRET in production —
// the fallback below is dev-only and must never be used once deployed, since anyone
// with the source code could otherwise forge valid tokens for any user.
var secretKey = func() []byte {
	if s := os.Getenv("JWT_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte("super-secret-flowwithlit-key")
}()

// GenerateTokens creates both an access token (24h) and a refresh token (7 days)
func GenerateTokens(userID uint, email string) (string, string, error) {
	// Access Token (24 hours)
	accessClaims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	signedAccessToken, err := accessToken.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}

	// Refresh Token (7 days)
	refreshClaims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshToken.SignedString(secretKey)
	if err != nil {
		return "", "", err
	}

	return signedAccessToken, signedRefreshToken, nil
}

// ValidateToken parses and verifies a JWT string, returning the claims if valid
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	// Extract the claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
