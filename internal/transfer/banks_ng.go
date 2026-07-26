package transfer

import "flowwithlit/internal/integration/flutterwave"

// NigerianBanksStatic is a NIBSS-style code list used when a live bank-list API
// is unavailable. Codes match common Flutterwave / NIBSS values for name enquiry.
func NigerianBanksStatic() []flutterwave.BankItem {
	return []flutterwave.BankItem{
		{Code: "044", Name: "Access Bank"},
		{Code: "063", Name: "Access Bank (Diamond)"},
		{Code: "023", Name: "Citibank Nigeria"},
		{Code: "050", Name: "Ecobank Nigeria"},
		{Code: "070", Name: "Fidelity Bank"},
		{Code: "011", Name: "First Bank of Nigeria"},
		{Code: "214", Name: "First City Monument Bank"},
		{Code: "058", Name: "Guaranty Trust Bank"},
		{Code: "030", Name: "Heritage Bank"},
		{Code: "301", Name: "Jaiz Bank"},
		{Code: "082", Name: "Keystone Bank"},
		{Code: "526", Name: "Parallex Bank"},
		{Code: "076", Name: "Polaris Bank"},
		{Code: "101", Name: "Providus Bank"},
		{Code: "221", Name: "Stanbic IBTC Bank"},
		{Code: "068", Name: "Standard Chartered Bank"},
		{Code: "232", Name: "Sterling Bank"},
		{Code: "100", Name: "Suntrust Bank"},
		{Code: "032", Name: "Union Bank of Nigeria"},
		{Code: "033", Name: "United Bank for Africa"},
		{Code: "215", Name: "Unity Bank"},
		{Code: "035", Name: "Wema Bank"},
		{Code: "057", Name: "Zenith Bank"},
		{Code: "50515", Name: "Moniepoint MFB"},
		{Code: "999991", Name: "OPay"},
		{Code: "999992", Name: "PalmPay"},
		{Code: "120001", Name: "Kuda Bank"},
		{Code: "090267", Name: "Kuda Microfinance Bank"},
		{Code: "090405", Name: "Moniepoint Microfinance Bank"},
		{Code: "090110", Name: "VFD Microfinance Bank"},
		{Code: "090328", Name: "Eyowo"},
		{Code: "090286", Name: "Safe Haven MFB"},
		{Code: "090325", Name: "Sparkle Microfinance Bank"},
		{Code: "090175", Name: "Rubies MFB"},
		{Code: "090270", Name: "AB Microfinance Bank"},
		{Code: "090260", Name: "Carbon"},
		{Code: "090267", Name: "Kuda"},
		{Code: "50211", Name: "Kuda Bank"},
		{Code: "090578", Name: "FairMoney MFB"},
		{Code: "090551", Name: "Fairmoney Microfinance Bank"},
		{Code: "000", Name: "Other / Enter bank code manually"},
	}
}
