package models

import "time"

type WebhookLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Provider     string    `gorm:"size:50;not null;index" json:"provider"`   // onepipe, flutterwave, circle
	EventType    string    `gorm:"size:100" json:"event_type"`
	Payload      string    `gorm:"type:json" json:"payload"`
	StatusCode   int       `json:"status_code"`
	Status       string    `gorm:"size:20;not null;index;default:'received'" json:"status"` // received, processed, failed
	ErrorMessage string    `gorm:"type:text" json:"error_message"`
	ProcessingMs int64     `json:"processing_ms"`
	IPAddress    string    `gorm:"size:45" json:"ip_address"`
	CreatedAt    time.Time `json:"created_at"`
}
