package response

import (
	"encoding/json"
	"net/http"
)

// Success sends a standardized success JSON response
func Success(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"data":   data,
	})
}

// Error sends a standardized error JSON response
func Error(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  false,
		"message": message,
	})
}

// ParseJSON reads the JSON body into the provided struct pointer and handles errors
func ParseJSON(r *http.Request, v interface{}) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return err
	}
	return nil
}
