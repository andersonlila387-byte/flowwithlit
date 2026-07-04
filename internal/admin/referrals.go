package admin

import (
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/referral"
	"flowwithlit/pkg/response"
)

func GetReferralStatsHandler(w http.ResponseWriter, r *http.Request) {
	var total, pending, paid int64
	var totalPaid float64

	database.DB.Model(&models.Referral{}).Count(&total)
	database.DB.Model(&models.Referral{}).Where("status = ?", "pending").Count(&pending)
	database.DB.Model(&models.Referral{}).Where("status = ?", "paid").Count(&paid)
	database.DB.Model(&models.Referral{}).Where("status = ?", "paid").
		Select("COALESCE(SUM(reward_amount), 0)").Scan(&totalPaid)

	cfg := referral.LoadConfig()

	response.Success(w, http.StatusOK, map[string]interface{}{
		"total_referrals":   total,
		"pending":           pending,
		"paid":              paid,
		"total_commissions": totalPaid,
		"config":            cfg,
	})
}

func GetReferralsHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.Referral{}).Count(&total)

	var rows []models.Referral
	database.DB.Order("created_at desc").Limit(limit).Offset(offset).Find(&rows)

	type item struct {
		models.Referral
		ReferrerEmail string `json:"referrer_email"`
		ReferrerName  string `json:"referrer_name"`
		RefereeEmail  string `json:"referee_email"`
		RefereeName   string `json:"referee_name"`
	}

	list := make([]item, 0, len(rows))
	for _, ref := range rows {
		var referrer, referee models.User
		database.DB.Select("email", "first_name", "last_name").First(&referrer, ref.ReferrerID)
		database.DB.Select("email", "first_name", "last_name").First(&referee, ref.RefereeID)
		list = append(list, item{
			Referral:      ref,
			ReferrerEmail: referrer.Email,
			ReferrerName:  referrer.FirstName + " " + referrer.LastName,
			RefereeEmail:  referee.Email,
			RefereeName:   referee.FirstName + " " + referee.LastName,
		})
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"referrals": list,
		"meta": map[string]interface{}{
			"page":        page,
			"per_page":    limit,
			"total_items": total,
		},
	})
}