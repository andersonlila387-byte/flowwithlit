package admin

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/response"
)

type MerchantDetail struct {
	ID              uint      `json:"id"`
	Email           string    `json:"email"`
	FirstName       string    `json:"first_name"`
	LastName        string    `json:"last_name"`
	KYCLevel        int       `json:"kyc_level"`
	IsEmailVerified bool      `json:"is_email_verified"`
	CreatedAt       time.Time `json:"created_at"`
	BusinessName    string    `json:"business_name"`
	CountryCode     string    `json:"country_code"`
	Industry        string    `json:"industry"`
	BalanceNGN      float64   `json:"balance_ngn"`
}

type MerchantStats struct {
	TotalMerchants int64 `json:"total_merchants"`
	Active30d      int64 `json:"active_30d"`
	PendingKYC     int64 `json:"pending_kyc"`
}

type MerchantListResponse struct {
	Merchants []MerchantDetail `json:"merchants"`
	Stats     MerchantStats    `json:"stats"`
	Meta      PaginationMeta   `json:"meta"`
}

func GetMerchantsHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 25
	offset := (page - 1) * limit

	db := database.DB

	var total int64
	db.Model(&models.User{}).Where("account_type = ?", "MERCHANT").Count(&total)

	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	var active30d int64
	db.Model(&models.User{}).Where("account_type = ? AND updated_at > ?", "MERCHANT", thirtyDaysAgo).Count(&active30d)

	var pendingKYC int64
	db.Model(&models.User{}).Where("account_type = ? AND kyc_level = 0", "MERCHANT").Count(&pendingKYC)

	var users []models.User
	db.Where("account_type = ?", "MERCHANT").Order("created_at desc").Limit(limit).Offset(offset).Find(&users)

	var userIDs []uint
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}

	profileMap := make(map[uint]models.BusinessProfile)
	walletMap := make(map[uint]float64)

	if len(userIDs) > 0 {
		var profiles []models.BusinessProfile
		db.Where("user_id IN ?", userIDs).Find(&profiles)
		for _, p := range profiles {
			profileMap[p.UserID] = p
		}

		var wallets []models.Wallet
		db.Where("user_id IN ? AND currency = ?", userIDs, "NGN").Find(&wallets)
		for _, w := range wallets {
			walletMap[w.UserID] = w.Balance
		}
	}

	merchants := make([]MerchantDetail, 0)
	for _, u := range users {
		p := profileMap[u.ID]
		merchants = append(merchants, MerchantDetail{
			ID:              u.ID,
			Email:           u.Email,
			FirstName:       u.FirstName,
			LastName:        u.LastName,
			KYCLevel:        u.KYCLevel,
			IsEmailVerified: u.IsEmailVerified,
			CreatedAt:       u.CreatedAt,
			BusinessName:    p.BusinessName,
			CountryCode:     p.CountryCode,
			Industry:        p.Industry,
			BalanceNGN:      walletMap[u.ID],
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	response.Success(w, http.StatusOK, MerchantListResponse{
		Merchants: merchants,
		Stats: MerchantStats{
			TotalMerchants: total,
			Active30d:      active30d,
			PendingKYC:     pendingKYC,
		},
		Meta: PaginationMeta{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  int(total),
			PerPage:     limit,
		},
	})
}
