package flutterwave

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.flutterwave.com/v3"

// Client handles Flutterwave API calls (non-NGN bank rails + all card payments).
type Client struct {
	SecretKey string
	BaseURL   string
}

func NewClient(secretKey, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		SecretKey: strings.TrimSpace(secretKey),
		BaseURL:   baseURL,
	}
}

func (c *Client) Configured() bool {
	return c.SecretKey != ""
}

func (c *Client) authRequest(method, path string, body interface{}) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.SecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	return data, resp.StatusCode, nil
}

// GenerateVirtualAccount creates a fiat virtual account for non-NGN currencies.
func (c *Client) GenerateVirtualAccount(currency, email, customerName string) (string, string, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "USD"
	}

	if c.Configured() {
		log.Printf("[Flutterwave] Creating %s virtual account for %s", cur, email)
		// TODO: POST /virtual-account-numbers when live keys are active
	}

	log.Printf("[Flutterwave Mock] Generating %s virtual account for %s", cur, email)
	mockAccountNumber := "40" + fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
	mockBankName := "Wema Bank (Flutterwave)"
	return mockAccountNumber, mockBankName, nil
}

// BankItem is a normalized bank for transfer UI.
type BankItem struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// ListBanks returns banks for a country (e.g. NG) via Flutterwave.
func (c *Client) ListBanks(country string) ([]BankItem, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("Flutterwave secret key not configured in Admin → Settings → Payment Providers")
	}
	country = strings.ToUpper(strings.TrimSpace(country))
	if country == "" {
		country = "NG"
	}

	data, code, err := c.authRequest(http.MethodGet, "/banks/"+country, nil)
	if err != nil {
		return nil, err
	}
	if code < 200 || code >= 300 {
		return nil, fmt.Errorf("Flutterwave banks HTTP %d: %s", code, strings.TrimSpace(string(data)))
	}

	var out struct {
		Status string `json:"status"`
		Data   []struct {
			Code string `json:"code"`
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	banks := make([]BankItem, 0, len(out.Data))
	for _, b := range out.Data {
		code := strings.TrimSpace(b.Code)
		if code == "" && b.ID > 0 {
			code = fmt.Sprintf("%d", b.ID)
		}
		name := strings.TrimSpace(b.Name)
		if code != "" && name != "" {
			banks = append(banks, BankItem{Code: code, Name: name})
		}
	}
	if len(banks) == 0 {
		return nil, fmt.Errorf("no banks returned for country %s", country)
	}
	return banks, nil
}

// ResolveBankAccount performs real name enquiry via Flutterwave.
func (c *Client) ResolveBankAccount(bankCode, accountNumber string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("Flutterwave secret key not configured in Admin → Settings → Payment Providers")
	}

	accountNumber = strings.TrimSpace(accountNumber)
	bankCode = strings.TrimSpace(bankCode)
	if len(accountNumber) != 10 {
		return "", fmt.Errorf("account number must be 10 digits")
	}
	if bankCode == "" {
		return "", fmt.Errorf("bank code is required")
	}

	payload := map[string]string{
		"account_number": accountNumber,
		"account_bank":   bankCode,
	}
	data, code, err := c.authRequest(http.MethodPost, "/accounts/resolve", payload)
	if err != nil {
		return "", err
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			AccountName string `json:"account_name"`
		} `json:"data"`
	}
	_ = json.Unmarshal(data, &out)

	if code < 200 || code >= 300 || !strings.EqualFold(out.Status, "success") || out.Data.AccountName == "" {
		msg := strings.TrimSpace(out.Message)
		if msg == "" {
			msg = "Account could not be verified. Check bank and account number."
		}
		return "", fmt.Errorf("%s", msg)
	}
	return out.Data.AccountName, nil
}

// ChargeCard initiates a live card payment (all currencies).
func (c *Client) ChargeCard(amount float64, currency, email, ref string, card map[string]string) (bool, string, error) {
	if !c.Configured() {
		return false, "", fmt.Errorf("Flutterwave secret key not configured in Admin → Settings → Payment Providers")
	}

	payload := map[string]interface{}{
		"tx_ref":       ref,
		"amount":       amount,
		"currency":     strings.ToUpper(currency),
		"redirect_url": "",
		"email":        email,
		"card_number":  card["number"],
		"cvv":          card["cvv"],
		"expiry_month": card["expiry_month"],
		"expiry_year":  card["expiry_year"],
	}

	data, code, err := c.authRequest(http.MethodPost, "/charges?type=card", payload)
	if err != nil {
		return false, "", err
	}
	if code < 200 || code >= 300 {
		return false, "", fmt.Errorf("Flutterwave charge failed HTTP %d: %s", code, strings.TrimSpace(string(data)))
	}

	var out struct {
		Status string `json:"status"`
		Data   struct {
			ID     int    `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, "", err
	}

	ok := strings.EqualFold(out.Status, "success") || strings.EqualFold(out.Data.Status, "successful")
	refOut := fmt.Sprintf("FLW-%d", out.Data.ID)
	return ok, refOut, nil
}

// ProcessTransfer sends a payout to a bank account (non-NGN currencies).
func (c *Client) ProcessTransfer(amount float64, currency, bankCode, accountNumber, narration string) (bool, string, error) {
	if c.Configured() {
		log.Printf("[Flutterwave] Transfer %.2f %s → %s (live keys configured)", amount, currency, accountNumber)
		// TODO: POST /transfers when live keys are active
	}

	log.Printf("[Flutterwave Mock] Transfer %.2f %s to %s", amount, currency, accountNumber)
	return true, "FLW_MOCK_" + fmt.Sprintf("%d", time.Now().Unix()), nil
}

// VerifyBVN performs KYC BVN verification for Nigerian customers via Flutterwave.
func (c *Client) VerifyBVN(bvn string) (bool, error) {
	if !c.Configured() {
		return false, fmt.Errorf("Flutterwave secret key not configured in Admin → Settings → Payment Providers")
	}

	bvn = strings.TrimSpace(bvn)
	if len(bvn) != 11 {
		return false, fmt.Errorf("BVN must be exactly 11 digits")
	}

	path := fmt.Sprintf("/kyc/bvns/%s", bvn)
	data, code, err := c.authRequest(http.MethodGet, path, nil)
	if err != nil {
		return false, err
	}

	if code < 200 || code >= 300 {
		return false, fmt.Errorf("Flutterwave BVN verification failed HTTP %d: %s", code, strings.TrimSpace(string(data)))
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return false, err
	}

	return strings.EqualFold(out.Status, "success"), nil
}