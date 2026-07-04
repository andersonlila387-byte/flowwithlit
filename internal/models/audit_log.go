package models

import "time"

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AdminID    uint      `gorm:"index" json:"admin_id"`
	AdminEmail string    `gorm:"size:255" json:"admin_email"`
	Action     string    `gorm:"size:100;not null;index" json:"action"` // login, settings_update, kyc_approve, user_suspend …
	Resource   string    `gorm:"size:100" json:"resource"`              // user, merchant, settings, kyc
	ResourceID string    `gorm:"size:100" json:"resource_id"`
	Details    string    `gorm:"type:json" json:"details"`
	IPAddress  string    `gorm:"size:45" json:"ip_address"`
	CreatedAt  time.Time `json:"created_at"`
}
