package admin

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

type TicketStats struct {
	Open       int64 `json:"open"`
	InProgress int64 `json:"in_progress"`
	Resolved   int64 `json:"resolved"`
	Closed     int64 `json:"closed"`
}

type TicketListResponse struct {
	Tickets []models.SupportTicket `json:"tickets"`
	Stats   TicketStats            `json:"stats"`
	Meta    PaginationMeta         `json:"meta"`
}

func GetTicketsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 25
	offset := (page - 1) * limit

	db := database.DB

	var total, open, inProgress, resolved, closed int64
	db.Model(&models.SupportTicket{}).Count(&total)
	db.Model(&models.SupportTicket{}).Where("status = ?", "open").Count(&open)
	db.Model(&models.SupportTicket{}).Where("status = ?", "in_progress").Count(&inProgress)
	db.Model(&models.SupportTicket{}).Where("status = ?", "resolved").Count(&resolved)
	db.Model(&models.SupportTicket{}).Where("status = ?", "closed").Count(&closed)

	var tickets []models.SupportTicket
	db.Order("created_at desc").Limit(limit).Offset(offset).Find(&tickets)
	if tickets == nil {
		tickets = make([]models.SupportTicket, 0)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, TicketListResponse{
		Tickets: tickets,
		Stats:   TicketStats{Open: open, InProgress: inProgress, Resolved: resolved, Closed: closed},
		Meta:    PaginationMeta{CurrentPage: page, TotalPages: totalPages, TotalItems: int(total), PerPage: limit},
	})
}

type ReplyTicketRequest struct {
	Status     string `json:"status"`
	AdminReply string `json:"admin_reply"`
}

func ReplyTicketHandler(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		response.Error(w, http.StatusBadRequest, "Invalid ticket ID")
		return
	}

	var req ReplyTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var ticket models.SupportTicket
	if err := database.DB.First(&ticket, id).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Ticket not found")
		return
	}

	if req.AdminReply != "" {
		ticket.AdminReply = req.AdminReply
	}
	allowed := map[string]bool{"open": true, "in_progress": true, "resolved": true, "closed": true}
	if req.Status != "" && allowed[req.Status] {
		ticket.Status = req.Status
	}

	if err := database.DB.Save(&ticket).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update ticket")
		return
	}

	if reply := strings.TrimSpace(req.AdminReply); reply != "" {
		to := strings.TrimSpace(ticket.UserEmail)
		firstName := ""
		if ticket.UserID > 0 {
			var u models.User
			if err := database.DB.Select("email, first_name").First(&u, ticket.UserID).Error; err == nil {
				if to == "" {
					to = strings.TrimSpace(u.Email)
				}
				firstName = u.FirstName
			}
		}
		if to != "" {
			_ = email.SendTicketReply(to, firstName, ticket.Reference, ticket.Subject, reply, ticket.Status)
		}
	}

	response.Success(w, http.StatusOK, ticket)
}
