package transaction

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// RequestFlowTagPaymentHandler — POST /flowtag/request (P2P payment request)
func RequestFlowTagPaymentHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		PayerEmail string  `json:"payer_email"`
		PayerTag   string  `json:"payer_tag"`
		Currency   string  `json:"currency"`
		Amount     float64 `json:"amount"`
		Note       string  `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	lookupQuery := req.PayerEmail
	if lookupQuery == "" {
		lookupQuery = req.PayerTag
	}
	if lookupQuery == "" {
		response.Error(w, http.StatusBadRequest, "payer @tag or email is required")
		return
	}

	payerUser, payerEmail, _ := resolveRecipientQuery(lookupQuery)
	req.PayerEmail = payerEmail
	if req.PayerEmail == "" {
		response.Error(w, http.StatusBadRequest, "Invalid payer")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Amount must be greater than zero")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	var requester models.User
	if err := database.DB.First(&requester, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if requester.Email == req.PayerEmail {
		response.Error(w, http.StatusBadRequest, "You cannot request payment from yourself")
		return
	}

	token, err := GenerateRandomString(16)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}
	ref, _ := GenerateRandomString(8)

	var payerID *uint
	if payerUser != nil {
		payerID = &payerUser.ID
	}

	payReq := models.FlowTagPaymentRequest{
		Reference:   "FTR_" + ref,
		RequesterID: userID,
		PayerEmail:  req.PayerEmail,
		PayerID:     payerID,
		Currency:    req.Currency,
		Amount:      req.Amount,
		Note:        req.Note,
		Status:      "pending",
		PayToken:    token,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	if err := database.DB.Create(&payReq).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create payment request")
		return
	}

	requesterName := requester.FirstName + " " + requester.LastName
	if requesterName == " " {
		requesterName = requester.Email
	}
	payURL := fmt.Sprintf("%s/app/flowtags.php?pay_token=%s", getAppBaseURL(), token)

	_ = email.SendFlowTagReceived(
		req.PayerEmail,
		requesterName,
		requester.Email,
		req.Amount,
		req.Currency,
		payURL,
		"7 days",
	)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Payment request sent successfully",
		"reference": payReq.Reference,
		"pay_token": payReq.PayToken,
		"pay_url":   payURL,
		"expires_at": payReq.ExpiresAt,
	})
}

// GetFlowTagRequestsHandler — GET /flowtag/requests
func GetFlowTagRequestsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	database.DB.Select("email").First(&user, userID)

	var sent []models.FlowTagPaymentRequest
	database.DB.Where("requester_id = ?", userID).Order("created_at desc").Limit(50).Find(&sent)

	var received []models.FlowTagPaymentRequest
	database.DB.Where("payer_email = ?", user.Email).Order("created_at desc").Limit(50).Find(&received)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"sent":     sent,
		"received": received,
	})
}

// PayFlowTagRequestHandler — POST /flowtag/request/pay
func PayFlowTagRequestHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		PayToken string `json:"pay_token"`
		RequestID uint  `json:"request_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var pinUser models.User
	database.DB.First(&pinUser, userID)
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN in Settings first")
		return
	}

	db := database.DB
	tx := db.Begin()

	var payReq models.FlowTagPaymentRequest
	q := tx.Set("gorm:query_option", "FOR UPDATE")
	if req.PayToken != "" {
		q = q.Where("pay_token = ?", req.PayToken)
	} else if req.RequestID > 0 {
		q = q.Where("id = ?", req.RequestID)
	} else {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "pay_token or request_id is required")
		return
	}
	if err := q.First(&payReq).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusNotFound, "Payment request not found")
		return
	}

	if payReq.Status != "pending" {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "This request is no longer pending")
		return
	}
	if time.Now().After(payReq.ExpiresAt) {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "This payment request has expired")
		return
	}
	if pinUser.Email != payReq.PayerEmail {
		tx.Rollback()
		response.Error(w, http.StatusForbidden, "Only the requested payer can complete this payment")
		return
	}

	var payerWallet models.Wallet
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", userID, payReq.Currency).First(&payerWallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "Wallet not found for currency "+payReq.Currency)
		return
	}
	if payerWallet.Balance < payReq.Amount {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "Insufficient balance")
		return
	}
	payerWallet.Balance -= payReq.Amount
	if err := tx.Save(&payerWallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update payer wallet")
		return
	}

	var receiverWallet models.Wallet
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", payReq.RequesterID, payReq.Currency).First(&receiverWallet).Error; err != nil {
		receiverWallet = models.Wallet{UserID: payReq.RequesterID, Currency: payReq.Currency, Balance: 0}
		if err := tx.Create(&receiverWallet).Error; err != nil {
			tx.Rollback()
			response.Error(w, http.StatusInternalServerError, "Failed to create receiver wallet")
			return
		}
	}
	receiverWallet.Balance += payReq.Amount
	if err := tx.Save(&receiverWallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update receiver wallet")
		return
	}

	now := time.Now()
	payReq.Status = "paid"
	payReq.PaidAt = &now
	payReq.PayerID = &userID
	if err := tx.Save(&payReq).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update request")
		return
	}

	tx.Create(&models.Transaction{
		UserID: userID, Type: "FLOWTAG_REQUEST_PAY", Currency: payReq.Currency,
		Amount: payReq.Amount, Status: "COMPLETED", Reference: payReq.Reference,
		BalanceAfter: payerWallet.Balance,
	})
	tx.Create(&models.Transaction{
		UserID: payReq.RequesterID, Type: "FLOWTAG_REQUEST_RECEIVE", Currency: payReq.Currency,
		Amount: payReq.Amount, Status: "COMPLETED", Reference: payReq.Reference,
		BalanceAfter: receiverWallet.Balance,
	})

	tx.Commit()
	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Payment completed successfully",
		"reference": payReq.Reference,
		"amount":    payReq.Amount,
		"currency":  payReq.Currency,
	})
}

// DeclineFlowTagRequestHandler — POST /flowtag/request/decline
func DeclineFlowTagRequestHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		RequestID uint   `json:"request_id"`
		PayToken    string `json:"pay_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	var user models.User
	database.DB.Select("email").First(&user, userID)

	var payReq models.FlowTagPaymentRequest
	q := database.DB
	if req.PayToken != "" {
		q = q.Where("pay_token = ?", req.PayToken)
	} else {
		q = q.Where("id = ?", req.RequestID)
	}
	if err := q.First(&payReq).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Payment request not found")
		return
	}
	if user.Email != payReq.PayerEmail {
		response.Error(w, http.StatusForbidden, "Only the payer can decline this request")
		return
	}
	if payReq.Status != "pending" {
		response.Error(w, http.StatusBadRequest, "Request is not pending")
		return
	}

	now := time.Now()
	payReq.Status = "declined"
	payReq.DeclinedAt = &now
	database.DB.Save(&payReq)

	response.Success(w, http.StatusOK, map[string]string{"message": "Payment request declined"})
}

func getAppBaseURL() string {
	if u := os.Getenv("FRONTEND_URL"); u != "" {
		return u
	}
	return "http://localhost/flowwithlit"
}