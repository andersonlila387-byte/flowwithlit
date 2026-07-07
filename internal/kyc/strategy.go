package kyc

import (
	"errors"
	"fmt"
	"strings"

	"flowwithlit/internal/integration/smileid"
	"flowwithlit/internal/settings"
)

// IdentityPayload is the unified request sent by the frontend
type IdentityPayload struct {
	CountryCode   string `json:"country_code"`
	PrimaryIDType string `json:"primary_id_type"`
	PrimaryIDVal  string `json:"primary_id_val"`
	SecondaryID   string `json:"secondary_id"`
	UserID        string `json:"user_id"` // optional — for Smile ID partner_params
}

// KYCProvider defines the contract for any identity verification engine
type KYCProvider interface {
	VerifyIdentity(payload IdentityPayload) (string, error)
	Name() string
}

// ----------------------------------------------------------------------------
// MOCK PROVIDER
// ----------------------------------------------------------------------------

type MockProvider struct{}

func (m *MockProvider) Name() string { return "MockProvider" }

func (m *MockProvider) VerifyIdentity(payload IdentityPayload) (string, error) {
	country := strings.ToUpper(payload.CountryCode)

	switch country {
	case "NG":
		if payload.PrimaryIDType != "BVN" && payload.PrimaryIDType != "NIN" {
			return "failed", errors.New("Nigeria requires BVN or NIN")
		}
		if payload.PrimaryIDType == "BVN" && len(payload.PrimaryIDVal) != 11 {
			return "failed", errors.New("Mock BVN must be 11 digits")
		}
	case "US":
		if payload.PrimaryIDType != "SSN" && payload.PrimaryIDType != "EIN" {
			return "failed", errors.New("US requires SSN or EIN")
		}
		if len(payload.PrimaryIDVal) < 9 {
			return "failed", errors.New("Invalid Mock SSN/EIN length")
		}
	case "GB":
		if payload.PrimaryIDType != "CRN" {
			return "failed", errors.New("UK requires Company Registration Number")
		}
	default:
		if payload.PrimaryIDType != "PASSPORT" && payload.PrimaryIDType != "NATIONAL_ID" {
			return "failed", fmt.Errorf("country %s requires PASSPORT or NATIONAL_ID", country)
		}
	}

	return "approved", nil
}

// ----------------------------------------------------------------------------
// SMILE ID PROVIDER
// ----------------------------------------------------------------------------

type SmileIDProvider struct {
	client *smileid.Client
}

func NewSmileIDProvider() *SmileIDProvider {
	cfg := settings.SmileID()
	return &SmileIDProvider{client: smileid.NewClient(cfg)}
}

func (s *SmileIDProvider) Name() string { return "SmileID" }

func (s *SmileIDProvider) VerifyIdentity(payload IdentityPayload) (string, error) {
	ok, _, err := s.client.VerifyIDNumber(
		payload.CountryCode,
		payload.PrimaryIDType,
		payload.PrimaryIDVal,
		"",
		payload.UserID,
	)
	if err != nil {
		return "failed", err
	}
	if !ok {
		return "failed", errors.New("Smile ID verification failed")
	}
	return "approved", nil
}

// ----------------------------------------------------------------------------
// FLUTTERWAVE + HYBRID MANUAL PROVIDER
// ----------------------------------------------------------------------------

type FlutterwaveProvider struct{}

func (f *FlutterwaveProvider) Name() string { return "Flutterwave" }

func (f *FlutterwaveProvider) VerifyIdentity(payload IdentityPayload) (string, error) {
	// Flutterwave only supports BVN for Nigeria
	if strings.ToUpper(payload.CountryCode) == "NG" && payload.PrimaryIDType == "BVN" {
		client := settings.FlutterwaveClient()
		if client.Configured() {
			ok, err := client.VerifyBVN(payload.PrimaryIDVal)
			if err != nil {
				return "failed", err
			}
			if ok {
				return "approved", nil
			}
			return "failed", errors.New("BVN verification failed")
		}
		// If Flutterwave keys aren't configured, fallback to manual review
		return "pending", nil
	}

	// For international users or other ID types, default to manual review
	return "pending", nil
}