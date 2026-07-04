package checkout

import (
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

// merchantSupportEmail returns the merchant's support contact for customer receipts.
// Uses business_profiles.support_email, falling back to the merchant account email.
func merchantSupportEmail(userID uint) string {
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		if email := strings.TrimSpace(profile.SupportEmail); email != "" {
			return email
		}
	}

	var user models.User
	if err := database.DB.Select("email").First(&user, userID).Error; err == nil {
		return strings.TrimSpace(user.Email)
	}

	return ""
}