package push

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

// Configured reports whether an FCM server key is present.
func Configured() bool {
	return strings.TrimSpace(os.Getenv("FCM_SERVER_KEY")) != ""
}

// RegisterOrUpdate stores a device push token for a user.
func RegisterOrUpdate(userID uint, deviceID, token, platform, appVersion string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("push token is required")
	}
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform != "ios" && platform != "android" {
		return fmt.Errorf("platform must be ios or android")
	}

	now := time.Now()
	var existing models.PushDevice
	err := database.DB.Where("token = ?", token).First(&existing).Error
	if err == nil {
		return database.DB.Model(&existing).Updates(map[string]interface{}{
			"user_id":     userID,
			"device_id":   deviceID,
			"platform":    platform,
			"app_version": appVersion,
			"enabled":     true,
			"last_seen_at": now,
		}).Error
	}

	// One active token row per device_id+user if possible
	_ = database.DB.Where("user_id = ? AND device_id = ? AND device_id <> ''", userID, deviceID).
		Delete(&models.PushDevice{})

	row := models.PushDevice{
		UserID:     userID,
		DeviceID:   deviceID,
		Token:      token,
		Platform:   platform,
		AppVersion: appVersion,
		Enabled:    true,
		LastSeenAt: &now,
	}
	return database.DB.Create(&row).Error
}

// Unregister disables a token for the user.
func Unregister(userID uint, token, deviceID string) error {
	q := database.DB.Model(&models.PushDevice{}).Where("user_id = ?", userID)
	if token != "" {
		q = q.Where("token = ?", token)
	} else if deviceID != "" {
		q = q.Where("device_id = ?", deviceID)
	} else {
		return fmt.Errorf("token or device_id required")
	}
	return q.Update("enabled", false).Error
}

// SendToUser pushes a notification to all enabled devices for the user.
// Soft mode: if FCM_SERVER_KEY is not set, logs and returns nil (in-app notifications still work).
func SendToUser(userID uint, title, body string, data map[string]string) error {
	var devices []models.PushDevice
	if err := database.DB.Where("user_id = ? AND enabled = ?", userID, true).Find(&devices).Error; err != nil {
		return err
	}
	if len(devices) == 0 {
		return nil
	}
	if !Configured() {
		log.Printf("[Push] FCM_SERVER_KEY not set — skipped push to user %d (%d devices). Title=%q", userID, len(devices), title)
		return nil
	}

	key := strings.TrimSpace(os.Getenv("FCM_SERVER_KEY"))
	for _, d := range devices {
		if err := sendFCM(key, d.Token, title, body, data); err != nil {
			log.Printf("[Push] FCM send failed user=%d platform=%s: %v", userID, d.Platform, err)
			// Drop invalid tokens
			if strings.Contains(err.Error(), "NotRegistered") || strings.Contains(err.Error(), "InvalidRegistration") {
				database.DB.Model(&d).Update("enabled", false)
			}
		}
	}
	return nil
}

func sendFCM(serverKey, deviceToken, title, body string, data map[string]string) error {
	payload := map[string]interface{}{
		"to": deviceToken,
		"notification": map[string]string{
			"title": title,
			"body":  body,
			"sound": "default",
		},
		"priority": "high",
	}
	if data != nil {
		payload["data"] = data
	}
	raw, _ := json.Marshal(payload)

	req, err := http.NewRequest(http.MethodPost, "https://fcm.googleapis.com/fcm/send", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "key="+serverKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result struct {
		Success int `json:"success"`
		Failure int `json:"failure"`
		Results []struct {
			Error string `json:"error"`
		} `json:"results"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result.Failure > 0 && len(result.Results) > 0 && result.Results[0].Error != "" {
		return fmt.Errorf("%s", result.Results[0].Error)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("fcm http %d", resp.StatusCode)
	}
	return nil
}
