package company

import (
	"encoding/json"
	"net/http"

	"flowwithlit/pkg/response"
)

// PublicHandler returns company contact info (no auth).
func PublicHandler(w http.ResponseWriter, r *http.Request) {
	info := Get()
	response.Success(w, http.StatusOK, info)
}

// AdminGetHandler returns company info for the admin settings UI.
func AdminGetHandler(w http.ResponseWriter, r *http.Request) {
	info := Get()
	response.Success(w, http.StatusOK, info)
}

// AdminUpdateHandler saves company info from the admin settings UI.
func AdminUpdateHandler(w http.ResponseWriter, r *http.Request) {
	var info Info
	if err := json.NewDecoder(r.Body).Decode(&info); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid company info payload")
		return
	}

	if err := Save(info); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save company info")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Company info updated successfully",
		"data":    Get(),
	})
}