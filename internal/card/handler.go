package card

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

// GetCardsHandler lists all virtual cards for the user
func GetCardsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var cards []models.VirtualCard
	database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&cards)

	// Mask sensitive data
	for i := range cards {
		cards[i].CardNumber = ""
		cards[i].CVV = ""
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"cards": cards,
	})
}

// CreateCardRequest
type CreateCardRequest struct {
	Type     string  `json:"type"`     // standard, burner
	Currency string  `json:"currency"` // USD, NGN
	Limit    float64 `json:"limit"`
}

// CreateCardHandler issues a new virtual card
func CreateCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	if req.Type == "" {
		req.Type = "standard"
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.Limit == 0 {
		req.Limit = 2000
	}

	// Generate fake card details (in prod: integrate with card issuer)
	last4 := strconv.Itoa(1000 + rand.Intn(8999))
	expMonth := 12
	expYear := time.Now().Year() + 3

	card := models.VirtualCard{
		UserID:      userID,
		Last4:       last4,
		CardNumber:  "4111 1111 1111 " + last4, // test number
		ExpiryMonth: expMonth,
		ExpiryYear:  expYear,
		CVV:         "123", // placeholder
		Type:        req.Type,
		Currency:    req.Currency,
		Balance:     0,
		Status:      "active",
		DailyLimit:  req.Limit,
	}

	if err := database.DB.Create(&card).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create card")
		return
	}

	// Return masked version
	card.CardNumber = ""
	card.CVV = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Virtual card created",
		"card":    card,
	})
}

// FundCardRequest
type FundCardRequest struct {
	Amount float64 `json:"amount"`
	PIN    string  `json:"pin"`
}

// FundCardHandler moves money from main wallet to card
func FundCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	cardIDStr := chi.URLParam(r, "id")
	cardID, _ := strconv.Atoi(cardIDStr)

	var req FundCardRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Invalid amount")
		return
	}

	// Verify PIN
	if req.PIN == "" {
		response.Error(w, http.StatusBadRequest, "Transaction PIN is required")
		return
	}
	var pinUser models.User
	database.DB.First(&pinUser, userID)
	if pinUser.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Please set up your 4-digit Transaction PIN first in Settings")
		return
	}
	dummy := models.User{Password: pinUser.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect PIN")
		return
	}

	// Debit main wallet (NGN or USD based on card)
	var card models.VirtualCard
	if err := database.DB.Where("id = ? AND user_id = ?", cardID, userID).First(&card).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Card not found")
		return
	}

	sourceCurrency := "NGN"
	if card.Currency == "USD" {
		sourceCurrency = "USDT" // or USD wallet
	}

	// Simple debit from source wallet
	var sourceWallet models.Wallet
	if err := database.DB.Where("user_id = ? AND currency = ?", userID, sourceCurrency).First(&sourceWallet).Error; err != nil {
		response.Error(w, http.StatusBadRequest, "Source wallet not found")
		return
	}
	if sourceWallet.Balance < req.Amount {
		response.Error(w, http.StatusBadRequest, "Insufficient funds")
		return
	}

	sourceWallet.Balance -= req.Amount
	database.DB.Save(&sourceWallet)

	card.Balance += req.Amount
	database.DB.Save(&card)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Card funded successfully",
		"card":    card,
	})
}

// FreezeCardHandler toggles freeze status
func FreezeCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	cardIDStr := chi.URLParam(r, "id")
	cardID, _ := strconv.Atoi(cardIDStr)

	var card models.VirtualCard
	if err := database.DB.Where("id = ? AND user_id = ?", cardID, userID).First(&card).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Card not found")
		return
	}

	if card.Status == "frozen" {
		card.Status = "active"
	} else {
		card.Status = "frozen"
	}
	database.DB.Save(&card)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Card status updated",
		"status":  card.Status,
	})
}

// RevealCardHandler returns full details (should require additional auth in prod)
func RevealCardHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	cardIDStr := chi.URLParam(r, "id")
	cardID, _ := strconv.Atoi(cardIDStr)

	var card models.VirtualCard
	if err := database.DB.Where("id = ? AND user_id = ?", cardID, userID).First(&card).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Card not found")
		return
	}

	// In production: require PIN verification here

	response.Success(w, http.StatusOK, map[string]interface{}{
		"card_number":  card.CardNumber,
		"cvv":          card.CVV,
		"expiry_month": card.ExpiryMonth,
		"expiry_year":  card.ExpiryYear,
	})
}
