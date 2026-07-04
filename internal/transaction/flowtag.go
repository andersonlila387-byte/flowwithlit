package transaction

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	walletPkg "flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// GenerateRandomString generates a secure random string of given byte length
func GenerateRandomString(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func SendFlowTagHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		RecipientEmail string  `json:"recipient_email"`
		RecipientTag   string  `json:"recipient_tag"`
		Currency       string  `json:"currency"`
		Amount         float64 `json:"amount"`
		Note           string  `json:"note"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Amount must be greater than zero")
		return
	}

	lookupQuery := req.RecipientEmail
	if lookupQuery == "" {
		lookupQuery = req.RecipientTag
	}
	if lookupQuery == "" {
		response.Error(w, http.StatusBadRequest, "Recipient @tag or email is required")
		return
	}

	recipientUser, recipientEmail, isMember := resolveRecipientQuery(lookupQuery)
	if recipientEmail == "" {
		response.Error(w, http.StatusBadRequest, "Invalid recipient")
		return
	}
	if isMember && recipientUser != nil && recipientUser.ID == userID {
		response.Error(w, http.StatusBadRequest, "You cannot send to yourself")
		return
	}

	// Enforce PIN for sending
	var pinUser models.User
	database.DB.First(&pinUser, userID)
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN in Settings first")
		return
	}

	if req.Currency == "" {
		req.Currency = "NGN"
	}

	// Support common currencies
	allowedCurrencies := map[string]bool{"NGN": true, "USD": true, "USDT": true, "EUR": true}
	if !allowedCurrencies[req.Currency] {
		response.Error(w, http.StatusBadRequest, "Unsupported currency")
		return
	}

	ref, _ := GenerateRandomString(8)
	flowRef := "FT_" + ref

	// Instant transfer for registered members
	if isMember && recipientUser != nil {
		desc := "FlowTag transfer"
		if req.Note != "" {
			desc = req.Note
		}
		if err := walletPkg.InternalTransfer(userID, recipientUser.ID, req.Amount, req.Currency, flowRef, desc); err != nil {
			response.Error(w, http.StatusBadRequest, err.Error())
			return
		}

		token, _ := GenerateRandomString(16)
		flowTag := models.FlowTag{
			Reference:      flowRef,
			SenderID:       userID,
			RecipientEmail: recipientEmail,
			Currency:       req.Currency,
			Amount:         req.Amount,
			Status:         "Claimed",
			ClaimToken:     token,
			ExpiresAt:      time.Now().Add(24 * time.Hour),
		}
		database.DB.Create(&flowTag)

		response.Success(w, http.StatusOK, map[string]interface{}{
			"message":      fmt.Sprintf("Sent instantly to @%s", recipientUser.FlowTagUsername),
			"reference":    flowRef,
			"instant":      true,
			"recipient_tag": recipientUser.FlowTagUsername,
		})
		return
	}

	db := database.DB

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var wallet models.Wallet
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", userID, req.Currency).First(&wallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "Wallet not found for currency "+req.Currency)
		return
	}

	if wallet.Balance < req.Amount {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "Insufficient balance")
		return
	}

	wallet.Balance -= req.Amount
	if err := tx.Save(&wallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update wallet")
		return
	}

	token, err := GenerateRandomString(16)
	if err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	flowTag := models.FlowTag{
		Reference:      flowRef,
		SenderID:       userID,
		RecipientEmail: recipientEmail,
		Currency:       req.Currency,
		Amount:         req.Amount,
		Status:         "Pending",
		ClaimToken:     token,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}

	if err := tx.Create(&flowTag).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to create FlowTag")
		return
	}

	txn := models.Transaction{
		UserID:       userID,
		Type:         "FLOWTAG_SEND",
		Currency:     req.Currency,
		Amount:       req.Amount,
		Status:       "COMPLETED",
		Reference:    flowTag.Reference,
		BalanceAfter: wallet.Balance,
	}

	if err := tx.Create(&txn).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to log transaction")
		return
	}

	tx.Commit()

	senderName := "A Flowwithlit user"
	senderEmail := ""
	var sender models.User
	if err := database.DB.Select("first_name", "last_name", "email").First(&sender, userID).Error; err == nil {
		if sender.FirstName != "" || sender.LastName != "" {
			senderName = fmt.Sprintf("%s %s", sender.FirstName, sender.LastName)
		}
		senderEmail = sender.Email
	}

	claimURL := fmt.Sprintf("%s/claim.php?token=%s", getBaseURL(), token)
	expiresIn := "24 hours"

	_ = email.SendFlowTagReceived(
		recipientEmail,
		senderName,
		senderEmail,
		req.Amount,
		req.Currency,
		claimURL,
		expiresIn,
	)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "FlowTag sent successfully. The recipient will receive an email to claim the funds.",
		"reference": flowTag.Reference,
		"instant":   false,
	})
}

// getBaseURL returns the frontend base for links (configurable via env)
func getBaseURL() string {
	if u := os.Getenv("FRONTEND_URL"); u != "" {
		return u
	}
	return "http://localhost:8080" // default for dev (adjust for prod)
}

func ClaimFlowTagHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	db := database.DB

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var flowTag models.FlowTag
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("claim_token = ?", req.Token).First(&flowTag).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusNotFound, "Invalid claim token")
		return
	}

	if flowTag.Status != "Pending" {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "This FlowTag has already been claimed or cancelled")
		return
	}

	if time.Now().After(flowTag.ExpiresAt) {
		tx.Rollback()
		response.Error(w, http.StatusBadRequest, "This FlowTag has expired")
		return
	}

	var wallet models.Wallet
	currency := flowTag.Currency
	if currency == "" {
		currency = "NGN"
	}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", userID, currency).First(&wallet).Error; err != nil {
		// Create wallet if missing for this currency
		wallet = models.Wallet{UserID: userID, Currency: currency, Balance: 0}
		if err := tx.Create(&wallet).Error; err != nil {
			tx.Rollback()
			response.Error(w, http.StatusInternalServerError, "Failed to create wallet")
			return
		}
	}

	wallet.Balance += flowTag.Amount
	if err := tx.Save(&wallet).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update wallet")
		return
	}

	flowTag.Status = "Claimed"
	if err := tx.Save(&flowTag).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to update FlowTag status")
		return
	}

	txn := models.Transaction{
		UserID:       userID,
		Type:         "FLOWTAG_CLAIM",
		Currency:     currency,
		Amount:       flowTag.Amount,
		Status:       "COMPLETED",
		Reference:    flowTag.Reference,
		BalanceAfter: wallet.Balance,
	}

	if err := tx.Create(&txn).Error; err != nil {
		tx.Rollback()
		response.Error(w, http.StatusInternalServerError, "Failed to log transaction")
		return
	}

	tx.Commit()

	var claimer models.User
	if err := database.DB.Select("email").First(&claimer, userID).Error; err == nil {
		var sender models.User
		if err := database.DB.Select("email, first_name").First(&sender, flowTag.SenderID).Error; err == nil {
			_ = email.SendFlowTagPaymentCompleted(
				sender.Email, sender.FirstName, claimer.Email, flowTag.Reference,
				flowTag.Amount, currency, time.Now(),
			)
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Funds claimed successfully!",
		"amount":  flowTag.Amount,
	})
}

func GetFlowTagHistoryHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user models.User
	database.DB.Select("email").First(&user, userID)

	var sent []models.FlowTag
	database.DB.Where("sender_id = ?", userID).Order("created_at desc").Limit(50).Find(&sent)

	var received []models.FlowTag
	database.DB.Where("recipient_email = ?", user.Email).Order("created_at desc").Limit(50).Find(&received)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"sent":     sent,
		"received": received,
	})
}

func LookupRecipientHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		query = r.URL.Query().Get("email")
	}
	if query == "" {
		response.Error(w, http.StatusBadRequest, "q or email query param is required")
		return
	}

	user, _, found := resolveRecipientQuery(query)
	if !found || user == nil {
		isEmail := strings.Contains(query, "@") && !strings.HasPrefix(strings.TrimSpace(query), "@")
		display := query
		if isEmail {
			display = strings.ToLower(strings.TrimSpace(query))
		}
		msg := "No Flowwithlit account found. They'll receive an email and can sign up to claim the funds."
		if !isEmail {
			msg = "FlowTag not found. Try another @tag or use an email address."
		}
		response.Success(w, http.StatusOK, map[string]interface{}{
			"found":   false,
			"email":   display,
			"query":   query,
			"is_email": isEmail,
			"message": msg,
		})
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"found":           true,
		"first_name":      user.FirstName,
		"last_name":       user.LastName,
		"email":           user.Email,
		"flowtag_username": user.FlowTagUsername,
		"display":         "@" + user.FlowTagUsername,
	})
}
