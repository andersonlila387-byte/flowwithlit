package onepipe

import (
	"fmt"
	"log"
	"strings"
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

func (c *Client) requireKeys() error {
	if !c.Configured() {
		return fmt.Errorf("OnePipe not configured — set onepipe_api_key and onepipe_secret in Admin → Settings (see key-get.md)")
	}
	return nil
}

// GenerateVirtualAccount requests a dedicated NGN virtual account.
// No mock: fails clearly if keys missing or live call not ready.
func (c *Client) GenerateVirtualAccount(firstName, lastName, email, phone string) (string, string, error) {
	if err := c.requireKeys(); err != nil {
		return "", "", err
	}
	log.Printf("[OnePipe] VA requested for %s %s email=%s — live HTTP virtual-accounts call must match your OnePipe contract", firstName, lastName, email)
	// Live HTTP integration depends on your OnePipe product plan / endpoints.
	// Prefer failing loudly over fake account numbers that break deposits.
	return "", "", fmt.Errorf("OnePipe virtual account live call not finished for this deployment — keys are present; complete OnePipe VA API wiring or use a working NGN rail (see key-get.md)")
}

// ProcessTransfer sends NGN from Flowwithlit to an external Nigerian bank account.
func (c *Client) ProcessTransfer(amount float64, bankCode, accountNumber, narration string) (bool, string, error) {
	if err := c.requireKeys(); err != nil {
		return false, "", err
	}
	log.Printf("[OnePipe] Transfer %.2f → %s bank=%s", amount, accountNumber, bankCode)
	return false, "", fmt.Errorf("OnePipe transfer live call not finished for this deployment — keys are present; complete OnePipe transfer API wiring (see key-get.md)")
}
