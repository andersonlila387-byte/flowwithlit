package user

import (
	"encoding/json"
	"net/http"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func GetNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var notifications []models.Notification
	database.DB.Where("user_id = ?", userID).Order("created_at desc").Limit(50).Find(&notifications)

	var unreadCount int64
	database.DB.Model(&models.Notification{}).Where("user_id = ? AND is_read = false", userID).Count(&unreadCount)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"unread_count":  unreadCount,
	})
}

func MarkNotificationsReadHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		IDs []uint `json:"ids"` // empty = mark all read
	}
	json.NewDecoder(r.Body).Decode(&req)

	q := database.DB.Model(&models.Notification{}).Where("user_id = ?", userID)
	if len(req.IDs) > 0 {
		q = q.Where("id IN ?", req.IDs)
	}
	q.Update("is_read", true)

	response.Success(w, http.StatusOK, map[string]string{"message": "Notifications marked as read"})
}

type UpdateNotificationPreferencesRequest struct {
	SmsNotificationsEnabled *bool `json:"sms_notifications_enabled"`
}

func UpdateNotificationPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req UpdateNotificationPreferencesRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.SmsNotificationsEnabled == nil {
		response.Error(w, http.StatusBadRequest, "sms_notifications_enabled is required")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if *req.SmsNotificationsEnabled && user.Phone == "" {
		response.Error(w, http.StatusBadRequest, "Add a phone number to your profile before enabling SMS notifications")
		return
	}

	if err := database.DB.Model(&user).Update("sms_notifications_enabled", *req.SmsNotificationsEnabled).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update notification preferences")
		return
	}

	user.SmsNotificationsEnabled = *req.SmsNotificationsEnabled
	user.Password = ""
	user.TransactionPin = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Notification preferences updated",
		"user":    user,
	})
}

// GetPendingBroadcastModalHandler returns the newest broadcast notification that still needs a one-time popup.
func GetPendingBroadcastModalHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var notif models.Notification
	err := database.DB.Where(
		"user_id = ? AND source = ? AND modal_dismissed = ?",
		userID, "broadcast", false,
	).Order("created_at desc").First(&notif).Error

	if err != nil {
		response.Success(w, http.StatusOK, map[string]interface{}{
			"notification": nil,
		})
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"notification": notif,
	})
}

// DismissBroadcastModalHandler marks a broadcast popup as seen (one-time per user per broadcast).
func DismissBroadcastModalHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		ID uint `json:"id"`
	}
	if err := response.ParseJSON(r, &req); err != nil || req.ID == 0 {
		response.Error(w, http.StatusBadRequest, "Notification id is required")
		return
	}

	result := database.DB.Model(&models.Notification{}).
		Where("user_id = ? AND id = ? AND source = ?", userID, req.ID, "broadcast").
		Updates(map[string]interface{}{
			"modal_dismissed": true,
			"is_read":         true,
		})

	if result.RowsAffected == 0 {
		response.Error(w, http.StatusNotFound, "Broadcast notification not found")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "Broadcast dismissed"})
}
