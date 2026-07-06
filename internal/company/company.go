package company

import (
	"encoding/json"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

const SettingKey = "company_info"

// Info is the single source of truth for Flowwithlit company contact details.
type Info struct {
	CompanyName          string `json:"company_name"`
	LegalName            string `json:"legal_name"`
	Tagline              string `json:"tagline"`
	AddressLine1         string `json:"address_line1"`
	AddressLine2         string `json:"address_line2"`
	City                 string `json:"city"`
	StateRegion          string `json:"state_region"`
	Country              string `json:"country"`
	PostalCode           string `json:"postal_code"`
	SupportEmail         string `json:"support_email"`
	SalesEmail           string `json:"sales_email"`
	Phone                string `json:"phone"`
	PhoneDisplay         string `json:"phone_display"`
	SocialTwitter        string `json:"social_twitter"`
	SocialGithub         string `json:"social_github"`
	SocialLinkedin       string `json:"social_linkedin"`
	SocialFacebook       string `json:"social_facebook"`
	SocialInstagram      string `json:"social_instagram"`
	CopyrightName        string `json:"copyright_name"`
	RegistrationNumber   string `json:"registration_number"`
	RegulatoryDisclosure string `json:"regulatory_disclosure"`
}

// Defaults returns the platform default company profile.
func Defaults() Info {
	return Info{
		CompanyName:          "Flowwithlit",
		LegalName:            "Flowwithlit Inc.",
		Tagline:              "Unified Payments, Banking & Crypto",
		AddressLine1:         "123 Financial Way",
		AddressLine2:         "Innovation District",
		City:                 "Lagos",
		StateRegion:          "Lagos State",
		Country:              "Nigeria",
		PostalCode:           "",
		SupportEmail:         "support@flowwithlit.com",
		SalesEmail:           "sales@flowwithlit.com",
		Phone:                "+1234567890",
		PhoneDisplay:         "+1 (234) 567-890",
		CopyrightName:        "Flowwithlit Payments",
		RegistrationNumber:   "RC-000000",
		RegulatoryDisclosure: "Flowwithlit is not a bank. Payment and banking features are provided in partnership with [LICENSED PARTNER NAME], licensed by [REGULATOR NAME].",
	}
}

// Get loads company info from the database, falling back to defaults.
func Get() Info {
	info := Defaults()
	var setting models.SystemSetting
	if err := database.DB.Where("`key` = ?", SettingKey).First(&setting).Error; err != nil {
		return info
	}
	if strings.TrimSpace(setting.Value) == "" {
		return info
	}
	var stored Info
	if err := json.Unmarshal([]byte(setting.Value), &stored); err != nil {
		return info
	}
	return MergeWithDefaults(stored)
}

// Save persists company info as JSON in system_settings.
func Save(info Info) error {
	info = MergeWithDefaults(info)
	raw, err := json.Marshal(info)
	if err != nil {
		return err
	}

	var setting models.SystemSetting
	err = database.DB.Where("`key` = ?", SettingKey).First(&setting).Error
	if err != nil {
		setting = models.SystemSetting{Key: SettingKey, Value: string(raw)}
	} else {
		setting.Value = string(raw)
	}
	return database.DB.Save(&setting).Error
}

// MergeWithDefaults fills empty fields from defaults.
func MergeWithDefaults(info Info) Info {
	def := Defaults()
	if strings.TrimSpace(info.CompanyName) == "" {
		info.CompanyName = def.CompanyName
	}
	if strings.TrimSpace(info.LegalName) == "" {
		info.LegalName = def.LegalName
	}
	if strings.TrimSpace(info.Tagline) == "" {
		info.Tagline = def.Tagline
	}
	if strings.TrimSpace(info.AddressLine1) == "" {
		info.AddressLine1 = def.AddressLine1
	}
	if strings.TrimSpace(info.City) == "" {
		info.City = def.City
	}
	if strings.TrimSpace(info.Country) == "" {
		info.Country = def.Country
	}
	if strings.TrimSpace(info.SupportEmail) == "" {
		info.SupportEmail = def.SupportEmail
	}
	if strings.TrimSpace(info.SalesEmail) == "" {
		info.SalesEmail = def.SalesEmail
	}
	if strings.TrimSpace(info.Phone) == "" {
		info.Phone = def.Phone
	}
	if strings.TrimSpace(info.PhoneDisplay) == "" {
		info.PhoneDisplay = def.PhoneDisplay
	}
	if strings.TrimSpace(info.CopyrightName) == "" {
		info.CopyrightName = def.CopyrightName
	}
	if strings.TrimSpace(info.RegistrationNumber) == "" {
		info.RegistrationNumber = def.RegistrationNumber
	}
	if strings.TrimSpace(info.RegulatoryDisclosure) == "" {
		info.RegulatoryDisclosure = def.RegulatoryDisclosure
	}
	return info
}

// FormattedAddress returns a single-line mailing address for footers.
func (i Info) FormattedAddress() string {
	streetParts := make([]string, 0, 2)
	if s := strings.TrimSpace(i.AddressLine1); s != "" {
		streetParts = append(streetParts, s)
	}
	if s := strings.TrimSpace(i.AddressLine2); s != "" {
		streetParts = append(streetParts, s)
	}
	street := strings.Join(streetParts, ", ")

	locParts := make([]string, 0, 3)
	if s := strings.TrimSpace(i.City); s != "" {
		locParts = append(locParts, s)
	}
	if s := strings.TrimSpace(i.PostalCode); s != "" {
		locParts = append(locParts, s)
	}
	if s := strings.TrimSpace(i.Country); s != "" {
		locParts = append(locParts, s)
	}
	location := strings.Join(locParts, ", ")

	switch {
	case street != "" && location != "":
		return street + " · " + location
	case street != "":
		return street
	default:
		return location
	}
}

// TemplateVars returns flat placeholders for HTML email templates.
func (i Info) TemplateVars() map[string]string {
	return map[string]string{
		"company_name":          i.CompanyName,
		"company_legal_name":    i.LegalName,
		"company_tagline":       i.Tagline,
		"company_address":       i.FormattedAddress(),
		"support_email":         i.SupportEmail,
		"sales_email":           i.SalesEmail,
		"company_phone":         i.PhoneDisplay,
		"company_phone_tel":     i.Phone,
		"copyright_name":        i.CopyrightName,
		"registration_number":   i.RegistrationNumber,
		"regulatory_disclosure": i.RegulatoryDisclosure,
		"social_twitter":        i.SocialTwitter,
		"social_github":         i.SocialGithub,
		"social_linkedin":       i.SocialLinkedin,
		"social_facebook":       i.SocialFacebook,
		"social_instagram":      i.SocialInstagram,
	}
}
