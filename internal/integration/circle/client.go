package circle

import (
	"log"
)

// Client handles all communication with the Circle APIs (USDC/Web3)
type Client struct {
	APIKey  string
	BaseURL string
}

// NewClient initializes a new Circle integration client
func NewClient(apiKey, baseURL string) *Client {
	return &Client{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
}

// GenerateWalletAddress requests a blockchain address for USDC deposits
func (c *Client) GenerateWalletAddress(chain string) (string, error) {
	log.Printf("[Circle Mock] Generating %s wallet address", chain)
	
	// Mock data
	mockAddress := "0x71C7656EC7ab88b098defB751B7401B5f6d8976F"
	
	return mockAddress, nil
}
