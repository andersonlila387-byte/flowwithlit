package models

import (
	"time"

	"gorm.io/gorm"
)

type ApiCredentials struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	PubKeyTest    string         `gorm:"size:64;not null" json:"pub_key_test"`
	SecKeyTest    string         `gorm:"size:64;not null" json:"sec_key_test"`
	PubKeyLive    string         `gorm:"size:64;not null" json:"pub_key_live"`
	SecKeyLive    string         `gorm:"size:64;not null" json:"sec_key_live"`
	WebhookURL    string         `gorm:"size:512" json:"webhook_url"`
	CallbackURL   string         `gorm:"size:512" json:"callback_url"`
	WebhookSecret string         `gorm:"size:64" json:"-"`
	IsLive        bool           `gorm:"default:false" json:"is_live"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
