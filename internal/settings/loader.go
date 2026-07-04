package settings

import (
	"strings"
	"sync"

	"flowwithlit/internal/database"
	"flowwithlit/internal/integration/flutterwave"
	"flowwithlit/internal/integration/onepipe"
	"flowwithlit/internal/integration/smileid"
	"flowwithlit/internal/models"
)

var cache sync.Map

// Get returns a system setting value (empty string if missing).
func Get(key string) string {
	if v, ok := cache.Load(key); ok {
		return v.(string)
	}
	var s models.SystemSetting
	if err := database.DB.Where("`key` = ?", key).First(&s).Error; err != nil {
		return ""
	}
	cache.Store(key, s.Value)
	return s.Value
}

// Invalidate clears the in-process settings cache (call after admin updates).
func Invalidate() {
	cache = sync.Map{}
}

// KYCProvider returns the active KYC engine from admin settings.
func KYCProvider() string {
	v := strings.ToLower(strings.TrimSpace(Get("kyc_provider")))
	if v == "" {
		return "smileid"
	}
	return v
}

// SmileID returns Smile ID credentials from admin settings.
func SmileID() smileid.Config {
	env := strings.ToLower(strings.TrimSpace(Get("smile_environment")))
	if env == "" {
		env = "sandbox"
	}
	return smileid.Config{
		PartnerID:   strings.TrimSpace(Get("smile_partner_id")),
		APIKey:      strings.TrimSpace(Get("smile_api_key")),
		Environment: env,
		CallbackURL: strings.TrimSpace(Get("smile_callback_url")),
	}
}

// OnePipeClient builds a OnePipe client using admin payment keys.
func OnePipeClient() *onepipe.Client {
	return onepipe.NewClient(
		strings.TrimSpace(Get("onepipe_api_key")),
		strings.TrimSpace(Get("onepipe_secret")),
		"",
	)
}

// FlutterwaveClient builds a Flutterwave client using admin payment keys.
func FlutterwaveClient() *flutterwave.Client {
	return flutterwave.NewClient(strings.TrimSpace(Get("flutterwave_secret_key")), "")
}