package models

import (
	"time"

	"gorm.io/gorm"
)

type SecureTransfer struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	SenderID        uint           `gorm:"index;not null" json:"sender_id"`
	RecipientEmail  string         `gorm:"size:255;not null;index" json:"recipient_email"`
	RecipientID     *uint          `gorm:"index" json:"recipient_id,omitempty"`
	Reference       string         `gorm:"size:30;uniqueIndex;not null" json:"reference"`
	AccessKey  string         `gorm:"size:20;uniqueIndex;not null" json:"access_key"`
	Amount     float64        `gorm:"not null" json:"amount"`
	Currency   string         `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	Note       string         `gorm:"size:500" json:"note"`
	Status     string         `gorm:"size:20;not null;index;default:'pending'" json:"status"` // pending, claimed, expired, refunded
	SenderName string         `gorm:"size:255" json:"sender_name"`
	ExpiresAt  time.Time      `json:"expires_at"`
	ClaimedAt  *time.Time     `json:"claimed_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
