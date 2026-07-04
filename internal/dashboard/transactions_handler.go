package dashboard

import (
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func TransactionsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "User ID not found in context")
		return
	}

	env := envfilter.Parse(r)

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 {
		limit = 10
	}

	offset := (page - 1) * limit

	var txns []models.Transaction
	var total int64

	query := envfilter.ApplyTxnFilter(database.DB.Model(&models.Transaction{}), env).
		Where("user_id = ?", userID)

	status := r.URL.Query().Get("status")
	if status != "" && status != "all" {
		query = query.Where("LOWER(status) = ?", status)
	}

	currency := r.URL.Query().Get("currency")
	if currency != "" && currency != "all" {
		if currency == "ngn" {
			query = query.Where("currency = ?", "NGN")
		} else if currency == "crypto" {
			query = query.Where("currency != ?", "NGN")
		}
	}

	query.Count(&total)

	query.Order("created_at desc").Offset(offset).Limit(limit).Find(&txns)

	totalPages := (int(total) + limit - 1) / limit

	response.Success(w, http.StatusOK, map[string]interface{}{
		"env":           env,
		"transactions":  txns,
		"total":         total,
		"page":          page,
		"limit":         limit,
		"total_pages":   totalPages,
		"has_next_page": page < totalPages,
		"has_prev_page": page > 1,
	})
}