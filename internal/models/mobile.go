package models

import "time"

// BiometricCredential lets a mobile install log in / authorize payments after
// the OS biometric (fingerprint / Face ID) unlocks a device-held secret.
// The server never receives raw biometric data — only a high-entropy token.
type BiometricCredential struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index;uniqueIndex:idx_bio_user_device" json:"user_id"`
	DeviceID     string     `gorm:"size:128;not null;uniqueIndex:idx_bio_user_device" json:"device_id"`
	TokenHash    string     `gorm:"size:255;not null" json:"-"`
	DeviceLabel  string     `gorm:"size:120" json:"device_label"`
	Platform     string     `gorm:"size:20" json:"platform"` // ios | android
	LoginEnabled bool       `gorm:"default:true" json:"login_enabled"`
	PayEnabled   bool       `gorm:"default:true" json:"pay_enabled"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// PushDevice stores FCM/APNs device tokens for push notifications.
type PushDevice struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	DeviceID  string     `gorm:"size:128;index" json:"device_id"`
	Token     string     `gorm:"size:512;not null;uniqueIndex" json:"-"`
	Platform  string     `gorm:"size:20;not null" json:"platform"` // ios | android
	AppVersion string    `gorm:"size:40" json:"app_version,omitempty"`
	Enabled   bool       `gorm:"default:true" json:"enabled"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
