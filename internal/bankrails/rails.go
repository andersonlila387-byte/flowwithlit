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

// Resolve picks OnePipe for NGN and Flutterwave for all other fiat currencies.
func Resolve(currency, firstName, lastName, email, phone string) (Result, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "NGN"
	}

	if providers.ForBankTransfer(cur) == providers.OnePipe {
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