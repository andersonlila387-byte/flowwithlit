package admin

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/response"
)

type KYCStats struct {
	PendingReview int64 `json:"pending_review"`
	NeedsInfo     int64 `json:"needs_info"`
	ApprovedToday int64 `json:"approved_today"`
	Rejected7d    int64 `json:"rejected_7d"`
}

type BusinessProfileWithUser struct {
	models.BusinessProfile
	User models.User `json:"user"`
}

type KYCListResponse struct {
	Profiles []BusinessProfileWithUser `json:"profiles"`
	Stats    KYCStats                  `json:"stats"`
	Meta     PaginationMeta            `json:"meta"`
}

func GetKYCApprovalsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	statusFilter := r.URL.Query().Get("status")
	if statusFilter == "" {
		statusFilter = "pending"
	}

	db := database.DB

	// Real stats
	var pendingCount, needsInfoCount, approvedToday, rejected7d int64
	db.Model(&models.BusinessProfile{}).Where("kyc_status = ?", "pending").Count(&pendingCount)
	db.Model(&models.BusinessProfile{}).Where("kyc_status = ?", "needs_info").Count(&needsInfoCount)
	db.Model(&models.BusinessProfile{}).Where("kyc_status = ? AND DATE(updated_at) = CURDATE()", "approved").Count(&approvedToday)
	db.Model(&models.BusinessProfile{}).Where("kyc_status = ? AND updated_at >= NOW() - INTERVAL 7 DAY", "rejected").Count(&rejected7d)

	// Filtered list
	var total int64
	db.Model(&models.BusinessProfile{}).Where("kyc_status = ?", statusFilter).Count(&total)

	var profiles []models.BusinessProfile
	db.Where("kyc_status = ?", statusFilter).Order("created_at DESC").Limit(limit).Offset(offset).Find(&profiles)

	// Batch-fetch users
	var userIDs []uint
	for _, p := range profiles {
		userIDs = append(userIDs, p.UserID)
	}
	userMap := make(map[uint]models.User)
	if len(userIDs) > 0 {
		var users []models.User
		db.Select("id, email, first_name, last_name, kyc_level, is_email_verified, created_at").Where("id IN ?", userIDs).Find(&users)
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	results := make([]BusinessProfileWithUser, 0, len(profiles))
	for _, p := range profiles {
		results = append(results, BusinessProfileWithUser{
			BusinessProfile: p,
			User:            userMap[p.UserID],
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, KYCListResponse{
		Profiles: results,
		Stats: KYCStats{
			PendingReview: pendingCount,
			NeedsInfo:     needsInfoCount,
			ApprovedToday: approvedToday,
			Rejected7d:    rejected7d,
		},
		Meta: PaginationMeta{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  int(total),
			PerPage:     limit,
		},
	})
}

// ── Approve ────────────────────────────────────────────────────────────────────

func ApproveKYCHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID uint `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProfileID == 0 {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	adminIDVal, _ := r.Context().Value(AdminIDKey).(uint)
	var adminUser models.AdminUser
	database.DB.Select("email").Where("id = ?", adminIDVal).First(&adminUser)

	var profile models.BusinessProfile
	if err := database.DB.First(&profile, req.ProfileID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Business profile not found")
		return
	}

	var user models.User
	if err := database.DB.First(&user, profile.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	now := time.Now()
	_ = now

	// Update profile status
	database.DB.Model(&profile).Updates(map[string]interface{}{
		"kyc_status":        "approved",
		"reviewed_by_id":    adminIDVal,
		"reviewed_by_email": adminUser.Email,
	})

	// Upgrade user to Tier 2
	database.DB.Model(&user).Update("kyc_level", 2)

	// Every KYC-approved user gets a default deposit account (own name) and a
	// default USDT receiving address the moment they're approved — not lazily
	// created on first page visit.
	if _, err := wallet.EnsureDefaultDepositAccount(user.ID); err != nil {
		log.Printf("KYC approval: could not create default deposit account for user %d: %v", user.ID, err)
	}
	if _, err := wallet.EnsureDefaultCryptoAddress(user.ID); err != nil {
		log.Printf("KYC approval: could not create default crypto address for user %d: %v", user.ID, err)
	}

	// In-app notification
	database.DB.Create(&models.Notification{
		UserID:  user.ID,
		Type:    "system",
		Title:   "KYC Approved — You're now Tier 2!",
		Message: "Great news! Your KYC documents have been reviewed and approved. You now have full access to all platform features including higher transaction limits.",
	})

	WriteAuditLog(adminIDVal, adminUser.Email, "kyc_approved", "business_profile", strconv.Itoa(int(profile.ID)), profile.BusinessName, r.RemoteAddr)

	if to := strings.TrimSpace(user.Email); to != "" {
		_ = email.SendKYCApproved(to, user.FirstName)
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "KYC approved and user upgraded to Tier 2"})
}

// ── Reject ─────────────────────────────────────────────────────────────────────

func RejectKYCHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID uint   `json:"profile_id"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProfileID == 0 {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Reason == "" {
		req.Reason = "Your submission did not meet our verification requirements."
	}

	adminIDVal, _ := r.Context().Value(AdminIDKey).(uint)
	var adminUser models.AdminUser
	database.DB.Select("email").Where("id = ?", adminIDVal).First(&adminUser)

	var profile models.BusinessProfile
	if err := database.DB.First(&profile, req.ProfileID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Business profile not found")
		return
	}

	var user models.User
	if err := database.DB.First(&user, profile.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	database.DB.Model(&profile).Updates(map[string]interface{}{
		"kyc_status":        "rejected",
		"admin_notes":       req.Reason,
		"reviewed_by_id":    adminIDVal,
		"reviewed_by_email": adminUser.Email,
	})

	// Reset user KYC level so they can re-submit
	database.DB.Model(&user).Update("kyc_level", 0)

	database.DB.Create(&models.Notification{
		UserID:  user.ID,
		Type:    "alert",
		Title:   "KYC Submission Rejected",
		Message: "Unfortunately your KYC submission was rejected. Reason: " + req.Reason + ". Please update your details and resubmit.",
	})

	WriteAuditLog(adminIDVal, adminUser.Email, "kyc_rejected", "business_profile", strconv.Itoa(int(profile.ID)), req.Reason, r.RemoteAddr)

	if to := strings.TrimSpace(user.Email); to != "" {
		_ = email.SendKYCRejected(to, user.FirstName, req.Reason)
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "KYC rejected and user notified"})
}

// ── Request More Info ──────────────────────────────────────────────────────────

func RequestKYCInfoHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID uint   `json:"profile_id"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProfileID == 0 {
		response.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Message == "" {
		req.Message = "We need additional information to complete your KYC review. Please check your profile and resubmit."
	}

	adminIDVal, _ := r.Context().Value(AdminIDKey).(uint)
	var adminUser models.AdminUser
	database.DB.Select("email").Where("id = ?", adminIDVal).First(&adminUser)

	var profile models.BusinessProfile
	if err := database.DB.First(&profile, req.ProfileID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Business profile not found")
		return
	}

	var user models.User
	if err := database.DB.First(&user, profile.UserID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}

	database.DB.Model(&profile).Updates(map[string]interface{}{
		"kyc_status":        "needs_info",
		"admin_notes":       req.Message,
		"reviewed_by_id":    adminIDVal,
		"reviewed_by_email": adminUser.Email,
	})

	database.DB.Create(&models.Notification{
		UserID:  user.ID,
		Type:    "alert",
		Title:   "Action Required: KYC Info Needed",
		Message: req.Message,
	})

	WriteAuditLog(adminIDVal, adminUser.Email, "kyc_info_requested", "business_profile", strconv.Itoa(int(profile.ID)), req.Message, r.RemoteAddr)

	if to := strings.TrimSpace(user.Email); to != "" {
		_ = email.SendKYCNeedsInfo(to, user.FirstName, req.Message)
	}

	response.Success(w, http.StatusOK, map[string]string{"message": "Info request sent and user notified"})
}
