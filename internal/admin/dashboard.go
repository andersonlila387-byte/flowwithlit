package admin

import (
	"net/http"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"
)

type AdminDashboardStats struct {
	TotalUsers     int64   `json:"total_users"`
	TotalMerchants int64   `json:"total_merchants"`
	TotalTxns      int64   `json:"total_txns"`
	TotalVolume    float64 `json:"total_volume"`
	PendingKYC     int64   `json:"pending_kyc"`
	OpenDisputes   int64   `json:"open_disputes"`
	OpenTickets    int64   `json:"open_tickets"`
	FailedWebhooks int64   `json:"failed_webhooks_24h"`
	Revenue7d      float64 `json:"revenue_7d"`
}

func GetDashboardStatsHandler(w http.ResponseWriter, r *http.Request) {
	db := database.DB

	var stats AdminDashboardStats

	db.Model(&models.User{}).Where("account_type = ?", "USER").Count(&stats.TotalUsers)
	db.Model(&models.User{}).Where("account_type = ?", "MERCHANT").Count(&stats.TotalMerchants)
	liveTxns := envfilter.ApplyTxnFilter(db.Model(&models.Transaction{}), "live")
	liveTxns.Count(&stats.TotalTxns)

	db.Model(&models.User{}).Where("kyc_level = 0").Count(&stats.PendingKYC)
	db.Model(&models.Dispute{}).
		Joins(`LEFT JOIN transactions ON transactions.id = disputes.transaction_id`).
		Where(`disputes.transaction_id = 0 OR NOT `+envfilter.IsTestTxnSQL("transactions"), envfilter.IsTestTxnArgs()...).
		Where("disputes.status = ?", "open").
		Count(&stats.OpenDisputes)
	db.Model(&models.SupportTicket{}).Where("status IN ?", []string{"open", "in_progress"}).Count(&stats.OpenTickets)
	db.Model(&models.WebhookLog{}).Where("status = ? AND created_at >= NOW() - INTERVAL 24 HOUR", "failed").Count(&stats.FailedWebhooks)

	envfilter.ApplyTxnFilter(db.Model(&models.Transaction{}), "live").
		Where("LOWER(status) = ?", "successful").
		Select("COALESCE(SUM(amount), 0)").
		Row().Scan(&stats.TotalVolume)

	envfilter.ApplyTxnFilter(db.Model(&models.Transaction{}), "live").
		Where("LOWER(status) = ? AND created_at >= NOW() - INTERVAL 7 DAY", "successful").
		Select("COALESCE(SUM(fee), 0)").
		Row().Scan(&stats.Revenue7d)

	response.Success(w, http.StatusOK, stats)
}
