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

type UserWithBalance struct {
	models.User
	Balance float64 `json:"balance"`
}

type UserListResponse struct {
	Users []UserWithBalance `json:"users"`
	Stats UserStats         `json:"stats"`
	Meta  PaginationMeta    `json:"meta"`
}

type UserStats struct {
	TotalUsers   int64 `json:"total_users"`
	Active30d    int64 `json:"active_30d"`
	FlaggedUsers int64 `json:"flagged_users"`
}

type PaginationMeta struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
	TotalItems  int `json:"total_items"`
	PerPage     int `json:"per_page"`
}

func GetUsersHandler(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}
	limit := 50
	offset := (page - 1) * limit

	var total int64
	var active30d int64
	var flagged int64

	db := database.DB

	// Get Stats
	db.Model(&models.User{}).Count(&total)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	db.Model(&models.User{}).Where("updated_at > ?", thirtyDaysAgo).Count(&active30d)
	// We don't have a flagged column yet, mock it
	flagged = 0

	var users []models.User
	db.Order("created_at desc").Limit(limit).Offset(offset).Find(&users)

	// Fetch balances manually or via join. For simplicity here, query balances in one go if possible
	var userIDs []uint
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}

	var wallets []models.Wallet
	if len(userIDs) > 0 {
		db.Where("user_id IN ?", userIDs).Find(&wallets)
	}
	
	walletMap := make(map[uint]float64)
	for _, w := range wallets {
		// Just take the first balance if multiple (or default currency)
		walletMap[w.UserID] += w.Balance
	}

	var results []UserWithBalance
	// Need to initialize explicitly to return [] instead of null in JSON if empty
	results = make([]UserWithBalance, 0)
	for _, u := range users {
		results = append(results, UserWithBalance{
			User:    u,
			Balance: walletMap[u.ID],
		})
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))

	resp := UserListResponse{
		Users: results,
		Stats: UserStats{
			TotalUsers:   total,
			Active30d:    active30d,
			FlaggedUsers: flagged,
		},
		Meta: PaginationMeta{
			CurrentPage: page,
			TotalPages:  totalPages,
			TotalItems:  int(total),
			PerPage:     limit,
		},
	}

	response.Success(w, http.StatusOK, resp)
}
