package referral

import (
	"net/http"
	"os"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func MeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	code, err := EnsureUserCode(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to load referral code")
		return
	}

	cfg := LoadConfig()
	base := strings.TrimRight(os.Getenv("AUTH_URL"), "/")
	if base == "" {
		base = strings.TrimRight(os.Getenv("FRONTEND_URL"), "/")
	}
	if base == "" {
		base = "https://auth.flowwithlit.com"
	}
	link := base + "/signup?ref=" + code

	var invited, paid int64
	database.DB.Model(&models.Referral{}).Where("referrer_id = ?", userID).Count(&invited)
	database.DB.Model(&models.Referral{}).Where("referrer_id = ? AND status = ?", userID, "paid").Count(&paid)

	var totalEarned float64
	database.DB.Model(&models.Referral{}).
		Where("referrer_id = ? AND status = ?", userID, "paid").
		Select("COALESCE(SUM(reward_amount), 0)").
		Scan(&totalEarned)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"enabled":         cfg.Enabled,
		"referral_code":   code,
		"referral_link":   link,
		"reward_amount":   cfg.RewardAmount,
		"reward_currency": cfg.RewardCurrency,
		"min_qualifying":  cfg.MinQualifyingAmount,
		"invited_count":   invited,
		"paid_count":      paid,
		"total_earned":    totalEarned,
	})
}

func EarningsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var rows []models.Referral
	database.DB.Where("referrer_id = ?", userID).Order("created_at desc").Limit(50).Find(&rows)

	type row struct {
		ID               uint    `json:"id"`
		Status           string  `json:"status"`
		RewardAmount     float64 `json:"reward_amount"`
		RewardCurrency   string  `json:"reward_currency"`
		QualifyingAmount float64 `json:"qualifying_amount"`
		CreatedAt        string  `json:"created_at"`
		PaidAt           string  `json:"paid_at,omitempty"`
		RefereeEmail     string  `json:"referee_email"`
	}

	out := make([]row, 0, len(rows))
	for _, ref := range rows {
		var referee models.User
		database.DB.Select("email").First(&referee, ref.RefereeID)
		item := row{
			ID:               ref.ID,
			Status:           ref.Status,
			RewardAmount:     ref.RewardAmount,
			RewardCurrency:   ref.RewardCurrency,
			QualifyingAmount: ref.QualifyingAmount,
			CreatedAt:        ref.CreatedAt.Format("2006-01-02 15:04"),
			RefereeEmail:     maskEmail(referee.Email),
		}
		if ref.PaidAt != nil {
			item.PaidAt = ref.PaidAt.Format("2006-01-02 15:04")
		}
		out = append(out, item)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"referrals": out,
	})
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	if at < 2 {
		return "***"
	}
	return email[:2] + "***" + email[at:]
}
