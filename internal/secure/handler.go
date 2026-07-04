package secure

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

func genKey() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "ESC_" + hex.EncodeToString(b)[:6]
}

func frontendBaseURL() string {
	if u := os.Getenv("FRONTEND_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost/flowwithlit/app"
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	return len(parts) == 2 && len(parts[0]) > 0 && strings.Contains(parts[1], ".")
}

// CreateSecureTransferHandler — sender creates an escrow transfer to a recipient email
func CreateSecureTransferHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		RecipientEmail string  `json:"recipient_email"`
		Amount         float64 `json:"amount"`
		Currency       string  `json:"currency"`
		Note           string  `json:"note"`
		PIN            string  `json:"pin"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	req.RecipientEmail = normalizeEmail(req.RecipientEmail)
	if !isValidEmail(req.RecipientEmail) {
		response.Error(w, http.StatusBadRequest, "A valid recipient email is required")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Amount must be greater than zero")
		return
	}
	if req.PIN == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	var sender models.User
	if err := database.DB.First(&sender, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if normalizeEmail(sender.Email) == req.RecipientEmail {
		response.Error(w, http.StatusBadRequest, "You cannot send an E-Transfer to yourself")
		return
	}
	if sender.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your Transaction PIN first")
		return
	}
	dummy := models.User{Password: sender.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	var recipientUser models.User
	var recipientID *uint
	recipientFound := database.DB.Select("id, first_name, last_name, email").
		Where("email = ?", req.RecipientEmail).First(&recipientUser).Error == nil
	if recipientFound {
		recipientID = &recipientUser.ID
	}

	ref := "ESCT-" + time.Now().Format("20060102150405")
	accessKey := genKey()
	senderName := strings.TrimSpace(sender.FirstName + " " + sender.LastName)
	if senderName == "" {
		senderName = "A Flowwithlit user"
	}

	if err := walletPkg.DebitWallet(userID, req.Amount, 0, req.Currency, "secure_transfer", ref, "E-Transfer to "+req.RecipientEmail+": "+req.Note); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	transfer := models.SecureTransfer{
		SenderID:       userID,
		RecipientEmail: req.RecipientEmail,
		RecipientID:    recipientID,
		Reference:      ref,
		AccessKey:      accessKey,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Note:           req.Note,
		Status:         "pending",
		SenderName:     senderName,
		ExpiresAt:      time.Now().Add(72 * time.Hour),
	}

	if err := database.DB.Create(&transfer).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create secure transfer")
		return
	}

	amountFmt := fmt.Sprintf("%s %.2f", req.Currency, req.Amount)
	if recipientFound {
		database.DB.Create(&models.Notification{
			UserID:  recipientUser.ID,
			Type:    "transaction",
			Title:   "E-Transfer received",
			Message: fmt.Sprintf("%s sent you %s. Ask them for the access key, then claim at FlowTag → Claim E-Transfer.", senderName, amountFmt),
		})
	}

	claimPageURL := frontendBaseURL() + "/claim-transfer.php"
	expiresIn := "72 hours"

	_ = email.SendSecureTransferReceived(
		req.RecipientEmail,
		senderName,
		sender.Email,
		req.Amount,
		req.Currency,
		claimPageURL,
		expiresIn,
	)

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message":         "E-Transfer created. Share the access key with the recipient — they were notified by email.",
		"reference":       ref,
		"access_key":      accessKey,
		"amount":          req.Amount,
		"recipient_email": req.RecipientEmail,
		"notified":        true,
		"expires_at":      transfer.ExpiresAt,
	})
}

// LookupSecureTransferHandler — recipient verifies an access key
func LookupSecureTransferHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessKey string `json:"access_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccessKey == "" {
		response.Error(w, http.StatusBadRequest, "Access key is required")
		return
	}

	var transfer models.SecureTransfer
	if err := database.DB.Where("access_key = ?", strings.ToUpper(strings.TrimSpace(req.AccessKey))).First(&transfer).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Invalid access key. Please check and try again.")
		return
	}

	if transfer.Status != "pending" {
		response.Error(w, http.StatusBadRequest, "This transfer has already been "+transfer.Status+".")
		return
	}

	if time.Now().After(transfer.ExpiresAt) {
		database.DB.Model(&transfer).Update("status", "expired")
		response.Error(w, http.StatusBadRequest, "This transfer has expired.")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"transfer": map[string]interface{}{
			"reference":   transfer.Reference,
			"access_key":  transfer.AccessKey,
			"amount":      transfer.Amount,
			"currency":    transfer.Currency,
			"note":        transfer.Note,
			"sender_name": transfer.SenderName,
			"expires_at":  transfer.ExpiresAt,
		},
	})
}

// ClaimSecureTransferHandler — recipient claims the funds
func ClaimSecureTransferHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		AccessKey     string `json:"access_key"`
		ClaimMethod   string `json:"claim_method"` // "wallet" or "bank"
		BankCode      string `json:"bank_code"`
		AccountNumber string `json:"account_number"`
		AccountName   string `json:"account_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccessKey == "" {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var claimer models.User
	if err := database.DB.Select("id, email").First(&claimer, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	var transfer models.SecureTransfer
	if err := database.DB.Where("access_key = ?", strings.ToUpper(strings.TrimSpace(req.AccessKey))).First(&transfer).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Invalid access key.")
		return
	}

	if transfer.RecipientEmail != "" && normalizeEmail(claimer.Email) != normalizeEmail(transfer.RecipientEmail) {
		response.Error(w, http.StatusForbidden, "This E-Transfer was sent to a different email address. Sign in with the recipient email to claim.")
		return
	}

	if transfer.Status != "pending" {
		response.Error(w, http.StatusBadRequest, "This transfer has already been "+transfer.Status+".")
		return
	}

	if time.Now().After(transfer.ExpiresAt) {
		database.DB.Model(&transfer).Update("status", "expired")
		response.Error(w, http.StatusBadRequest, "This transfer has expired.")
		return
	}

	now := time.Now()
	database.DB.Model(&transfer).Updates(map[string]interface{}{
		"status":     "claimed",
		"claimed_at": now,
	})

	claimRef := "CLM-" + time.Now().Format("20060102150405")

	if req.ClaimMethod == "wallet" {
		txn := models.Transaction{
			UserID:            userID,
			Reference:         claimRef,
			Amount:            transfer.Amount,
			Fee:               0,
			Currency:          transfer.Currency,
			Type:              "secure_transfer_in",
			Status:            "successful",
			Provider:          "escrow",
			ProviderReference: transfer.Reference,
			Description:       "E-Transfer claimed from " + transfer.SenderName,
		}
		database.DB.Create(&txn)

		var wallet models.Wallet
		if err := database.DB.Where("user_id = ? AND currency = ?", userID, transfer.Currency).First(&wallet).Error; err == nil {
			database.DB.Model(&wallet).Update("balance", wallet.Balance+transfer.Amount)
		}
	}

	database.DB.Create(&models.Notification{
		UserID:  transfer.SenderID,
		Type:    "transaction",
		Title:   "E-Transfer claimed",
		Message: fmt.Sprintf("Your E-Transfer of %s %.2f to %s was claimed.", transfer.Currency, transfer.Amount, transfer.RecipientEmail),
	})

	var sender models.User
	if err := database.DB.Select("email, first_name").First(&sender, transfer.SenderID).Error; err == nil && sender.Email != "" {
		_ = email.SendSecureTransferClaimed(
			sender.Email,
			sender.FirstName,
			transfer.RecipientEmail,
			transfer.Reference,
			transfer.Amount,
			transfer.Currency,
			now,
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Transfer claimed successfully",
		"reference": claimRef,
		"amount":    transfer.Amount,
		"currency":  transfer.Currency,
	})
}