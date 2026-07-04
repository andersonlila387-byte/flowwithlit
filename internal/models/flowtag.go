package models

import (
	"time"

	"gorm.io/gorm"
)

type FlowTag struct {
	ID             uint      `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	Reference      string    `gorm:"uniqueIndex;not null" json:"reference"`
	SenderID       uint      `gorm:"not null" json:"sender_id"`
	RecipientEmail string    `gorm:"not null" json:"recipient_email"`
	Currency       string    `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	Amount         float64   `gorm:"type:decimal(20,4);not null" json:"amount"`
	Status         string    `gorm:"type:varchar(20);default:'Pending'" json:"status"` // Pending, Claimed, Cancelled
	ClaimToken     string    `gorm:"uniqueIndex;not null" json:"claim_token"`
	ExpiresAt      time.Time `json:"expires_at"`
}
