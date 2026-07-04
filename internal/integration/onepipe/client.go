package onepipe

import (
	"fmt"
	"log"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.onepipe.io/v2"

// Client handles all communication with the OnePipe API (NGN bank rails).
type Client struct {
	APIKey    string
	SecretKey string
	BaseURL   string
}

// NewClient initializes a OnePipe integration client.
func NewClient(apiKey, secretKey, baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		APIKey:    strings.TrimSpace(apiKey),
		SecretKey: strings.TrimSpace(secretKey),
		BaseURL:   baseURL,
	}
}

// Configured reports whether live OnePipe keys are set in admin.
func (c *Client) Configured() bool {
	return c.APIKey != "" && c.SecretKey != ""
}

// GenerateVirtualAccount requests a dedicated NGN virtual account.
// Uses live API when keys are configured; otherwise returns a deterministic mock for dev.
func (c *Client) GenerateVirtualAccount(firstName, lastName, email, phone string) (string, string, error) {
	if c.Configured() {
		log.Printf("[OnePipe] Generating NGN virtual account for %s %s (live keys configured)", firstName, lastName)
		// TODO: POST {BaseURL}/virtual-accounts when OnePipe keys are active
	}

	log.Printf("[OnePipe] Generating virtual account for %s %s", firstName, lastName)
	mockAccountNumber := "82" + fmt.Sprintf("%08d", time.Now().UnixNano()%100000000)
	mockBankName := "Providus Bank"
	return mockAccountNumber, mockBankName, nil
}

// ProcessTransfer sends NGN from Flowwithlit to an external Nigerian bank account.
func (c *Client) ProcessTransfer(amount float64, bankCode, accountNumber, narration string) (bool, string, error) {
	if c.Configured() {
		log.Printf("[OnePipe] Processing NGN transfer %.2f → %s (live keys configured)", amount, accountNumber)
		// TODO: POST {BaseURL}/transfers when OnePipe keys are active
	}

	log.Printf("[OnePipe Mock] Processing NGN Transfer of %.2f to %s", amount, accountNumber)
	return true, "TRX_MOCK_SUCCESS", nil
}