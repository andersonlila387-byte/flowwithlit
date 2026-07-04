package developer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

func genKey(prefix string) string {
	b := make([]byte, 20)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func ensureCredentials(userID uint) (*models.ApiCredentials, error) {
	var creds models.ApiCredentials
	err := database.DB.Where("user_id = ?", userID).First(&creds).Error
	if err != nil {
		creds = models.ApiCredentials{
			UserID:        userID,
			PubKeyTest:    genKey("flw_pub_test_"),
			SecKeyTest:    genKey("flw_sec_test_"),
			PubKeyLive:    genKey("flw_pub_live_"),
			SecKeyLive:    genKey("flw_sec_live_"),
			WebhookSecret: genKey("whsec_"),
		}
		err = database.DB.Create(&creds).Error
	}
	return &creds, err
}

func GetCredentialsHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	creds, err := ensureCredentials(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch credentials")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"credentials": map[string]interface{}{
			"pub_key_test":   creds.PubKeyTest,
			"sec_key_test":   creds.SecKeyTest,
			"pub_key_live":   creds.PubKeyLive,
			"sec_key_live":   creds.SecKeyLive,
			"webhook_url":    creds.WebhookURL,
			"callback_url":   creds.CallbackURL,
			"webhook_secret": creds.WebhookSecret,
			"is_live":        creds.IsLive,
		},
	})
}

func RotateKeysHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		Mode string `json:"mode"` // "test" | "live" | "both"
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Mode == "" {
		req.Mode = "both"
	}

	creds, err := ensureCredentials(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch credentials")
		return
	}

	updates := map[string]interface{}{}
	if req.Mode == "test" || req.Mode == "both" {
		updates["pub_key_test"] = genKey("flw_pub_test_")
		updates["sec_key_test"] = genKey("flw_sec_test_")
	}
	if req.Mode == "live" || req.Mode == "both" {
		updates["pub_key_live"] = genKey("flw_pub_live_")
		updates["sec_key_live"] = genKey("flw_sec_live_")
	}

	if err := database.DB.Model(creds).Updates(updates).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to rotate keys")
		return
	}

	database.DB.First(creds, creds.ID)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "API keys rotated successfully",
		"credentials": map[string]interface{}{
			"pub_key_test": creds.PubKeyTest,
			"sec_key_test": creds.SecKeyTest,
			"pub_key_live": creds.PubKeyLive,
			"sec_key_live": creds.SecKeyLive,
		},
	})
}

func UpdateWebhooksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		WebhookURL  string `json:"webhook_url"`
		CallbackURL string `json:"callback_url"`
		IsLive      bool   `json:"is_live"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	creds, err := ensureCredentials(userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to fetch credentials")
		return
	}

	if err := database.DB.Model(creds).Updates(map[string]interface{}{
		"webhook_url":  req.WebhookURL,
		"callback_url": req.CallbackURL,
		"is_live":      req.IsLive,
	}).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update webhook configuration")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Webhook configuration saved successfully",
	})
}
