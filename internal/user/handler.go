package user

import (
	"net/http"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// GetMeHandler returns the currently authenticated user's profile
func GetMeHandler(w http.ResponseWriter, r *http.Request) {
	// Retrieve userID from context (injected by the RequireAuth middleware)
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	hasTransactionPin := user.TransactionPin != ""

	// Double check sensitive fields are cleared
	user.Password = ""
	user.TransactionPin = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"user":                user,
		"has_transaction_pin": hasTransactionPin,
	})
}

type UpdatePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// UpdatePasswordHandler updates the user's password
func UpdatePasswordHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req UpdatePasswordRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if len(req.NewPassword) < 8 {
		response.Error(w, http.StatusBadRequest, "New password must be at least 8 characters")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	if err := user.CheckPassword(req.CurrentPassword); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect current password")
		return
	}

	if err := user.HashPassword(req.NewPassword); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to hash new password")
		return
	}

	if err := database.DB.Save(&user).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update password")
		return
	}

	_ = email.SendPasswordChanged(user.Email, user.FirstName, time.Now())

	response.Success(w, http.StatusOK, map[string]string{"message": "Password updated successfully"})
}

type SetupPINRequest struct {
	PIN string `json:"pin"`
}

// SetupPINHandler sets the user's transaction PIN
func SetupPINHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req SetupPINRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if len(req.PIN) != 4 {
		response.Error(w, http.StatusBadRequest, "PIN must be exactly 4 digits")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Just reuse the password hashing algorithm for the PIN
	if err := user.HashPassword(req.PIN); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to hash PIN")
		return
	}
	
	user.TransactionPin = user.Password // Store it in the TransactionPin field
	user.Password = "" // clear it from struct so we don't accidentally save it to the wrong field

	if err := database.DB.Model(&user).Update("transaction_pin", user.TransactionPin).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update PIN")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "PIN setup successfully"})
}

// VerifyPINHandler checks if the provided PIN is correct
func VerifyPINHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req SetupPINRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	// Create a dummy user struct to use the CheckPassword method
	dummy := models.User{Password: user.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect PIN")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "PIN verified successfully"})
}


type UpdateProfileRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

func UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req UpdateProfileRequest
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	updates := map[string]interface{}{}
	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}
	if req.Phone != "" {
		updates["phone"] = req.Phone
	}

	if len(updates) == 0 {
		response.Error(w, http.StatusBadRequest, "No fields to update")
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update profile")
		return
	}

	var user models.User
	database.DB.First(&user, userID)
	user.Password = ""

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Profile updated successfully",
		"user":    user,
	})
}

// UpdateProfileImageHandler updates the user's profile image URL
func UpdateProfileImageHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusInternalServerError, "User ID not found in context")
		return
	}

	var req struct {
		ProfileImageURL string `json:"profile_image_url"`
	}
	if err := response.ParseJSON(r, &req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.ProfileImageURL == "" {
		response.Error(w, http.StatusBadRequest, "Profile image URL is required")
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("profile_image", req.ProfileImageURL).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update profile image")
		return
	}

	response.Success(w, http.StatusOK, map[string]string{
		"message": "Profile image updated successfully",
	})
}
