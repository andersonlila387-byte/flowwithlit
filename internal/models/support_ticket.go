package models

import (
	"time"

	"gorm.io/gorm"
)

type SupportTicket struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	UserID     uint           `gorm:"index" json:"user_id"`
	Reference  string         `gorm:"size:50;uniqueIndex;not null" json:"reference"`
	Subject    string         `gorm:"size:255;not null" json:"subject"`
	Message    string         `gorm:"type:text;not null" json:"message"`
	Category   string         `gorm:"size:50;not null" json:"category"` // payment, account, kyc, withdrawal, other
	Status     string         `gorm:"size:20;not null;index;default:'open'" json:"status"` // open, in_progress, resolved, closed
	Priority   string         `gorm:"size:20;not null;default:'medium'" json:"priority"`   // low, medium, high, urgent
	AdminReply string         `gorm:"type:text" json:"admin_reply"`
	UserEmail  string         `gorm:"size:255" json:"user_email"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}
