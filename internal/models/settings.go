package models

import (
	"time"

	"gorm.io/gorm"
)

// SystemSetting stores key-value configuration for the platform (e.g. KYC provider)
type SystemSetting struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Key       string         `gorm:"uniqueIndex;not null;size:100" json:"key"` // e.g. "kyc_provider", "kyc_mode"
	Value     string         `gorm:"not null;type:text" json:"value"`          // e.g. "mock", "smileid", "test", "live"
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
