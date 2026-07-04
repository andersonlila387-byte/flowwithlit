package models

import (
	"time"
)

// Notification tracks alerts, transactions, and system messages for users
type Notification struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;index" json:"user_id"`
	Type           string    `gorm:"size:50;not null;index" json:"type"` // alert, transaction, system
	Title          string    `gorm:"size:255;not null" json:"title"`
	Message        string    `gorm:"type:text;not null" json:"message"`
	IsRead         bool      `gorm:"default:false;index" json:"is_read"`
	Source         string    `gorm:"size:30;index" json:"source"` // broadcast, system, transaction, ...
	BroadcastID    *uint     `gorm:"index" json:"broadcast_id,omitempty"`
	BroadcastType  string    `gorm:"size:50" json:"broadcast_type,omitempty"` // newsletter, maintenance, security_alert, ...
	ModalDismissed bool      `gorm:"default:false;index" json:"modal_dismissed"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
