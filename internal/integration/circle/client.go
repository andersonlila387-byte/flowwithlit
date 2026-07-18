package circle

import (
	"fmt"
	"log"
	"strings"
)

// Client handles communication with Circle APIs (USDC/Web3).
type Client struct {
	APIKey  string
	BaseURL string
}

// NewClient initializes a Circle integration client.
func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		APIKey:  strings.TrimSpace(apiKey),
		BaseURL: strings.TrimSpace(baseURL),
	}
}

// Configured reports whether a Circle API key is set.
func (c *Client) Configured() bool {
	return c != nil && c.APIKey != ""
}

// GenerateWalletAddress requests a blockchain address for deposits (no mock).
func (c *Client) GenerateWalletAddress(chain string) (string, error) {
	if !c.Configured() {
		return "", fmt.Errorf("Circle not configured — set circle_api_key in Admin → Settings (see key-get.md)")
	}
	log.Printf("[Circle] Wallet address requested chain=%s", chain)
	return "", fmt.Errorf("Circle wallet address live call not finished — keys present; complete Circle API wiring (see key-get.md)")
}

// ProcessWithdrawal sends crypto/USDC on-chain (no mock).
func (c *Client) ProcessWithdrawal(asset, destination string, amount float64) (bool, string, error) {
	if !c.Configured() {
		return false, "", fmt.Errorf("Circle not configured — set circle_api_key in Admin → Settings (see key-get.md)")
	}
	log.Printf("[Circle] Withdrawal %.8f %s → %s", amount, asset, destination)
	return false, "", fmt.Errorf("Circle withdrawal live call not finished — keys present; complete Circle payout API wiring (see key-get.md)")
}
