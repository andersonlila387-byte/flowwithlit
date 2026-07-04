package admin

import (
	"math"
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"

	"gorm.io/gorm"
)

type TransactionStats struct {
	TotalVolume float64 `json:"total_volume"`
	Successful  int64   `json:"successful"`
	Pending     int64   `json:"pending"`
	Failed      int64   `json:"failed"`
}

type TransactionListResponse struct {
	Transactions []models.Transaction `json:"transactions"`
	Stats        TransactionStats     `json:"stats"`
	Meta         PaginationMeta       `json:"meta"`
}

func GetTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	var total int64
	var totalVolume float64
	var successful int64
	var pending int64
	var failed int64

	db := database.DB
	liveModel := func() *gorm.DB {
		return envfilter.ApplyTxnFilter(db.Model(&models.Transaction{}), "live")
	}

	// Live money only — exclude sandbox/test checkouts
	liveModel().Count(&total)
	liveModel().Where("LOWER(status) = ?", "successful").Count(&successful)
	liveModel().Where("LOWER(status) = ?", "pending").Count(&pending)
	liveModel().Where("LOWER(status) = ?", "failed").Count(&failed)

	liveModel().
		Where("LOWER(status) = ?", "successful").
		Select("COALESCE(SUM(amount), 0)").
		Row().Scan(&totalVolume)

	var transactions []models.Transaction
	envfilter.ApplyTxnFilter(database.DB, "live").
		Order("created_at desc").
		Limit(limit).
		Offset(offset).
		Find(&transactions)

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	if transactions == nil {
		transactions = make([]models.Transaction, 0)
	}

	resp := TransactionListResponse{
		Transactions: transactions,
		Stats: TransactionStats{
			TotalVolume: totalVolume,
			Successful:  successful,
			Pending:     pending,
			Failed:      failed,
		},
		Meta: PaginationMeta{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  int(total),
			PerPage:     limit,
		},
	}

	response.Success(w, http.StatusOK, resp)
}
