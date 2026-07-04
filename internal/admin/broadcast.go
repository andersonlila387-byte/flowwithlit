package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

var allowedBroadcastTypes = map[string]bool{
	"newsletter": true, "promo": true, "maintenance": true,
	"security_alert": true, "update": true, "other": true,
}

var allowedBroadcastTargets = map[string]bool{
	"all": true, "merchants": true, "tier1": true, "tier2": true,
	"tier0": true, "unverified": true, "verified": true,
}

var allowedBroadcastChannels = map[string]bool{
	"in_app": true, "email": true, "both": true,
}

func GetBroadcastsHandler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	perPage := 20
	offset := (page - 1) * perPage

	var total int64
	database.DB.Model(&models.BroadcastMessage{}).Count(&total)

	var broadcasts []models.BroadcastMessage
	database.DB.Order("created_at DESC").Limit(perPage).Offset(offset).Find(&broadcasts)

	// Stats
	var totalRecipients int64
	database.DB.Model(&models.BroadcastMessage{}).Select("COALESCE(SUM(recipient_count), 0)").Scan(&totalRecipients)

	var sentThisWeek int64
	database.DB.Model(&models.BroadcastMessage{}).
		Where("created_at >= NOW() - INTERVAL 7 DAY").Count(&sentThisWeek)

	var lastBroadcast models.BroadcastMessage
	database.DB.Order("created_at DESC").First(&lastBroadcast)

	totalPages := int(total)/perPage + 1
	if int(total)%perPage == 0 && total > 0 {
		totalPages = int(total) / perPage
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"broadcasts": broadcasts,
		"stats": map[string]interface{}{
			"total_broadcasts":  total,
			"sent_this_week":    sentThisWeek,
			"total_recipients":  totalRecipients,
			"last_broadcast_at": lastBroadcast.CreatedAt,
		},
		"meta": map[string]interface{}{
			"current_page": page,
			"total_pages":  totalPages,
			"total_items":  total,
			"per_page":     perPage,
		},
	})
}

type SendBroadcastRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Target  string `json:"target"`
	Channel string `json:"channel"`
}

func SendBroadcastHandler(w http.ResponseWriter, r *http.Request) {
	var req SendBroadcastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Title == "" || req.Message == "" {
		response.Error(w, http.StatusBadRequest, "Title and message are required")
		return
	}
	if !allowedBroadcastTypes[req.Type] {
		req.Type = "newsletter"
	}
	if !allowedBroadcastTargets[req.Target] {
		req.Target = "all"
	}
	if req.Channel == "" {
		req.Channel = "in_app"
	}
	if !allowedBroadcastChannels[req.Channel] {
		req.Channel = "in_app"
	}

	adminIDVal, _ := r.Context().Value(AdminIDKey).(uint)
	var adminUser models.AdminUser
	database.DB.Select("email").Where("id = ?", adminIDVal).First(&adminUser)
	adminEmail := adminUser.Email

	// Resolve target users
	users, err := resolveTargetUsers(req.Target)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to resolve target users")
		return
	}

	// Determine notification type label
	notifType := "system"
	if req.Type == "promo" {
		notifType = "alert"
	}

	// Record broadcast first so we have an ID
	broadcast := models.BroadcastMessage{
		Title:          req.Title,
		Message:        req.Message,
		Type:           req.Type,
		Target:         req.Target,
		Channel:        req.Channel,
		SentByID:       adminIDVal,
		SentByEmail:    adminEmail,
		RecipientCount: len(users),
		Status:         "sent",
		CreatedAt:      time.Now(),
	}
	if err := database.DB.Create(&broadcast).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to record broadcast")
		return
	}

	broadcastID := broadcast.ID

	// Create in-app notifications in background
	if req.Channel == "in_app" || req.Channel == "both" {
		go func(userList []models.User, title, msg, nType, bType string, bID uint) {
			var notifications []models.Notification
			for _, u := range userList {
				idCopy := bID
				notifications = append(notifications, models.Notification{
					UserID:        u.ID,
					Type:          nType,
					Title:         title,
					Message:       msg,
					Source:        "broadcast",
					BroadcastID:   &idCopy,
					BroadcastType: bType,
				})
			}
			if len(notifications) > 0 {
				// Batch insert in chunks of 500
				for i := 0; i < len(notifications); i += 500 {
					end := i + 500
					if end > len(notifications) {
						end = len(notifications)
					}
					chunk := notifications[i:end]
					database.DB.Create(&chunk)
				}
			}
		}(users, req.Title, req.Message, notifType, req.Type, broadcastID)
	}

	emailSent := 0
	if req.Channel == "email" || req.Channel == "both" {
		ctaURL := email.DashboardURL()
		ctaLabel := "Open Dashboard"
		for _, u := range users {
			to := strings.TrimSpace(u.Email)
			if to == "" {
				continue
			}
			if err := email.SendBroadcast(to, req.Title, req.Message, ctaURL, ctaLabel); err == nil {
				emailSent++
			}
		}
	}

	WriteAuditLog(
		adminIDVal, adminEmail,
		"broadcast_sent",
		"broadcast", strconv.Itoa(int(broadcast.ID)),
		req.Target+" | "+req.Type+" | "+strconv.Itoa(len(users))+" recipients",
		r.RemoteAddr,
	)

	msg := "Broadcast sent successfully"
	if req.Channel == "email" {
		msg = "Broadcast emails dispatched"
	} else if req.Channel == "both" {
		msg = "Broadcast sent in-app and via email"
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":         msg,
		"recipient_count": len(users),
		"email_sent":      emailSent,
		"broadcast_id":    broadcast.ID,
		"channel":         req.Channel,
	})
}

func resolveTargetUsers(target string) ([]models.User, error) {
	var users []models.User
	q := database.DB.Select("id, email, first_name, last_name, kyc_level, account_type, is_email_verified")

	switch target {
	case "merchants":
		q = q.Where("account_type = ?", "MERCHANT")
	case "tier1":
		q = q.Where("kyc_level = ?", 1)
	case "tier2":
		q = q.Where("kyc_level = ?", 2)
	case "tier0":
		q = q.Where("kyc_level = ?", 0)
	case "unverified":
		q = q.Where("is_email_verified = ?", false)
	case "verified":
		q = q.Where("is_email_verified = ?", true)
	// "all" — no extra filter
	}

	if err := q.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}
