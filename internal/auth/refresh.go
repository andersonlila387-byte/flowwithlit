package auth

import (
	"net/http"

	"flowwithlit/pkg/response"
)

func RefreshHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement refresh token validation and new access token generation
	response.Success(w, http.StatusOK, map[string]string{"message": "Token refreshed"})
}
