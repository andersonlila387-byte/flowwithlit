package models

import "time"

type OutboundWebhookLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"index;not null" json:"user_id"`
	EventType    string    `gorm:"size:100;not null" json:"event_type"`
	TargetURL    string    `gorm:"size:512;not null" json:"target_url"`
	Payload      string    `gorm:"type:text" json:"payload"`
	StatusCode   int       `json:"status_code"`
	ResponseBody string    `gorm:"type:text" json:"response_body"`
	Attempts     int       `gorm:"default:1" json:"attempts"`
	CreatedAt    time.Time `json:"created_at"`
}
