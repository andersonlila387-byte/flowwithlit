package payroll

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settings"
	"flowwithlit/pkg/middleware"
	"flowwithlit/pkg/response"

	"github.com/go-chi/chi/v5"
)

type employeeRequest struct {
	FullName      string   `json:"full_name"`
	BankCode      string   `json:"bank_code"`
	AccountNumber string   `json:"account_number"`
	SalaryAmount  float64  `json:"salary_amount"`
	BonusAmount   *float64 `json:"bonus_amount"`
	Currency      string   `json:"currency"`
	Role          string   `json:"role"`
	Active        *bool    `json:"active"`
}

// GetEmployeesHandler lists the caller's payroll roster.
func GetEmployeesHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var employees []models.PayrollEmployee
	if err := database.DB.Where("user_id = ?", userID).Order("created_at desc").Find(&employees).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to load payroll roster")
		return
	}

	activeCount := 0
	var totalActiveSalary float64
	var totalActiveBonus float64
	for _, e := range employees {
		if e.Active {
			activeCount++
			totalActiveSalary += e.SalaryAmount
			totalActiveBonus += e.BonusAmount
		}
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"employees":           employees,
		"active_count":        activeCount,
		"total_active_salary": totalActiveSalary,
		"total_active_bonus":  totalActiveBonus,
		"total_active_payout": totalActiveSalary + totalActiveBonus,
	})
}

// CreateEmployeeHandler adds a new employee to the roster, resolving the account name up front.
func CreateEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var req employeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	req.BankCode = strings.TrimSpace(req.BankCode)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.Currency = strings.TrimSpace(req.Currency)

	if req.FullName == "" || req.BankCode == "" || req.AccountNumber == "" || req.SalaryAmount <= 0 {
		response.Error(w, http.StatusBadRequest, "full_name, bank_code, account_number, and a positive salary_amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "NGN"
	}

	accountName, err := settings.FlutterwaveClient().ResolveBankAccount(req.BankCode, req.AccountNumber)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Could not verify account: "+err.Error())
		return
	}

	employee := models.PayrollEmployee{
		UserID:        userID,
		FullName:      req.FullName,
		BankCode:      req.BankCode,
		AccountNumber: req.AccountNumber,
		AccountName:   accountName,
		SalaryAmount:  req.SalaryAmount,
		Currency:      req.Currency,
		Role:          req.Role,
		Active:        true,
	}
	if req.BonusAmount != nil && *req.BonusAmount >= 0 {
		employee.BonusAmount = *req.BonusAmount
	}

	if err := database.DB.Create(&employee).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to save employee")
		return
	}

	response.Success(w, http.StatusCreated, map[string]interface{}{
		"message":  "Employee added to payroll roster",
		"employee": employee,
	})
}

// UpdateEmployeeHandler edits an existing roster entry, scoped to the owner.
func UpdateEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid employee id")
		return
	}

	var employee models.PayrollEmployee
	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).First(&employee).Error; err != nil {
		response.Error(w, http.StatusNotFound, "Employee not found")
		return
	}

	var req employeeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid payload")
		return
	}

	// Only re-resolve the account name when bank_code/account_number actually changed,
	// so a salary-only edit doesn't cost an extra provider call.
	newBankCode := strings.TrimSpace(req.BankCode)
	newAccountNumber := strings.TrimSpace(req.AccountNumber)
	if newBankCode != "" && newAccountNumber != "" &&
		(newBankCode != employee.BankCode || newAccountNumber != employee.AccountNumber) {
		accountName, err := settings.FlutterwaveClient().ResolveBankAccount(newBankCode, newAccountNumber)
		if err != nil {
			response.Error(w, http.StatusBadRequest, "Could not verify account: "+err.Error())
			return
		}
		employee.BankCode = newBankCode
		employee.AccountNumber = newAccountNumber
		employee.AccountName = accountName
	}

	if fullName := strings.TrimSpace(req.FullName); fullName != "" {
		employee.FullName = fullName
	}
	if req.SalaryAmount > 0 {
		employee.SalaryAmount = req.SalaryAmount
	}
	if currency := strings.TrimSpace(req.Currency); currency != "" {
		employee.Currency = currency
	}
	employee.Role = req.Role
	if req.Active != nil {
		employee.Active = *req.Active
	}
	if req.BonusAmount != nil && *req.BonusAmount >= 0 {
		employee.BonusAmount = *req.BonusAmount
	}

	if err := database.DB.Save(&employee).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to update employee")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message":  "Employee updated",
		"employee": employee,
	})
}

// DeleteEmployeeHandler soft-deletes a roster entry, scoped to the owner.
func DeleteEmployeeHandler(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(uint)
	if !ok {
		response.Error(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, http.StatusBadRequest, "Invalid employee id")
		return
	}

	if err := database.DB.Where("id = ? AND user_id = ?", id, userID).Delete(&models.PayrollEmployee{}).Error; err != nil {
		response.Error(w, http.StatusInternalServerError, "Failed to remove employee")
		return
	}

	response.Success(w, http.StatusOK, map[string]interface{}{
		"message": "Employee removed from payroll roster",
	})
}
