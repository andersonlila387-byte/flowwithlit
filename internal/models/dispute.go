package models

import (
	"time"

	"gorm.io/gorm"
)

type Dispute struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"not null;index" json:"user_id"`
	TransactionID uint           `gorm:"index" json:"transaction_id"`
	Reference     string         `gorm:"size:100;uniqueIndex;not null" json:"reference"`
	Amount        float64        `gorm:"type:decimal(20,4)" json:"amount"`
	Currency      string         `gorm:"size:10;not null" json:"currency"`
	Reason        string         `gorm:"size:255;not null" json:"reason"`
	Details       string         `gorm:"type:text" json:"details"`
	Status        string         `gorm:"size:30;not null;index;default:'open'" json:"status"` // open, under_review, resolved, rejected
	Resolution    string         `gorm:"type:text" json:"resolution"`
	UserEmail     string         `gorm:"size:255" json:"user_email"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
