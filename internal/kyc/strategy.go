package kyc

import (
	"errors"
	"fmt"
	"strings"

	"flowwithlit/internal/integration/onepipe"
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
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	DOB           string `json:"date_of_birth"`
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

// ----------------------------------------------------------------------------
// ONEPIPE (server-side BVN / NIN only — never exposed to browser)
// ----------------------------------------------------------------------------

type OnePipeProvider struct{}

func (o *OnePipeProvider) Name() string { return "internal" } // never surface vendor name to clients

func (o *OnePipeProvider) VerifyIdentity(payload IdentityPayload) (string, error) {
	country := strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	if country != "" && country != "NG" {
		return "pending", nil
	}

	client := settings.OnePipeClient()
	if !client.Configured() {
		// No keys: manual review instead of hard fail
		return "pending", nil
	}

	cust := onepipe.Customer{
		Ref:       payload.UserID,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		Email:     payload.Email,
		Phone:     payload.Phone,
		DOB:       payload.DOB,
	}

	idType := strings.ToUpper(strings.TrimSpace(payload.PrimaryIDType))
	switch idType {
	case "BVN":
		res, err := client.LookupBVN(payload.PrimaryIDVal, cust)
		if err != nil {
			// Fall back to manual review rather than blocking signup on provider OTP quirks
			if strings.Contains(strings.ToLower(err.Error()), "otp") {
				return "pending", nil
			}
			return "failed", err
		}
		if res != nil && res.OK {
			return "approved", nil
		}
		return "failed", errors.New("BVN verification failed")
	case "NIN":
		res, err := client.LookupNIN(payload.PrimaryIDVal, cust)
		if err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "otp") {
				return "pending", nil
			}
			return "failed", err
		}
		if res != nil && res.OK {
			return "approved", nil
		}
		return "failed", errors.New("NIN verification failed")
	default:
		return "pending", nil
	}
}