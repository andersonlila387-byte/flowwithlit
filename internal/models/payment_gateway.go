package models

import (
	"time"
)

// Customer tracks the external customers who pay merchants
type Customer struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	MerchantUserID uint      `gorm:"not null;index" json:"merchant_user_id"`
	Name           string    `gorm:"size:255;not null" json:"name"`
	Email          string    `gorm:"size:255" json:"email"`
	Phone          string    `gorm:"size:20" json:"phone"`
	TotalSpent     float64   `gorm:"type:decimal(20,4);default:0" json:"total_spent"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// PaymentLink allows merchants to accept payments on their website or via a link
type PaymentLink struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Slug        string    `gorm:"size:100;uniqueIndex;not null" json:"slug"` // The URL slug e.g. /pay/donation
	Title       string    `gorm:"size:255;not null" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	Currency    string    `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	Amount      float64   `gorm:"type:decimal(20,4)" json:"amount"` // 0 or null if it's an open donation amount
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Invoice tracks specific bills sent to customers
type Invoice struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index" json:"user_id"` // Merchant
	CustomerID    uint      `gorm:"index" json:"customer_id"`
	InvoiceNumber string    `gorm:"size:100;uniqueIndex;not null" json:"invoice_number"`
	Amount        float64   `gorm:"type:decimal(20,4);not null" json:"amount"`
	Currency      string    `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	DueDate       time.Time `json:"due_date"`
	Status        string    `gorm:"size:20;not null;default:'unpaid'" json:"status"` // unpaid, paid, overdue, cancelled
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CheckoutSession tracks external customers attempting to pay via links or invoices
type CheckoutSession struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	PaymentLinkID *uint     `gorm:"index" json:"payment_link_id"`
	InvoiceID     *uint     `gorm:"index" json:"invoice_id"`
	CustomerID    *uint     `gorm:"index" json:"customer_id"`
	CustomerEmail string    `gorm:"size:255" json:"customer_email"` // Fallback if no CustomerID
	Amount        float64   `gorm:"type:decimal(20,4);not null" json:"amount"`
	Currency      string    `gorm:"size:10;not null" json:"currency"`
	Status        string    `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, successful, failed
	PaymentMethod string    `gorm:"size:50" json:"payment_method"` // card, bank_transfer
	TransactionID *uint     `gorm:"index" json:"transaction_id"` // Links to the master ledger once successful
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
