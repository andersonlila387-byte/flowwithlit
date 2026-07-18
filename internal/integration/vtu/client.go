package vtu

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client talks to a cheap SME/gifting VTU aggregator (VTPass-style).
// Env:
//   VTU_API_KEY    - API key / secret
//   VTU_BASE_URL   - e.g. https://vtpass.com/api  (optional)
// When not configured, Pay returns mock success so mobile UI can develop free.
type Client struct {
	APIKey  string
	BaseURL string
}

func NewFromEnv() *Client {
	base := strings.TrimSpace(os.Getenv("VTU_BASE_URL"))
	if base == "" {
		base = "https://vtpass.com/api"
	}
	return &Client{
		APIKey:  strings.TrimSpace(os.Getenv("VTU_API_KEY")),
		BaseURL: strings.TrimRight(base, "/"),
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.APIKey != ""
}

// PayDataOrAirtime attempts SME/gifting purchase.
// productCode maps to provider serviceID/variation (catalog product id).
func (c *Client) PayDataOrAirtime(category, productCode, phone string, amount float64, requestID string) (bool, string, error) {
	if !c.Configured() {
		log.Printf("[VTU Mock] %s product=%s phone=%s amount=%.2f ref=%s", category, productCode, phone, amount, requestID)
		return true, "VTU_MOCK_" + requestID, nil
	}

	// VTPass-compatible shape (common in NG SME/gifting). Adjust if you use ClubKonnect etc.
	body := map[string]interface{}{
		"request_id": requestID,
		"serviceID":  mapProductToService(productCode),
		"amount":     amount,
		"phone":      phone,
		"billersCode": phone,
	}
	if v := mapProductToVariation(productCode); v != "" {
		body["variation_code"] = v
	}

	raw, _ := json.Marshal(body)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/pay", bytes.NewReader(raw))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("api-key", c.APIKey)
	// Some providers use Authorization: Bearer
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	if sk := strings.TrimSpace(os.Getenv("VTU_SECRET_KEY")); sk != "" {
		req.Header.Set("secret-key", sk)
	}
	if pk := strings.TrimSpace(os.Getenv("VTU_PUBLIC_KEY")); pk != "" {
		req.Header.Set("public-key", pk)
	}

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("VTU HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out map[string]interface{}
	_ = json.Unmarshal(data, &out)
	// Accept common success codes
	code := fmt.Sprint(out["code"], out["response_description"], out["status"])
	ref := requestID
	if d, ok := out["requestId"].(string); ok && d != "" {
		ref = d
	}
	if d, ok := out["request_id"].(string); ok && d != "" {
		ref = d
	}
	ok := strings.Contains(strings.ToLower(code), "000") ||
		strings.Contains(strings.ToLower(fmt.Sprint(out["status"])), "success") ||
		resp.StatusCode == 200
	if !ok {
		return false, ref, fmt.Errorf("VTU rejected: %s", string(data))
	}
	return true, ref, nil
}

func mapProductToService(productID string) string {
	p := strings.ToLower(productID)
	switch {
	case strings.Contains(p, "mtn"):
		if strings.Contains(p, "airtime") {
			return "mtn"
		}
		return "mtn-data"
	case strings.Contains(p, "glo"):
		if strings.Contains(p, "airtime") {
			return "glo"
		}
		return "glo-data"
	case strings.Contains(p, "airtel"):
		if strings.Contains(p, "airtime") {
			return "airtel"
		}
		return "airtel-data"
	case strings.Contains(p, "9mobile") || strings.Contains(p, "etisalat"):
		if strings.Contains(p, "airtime") {
			return "etisalat"
		}
		return "etisalat-data"
	default:
		return productID
	}
}

func mapProductToVariation(productID string) string {
	// Optional: map catalog IDs to provider variation codes via env JSON later.
	// Empty means amount-only airtime or default plan.
	return strings.TrimSpace(os.Getenv("VTU_VARIATION_" + strings.ToUpper(strings.ReplaceAll(productID, "-", "_"))))
}
