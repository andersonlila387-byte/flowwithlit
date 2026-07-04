package models

import (
	"time"
)

// VirtualCard represents a 3D-secure virtual debit/credit card
type VirtualCard struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	
	// Card details (sensitive fields should be encrypted in production)
	Last4       string    `gorm:"size:4" json:"last4"`
	CardNumber  string    `gorm:"size:20" json:"-"` // Never return in API responses
	ExpiryMonth int       `json:"expiry_month"`
	ExpiryYear  int       `json:"expiry_year"`
	CVV         string    `gorm:"size:4" json:"-"` // Only returned on explicit reveal with PIN

	Type        string    `gorm:"size:20;default:'standard'" json:"type"` // standard, burner
	Currency    string    `gorm:"size:10;default:'USD'" json:"currency"`
	Balance     float64   `gorm:"type:decimal(20,4);default:0" json:"balance"`
	
	Status      string    `gorm:"size:20;default:'active'" json:"status"` // active, frozen, cancelled, expired
	DailyLimit  float64   `gorm:"type:decimal(20,4);default:5000" json:"daily_limit"`

	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
