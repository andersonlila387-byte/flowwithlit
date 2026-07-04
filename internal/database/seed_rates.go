package database

import (
	"log"

	"flowwithlit/internal/models"
)

// SeedReferenceData populates countries, currencies, crypto assets, and default rates once.
func SeedReferenceData() {
	seedCountries()
	seedCurrencies()
	seedCryptoAssets()
	seedExchangeRates()
}

func seedCountries() {
	var count int64
	DB.Model(&models.Country{}).Count(&count)
	if count > 0 {
		return
	}

	countries := []models.Country{
		{Code: "NG", Name: "Nigeria", DefaultCurrencyCode: "NGN", IsActive: true},
		{Code: "US", Name: "United States", DefaultCurrencyCode: "USD", IsActive: true},
		{Code: "GB", Name: "United Kingdom", DefaultCurrencyCode: "GBP", IsActive: true},
		{Code: "DE", Name: "Germany", DefaultCurrencyCode: "EUR", IsActive: true},
		{Code: "GH", Name: "Ghana", DefaultCurrencyCode: "GHS", IsActive: true},
		{Code: "KE", Name: "Kenya", DefaultCurrencyCode: "KES", IsActive: true},
		{Code: "ZA", Name: "South Africa", DefaultCurrencyCode: "ZAR", IsActive: true},
		{Code: "UG", Name: "Uganda", DefaultCurrencyCode: "UGX", IsActive: true},
		{Code: "TZ", Name: "Tanzania", DefaultCurrencyCode: "TZS", IsActive: true},
		{Code: "RW", Name: "Rwanda", DefaultCurrencyCode: "RWF", IsActive: true},
		{Code: "CM", Name: "Cameroon", DefaultCurrencyCode: "XAF", IsActive: true},
		{Code: "SN", Name: "Senegal", DefaultCurrencyCode: "XOF", IsActive: true},
		{Code: "CA", Name: "Canada", DefaultCurrencyCode: "CAD", IsActive: true},
		{Code: "AU", Name: "Australia", DefaultCurrencyCode: "AUD", IsActive: true},
	}
	for _, c := range countries {
		DB.Create(&c)
	}
	log.Println("✅ Seeded countries")
}

func seedCurrencies() {
	var count int64
	DB.Model(&models.Currency{}).Count(&count)
	if count > 0 {
		return
	}

	type cur struct {
		code, name, symbol, minor string
		decimals, sort            int
		fw, bank, card, enabled   bool
		isBase                    bool
	}

	list := []cur{
		{"NGN", "Nigerian Naira", "₦", "kobo", 2, 1, true, true, true, true, true},
		{"USD", "US Dollar", "$", "cents", 2, 2, true, true, true, true, false},
		{"GBP", "British Pound", "£", "pence", 2, 3, true, false, true, true, false},
		{"EUR", "Euro", "€", "cents", 2, 4, true, false, true, true, false},
		{"GHS", "Ghanaian Cedi", "GH₵", "pesewas", 2, 5, true, true, true, true, false},
		{"KES", "Kenyan Shilling", "KSh", "cents", 2, 6, true, true, true, true, false},
		{"ZAR", "South African Rand", "R", "cents", 2, 7, true, true, true, true, false},
		{"UGX", "Ugandan Shilling", "USh", "shillings", 0, 8, true, true, true, true, false},
		{"TZS", "Tanzanian Shilling", "TSh", "shillings", 2, 9, true, true, true, true, false},
		{"RWF", "Rwandan Franc", "FRw", "francs", 0, 10, true, true, true, true, false},
		{"XAF", "Central African CFA", "FCFA", "francs", 0, 11, true, true, true, true, false},
		{"XOF", "West African CFA", "CFA", "francs", 0, 12, true, true, true, true, false},
		{"CAD", "Canadian Dollar", "C$", "cents", 2, 13, true, false, true, true, false},
		{"AUD", "Australian Dollar", "A$", "cents", 2, 14, true, false, true, true, false},
	}

	for _, c := range list {
		DB.Create(&models.Currency{
			Code: c.code, Name: c.name, Symbol: c.symbol, Decimals: c.decimals,
			MinorUnit: c.minor, FlutterwaveSupported: c.fw, BankTransferSupported: c.bank,
			CardSupported: c.card, IsEnabled: c.enabled, IsBase: c.isBase, SortOrder: c.sort,
		})
	}
	log.Println("✅ Seeded currencies")
}

func seedCryptoAssets() {
	var count int64
	DB.Model(&models.CryptoAsset{}).Count(&count)
	if count > 0 {
		return
	}

	assets := []models.CryptoAsset{
		{Code: "USDT", Name: "Tether", Network: "TRON", NetworkTag: "TRC20", Decimals: 2, IconKey: "cryptocurrency-color:usdt", Color: "#26A17B", IsEnabled: true, SortOrder: 1},
		{Code: "BTC", Name: "Bitcoin", Network: "Bitcoin", NetworkTag: "BTC", Decimals: 8, IconKey: "cryptocurrency-color:btc", Color: "#F7931A", IsEnabled: true, SortOrder: 2},
		{Code: "ETH", Name: "Ethereum", Network: "Ethereum", NetworkTag: "ERC20", Decimals: 6, IconKey: "cryptocurrency-color:eth", Color: "#627EEA", IsEnabled: true, SortOrder: 3},
		{Code: "USDC", Name: "USD Coin", Network: "Ethereum", NetworkTag: "ERC20", Decimals: 2, IconKey: "cryptocurrency-color:usdc", Color: "#2775CA", IsEnabled: true, SortOrder: 4},
		{Code: "SOL", Name: "Solana", Network: "Solana", NetworkTag: "SPL", Decimals: 4, IconKey: "cryptocurrency-color:sol", Color: "#9945FF", IsEnabled: true, SortOrder: 5},
	}
	for _, a := range assets {
		DB.Create(&a)
	}
	log.Println("✅ Seeded crypto assets")
}

func seedExchangeRates() {
	var count int64
	DB.Model(&models.ExchangeRate{}).Count(&count)
	if count > 0 {
		return
	}

	// NGN is platform base — store major pairs (to NGN and inverses).
	pairs := map[string]float64{
		"USD_NGN": 1610.0,
		"GBP_NGN": 2040.0,
		"EUR_NGN": 1750.0,
		"GHS_NGN": 130.0,
		"KES_NGN": 12.5,
		"ZAR_NGN": 88.0,
		"CAD_NGN": 1190.0,
		"AUD_NGN": 1064.0,
		"NGN_USD": 0.00062,
		"NGN_GBP": 0.00049,
		"NGN_EUR": 0.00057,
		"NGN_GHS": 0.0077,
		"NGN_KES": 0.08,
		"NGN_ZAR": 0.0114,
		"NGN_CAD": 0.00084,
		"NGN_AUD": 0.00094,
		"BTC_USD": 97000.0,
		"ETH_USD": 3500.0,
		"SOL_USD": 180.0,
		"USDT_USD": 1.0,
		"USDC_USD": 1.0,
	}

	for pair, rate := range pairs {
		from, to := splitPair(pair)
		DB.Create(&models.ExchangeRate{
			FromCurrency: from,
			ToCurrency:   to,
			Rate:         rate,
			Source:       "manual",
		})
	}
	log.Println("✅ Seeded exchange rates")
}

func splitPair(pair string) (string, string) {
	for i := 1; i < len(pair)-1; i++ {
		if pair[i] == '_' {
			return pair[:i], pair[i+1:]
		}
	}
	return pair, ""
}