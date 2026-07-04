package models

import (
	"time"
)

// SystemProvider tracks the active BaaS providers and enables dynamic routing
type SystemProvider struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ProviderName string    `gorm:"size:50;uniqueIndex;not null" json:"provider_name"` // e.g. onepipe, flutterwave, circle
	ServiceType  string    `gorm:"size:100;not null" json:"service_type"` // e.g. ngn_virtual_accounts, usd_processing, crypto
	IsActive     bool      `gorm:"default:true" json:"is_active"`
	Priority     int       `gorm:"default:0" json:"priority"` // Higher priority is favored if multiple are active
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
