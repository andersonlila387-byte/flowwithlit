package admin

import (
	"math"
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"
)

type WebhookLogStats struct {
	Total     int64 `json:"total"`
	Processed int64 `json:"processed"`
	Failed    int64 `json:"failed"`
	Last24h   int64 `json:"last_24h"`
}

type WebhookLogListResponse struct {
	Logs  []models.WebhookLog `json:"logs"`
	Stats WebhookLogStats     `json:"stats"`
	Meta  PaginationMeta      `json:"meta"`
}

func GetWebhookLogsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	db := database.DB

	var total, processed, failed, last24h int64
	db.Model(&models.WebhookLog{}).Count(&total)
	db.Model(&models.WebhookLog{}).Where("status = ?", "processed").Count(&processed)
	db.Model(&models.WebhookLog{}).Where("status = ?", "failed").Count(&failed)
	db.Model(&models.WebhookLog{}).Where("created_at >= NOW() - INTERVAL 24 HOUR").Count(&last24h)

	var logs []models.WebhookLog
	db.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs)
	if logs == nil {
		logs = make([]models.WebhookLog, 0)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, WebhookLogListResponse{
		Logs:  logs,
		Stats: WebhookLogStats{Total: total, Processed: processed, Failed: failed, Last24h: last24h},
		Meta:  PaginationMeta{CurrentPage: page, TotalPages: totalPages, TotalItems: int(total), PerPage: limit},
	})
}
