package user

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func CreateTicketHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Subject  string `json:"subject"`
		Message  string `json:"message"`
		Category string `json:"category"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Subject == "" || req.Message == "" {
		response.Error(w, http.StatusBadRequest, "Subject and message are required")
		return
	}

	if req.Category == "" {
		req.Category = "other"
	}

	var u models.User
	database.DB.Select("email, first_name").First(&u, userID)

	b := make([]byte, 5)
	rand.Read(b)
	ref := "TKT_" + hex.EncodeToString(b)[:8]

	ticket := models.SupportTicket{
		UserID:    userID,
		Reference: ref,
		Subject:   req.Subject,
		Message:   req.Message,
		Category:  req.Category,
		Status:    "open",
		Priority:  "medium",
		UserEmail: u.Email,
	}

	if err := database.DB.Create(&ticket).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create ticket")
		return
	}

	if to := strings.TrimSpace(u.Email); to != "" {
		_ = email.SendTicketCreated(
			to, u.FirstName, ticket.Reference, ticket.Subject,
			ticket.Category, ticket.Priority, ticket.Status,
		)
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message":   "Your support ticket has been submitted. We will respond within 24 hours.",
		"ticket":    ticket,
		"reference": ticket.Reference,
	})
}
