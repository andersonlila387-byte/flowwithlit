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
	ensureOnce sync.Once
)

// GetAllRates returns a flat map of FROM_TO keys for checkout / swap consumers.
func GetAllRates() map[string]float64 {
	EnsureCoreSwapPairs()
	if cached := getCache(); cached != nil {
		return cached
	}

	out := make(map[string]float64)
	var rows []models.ExchangeRate
	database.DB.Find(&rows)
	for _, r := range rows {
		from := strings.ToUpper(strings.TrimSpace(r.FromCurrency))
		to := strings.ToUpper(strings.TrimSpace(r.ToCurrency))
		key := from + "_" + to
		adjusted := applySpread(r.Rate, r.SpreadPercent)
		out[key] = adjusted
	}

	setCache(out)
	return out
}

// GetRate returns how many units of `to` equal 1 unit of `from` (multiply amount by rate).
// Supports: direct pair, inverse, USDT/USDC pegged to USD, and NGN bridge.
func GetRate(from, to string) float64 {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if from == "" || to == "" {
		return 0
	}
	if from == to {
		return 1.0
	}

	// Stablecoins pegged 1:1 to USD for rate purposes
	if isUSDStable(from) && isUSDStable(to) {
		return 1.0
	}

	all := GetAllRates()
	if v := lookupPair(all, from, to); v > 0 {
		return v
	}

	// USDT/USDC ↔ anything via USD
	if isUSDStable(from) {
		if v := lookupPair(all, "USD", to); v > 0 {
			return v
		}
	}
	if isUSDStable(to) {
		if v := lookupPair(all, from, "USD"); v > 0 {
			return v
		}
	}

	// Bridge via NGN when both sides have a path to NGN
	if from != "NGN" && to != "NGN" {
		toNGN := lookupPair(all, from, "NGN")
		if toNGN <= 0 && isUSDStable(from) {
			toNGN = lookupPair(all, "USD", "NGN")
		}
		fromNGN := lookupPair(all, "NGN", to)
		if fromNGN <= 0 && isUSDStable(to) {
			fromNGN = lookupPair(all, "NGN", "USD")
		}
		if toNGN > 0 && fromNGN > 0 {
			return toNGN * fromNGN
		}
	}

	return 0
}

func isUSDStable(code string) bool {
	return code == "USDT" || code == "USDC"
}

// lookupPair tries direct then inverse.
func lookupPair(all map[string]float64, from, to string) float64 {
	if v, ok := all[from+"_"+to]; ok && v > 0 {
		return v
	}
	if v, ok := all[to+"_"+from]; ok && v > 0 {
		return 1.0 / v
	}
	return 0
}

// Convert converts amount using stored rates (via NGN / USD peg as needed).
func Convert(amount float64, from, to string) float64 {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))
	if from == to {
		return amount
	}
	if r := GetRate(from, to); r > 0 {
		return amount * r
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

// EnsureCoreSwapPairs inserts missing USDT/USD/NGN pairs so swap never falls back to 1:1.
// Safe to call often — only creates rows that do not exist.
func EnsureCoreSwapPairs() {
	ensureOnce.Do(func() {
		if database.DB == nil {
			return
		}
		// Prefer live USD_NGN if admin already set it
		usdNgn := 1610.0
		var existing models.ExchangeRate
		if err := database.DB.Where("from_currency = ? AND to_currency = ?", "USD", "NGN").First(&existing).Error; err == nil && existing.Rate > 0 {
			usdNgn = existing.Rate
		} else if err := database.DB.Where("from_currency = ? AND to_currency = ?", "NGN", "USD").First(&existing).Error; err == nil && existing.Rate > 0 {
			usdNgn = 1.0 / existing.Rate
		}

		ngnUsd := 1.0 / usdNgn
		// Default platform pairs (admin can override later)
		defaults := []struct {
			from, to string
			rate     float64
		}{
			{"USD", "NGN", usdNgn},
			{"NGN", "USD", ngnUsd},
			{"USDT", "NGN", usdNgn},
			{"NGN", "USDT", ngnUsd},
			{"USDC", "NGN", usdNgn},
			{"NGN", "USDC", ngnUsd},
			{"USDT", "USD", 1.0},
			{"USD", "USDT", 1.0},
			{"USDC", "USD", 1.0},
			{"USD", "USDC", 1.0},
			{"USDT", "USDC", 1.0},
			{"USDC", "USDT", 1.0},
		}

		for _, d := range defaults {
			var row models.ExchangeRate
			err := database.DB.Where("from_currency = ? AND to_currency = ?", d.from, d.to).First(&row).Error
			if err == nil {
				continue // keep admin-managed rates
			}
			_ = database.DB.Create(&models.ExchangeRate{
				FromCurrency: d.from,
				ToCurrency:   d.to,
				Rate:         d.rate,
				Source:       "system",
			}).Error
		}
		InvalidateCache()
	})
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
