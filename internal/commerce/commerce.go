package commerce

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"
)

type CreatePaymentLinkRequest struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Currency    string  `json:"currency"`
	Amount      float64 `json:"amount"` // 0 means open amount
}

func CreatePaymentLinkHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreatePaymentLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Title == "" {
		response.Error(w, http.StatusBadRequest, "Payment link title is required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	baseSlug := strings.ToLower(strings.ReplaceAll(req.Title, " ", "-"))
	slug := baseSlug + "-" + generateRandomString(6)

	link := models.PaymentLink{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Currency:    req.Currency,
		Amount:      req.Amount,
		Slug:        slug,
		IsActive:    true,
	}

	if err := database.DB.Create(&link).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create payment link")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Payment link created successfully",
		"link":    link,
		"url":     "https://pay.flowwithlit.com/l/" + link.Slug,
	})
}

func GetPaymentLinksHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var links []models.PaymentLink
	if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&links).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve payment links")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"links": links,
		"total": len(links),
	})
}

func TogglePaymentLinkHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		LinkID uint `json:"link_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.LinkID == 0 {
		response.Error(w, http.StatusBadRequest, "link_id is required")
		return
	}

	var link models.PaymentLink
	if err := database.DB.Where("id = ? AND user_id = ?", req.LinkID, userID).First(&link).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Payment link not found")
		return
	}

	link.IsActive = !link.IsActive
	database.DB.Save(&link)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Payment link updated",
		"is_active": link.IsActive,
	})
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(strings.Replace(time.Now().String(), " ", "", -1)))[:n]
	}
	return hex.EncodeToString(b)[:n]
}

type CreateInvoiceRequest struct {
	CustomerName  string  `json:"customer_name"`
	CustomerEmail string  `json:"customer_email"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	DueDate       string  `json:"due_date"`
}

func CreateInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.CustomerName == "" || req.Amount <= 0 {
		response.Error(w, http.StatusBadRequest, "Customer name and valid amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	var customer models.Customer
	err := database.DB.Where("merchant_user_id = ? AND email = ?", userID, req.CustomerEmail).First(&customer).Error
	if err != nil {
		customer = models.Customer{
			MerchantUserID: userID,
			Name:           req.CustomerName,
			Email:          req.CustomerEmail,
		}
		database.DB.Create(&customer)
	}

	invoiceNumber := "INV-" + generateRandomString(8)
	dueDate := time.Now().AddDate(0, 0, 14)
	if strings.TrimSpace(req.DueDate) != "" {
		if parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.DueDate)); err == nil {
			dueDate = parsed
		}
	}

	invoice := models.Invoice{
		UserID:        userID,
		CustomerID:    customer.ID,
		InvoiceNumber: invoiceNumber,
		Amount:        req.Amount,
		Currency:      req.Currency,
		DueDate:       dueDate,
		Status:        "unpaid",
	}

	if err := database.DB.Create(&invoice).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to create invoice")
		return
	}

	merchantName := "Merchant"
	var merchant models.User
	if err := database.DB.Select("first_name, last_name, email").First(&merchant, userID).Error; err == nil {
		if name := strings.TrimSpace(merchant.FirstName + " " + merchant.LastName); name != "" {
			merchantName = name
		} else if merchant.Email != "" {
			merchantName = merchant.Email
		}
	}
	var profile models.BusinessProfile
	if err := database.DB.Where("user_id = ?", userID).First(&profile).Error; err == nil {
		if name := strings.TrimSpace(profile.BusinessName); name != "" {
			merchantName = name
		}
	}

	payURL := email.InvoicePayURL(invoice.InvoiceNumber)
	if customerEmail := strings.TrimSpace(req.CustomerEmail); customerEmail != "" {
		_ = email.SendInvoiceSent(
			customerEmail, req.CustomerName, merchantName, invoice.InvoiceNumber,
			"Invoice from "+merchantName, dueDate.Format("Jan 02, 2006"),
			req.Amount, req.Currency,
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Invoice created successfully",
		"invoice": invoice,
		"url":     payURL,
	})
}

func RemindInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		InvoiceID uint `json:"invoice_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InvoiceID == 0 {
		response.Error(w, http.StatusBadRequest, "invoice_id is required")
		return
	}

	var row struct {
		models.Invoice
		CustomerName  string `json:"customer_name"`
		CustomerEmail string `json:"customer_email"`
	}
	err := database.DB.Table("invoices").
		Select("invoices.*, customers.name as customer_name, customers.email as customer_email").
		Joins("LEFT JOIN customers ON customers.id = invoices.customer_id").
		Where("invoices.id = ? AND invoices.user_id = ?", req.InvoiceID, userID).
		Scan(&row).Error
	if err != nil || row.ID == 0 {
		response.Error(w, http.StatusNotFound, "Invoice not found")
		return
	}
	if row.Status == "paid" {
		response.Error(w, http.StatusBadRequest, "Invoice is already paid")
		return
	}

	customerEmail := strings.TrimSpace(row.CustomerEmail)
	if customerEmail == "" {
		response.Error(w, http.StatusBadRequest, "Customer has no email on file")
		return
	}

	daysOverdue := 0
	if time.Now().After(row.DueDate) {
		daysOverdue = int(time.Since(row.DueDate).Hours() / 24)
		if daysOverdue < 1 {
			daysOverdue = 1
		}
	}

	_ = email.SendInvoiceReminder(
		customerEmail, row.CustomerName, row.InvoiceNumber,
		row.DueDate.Format("Jan 02, 2006"), daysOverdue,
		row.Amount, row.Currency,
	)

	response.Success(w, http.StatusOK, map[string]string{"message": "Invoice reminder sent"})
}

func MarkInvoicePaidHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req struct {
		InvoiceID uint   `json:"invoice_id"`
		Reference string `json:"reference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.InvoiceID == 0 {
		response.Error(w, http.StatusBadRequest, "invoice_id is required")
		return
	}

	var row struct {
		models.Invoice
		CustomerEmail string `json:"customer_email"`
	}
	err := database.DB.Table("invoices").
		Select("invoices.*, customers.email as customer_email").
		Joins("LEFT JOIN customers ON customers.id = invoices.customer_id").
		Where("invoices.id = ? AND invoices.user_id = ?", req.InvoiceID, userID).
		Scan(&row).Error
	if err != nil || row.ID == 0 {
		response.Error(w, http.StatusNotFound, "Invoice not found")
		return
	}

	now := time.Now()
	ref := strings.TrimSpace(req.Reference)
	if ref == "" {
		ref = "INV-PAY-" + now.Format("20060102150405")
	}

	database.DB.Model(&models.Invoice{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
		"status": "paid",
	})

	var merchant models.User
	if err := database.DB.Select("email, first_name").First(&merchant, userID).Error; err == nil {
		_ = email.SendInvoicePaid(
			merchant.Email, merchant.FirstName, row.InvoiceNumber,
			row.CustomerEmail, ref, row.Amount, row.Currency, now,
		)
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":   "Invoice marked as paid",
		"reference": ref,
	})
}

type InvoiceWithCustomer struct {
	models.Invoice
	CustomerName  string `json:"customer_name"`
	CustomerEmail string `json:"customer_email"`
}

func GetInvoicesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var rows []struct {
		models.Invoice
		CustomerName  string `json:"customer_name"`
		CustomerEmail string `json:"customer_email"`
	}

	database.DB.Table("invoices").
		Select("invoices.*, customers.name as customer_name, customers.email as customer_email").
		Joins("LEFT JOIN customers ON customers.id = invoices.customer_id").
		Where("invoices.user_id = ?", userID).
		Order("invoices.created_at desc").
		Scan(&rows)

	response.Success(w, http.StatusOK, map[string]interface{}{
		"invoices": rows,
		"total":    len(rows),
	})
}

func GetCustomersHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var customers []models.Customer
	if err := database.DB.Where("merchant_user_id = ?", userID).Order("updated_at desc").Find(&customers).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to retrieve customers")
		return
	}

	response.Success(w, http.StatusOK, customers)
}
