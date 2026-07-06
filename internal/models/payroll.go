package models

import (
	"time"

	"gorm.io/gorm"
)

// PayrollEmployee is one roster entry (a recurring salary recipient) for a merchant.
type PayrollEmployee struct {
	ID            uint    `gorm:"primaryKey" json:"id"`
	UserID        uint    `gorm:"not null;index" json:"user_id"`
	FullName      string  `gorm:"size:255;not null" json:"full_name"`
	BankCode      string  `gorm:"size:20;not null" json:"bank_code"`
	AccountNumber string  `gorm:"size:20;not null" json:"account_number"`
	AccountName   string  `gorm:"size:255" json:"account_name"`
	SalaryAmount  float64 `gorm:"type:decimal(20,4);not null" json:"salary_amount"`
	// Recurring bonus added on top of SalaryAmount every run (0 = no standing bonus).
	// A future one-off/per-run bonus override can still be added on PayrollRunItem without touching this column.
	BonusAmount float64        `gorm:"type:decimal(20,4);not null;default:0" json:"bonus_amount"`
	Currency    string         `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	Role        string         `gorm:"size:100" json:"role"`
	Active      bool           `gorm:"default:true;index" json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// PayrollSettings is the one-per-user Auto Pay configuration.
type PayrollSettings struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	UserID            uint       `gorm:"uniqueIndex;not null" json:"user_id"`
	AutoPayEnabled    bool       `gorm:"default:false" json:"auto_pay_enabled"`
	PayDayRule        string     `gorm:"size:20;not null;default:'last_business_day'" json:"pay_day_rule"` // 'last_business_day' | 'fixed_day'
	FixedDayOfMonth   int        `gorm:"default:28" json:"fixed_day_of_month"`
	Timezone          string     `gorm:"size:64;not null;default:'Africa/Lagos'" json:"timezone"`
	ReviewWindowDays  int        `gorm:"default:5" json:"review_window_days"`
	SpendingCapAmount float64    `gorm:"type:decimal(20,4);not null;default:0" json:"spending_cap_amount"`
	PinVerifiedAt     *time.Time `json:"pin_verified_at,omitempty"`
	LastRunCheckedAt  *time.Time `json:"last_run_checked_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// PayrollRun is one payday's disbursement batch (draft -> ... -> completed/failed/cancelled).
type PayrollRun struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	UserID uint   `gorm:"not null;index" json:"user_id"`
	Status string `gorm:"size:20;not null;index;default:'draft'" json:"status"`
	// draft | pending_review | processing | completed | partially_failed | failed | cancelled
	ScheduledDate  time.Time  `gorm:"index" json:"scheduled_date"`
	ReviewDeadline time.Time  `json:"review_deadline"`
	Currency       string     `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	EmployeeCount  int        `gorm:"default:0" json:"employee_count"`
	TotalAmount    float64    `gorm:"type:decimal(20,4);not null;default:0" json:"total_amount"`
	TotalFee       float64    `gorm:"type:decimal(20,4);not null;default:0" json:"total_fee"`
	BatchRef       string     `gorm:"size:40;uniqueIndex" json:"batch_ref"`
	CapExceeded    bool       `gorm:"default:false" json:"cap_exceeded"`
	RemindedAt     *time.Time `json:"reminded_at,omitempty"`
	CancelledByID  *uint      `json:"cancelled_by_id,omitempty"`
	CancelReason   string     `gorm:"size:255" json:"cancel_reason,omitempty"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	RetryCount     int        `gorm:"default:0" json:"retry_count"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	FailureReason  string     `gorm:"size:500" json:"failure_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// PayrollRunItem is one employee's line within a PayrollRun (snapshot, editable while run is draft/pending_review).
type PayrollRunItem struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	PayrollRunID      uint      `gorm:"not null;index" json:"payroll_run_id"`
	PayrollEmployeeID uint      `gorm:"not null;index" json:"payroll_employee_id"`
	FullName          string    `gorm:"size:255;not null" json:"full_name"`
	BankCode          string    `gorm:"size:20;not null" json:"bank_code"`
	AccountNumber     string    `gorm:"size:20;not null" json:"account_number"`
	AccountName       string    `gorm:"size:255" json:"account_name"`
	Amount            float64   `gorm:"type:decimal(20,4);not null" json:"amount"`                 // total paid this run = salary + bonus
	BonusAmount       float64   `gorm:"type:decimal(20,4);not null;default:0" json:"bonus_amount"` // breakdown snapshot, editable per-run before execution
	Status            string    `gorm:"size:20;not null;index;default:'pending'" json:"status"`    // pending | successful | failed | excluded
	TransactionRef    string    `gorm:"size:255" json:"transaction_ref,omitempty"`
	ErrorMessage      string    `gorm:"size:500" json:"error_message,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
