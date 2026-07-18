package user

import (
	"encoding/json"
	"net/http"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/push"
	"flowwithlit/pkg/response"
)

// RegisterPushHandler — POST /user/push/register
// Saves FCM/APNs device token so the backend can send push notifications.
func RegisterPushHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		Token      string `json:"token"`
		Platform   string `json:"platform"` // ios | android
		DeviceID   string `json:"device_id"`
		AppVersion string `json:"app_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	deviceID := clientDeviceID(r, req.DeviceID)
	if err := push.RegisterOrUpdate(userID, deviceID, req.Token, req.Platform, req.AppVersion); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":        "Push token registered",
		"push_configured": push.Configured(),
		"note":           "If push_configured is false, tokens are stored but delivery needs FCM_SERVER_KEY on the API server",
	})
}

// UnregisterPushHandler — POST /user/push/unregister
func UnregisterPushHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var req struct {
		Token    string `json:"token"`
		DeviceID string `json:"device_id"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	deviceID := clientDeviceID(r, req.DeviceID)
	if err := push.Unregister(userID, strings.TrimSpace(req.Token), deviceID); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(w, http.StatusOK, map[string]string{"message": "Push token unregistered"})
}

// PushStatusHandler — GET /user/push/status
func PushStatusHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var count int64
	database.DB.Model(&models.PushDevice{}).Where("user_id = ? AND enabled = ?", userID, true).Count(&count)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"registered_devices": count,
		"push_configured":    push.Configured(),
	})
}
