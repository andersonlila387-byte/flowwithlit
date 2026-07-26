package bankrails

import (
	"fmt"
	"strings"

	"flowwithlit/internal/integration/onepipe"
	"flowwithlit/internal/providers"
	"flowwithlit/internal/settings"
)

// Result is a virtual-account deposit instruction.
// Provider is stored internally for settlement routing — never show rail brands to end users.
type Result struct {
	Currency      string
	BankName      string
	AccountNumber string
	Provider      string
	Configured    bool
}

// Customer extras for richer open_account calls (optional).
type CustomerOpts struct {
	UserID  string
	Address string
	DOB     string
	Gender  string
}

// Resolve picks the NGN rail (OnePipe default, PalmPay when switched) or Flutterwave for other fiat.
// All vendor HTTP happens server-side only.
func Resolve(currency, firstName, lastName, email, phone string) (Result, error) {
	return ResolveWithOpts(currency, firstName, lastName, email, phone, CustomerOpts{})
}

// ResolveWithOpts is Resolve plus optional identity fields for open_account.
func ResolveWithOpts(currency, firstName, lastName, email, phone string, opts CustomerOpts) (Result, error) {
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
			return Result{}, publicRailError(err)
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
		va, err := client.OpenVirtualAccount(onepipe.Customer{
			Ref:       opts.UserID,
			FirstName: firstName,
			LastName:  lastName,
			Email:     email,
			Phone:     phone,
			Address:   opts.Address,
			DOB:       opts.DOB,
			Gender:    opts.Gender,
		})
		if err != nil {
			return Result{}, publicRailError(err)
		}
		bank := va.BankName
		if bank == "" {
			bank = "Nigerian Bank"
		}
		return Result{
			Currency:      "NGN",
			BankName:      bank,
			AccountNumber: va.AccountNumber,
			Provider:      providers.OnePipe,
			Configured:    client.Configured(),
		}, nil
	}

	fw := settings.FlutterwaveClient()
	name := strings.TrimSpace(firstName + " " + lastName)
	acct, bank, err := fw.GenerateVirtualAccount(cur, email, name)
	if err != nil {
		return Result{}, publicRailError(err)
	}
	return Result{
		Currency:      cur,
		BankName:      bank,
		AccountNumber: acct,
		Provider:      providers.Flutterwave,
		Configured:    fw.Configured(),
	}, nil
}

// publicRailError strips vendor names from errors that may reach JSON APIs used by browsers.
func publicRailError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	for _, v := range []string{"onepipe", "flutterwave", "palmpay", "circle"} {
		if strings.Contains(lower, v) {
			return fmt.Errorf("could not generate deposit account right now — please try again later or contact support")
		}
	}
	return err
}
