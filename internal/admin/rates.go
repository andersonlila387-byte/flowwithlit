package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
	"flowwithlit/pkg/response"
)

type rateUpdateItem struct {
	FromCurrency  string  `json:"from_currency"`
	ToCurrency    string  `json:"to_currency"`
	Rate          float64 `json:"rate"`
	SpreadPercent float64 `json:"spread_percent"`
}

// GetCurrenciesHandler — GET /admin/currencies
func GetCurrenciesHandler(w http.ResponseWriter, r *http.Request) {
	var fiat []models.Currency
	database.DB.Order("sort_order asc").Find(&fiat)

	var crypto []models.CryptoAsset
	database.DB.Order("sort_order asc").Find(&crypto)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"fiat":   fiat,
		"crypto": crypto,
	})
}

// GetExchangeRatesHandler — GET /admin/exchange-rates
func GetExchangeRatesHandler(w http.ResponseWriter, r *http.Request) {
	var rows []models.ExchangeRate
	database.DB.Order("from_currency asc, to_currency asc").Find(&rows)

	var logs []models.RateChangeLog
	database.DB.Order("created_at desc").Limit(50).Find(&logs)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"rates":      rows,
		"audit_logs": logs,
		"flat":       rates.GetAllRates(),
	})
}

// UpdateExchangeRatesHandler — PUT /admin/exchange-rates
func UpdateExchangeRatesHandler(w http.ResponseWriter, r *http.Request) {
	adminID, _ := r.Context().Value(AdminIDKey).(uint)

	var req struct {
		Rates  []rateUpdateItem `json:"rates"`
		Reason string           `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.Rates) == 0 {
		response.Error(w, http.StatusBadRequest, "No rates provided")
		return
	}

	for _, item := range req.Rates {
		from := strings.ToUpper(strings.TrimSpace(item.FromCurrency))
		to := strings.ToUpper(strings.TrimSpace(item.ToCurrency))
		if from == "" || to == "" || item.Rate <= 0 {
			continue
		}

		var existing models.ExchangeRate
		err := database.DB.Where("from_currency = ? AND to_currency = ?", from, to).First(&existing).Error
		oldRate := 0.0
		if err == nil {
			oldRate = existing.Rate
			existing.Rate = item.Rate
			existing.Source = "manual"
			existing.SpreadPercent = item.SpreadPercent
			existing.UpdatedBy = &adminID
			database.DB.Save(&existing)
		} else {
			existing = models.ExchangeRate{
				FromCurrency:  from,
				ToCurrency:    to,
				Rate:          item.Rate,
				Source:        "manual",
				SpreadPercent: item.SpreadPercent,
				UpdatedBy:     &adminID,
			}
			database.DB.Create(&existing)
		}

		database.DB.Create(&models.RateChangeLog{
			FromCurrency: from,
			ToCurrency:   to,
			OldRate:      oldRate,
			NewRate:      item.Rate,
			ChangedBy:    adminID,
			Reason:       req.Reason,
		})
	}

	rates.InvalidateCache()
	response.Success(w, http.StatusOK, map[string]string{"message": "Exchange rates updated"})
}

// ToggleCurrencyHandler — POST /admin/currencies/toggle
func ToggleCurrencyHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code    string `json:"code"`
		Enabled bool   `json:"enabled"`
		Type    string `json:"type"` // fiat | crypto
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	code := strings.ToUpper(strings.TrimSpace(req.Code))
	if code == "" {
		response.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	if req.Type == "crypto" {
		database.DB.Model(&models.CryptoAsset{}).Where("code = ?", code).Update("is_enabled", req.Enabled)
	} else {
		database.DB.Model(&models.Currency{}).Where("code = ?", code).Update("is_enabled", req.Enabled)
	}

	rates.InvalidateCache()
	response.Success(w, http.StatusOK, map[string]string{"message": "Updated"})
}

// GetRatePreviewHandler — GET /admin/exchange-rates/preview?amount=500000&from=NGN&to=USD
func GetRatePreviewHandler(w http.ResponseWriter, r *http.Request) {
	amount, _ := strconv.ParseFloat(r.URL.Query().Get("amount"), 64)
	from := strings.ToUpper(r.URL.Query().Get("from"))
	to := strings.ToUpper(r.URL.Query().Get("to"))
	if amount <= 0 || from == "" || to == "" {
		response.Error(w, http.StatusBadRequest, "amount, from, and to are required")
		return
	}
	converted := rates.Convert(amount, from, to)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"from":      from,
		"to":        to,
		"amount":    amount,
		"converted": converted,
		"rate":      rates.GetRate(from, to),
	})
}