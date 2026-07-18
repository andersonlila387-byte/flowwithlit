package palmpay

import (
	"fmt"
	"log"
	"strings"
)

// Client is the PalmPay integration for future NGN bank rails.
type Client struct {
	APIKey     string
	SecretKey  string
	MerchantID string
	BaseURL    string
}

const defaultBaseURL = "https://open-gw-prod.palmpay-inc.com"

// NewClient initializes a PalmPay client from admin settings.
func NewClient(apiKey, secretKey, merchantID, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		APIKey:     strings.TrimSpace(apiKey),
		SecretKey:  strings.TrimSpace(secretKey),
		MerchantID: strings.TrimSpace(merchantID),
		BaseURL:    baseURL,
	}
}

// Configured reports whether PalmPay live keys are present in admin.
func (c *Client) Configured() bool {
	return c.APIKey != "" && c.SecretKey != ""
}

func (c *Client) requireKeys() error {
	if !c.Configured() {
		return fmt.Errorf("PalmPay not configured — set palmpay_api_key and palmpay_secret in Admin → Settings (see key-get.md)")
	}
	return nil
}

// GenerateVirtualAccount creates an NGN virtual account via PalmPay (no mock).
func (c *Client) GenerateVirtualAccount(firstName, lastName, email, phone string) (string, string, error) {
	if err := c.requireKeys(); err != nil {
		return "", "", err
	}
	log.Printf("[PalmPay] VA requested for %s %s", firstName, lastName)
	return "", "", fmt.Errorf("PalmPay virtual account live call not finished — keys present; complete PalmPay collection API or keep ngn_bank_provider=onepipe (see key-get.md)")
}

// ProcessTransfer sends NGN payouts via PalmPay (no mock).
func (c *Client) ProcessTransfer(amount float64, bankCode, accountNumber, narration string) (bool, string, error) {
	if err := c.requireKeys(); err != nil {
		return false, "", err
	}
	log.Printf("[PalmPay] Transfer %.2f → %s", amount, accountNumber)
	return false, "", fmt.Errorf("PalmPay transfer live call not finished — keys present; complete PalmPay disburse API (see key-get.md)")
}
