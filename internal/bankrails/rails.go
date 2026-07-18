package bankrails

import (
	"strings"

	"flowwithlit/internal/providers"
	"flowwithlit/internal/settings"
)

// Result is a virtual-account deposit instruction.
type Result struct {
	Currency      string
	BankName      string
	AccountNumber string
	Provider      string
	Configured    bool
}

// Resolve picks the NGN rail (OnePipe default, PalmPay when switched) or Flutterwave for other fiat.
// PalmPay is only used when Admin sets ngn_bank_provider=palmpay — otherwise OnePipe stays live.
func Resolve(currency, firstName, lastName, email, phone string) (Result, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "NGN"
	}

	preferredNGN := settings.NGNBankProvider()
	rail := providers.ForBankTransfer(cur, preferredNGN)

	switch rail {
	case providers.PalmPay:
		client := settings.PalmPayClient()
		acct, bank, err := client.GenerateVirtualAccount(firstName, lastName, email, phone)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Currency:      "NGN",
			BankName:      bank,
			AccountNumber: acct,
			Provider:      providers.PalmPay,
			Configured:    client.Configured(),
		}, nil

	case providers.OnePipe:
		client := settings.OnePipeClient()
		acct, bank, err := client.GenerateVirtualAccount(firstName, lastName, email, phone)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Currency:      "NGN",
			BankName:      bank,
			AccountNumber: acct,
			Provider:      providers.OnePipe,
			Configured:    client.Configured(),
		}, nil
	}

	fw := settings.FlutterwaveClient()
	name := strings.TrimSpace(firstName + " " + lastName)
	acct, bank, err := fw.GenerateVirtualAccount(cur, email, name)
	if err != nil {
		return Result{}, err
	}
	return Result{
		Currency:      cur,
		BankName:      bank,
		AccountNumber: acct,
		Provider:      providers.Flutterwave,
		Configured:    fw.Configured(),
	}, nil
}
