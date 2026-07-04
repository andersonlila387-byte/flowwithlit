package transaction

import (
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

// resolveRecipientQuery looks up a user by @flowtag or email. Returns nil user if not a member.
func resolveRecipientQuery(query string) (*models.User, string, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, "", false
	}

	if strings.HasPrefix(q, "@") {
		tag := strings.TrimPrefix(q, "@")
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			return nil, "", false
		}
		var user models.User
		if err := database.DB.Select("id, first_name, last_name, email, flow_tag_username").
			Where("flow_tag_username = ?", tag).First(&user).Error; err != nil {
			return nil, "", false
		}
		return &user, user.Email, true
	}

	if strings.Contains(q, "@") && !strings.HasPrefix(q, "@") {
		email := strings.ToLower(q)
		var user models.User
		if err := database.DB.Select("id, first_name, last_name, email, flow_tag_username").
			Where("email = ?", email).First(&user).Error; err != nil {
			return nil, email, false
		}
		return &user, user.Email, true
	}

	tag := strings.ToLower(q)
	var user models.User
	if err := database.DB.Select("id, first_name, last_name, email, flow_tag_username").
		Where("flow_tag_username = ?", tag).First(&user).Error; err != nil {
		return nil, "", false
	}
	return &user, user.Email, true
}