package dashboard

import (
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/internal/rates"
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
	// Dashboard wallet tiles always show LIVE spendable balances (never sandbox/fake).
	balances := envfilter.LiveBalances(userID)

	// Today's revenue — LIVE NGN-equivalent successful volume (meaningful even in test UI mode)
	var todayRevenue float64
	today := time.Now().Truncate(24 * time.Hour)
	var todayTxns []models.Transaction
	envfilter.ApplyTxnFilter(database.DB.Where("user_id = ?", userID), "live").
		Where("LOWER(status) IN ? AND created_at >= ?", []string{"successful", "success", "completed", "paid"}, today).
		Find(&todayTxns)
	for _, t := range todayTxns {
		todayRevenue += txnNGNVolume(t)
	}

	// Pending escrows (env-scoped history)
	var escrowsPending int64
	escrowQ := database.DB.Model(&models.Transaction{}).
		Where("user_id = ? AND type = ? AND LOWER(status) = ?", userID, "Escrow", "pending")
	envfilter.ApplyTxnFilter(escrowQ, env).Count(&escrowsPending)

	// Recent activity (env-scoped)
	var recentActivity []models.Transaction
	envfilter.ApplyTxnFilter(database.DB.Where("user_id = ?", userID), env).
		Order("created_at desc").
		Limit(8).
		Find(&recentActivity)
	if recentActivity == nil {
		recentActivity = []models.Transaction{}
	}

	// Chart data — always last 7 days of LIVE volume in NGN so the graph is useful
	chartLabels := make([]string, 0, 7)
	chartData := make([]float64, 0, 7)

	for i := 6; i >= 0; i-- {
		day := time.Now().AddDate(0, 0, -i)
		chartLabels = append(chartLabels, day.Format("Mon"))
		startOfDay := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, day.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		var dayTxns []models.Transaction
		envfilter.ApplyTxnFilter(database.DB.Where("user_id = ?", userID), "live").
			Where("created_at >= ? AND created_at < ? AND LOWER(status) IN ?",
				startOfDay, endOfDay, []string{"successful", "success", "completed", "paid"}).
			Find(&dayTxns)

		var daily float64
		for _, t := range dayTxns {
			daily += txnNGNVolume(t)
		}
		chartData = append(chartData, daily)
	}

	out := map[string]interface{}{
		"env":             env,
		"balances":        balances,
		"spendable":       true,
		"today_revenue":   todayRevenue,
		"escrows_pending": escrowsPending,
		"recent_activity": recentActivity,
		"chart_labels":    chartLabels,
		"chart_data":      chartData,
		"chart_currency":  "NGN",
	}
	if env == "test" {
		out["sandbox_balances"] = envfilter.SandboxBalances(userID)
		out["sandbox_note"] = "Test payments are sandbox-only and never mix into your spendable wallet."
	}
	response.Success(w, http.StatusOK, out)
}

// txnNGNVolume converts a transaction amount to NGN for chart/volume tiles.
func txnNGNVolume(t models.Transaction) float64 {
	// Prefer settled amount when present
	if t.SettledAmount > 0 {
		cur := strings.ToUpper(strings.TrimSpace(t.SettledCurrency))
		if cur == "" || cur == "NGN" {
			return t.SettledAmount
		}
		if n := rates.Convert(t.SettledAmount, cur, "NGN"); n > 0 {
			return n
		}
	}
	cur := strings.ToUpper(strings.TrimSpace(t.Currency))
	if cur == "" || cur == "NGN" {
		return t.Amount
	}
	if n := rates.Convert(t.Amount, cur, "NGN"); n > 0 {
		return n
	}
	// Unknown FX — still count raw amount so chart is not empty
	return t.Amount
}
