package bills

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settings"
	userPkg "flowwithlit/internal/user"
	walletPkg "flowwithlit/internal/wallet"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

// CategoriesHandler — GET /user/bills/categories
func CategoriesHandler(w http.ResponseWriter, r *http.Request) {
	fw := settings.FlutterwaveClient()
	response.Success(w, http.StatusOK, map[string]interface{}{
		"categories":          Categories(),
		"provider":            "flutterwave",
		"provider_configured": fw.Configured(),
		"mode":                map[bool]string{true: "live_or_ready", false: "mock"}[fw.Configured()],
		"note":                "When Flutterwave keys are set in Admin, purchases route to Flutterwave bills. Until then, mock mode debits wallet and marks success for UI testing (no real airtime sent).",
	})
}

// ProductsHandler — GET /user/bills/products?category=airtime
func ProductsHandler(w http.ResponseWriter, r *http.Request) {
	cat := strings.TrimSpace(r.URL.Query().Get("category"))
	response.Success(w, http.StatusOK, map[string]interface{}{
		"category": cat,
		"products": Products(cat),
	})
}

// PurchaseRequest buys airtime/data/etc from the user wallet.
type PurchaseRequest struct {
	ProductID    string  `json:"product_id"`
	Customer     string  `json:"customer"` // phone number or meter/smartcard
	Amount       float64 `json:"amount"`   // required when product.amount == 0
	Currency     string  `json:"currency"`
	PIN          string  `json:"pin"`
	PaymentToken string  `json:"payment_token"`
	Narration    string  `json:"narration"`
}

// PurchaseHandler — POST /user/bills/purchase
func PurchaseHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req PurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	product := FindProduct(strings.TrimSpace(req.ProductID))
	if product == nil {
		response.Error(w, http.StatusBadRequest, "Unknown product_id — use GET /user/bills/products")
		return
	}

	customer := strings.TrimSpace(req.Customer)
	if customer == "" {
		response.Error(w, http.StatusBadRequest, "customer is required (phone number, meter, or smartcard ID)")
		return
	}

	amount := product.Amount
	if amount <= 0 {
		amount = req.Amount
	}
	if amount < 50 {
		response.Error(w, http.StatusBadRequest, "Minimum amount is ₦50")
		return
	}
	if amount > 500000 {
		response.Error(w, http.StatusBadRequest, "Amount too large")
		return
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = product.Currency
	}
	if currency == "" {
		currency = "NGN"
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if err := userPkg.VerifyDebitAuth(user, req.PIN, req.PaymentToken); err != nil {
		userPkg.WriteDebitAuthError(w, err)
		return
	}

	// Junior account rules: freeze + airtime/data only + spend caps
	if user.IsJunior() {
		if user.JuniorFrozen {
			response.Error(w, http.StatusForbidden, "This kids account is frozen by the parent")
			return
		}
		if product.CategoryID != "airtime" && product.CategoryID != "data" {
			response.Error(w, http.StatusForbidden, "Kids accounts can only buy airtime and data")
			return
		}
		if !user.AllowAirtimeData {
			response.Error(w, http.StatusForbidden, "Parent has disabled airtime/data for this kids account")
			return
		}
		if amount > user.DailySpendLimit && user.DailySpendLimit > 0 {
			response.Error(w, http.StatusForbidden, fmt.Sprintf("Over daily spend limit (₦%.0f)", user.DailySpendLimit))
			return
		}
	}

	ref := fmt.Sprintf("BILL-%s-%d", strings.ToUpper(product.CategoryID), time.Now().UnixNano()%1e12)
	desc := fmt.Sprintf("%s · %s · %s", product.Name, product.Provider, customer)
	if n := strings.TrimSpace(req.Narration); n != "" {
		desc = desc + " · " + n
	}

	// Debit user wallet first
	if err := walletPkg.DebitWallet(userID, amount, 0, currency, "flutterwave", ref, desc); err != nil {
		activity.Error("bill", "purchase_failed", err.Error(), activity.UID(userID), ref, r.RemoteAddr)
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	fw := settings.FlutterwaveClient()
	providerRef := ""
	status := "successful"
	mode := "mock"

	if fw.Configured() {
		mode = "live"
		okPay, pRef, err := fw.PayBill(product.CategoryID, product.BillerCode, product.ItemCode, customer, amount, currency, ref)
		if err != nil || !okPay {
			// Refund wallet on provider failure
			_ = walletPkg.FundWallet(userID, amount, currency, "refund", ref+"-RFND", "Refund failed bill: "+desc)
			msg := "Bill payment failed"
			if err != nil {
				msg = err.Error()
			}
			activity.Error("bill", "provider_failed", msg, activity.UID(userID), ref, r.RemoteAddr)
			response.Error(w, http.StatusBadGateway, msg)
			return
		}
		providerRef = pRef
		database.DB.Model(&models.Transaction{}).Where("reference = ?", ref).Updates(map[string]interface{}{
			"provider_reference": providerRef,
			"type":               "bill_payment",
		})
	} else {
		log.Printf("[Bills Mock] %s amount=%.2f customer=%s ref=%s", product.ID, amount, customer, ref)
		providerRef = "MOCK-" + ref
		database.DB.Model(&models.Transaction{}).Where("reference = ?", ref).Updates(map[string]interface{}{
			"provider_reference": providerRef,
			"type":               "bill_payment",
		})
	}

	activity.Success("bill", "purchase_ok", desc, activity.UID(userID), ref, r.RemoteAddr)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"reference":          ref,
		"provider_reference": providerRef,
		"status":             status,
		"mode":               mode,
		"product":            product,
		"customer":           customer,
		"amount":             amount,
		"currency":           currency,
		"message":            map[bool]string{true: "Bill paid successfully", false: "Mock bill paid (wallet debited). Configure Flutterwave for live airtime/data."}[fw.Configured()],
	})
}

// HistoryHandler — GET /user/bills/history
func HistoryHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	var txns []models.Transaction
	database.DB.Where("user_id = ? AND type = ?", userID, "bill_payment").
		Order("created_at desc").Limit(50).Find(&txns)
	response.Success(w, http.StatusOK, map[string]interface{}{
		"transactions": txns,
	})
}
