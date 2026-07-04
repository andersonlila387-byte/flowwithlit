package rates

import (
	"strings"
	"sync"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

var (
	cacheMu    sync.RWMutex
	cacheData  map[string]float64
	cacheUntil time.Time
	cacheTTL   = 60 * time.Second
)

// GetAllRates returns a flat map of FROM_TO keys for checkout / swap consumers.
func GetAllRates() map[string]float64 {
	if cached := getCache(); cached != nil {
		return cached
	}

	out := make(map[string]float64)
	var rows []models.ExchangeRate
	database.DB.Find(&rows)
	for _, r := range rows {
		key := r.FromCurrency + "_" + r.ToCurrency
		adjusted := applySpread(r.Rate, r.SpreadPercent)
		out[key] = adjusted
	}

	setCache(out)
	return out
}

// GetRate returns the rate from → to, trying direct pair then inverse.
func GetRate(from, to string) float64 {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if from == to {
		return 1.0
	}

	all := GetAllRates()
	if v, ok := all[from+"_"+to]; ok && v > 0 {
		return v
	}
	if v, ok := all[to+"_"+from]; ok && v > 0 {
		return 1.0 / v
	}
	return 0
}

// Convert converts amount using stored rates (via NGN bridge if needed).
func Convert(amount float64, from, to string) float64 {
	from = strings.ToUpper(from)
	to = strings.ToUpper(to)
	if from == to {
		return amount
	}
	if r := GetRate(from, to); r > 0 {
		return amount * r
	}
	// Bridge through NGN
	if from != "NGN" && to != "NGN" {
		ngn := Convert(amount, from, "NGN")
		if r := GetRate("NGN", to); r > 0 {
			return ngn * r
		}
	}
	return 0
}

// GetEnabledCurrencies for public checkout dropdown.
func GetEnabledCurrencies() []models.Currency {
	var list []models.Currency
	database.DB.Where("is_enabled = ?", true).Order("sort_order asc").Find(&list)
	return list
}

// GetEnabledCryptoAssets for checkout crypto picker.
func GetEnabledCryptoAssets() []models.CryptoAsset {
	var list []models.CryptoAsset
	database.DB.Where("is_enabled = ?", true).Order("sort_order asc").Find(&list)
	return list
}

// CryptoUSDPrice returns USD price per 1 unit of crypto asset.
func CryptoUSDPrice(code string) float64 {
	code = strings.ToUpper(code)
	if code == "USDT" || code == "USDC" {
		return 1.0
	}
	return GetRate(code, "USD")
}

func InvalidateCache() {
	cacheMu.Lock()
	cacheUntil = time.Time{}
	cacheMu.Unlock()
}

func applySpread(rate, spreadPercent float64) float64 {
	if spreadPercent <= 0 {
		return rate
	}
	return rate * (1 - spreadPercent/100)
}

func getCache() map[string]float64 {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if time.Now().Before(cacheUntil) && cacheData != nil {
		copy := make(map[string]float64, len(cacheData))
		for k, v := range cacheData {
			copy[k] = v
		}
		return copy
	}
	return nil
}

func setCache(data map[string]float64) {
	cacheMu.Lock()
	cacheData = data
	cacheUntil = time.Now().Add(cacheTTL)
	cacheMu.Unlock()
}