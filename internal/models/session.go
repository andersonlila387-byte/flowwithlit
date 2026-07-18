package models

import (
	"time"
)

// Session represents an active login session for a user
type Session struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index" json:"user_id"`
	Token       string    `gorm:"uniqueIndex;not null;size:500" json:"-"`
	IPAddress   string    `gorm:"size:45" json:"ip_address"`
	UserAgent   string    `gorm:"size:500" json:"user_agent"`
	Device      string    `gorm:"size:100" json:"device"`
	Fingerprint string    `gorm:"size:64;index" json:"fingerprint,omitempty"`
	LastActive  time.Time `json:"last_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// TrustedDevice remembers a browser/device after new-device email verification
// so the user is not asked for a code on every login from the same device.
type TrustedDevice struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"not null;index;uniqueIndex:idx_user_device_fp" json:"user_id"`
	Fingerprint string    `gorm:"size:64;not null;uniqueIndex:idx_user_device_fp" json:"fingerprint"`
	Device      string    `gorm:"size:100" json:"device"`
	UserAgent   string    `gorm:"size:500" json:"user_agent"`
	IPAddress   string    `gorm:"size:45" json:"ip_address"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	CreatedAt   time.Time `json:"created_at"`
}
