package admin

import (
	"encoding/json"
	"net/http"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settings"
	"flowwithlit/pkg/response"
)

type UpdateSettingRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func UpdateSettingHandler(w http.ResponseWriter, r *http.Request) {
	var req UpdateSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var setting models.SystemSetting
	// Upsert setting
	err := database.DB.Where("`key` = ?", req.Key).First(&setting).Error
	if err != nil {
		setting = models.SystemSetting{Key: req.Key, Value: req.Value}
	} else {
		setting.Value = req.Value
	}

	if err := database.DB.Save(&setting).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update setting")
		return
	}
	settings.Invalidate()

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Setting updated successfully",
		"key":     req.Key,
		"value":   req.Value,
	})
}

func GetSettingsHandler(w http.ResponseWriter, r *http.Request) {
	var settings []models.SystemSetting
	if err := database.DB.Find(&settings).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve settings")
		return
	}

	// Format output nicely
	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}

	if result["kyc_provider"] == "" {
		result["kyc_provider"] = "mock"
	}
	if result["smile_environment"] == "" {
		result["smile_environment"] = "sandbox"
	}

	response.Success(w, http.StatusOK, result)
}
