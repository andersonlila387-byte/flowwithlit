package smileid

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Config holds Smile ID credentials from admin settings.
type Config struct {
	PartnerID   string
	APIKey      string
	Environment string // sandbox | production
	CallbackURL string
}

func (c Config) Configured() bool {
	return strings.TrimSpace(c.PartnerID) != "" && strings.TrimSpace(c.APIKey) != ""
}

func (c Config) baseURL() string {
	if strings.EqualFold(c.Environment, "production") {
		return "https://api.smileidentity.com"
	}
	return "https://testapi.smileidentity.com"
}

// Client talks to Smile ID REST API (Basic KYC / ID number verification).
type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg}
}

type verifyRequest struct {
	SourceSDK        string                 `json:"source_sdk"`
	SourceSDKVersion string                 `json:"source_sdk_version"`
	PartnerID        string                 `json:"partner_id"`
	Signature        string                 `json:"signature"`
	Timestamp        string                 `json:"timestamp"`
	Country          string                 `json:"country"`
	IDType           string                 `json:"id_type"`
	IDNumber         string                 `json:"id_number"`
	CallbackURL      string                 `json:"callback_url,omitempty"`
	PartnerParams    map[string]string      `json:"partner_params"`
}

type verifyResponse struct {
	ResultCode string `json:"ResultCode"`
	ResultText string `json:"ResultText"`
	SmileJobID string `json:"SmileJobID"`
}

// GenerateSignature creates the HMAC signature Smile ID requires.
func GenerateSignature(apiKey, partnerID, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(timestamp))
	mac.Write([]byte(partnerID))
	mac.Write([]byte("sid_request"))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func mapIDType(country, primaryType string) (string, error) {
	t := strings.ToUpper(strings.TrimSpace(primaryType))
	cc := strings.ToUpper(strings.TrimSpace(country))

	switch cc {
	case "NG":
		switch t {
		case "BVN":
			return "BVN", nil
		case "NIN":
			return "NIN", nil
		default:
			return "", fmt.Errorf("Nigeria requires BVN or NIN (got %s)", primaryType)
		}
	default:
		switch t {
		case "PASSPORT":
			return "PASSPORT", nil
		case "NATIONAL_ID", "NATIONAL ID":
			return "NATIONAL_ID", nil
		case "DRIVERS_LICENSE", "DRIVER_LICENSE":
			return "DRIVERS_LICENSE", nil
		default:
			return "", fmt.Errorf("unsupported ID type %s for country %s", primaryType, country)
		}
	}
}

// VerifyIDNumber runs synchronous Basic KYC against Smile ID.
func (c *Client) VerifyIDNumber(country, primaryType, idNumber, jobID, userID string) (bool, string, error) {
	if !c.cfg.Configured() {
		return false, "", errors.New("Smile ID API keys not configured — add Partner ID and API Key in Admin → Settings → KYC")
	}

	idType, err := mapIDType(country, primaryType)
	if err != nil {
		return false, "", err
	}

	idNumber = strings.TrimSpace(idNumber)
	cc := strings.ToUpper(strings.TrimSpace(country))

	if cc == "NG" && idType == "BVN" && len(idNumber) != 11 {
		return false, "", errors.New("BVN must be 11 digits")
	}

	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	sig := GenerateSignature(c.cfg.APIKey, c.cfg.PartnerID, ts)

	if jobID == "" {
		jobID = fmt.Sprintf("fwl_%d", time.Now().UnixNano())
	}
	if userID == "" {
		userID = jobID
	}

	body := verifyRequest{
		SourceSDK:        "rest_api",
		SourceSDKVersion: "1.0.0",
		PartnerID:        c.cfg.PartnerID,
		Signature:        sig,
		Timestamp:        ts,
		Country:          cc,
		IDType:           idType,
		IDNumber:         idNumber,
		CallbackURL:      c.cfg.CallbackURL,
		PartnerParams: map[string]string{
			"job_id":  jobID,
			"user_id": userID,
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return false, "", err
	}

	url := c.cfg.baseURL() + "/v2/verify"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return false, "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("Smile ID request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, "", fmt.Errorf("Smile ID HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out verifyResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return false, "", fmt.Errorf("invalid Smile ID response: %w", err)
	}

	// 0810 = ID Number Validated (BVN/NIN match)
	if out.ResultCode == "0810" {
		return true, out.ResultText, nil
	}

	msg := out.ResultText
	if msg == "" {
		msg = "identity verification failed (code " + out.ResultCode + ")"
	}
	return false, msg, errors.New(msg)
}