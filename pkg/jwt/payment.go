package jwt

import (
	"errors"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// GeneratePaymentToken issues a short-lived token that can replace PIN for one debit window.
func GeneratePaymentToken(userID uint, deviceID string) (string, int, error) {
	ttl := 90 * time.Second
	claims := jwtlib.MapClaims{
		"user_id":   userID,
		"device_id": deviceID,
		"purpose":   "payment",
		"exp":       time.Now().Add(ttl).Unix(),
		"iat":       time.Now().Unix(),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signed, err := token.SignedString(secretKey)
	if err != nil {
		return "", 0, err
	}
	return signed, int(ttl.Seconds()), nil
}

// ValidatePaymentToken checks a payment authorization token for this user.
func ValidatePaymentToken(tokenString string, userID uint) (deviceID string, err error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", err
	}
	purpose, _ := claims["purpose"].(string)
	if purpose != "payment" {
		return "", errors.New("not a payment token")
	}
	uid, ok := claims["user_id"].(float64)
	if !ok || uint(uid) != userID {
		return "", errors.New("payment token user mismatch")
	}
	deviceID, _ = claims["device_id"].(string)
	return deviceID, nil
}
