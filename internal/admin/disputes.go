package admin

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"flowwithlit/internal/database"
	"flowwithlit/internal/envfilter"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"

	"gorm.io/gorm"
)

type DisputeStats struct {
	Open        int64 `json:"open"`
	UnderReview int64 `json:"under_review"`
	Resolved    int64 `json:"resolved"`
	Rejected    int64 `json:"rejected"`
}

type DisputeListResponse struct {
	Disputes []models.Dispute `json:"disputes"`
	Stats    DisputeStats     `json:"stats"`
	Meta     PaginationMeta   `json:"meta"`
}

func GetDisputesHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 25
	offset := (page - 1) * limit

	db := database.DB
	liveDisputes := func() *gorm.DB {
		return db.Model(&models.Dispute{}).
			Joins(`LEFT JOIN transactions ON transactions.id = disputes.transaction_id`).
			Where(`disputes.transaction_id = 0 OR NOT `+envfilter.IsTestTxnSQL("transactions"), envfilter.IsTestTxnArgs()...)
	}

	var total, open, underReview, resolved, rejected int64
	liveDisputes().Count(&total)
	liveDisputes().Where("disputes.status = ?", "open").Count(&open)
	liveDisputes().Where("disputes.status = ?", "under_review").Count(&underReview)
	liveDisputes().Where("disputes.status = ?", "resolved").Count(&resolved)
	liveDisputes().Where("disputes.status = ?", "rejected").Count(&rejected)

	var disputes []models.Dispute
	liveDisputes().Order("disputes.created_at desc").Limit(limit).Offset(offset).Find(&disputes)
	if disputes == nil {
		disputes = make([]models.Dispute, 0)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, DisputeListResponse{
		Disputes: disputes,
		Stats:    DisputeStats{Open: open, UnderReview: underReview, Resolved: resolved, Rejected: rejected},
		Meta:     PaginationMeta{CurrentPage: page, TotalPages: totalPages, TotalItems: int(total), PerPage: limit},
	})
}

type UpdateDisputeRequest struct {
	Status     string `json:"status"`
	Resolution string `json:"resolution"`
}

func UpdateDisputeHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		response.Error(w, http.StatusBadRequest, "Invalid dispute ID")
		return
	}

	var req UpdateDisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	allowed := map[string]bool{"open": true, "under_review": true, "resolved": true, "rejected": true}
	if !allowed[req.Status] {
		response.Error(w, http.StatusBadRequest, "Invalid status value")
		return
	}

	var dispute models.Dispute
	if err := database.DB.First(&dispute, id).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Dispute not found")
		return
	}

	dispute.Status = req.Status
	if req.Resolution != "" {
		dispute.Resolution = req.Resolution
	}

	if err := database.DB.Save(&dispute).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update dispute")
		return
	}

	response.Success(w, http.StatusOK, dispute)
}
