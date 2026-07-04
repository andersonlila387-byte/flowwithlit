package settlement

import (
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

// countryCurrencyFallback mirrors seed data for countries not yet in DB.
var countryCurrencyFallback = map[string]string{
	"NG": "NGN",
	"US": "USD",
	"GB": "GBP",
	"DE": "EUR",
	"GH": "GHS",
	"KE": "KES",
	"ZA": "ZAR",
	"UG": "UGX",
	"TZ": "TZS",
	"RW": "RWF",
	"CM": "XAF",
	"SN": "XOF",
	"CA": "CAD",
	"AU": "AUD",
	"EG": "EGP",
}

// CurrencyForCountry returns the default fiat currency for an ISO country code.
func CurrencyForCountry(countryCode string) string {
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	if code == "" {
		return DefaultFiat
	}

	var country models.Country
	if err := database.DB.Where("code = ? AND is_active = ?", code, true).First(&country).Error; err == nil {
		if c := strings.ToUpper(strings.TrimSpace(country.DefaultCurrencyCode)); c != "" {
			return c
		}
	}

	if c, ok := countryCurrencyFallback[code]; ok {
		return c
	}
	return "USD"
}