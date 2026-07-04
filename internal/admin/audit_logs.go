package admin

import (
	"math"
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"
)

type AuditLogListResponse struct {
	Logs []models.AuditLog `json:"logs"`
	Meta PaginationMeta    `json:"meta"`
}

func GetAuditLogsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.AuditLog{}).Count(&total)

	var logs []models.AuditLog
	database.DB.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs)
	if logs == nil {
		logs = make([]models.AuditLog, 0)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, AuditLogListResponse{
		Logs: logs,
		Meta: PaginationMeta{CurrentPage: page, TotalPages: totalPages, TotalItems: int(total), PerPage: limit},
	})
}

// WriteAuditLog is a helper called by other admin handlers to record actions
func WriteAuditLog(adminID uint, adminEmail, action, resource, resourceID, details, ip string) {
	log := models.AuditLog{
		AdminID:    adminID,
		AdminEmail: adminEmail,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		IPAddress:  ip,
	}
	database.DB.Create(&log)
}
