package jwt

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// secretKey is lazy-loaded on first use via getSecretKey().
// This ensures godotenv.Load() in main() runs first before we read JWT_SECRET.
// A package-level var initializer would run BEFORE main() — meaning the .env
// file would not yet be loaded and os.Getenv("JWT_SECRET") would always be empty.
var (
	secretKeyOnce sync.Once
	secretKeyVal  []byte
)

// getSecretKey returns the JWT signing secret, loading it from env on first call.
func getSecretKey() []byte {
	secretKeyOnce.Do(func() {
		s := strings.TrimSpace(os.Getenv("JWT_SECRET"))
		if s == "" {
			isProd := strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production")
			if isProd {
				panic("FATAL: JWT_SECRET environment variable is not set. " +
					"Set it to a long random string before starting the server in production.")
			}
			fmt.Println("WARNING: JWT_SECRET not set — using insecure dev fallback. NEVER run this in production.")
			secretKeyVal = []byte("super-secret-flowwithlit-dev-only")
			return
		}
		secretKeyVal = []byte(s)
	})
	return secretKeyVal
}

// GenerateTokens creates both an access token (24h) and a refresh token (7 days).
func GenerateTokens(userID uint, email string) (string, string, error) {
	// Access Token (24 hours)
	accessClaims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"purpose": "access",
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
		"iat":     time.Now().Unix(),
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	signedAccessToken, err := accessToken.SignedString(getSecretKey())
	if err != nil {
		return "", "", err
	}

	// Refresh Token (7 days)
	refreshClaims := jwt.MapClaims{
		"user_id": userID,
		"purpose": "refresh",
		"exp":     time.Now().Add(time.Hour * 24 * 7).Unix(),
		"iat":     time.Now().Unix(),
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	signedRefreshToken, err := refreshToken.SignedString(getSecretKey())
	if err != nil {
		return "", "", err
	}

	return signedAccessToken, signedRefreshToken, nil
}

// GenerateTempToken issues a short-lived (5 min) token with a restricted purpose
// claim. Used for intermediate auth steps (e.g. 2FA pending) so that a stolen
// token cannot be used as a real access token on protected endpoints.
func GenerateTempToken(userID uint, purpose string) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"purpose": purpose,
		"exp":     time.Now().Add(5 * time.Minute).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(getSecretKey())
}

// ValidateToken parses and verifies a JWT string, returning the claims if valid.
func ValidateToken(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Reject any token not signed with HMAC — prevents alg=none attacks.
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return getSecretKey(), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// ValidatePurposeToken validates a token and also checks that its purpose claim
// matches the expected value. Use this for temp/restricted tokens (e.g. 2fa_pending).
func ValidatePurposeToken(tokenString, expectedPurpose string) (jwt.MapClaims, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}
	purpose, _ := claims["purpose"].(string)
	if purpose != expectedPurpose {
		return nil, fmt.Errorf("token purpose mismatch: got %q, want %q", purpose, expectedPurpose)
	}
	return claims, nil
}

