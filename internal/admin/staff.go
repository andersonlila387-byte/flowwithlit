package admin

import (
	"encoding/json"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"net/http"
)

func GetStaffHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var staff []models.AdminUser
	database.DB.Select("id, email, role, is_active, last_login, created_at").Order("created_at desc").Find(&staff)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"staff": staff},
	})
}

func UpdateStaffHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	callerID := r.Context().Value(AdminIDKey).(uint)

	var req struct {
		ID     uint   `json:"id"`
		Action string `json:"action"` // deactivate | activate | change_role
		Role   string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 || req.Action == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "id and action are required"})
		return
	}

	if req.ID == callerID {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Cannot modify your own account"})
		return
	}

	var staff models.AdminUser
	if err := database.DB.First(&staff, req.ID).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Staff member not found"})
		return
	}

	switch req.Action {
	case "deactivate":
		database.DB.Model(&staff).Update("is_active", false)
	case "activate":
		database.DB.Model(&staff).Update("is_active", true)
	case "change_role":
		if req.Role == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "role is required for change_role"})
			return
		}
		database.DB.Model(&staff).Update("role", req.Role)
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Invalid action"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "Staff member updated successfully"})
}
