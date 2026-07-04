package developer

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

// DispatchWebhook sends a webhook event to a user's registered webhook URL and logs the result.
// Call this whenever a payment event occurs (charge.success, transfer.completed, etc.)
func DispatchWebhook(userID uint, eventType string, payload interface{}) {
	var creds models.ApiCredentials
	if err := database.DB.Where("user_id = ?", userID).First(&creds).Error; err != nil {
		return
	}
	if creds.WebhookURL == "" {
		return
	}

	body, _ := json.Marshal(map[string]interface{}{
		"event":      eventType,
		"data":       payload,
		"created_at": time.Now().UTC().Format(time.RFC3339),
	})

	sig := signPayload(body, creds.WebhookSecret)

	statusCode, respBody := doPost(creds.WebhookURL, body, sig)

	entry := models.OutboundWebhookLog{
		UserID:       userID,
		EventType:    eventType,
		TargetURL:    creds.WebhookURL,
		Payload:      string(body),
		StatusCode:   statusCode,
		ResponseBody: respBody,
		Attempts:     1,
	}
	database.DB.Create(&entry)
}

func signPayload(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func doPost(url string, body []byte, sig string) (int, string) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flowwithlit-Signature", sig)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// GetWebhookLogsHandler returns the last 50 outbound webhook delivery logs for the authenticated user.
func GetWebhookLogsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var logs []models.OutboundWebhookLog
	database.DB.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(50).
		Find(&logs)

	response.Success(w, http.StatusOK, map[string]interface{}{"logs": logs})
}

// RetryWebhookHandler re-sends a previously failed webhook delivery.
func RetryWebhookHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "Invalid log ID")
		return
	}

	var entry models.OutboundWebhookLog
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&entry).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Log not found")
		return
	}

	var creds models.ApiCredentials
	if err := database.DB.Where("user_id = ?", userID).First(&creds).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Credentials not found")
		return
	}

	sig := signPayload([]byte(entry.Payload), creds.WebhookSecret)
	statusCode, respBody := doPost(entry.TargetURL, []byte(entry.Payload), sig)

	database.DB.Model(&entry).Updates(map[string]interface{}{
		"status_code":   statusCode,
		"response_body": respBody,
		"attempts":      entry.Attempts + 1,
	})

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":     "Webhook retried",
		"status_code": statusCode,
	})
}
