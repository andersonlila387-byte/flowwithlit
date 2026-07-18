package admin

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"
)

// GetActivityHandler — GET /admin/activity
// Unified ops feed: activity logs + health counters so admins can spot breakdowns fast.
func GetActivityHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	area := strings.TrimSpace(r.URL.Query().Get("area"))
	level := strings.TrimSpace(r.URL.Query().Get("level"))
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	db := database.DB.Model(&models.ActivityLog{})
	if area != "" {
		db = db.Where("area = ?", area)
	}
	if level != "" {
		db = db.Where("level = ?", level)
	}
	if q != "" {
		like := "%" + q + "%"
		db = db.Where("message LIKE ? OR event LIKE ? OR reference LIKE ?", like, like, like)
	}

	var total int64
	db.Count(&total)

	var logs []models.ActivityLog
	db.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs)
	if logs == nil {
		logs = make([]models.ActivityLog, 0)
	}

	// Health snapshot (last 24h)
	since := time.Now().Add(-24 * time.Hour)
	var (
		actErrors24h    int64
		actWarnings24h  int64
		failedTxns24h   int64
		successTxns24h  int64
		failedWebhooks  int64
		pendingDisputes int64
		openTickets     int64
	)
	database.DB.Model(&models.ActivityLog{}).Where("level = ? AND created_at >= ?", "error", since).Count(&actErrors24h)
	database.DB.Model(&models.ActivityLog{}).Where("level = ? AND created_at >= ?", "warning", since).Count(&actWarnings24h)
	database.DB.Model(&models.Transaction{}).Where("status = ? AND created_at >= ?", "failed", since).Count(&failedTxns24h)
	database.DB.Model(&models.Transaction{}).Where("status = ? AND created_at >= ?", "successful", since).Count(&successTxns24h)
	database.DB.Model(&models.WebhookLog{}).Where("status = ? AND created_at >= ?", "failed", since).Count(&failedWebhooks)
	database.DB.Model(&models.Dispute{}).Where("status IN ?", []string{"open", "pending", "under_review"}).Count(&pendingDisputes)
	database.DB.Model(&models.SupportTicket{}).Where("status IN ?", []string{"open", "pending", "in_progress"}).Count(&openTickets)

	// Recent failed transactions for quick drill-down
	var failedTx []models.Transaction
	database.DB.Where("status = ?", "failed").Order("created_at desc").Limit(10).Find(&failedTx)
	if failedTx == nil {
		failedTx = []models.Transaction{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"logs": logs,
		"health": map[string]interface{}{
			"errors_24h":         actErrors24h,
			"warnings_24h":       actWarnings24h,
			"failed_txns_24h":    failedTxns24h,
			"success_txns_24h":   successTxns24h,
			"failed_webhooks_24h": failedWebhooks,
			"pending_disputes":   pendingDisputes,
			"open_tickets":       openTickets,
		},
		"recent_failed_transactions": failedTx,
		"areas": []string{
			"auth", "transfer", "bill", "webhook", "wallet", "biometric",
			"push", "kyc", "checkout", "card", "system",
		},
		"meta": PaginationMeta{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  int(total),
			PerPage:     limit,
		},
	})
}
