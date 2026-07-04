package models

import (
	"time"
)

// Session represents an active login session for a user
type Session struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null;size:500" json:"-"`
	IPAddress string    `gorm:"size:45" json:"ip_address"`
	UserAgent string    `gorm:"size:500" json:"user_agent"`
	Device    string    `gorm:"size:100" json:"device"`
	LastActive time.Time `json:"last_active"`
	CreatedAt time.Time `json:"created_at"`
}
