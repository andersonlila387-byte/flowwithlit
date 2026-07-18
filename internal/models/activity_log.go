package models

import "time"

// ActivityLog is a cross-system trail for ops monitoring (logins, money moves,
// bills, webhooks summary, mobile events). Separate from admin AuditLog.
type ActivityLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	// Area groups the product surface: auth, transfer, bill, webhook, wallet,
	// biometric, push, kyc, checkout, system
	Area      string    `gorm:"size:40;index;not null" json:"area"`
	// Level: info | warning | error | success
	Level     string    `gorm:"size:20;index;not null;default:'info'" json:"level"`
	Event     string    `gorm:"size:80;index;not null" json:"event"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	UserID    *uint     `gorm:"index" json:"user_id,omitempty"`
	// Reference ties to transaction ref, session, webhook id, etc.
	Reference string    `gorm:"size:120;index" json:"reference,omitempty"`
	// Meta is optional free-form JSON string for debugging
	Meta      string    `gorm:"type:text" json:"meta,omitempty"`
	IPAddress string    `gorm:"size:45" json:"ip_address,omitempty"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
