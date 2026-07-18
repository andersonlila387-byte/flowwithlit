package palmpay

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// Client is the PalmPay integration stub for future NGN bank rails.
// Live API calls are intentionally not wired yet so OnePipe keeps handling NGN
// until Admin sets ngn_bank_provider=palmpay and keys are configured.
type Client struct {
	APIKey    string
	SecretKey string
	MerchantID string
	BaseURL   string
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

// GenerateVirtualAccount will create an NGN virtual account via PalmPay.
// Until the live integration is completed, returns a deterministic mock for dev only.
func (c *Client) GenerateVirtualAccount(firstName, lastName, email, phone string) (string, string, error) {
	if c.Configured() {
		log.Printf("[PalmPay] Generating NGN virtual account for %s %s (keys present — live API not wired yet)", firstName, lastName)
		// TODO: POST PalmPay virtual-account / collection account API when go-live is approved.
	}

	log.Printf("[PalmPay Mock] Generating virtual account for %s %s", firstName, lastName)
	mockAccountNumber := "91" + fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
	mockBankName := "PalmPay"
	return mockAccountNumber, mockBankName, nil
}

// ProcessTransfer will send NGN payouts via PalmPay.
func (c *Client) ProcessTransfer(amount float64, bankCode, accountNumber, narration string) (bool, string, error) {
	if c.Configured() {
		log.Printf("[PalmPay] Processing NGN transfer %.2f → %s (keys present — live API not wired yet)", amount, accountNumber)
		// TODO: POST PalmPay transfer/disburse API when go-live is approved.
	}

	log.Printf("[PalmPay Mock] Processing NGN Transfer of %.2f to %s", amount, accountNumber)
	return true, "TRX_PALMPAY_MOCK", nil
}
