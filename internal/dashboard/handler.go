package dashboard

import (
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	env := envfilter.Parse(r)
	balances := envfilter.BalancesForEnv(userID, env)

	// Today's revenue (successful only, env-scoped)
	var todayRevenue float64
	today := time.Now().Truncate(24 * time.Hour)
	envfilter.ApplyTxnFilter(database.DB.Model(&models.Transaction{}), env).
		Where("user_id = ? AND LOWER(status) = ? AND created_at >= ?", userID, "successful", today).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&todayRevenue)

	// Pending escrows (live activity only — not sandbox checkout)
	var escrowsPending int64
	escrowQ := database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND type = ? AND LOWER(status) = ?", userID, "Escrow", "pending")
	if env == "test" {
		envfilter.ApplyTxnFilter(escrowQ, env).Count(&escrowsPending)
	} else {
		envfilter.ApplyTxnFilter(escrowQ, env).Count(&escrowsPending)
	}

	// Recent activity
	var recentActivity []models.Transaction
	envfilter.ApplyTxnFilter(database.DB.Where("user_id = ?", userID), env).
		Order("created_at desc").
		Limit(5).
		Find(&recentActivity)

	// Chart data — 7-day volume
	var chartLabels []string
	var chartData []float64

	for i := 6; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i).Format("Mon")
		startOfDay := time.Now().AddDate(0, 0, -i).Truncate(24 * time.Hour)
		endOfDay := startOfDay.Add(24 * time.Hour)

		var dailyVolume float64
		envfilter.ApplyTxnFilter(database.DB.Model(&models.Transaction{}), env).
			Where("user_id = ? AND created_at >= ? AND created_at < ? AND LOWER(status) = ?", userID, startOfDay, endOfDay, "successful").
			Select("COALESCE(SUM(amount), 0)").
			Scan(&dailyVolume)

		chartLabels = append(chartLabels, day)
		chartData = append(chartData, dailyVolume)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"env":              env,
		"balances":         balances,
		"today_revenue":    todayRevenue,
		"escrows_pending":  escrowsPending,
		"recent_activity":  recentActivity,
		"chart_labels":     chartLabels,
		"chart_data":       chartData,
	})
}