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

// CheckoutSession locks in the amount/currency/customer for a checkout attempt
// server-side, so the hosted checkout page and ChargeHandler never have to trust
// a client-supplied amount. Created by the merchant's own backend (secret key)
// via POST /v1/checkout/sessions, or by our own PaymentLink/Invoice pay flows.
type CheckoutSession struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	Token         string     `gorm:"size:40;uniqueIndex;not null" json:"token"` // public identifier used in checkout URLs
	UserID        uint       `gorm:"not null;index" json:"user_id"`             // merchant this session belongs to
	IsTest        bool       `gorm:"default:false" json:"is_test"`              // created with a test or live key
	PaymentLinkID *uint      `gorm:"index" json:"payment_link_id"`
	InvoiceID     *uint      `gorm:"index" json:"invoice_id"`
	CustomerID    *uint      `gorm:"index" json:"customer_id"`
	CustomerEmail string     `gorm:"size:255" json:"customer_email"` // Fallback if no CustomerID
	CustomerName  string     `gorm:"size:255" json:"customer_name"`
	Amount        float64    `gorm:"type:decimal(20,4);not null" json:"amount"` // kobo / minor units, authoritative
	Currency      string     `gorm:"size:10;not null" json:"currency"`
	MerchantRef   string     `gorm:"size:100" json:"merchant_ref"` // merchant's own reference, becomes the transaction ref
	Meta          string     `gorm:"type:text" json:"-"`           // JSON-encoded passthrough meta
	Status        string     `gorm:"size:20;not null;default:'pending'" json:"status"` // pending, completed, expired
	PaymentMethod string     `gorm:"size:50" json:"payment_method"`                    // card, bank_transfer
	TransactionID *uint      `gorm:"index" json:"transaction_id"`                      // Links to the master ledger once successful
	ExpiresAt     time.Time  `json:"expires_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
