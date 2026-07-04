package user

import (
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
	"flowwithlit/internal/settlement"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

type UpdateSettlementPreferencesRequest struct {
	DefaultFiatCurrency   *string `json:"default_fiat_currency"`
	DefaultCryptoCurrency *string `json:"default_crypto_currency"`
}

// GetSettlementOptionsHandler lists enabled fiat + crypto choices for settings UI.
func GetSettlementOptionsHandler(w http.ResponseWriter, r *http.Request) {
	response.Success(w, http.StatusOK, map[string]interface{}{
		"fiat_currencies":  rates.GetEnabledCurrencies(),
		"crypto_assets":    rates.GetEnabledCryptoAssets(),
		"defaults": map[string]string{
			"fiat":   settlement.DefaultFiat,
			"crypto": settlement.DefaultCrypto,
		},
	})
}

// UpdateSettlementPreferencesHandler sets where incoming payments are auto-converted.
func UpdateSettlementPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateSettlementPreferencesRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updates := map[string]interface{}{}
	if req.DefaultFiatCurrency != nil {
		code := strings.ToUpper(strings.TrimSpace(*req.DefaultFiatCurrency))
		if !isEnabledFiat(code) {
			response.Error(w, http.StatusBadRequest, "Invalid or disabled fiat currency")
			return
		}
		updates["default_fiat_currency"] = code
	}
	if req.DefaultCryptoCurrency != nil {
		code := strings.ToUpper(strings.TrimSpace(*req.DefaultCryptoCurrency))
		if !isEnabledCrypto(code) {
			response.Error(w, http.StatusBadRequest, "Invalid or disabled crypto asset")
			return
		}
		updates["default_crypto_currency"] = code
	}

	if len(updates) == 0 {
		response.Error(w, http.StatusBadRequest, "No preferences to update")
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update preferences")
		return
	}

	var user models.User
	database.DB.First(&user, userID)
	user.Password = ""
	user.TransactionPin = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Settlement preferences updated",
		"user":    user,
	})
}

func isEnabledFiat(code string) bool {
	var count int64
	database.DB.Model(&models.Currency{}).Where("code = ? AND is_enabled = ?", code, true).Count(&count)
	return count > 0
}

func isEnabledCrypto(code string) bool {
	var count int64
	database.DB.Model(&models.CryptoAsset{}).Where("code = ? AND is_enabled = ?", code, true).Count(&count)
	return count > 0
}