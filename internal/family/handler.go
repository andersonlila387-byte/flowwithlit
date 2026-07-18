package family

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	walletPkg "flowwithlit/internal/wallet"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

// CreateJuniorRequest — parent creates a child account IN THE CHILD'S NAME.
// Parent must already be KYC-capable (adult). Child does not do full KYC alone.
type CreateJuniorRequest struct {
	FirstName         string  `json:"first_name"`
	LastName          string  `json:"last_name"`
	DateOfBirth       string  `json:"date_of_birth"` // YYYY-MM-DD
	Email             string  `json:"email"`         // optional unique login email for junior
	Phone             string  `json:"phone"`
	Password          string  `json:"password"` // junior login password (parent sets/shares)
	DailySpendLimit   float64 `json:"daily_spend_limit"`
	MonthlySpendLimit float64 `json:"monthly_spend_limit"`
	AllowAirtimeData  *bool   `json:"allow_airtime_data"`
	PIN               string  `json:"pin"` // parent transaction PIN to authorize creation
}

// ListJuniorsHandler — GET /user/family/juniors
func ListJuniorsHandler(w http.ResponseWriter, r *http.Request) {
	parentID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var kids []models.User
	database.DB.Where("parent_user_id = ? AND account_type = ?", parentID, "JUNIOR").
		Order("created_at desc").Find(&kids)

	type row struct {
		ID                uint       `json:"id"`
		FirstName         string     `json:"first_name"`
		LastName          string     `json:"last_name"`
		Email             string     `json:"email"`
		Phone             string     `json:"phone"`
		DateOfBirth       *time.Time `json:"date_of_birth"`
		DailySpendLimit   float64    `json:"daily_spend_limit"`
		MonthlySpendLimit float64    `json:"monthly_spend_limit"`
		JuniorFrozen      bool       `json:"junior_frozen"`
		AllowAirtimeData  bool       `json:"allow_airtime_data"`
		Balances          interface{} `json:"balances,omitempty"`
		CreatedAt         time.Time  `json:"created_at"`
	}
	out := make([]row, 0, len(kids))
	for _, k := range kids {
		var wallets []models.Wallet
		database.DB.Where("user_id = ?", k.ID).Find(&wallets)
		bals := map[string]float64{}
		for _, wlt := range wallets {
			bals[wlt.Currency] = wlt.Balance
		}
		out = append(out, row{
			ID: k.ID, FirstName: k.FirstName, LastName: k.LastName,
			Email: k.Email, Phone: k.Phone, DateOfBirth: k.DateOfBirth,
			DailySpendLimit: k.DailySpendLimit, MonthlySpendLimit: k.MonthlySpendLimit,
			JuniorFrozen: k.JuniorFrozen, AllowAirtimeData: k.AllowAirtimeData,
			Balances: bals, CreatedAt: k.CreatedAt,
		})
	}
	response.Success(w, http.StatusOK, map[string]interface{}{
		"juniors": out,
		"note":    "Accounts are in the child's name. Parent KYC verifies the family; parent sets spend limits and freeze.",
	})
}

// CreateJuniorHandler — POST /user/family/juniors
func CreateJuniorHandler(w http.ResponseWriter, r *http.Request) {
	parentID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateJuniorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	var parent models.User
	if err := database.DB.First(&parent, parentID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Parent not found")
		return
	}
	if parent.IsJunior() {
		response.Error(w, http.StatusForbidden, "Junior accounts cannot create other juniors")
		return
	}
	if !parent.IsAdultAccount() {
		response.Error(w, http.StatusForbidden, "Only adult accounts can create kids sub-accounts")
		return
	}
	// Parent should be email-verified; prefer KYC for live cash but allow create for product setup
	if !parent.IsEmailVerified {
		response.Error(w, http.StatusForbidden, "Verify your email before creating a kids account")
		return
	}
	if parent.TransactionPin == "" {
		response.Error(w, http.StatusBadRequest, "Set your transaction PIN before creating a kids account")
		return
	}
	dummy := models.User{Password: parent.TransactionPin}
	if err := dummy.CheckPassword(req.PIN); err != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	fn := strings.TrimSpace(req.FirstName)
	ln := strings.TrimSpace(req.LastName)
	if fn == "" || ln == "" {
		response.Error(w, http.StatusBadRequest, "Child first_name and last_name are required (account is in the kid's name)")
		return
	}
	dob, err := time.Parse("2006-01-02", strings.TrimSpace(req.DateOfBirth))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "date_of_birth must be YYYY-MM-DD")
		return
	}
	age := ageYears(dob)
	if age < 0 || age >= 18 {
		response.Error(w, http.StatusBadRequest, "Kids sub-accounts are for under-18 only. Age 18+ must register a normal account.")
		return
	}
	if age < 6 {
		response.Error(w, http.StatusBadRequest, "Minimum age for a kids wallet is 6 years (product policy)")
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" {
		// synthetic unique email so DB unique index works; junior can login with it + password
		email = fmt.Sprintf("junior.%d.%d@flowwithlit.kids", parentID, time.Now().UnixNano()%1e9)
	}
	pass := strings.TrimSpace(req.Password)
	if len(pass) < 6 {
		pass = randomPassword(10)
	}

	daily := req.DailySpendLimit
	if daily <= 0 {
		daily = 5000
	}
	monthly := req.MonthlySpendLimit
	if monthly <= 0 {
		monthly = 50000
	}
	allowAD := true
	if req.AllowAirtimeData != nil {
		allowAD = *req.AllowAirtimeData
	}

	kid := models.User{
		Email:             email,
		FirstName:         fn,
		LastName:          ln,
		Phone:             strings.TrimSpace(req.Phone),
		AccountType:       "JUNIOR",
		DateOfBirth:       &dob,
		ParentUserID:      &parentID,
		DailySpendLimit:   daily,
		MonthlySpendLimit: monthly,
		AllowAirtimeData:  allowAD,
		IsEmailVerified:   true, // parent-verified family product; not independent adult KYC
		KYCLevel:          0,
	}
	if err := kid.HashPassword(pass); err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to set password")
		return
	}
	username, err := database.GenerateFlowTagUsername(fn)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to generate username")
		return
	}
	kid.FlowTagUsername = username

	if err := database.DB.Create(&kid).Error; err != nil {
		response.Error(w, http.StatusConflict, "Could not create kids account (email may already exist)")
		return
	}
	// Ensure NGN wallet exists
	_, _ = walletPkg.EnsureWallet(database.DB, kid.ID, "NGN")

	activity.Success("family", "junior_created",
		fmt.Sprintf("Junior %s %s created by parent %d", fn, ln, parentID),
		&parentID, fmt.Sprintf("junior-%d", kid.ID), r.RemoteAddr)

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message": "Kids account created in the child's name. You control limits and freeze.",
		"junior": map[string]interface{}{
			"id":                  kid.ID,
			"first_name":          kid.FirstName,
			"last_name":           kid.LastName,
			"email":               kid.Email,
			"date_of_birth":       kid.DateOfBirth,
			"daily_spend_limit":   kid.DailySpendLimit,
			"monthly_spend_limit": kid.MonthlySpendLimit,
			"allow_airtime_data":  kid.AllowAirtimeData,
			"flowtag_username":    kid.FlowTagUsername,
		},
		// Return password only at creation so parent can share securely with child login
		"temporary_password": pass,
		"compliance_note":    "Parent KYC/identity covers the family relationship. Child is under 18; bank cash-out is blocked; spend limits apply.",
	})
}

// UpdateJuniorHandler — PUT /user/family/juniors/{id}
func UpdateJuniorHandler(w http.ResponseWriter, r *http.Request) {
	parentID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	jid, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid junior id")
		return
	}

	var kid models.User
	if err := database.DB.Where("id = ? AND parent_user_id = ? AND account_type = ?", jid, parentID, "JUNIOR").
		First(&kid).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Kids account not found")
		return
	}

	var req struct {
		DailySpendLimit   *float64 `json:"daily_spend_limit"`
		MonthlySpendLimit *float64 `json:"monthly_spend_limit"`
		JuniorFrozen      *bool    `json:"junior_frozen"`
		AllowAirtimeData  *bool    `json:"allow_airtime_data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	updates := map[string]interface{}{}
	if req.DailySpendLimit != nil {
		updates["daily_spend_limit"] = *req.DailySpendLimit
	}
	if req.MonthlySpendLimit != nil {
		updates["monthly_spend_limit"] = *req.MonthlySpendLimit
	}
	if req.JuniorFrozen != nil {
		updates["junior_frozen"] = *req.JuniorFrozen
	}
	if req.AllowAirtimeData != nil {
		updates["allow_airtime_data"] = *req.AllowAirtimeData
	}
	if len(updates) == 0 {
		response.Error(w, http.StatusBadRequest, "No fields to update")
		return
	}
	database.DB.Model(&kid).Updates(updates)
	activity.Info("family", "junior_updated", "Parent updated junior controls", &parentID, fmt.Sprintf("junior-%d", kid.ID), r.RemoteAddr)

	database.DB.First(&kid, kid.ID)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Kids account controls updated",
		"junior": map[string]interface{}{
			"id": kid.ID, "first_name": kid.FirstName, "last_name": kid.LastName,
			"daily_spend_limit": kid.DailySpendLimit, "monthly_spend_limit": kid.MonthlySpendLimit,
			"junior_frozen": kid.JuniorFrozen, "allow_airtime_data": kid.AllowAirtimeData,
		},
	})
}

// FundJuniorHandler — POST /user/family/juniors/{id}/fund
// Parent moves money from parent wallet to child wallet (allowance).
func FundJuniorHandler(w http.ResponseWriter, r *http.Request) {
	parentID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	idStr := chi.URLParam(r, "id")
	jid, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid junior id")
		return
	}

	var req struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
		PIN      string  `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}
	if req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "amount must be > 0")
		return
	}
	cur := strings.ToUpper(strings.TrimSpace(req.Currency))
	if cur == "" {
		cur = "NGN"
	}

	var parent models.User
	database.DB.First(&parent, parentID)
	dummy := models.User{Password: parent.TransactionPin}
	if parent.TransactionPin == "" || dummy.CheckPassword(req.PIN) != nil {
		response.Error(w, http.StatusUnauthorized, "Incorrect Transaction PIN")
		return
	}

	var kid models.User
	if err := database.DB.Where("id = ? AND parent_user_id = ? AND account_type = ?", jid, parentID, "JUNIOR").
		First(&kid).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Kids account not found")
		return
	}

	ref := fmt.Sprintf("ALLOW-%d-%d", kid.ID, time.Now().UnixNano()%1e12)
	if err := walletPkg.DebitWallet(parentID, req.Amount, 0, cur, "family", ref, "Allowance to "+kid.FirstName); err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := walletPkg.FundWallet(kid.ID, req.Amount, cur, "family", ref+"-IN", "Allowance from parent"); err != nil {
		// best-effort refund parent
		_ = walletPkg.FundWallet(parentID, req.Amount, cur, "refund", ref+"-RFND", "Refund failed allowance")
		response.Error(w, http.StatusInternalServerError, "Failed to credit kids wallet")
		return
	}
	activity.Success("family", "allowance_sent", "Allowance to junior", &parentID, ref, r.RemoteAddr)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Allowance sent to kids account",
		"reference": ref,
		"amount":    req.Amount,
		"currency":  cur,
		"junior_id": kid.ID,
	})
}

func ageYears(dob time.Time) int {
	now := time.Now()
	years := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		years--
	}
	return years
}

func randomPassword(n int) string {
	const letters = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		v, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[v.Int64()]
	}
	return string(b)
}
