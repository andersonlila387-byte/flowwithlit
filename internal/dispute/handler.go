package dispute

import (
	"encoding/json"
	"net/http"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"crypto/rand"
	"encoding/hex"
)

func generateRef(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func GetUserDisputesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var disputes []models.Dispute
	database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&disputes)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"disputes": disputes,
		"total":    len(disputes),
	})
}

func CreateDisputeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		TransactionRef string  `json:"transaction_ref"`
		Amount         float64 `json:"amount"`
		Currency       string  `json:"currency"`
		Reason         string  `json:"reason"`
		Details        string  `json:"details"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Reason == "" {
		response.Error(w, http.StatusBadRequest, "Reason is required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	// Optionally link to a real transaction
	var txnID uint
	if req.TransactionRef != "" {
		var txn models.Transaction
		if err := database.DB.Where("reference = ? AND user_id = ?", req.TransactionRef, userID).First(&txn).Error; err == nil {
			txnID = txn.ID
			if req.Amount == 0 {
				req.Amount = txn.Amount
			}
		}
	}

	var user models.User
	database.DB.Select("email").First(&user, userID)

	dispute := models.Dispute{
		UserID:        userID,
		TransactionID: txnID,
		Reference:     "DSP_" + generateRef(6),
		Amount:        req.Amount,
		Currency:      req.Currency,
		Reason:        req.Reason,
		Details:       req.Details,
		Status:        "open",
		UserEmail:     user.Email,
	}

	if err := database.DB.Create(&dispute).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to file dispute")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Dispute filed successfully",
		"dispute":   dispute,
		"reference": dispute.Reference,
	})
}
