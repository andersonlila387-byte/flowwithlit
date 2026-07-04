package models

import "time"

type BroadcastMessage struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Title          string    `gorm:"size:255;not null" json:"title"`
	Message        string    `gorm:"type:text;not null" json:"message"`
	Type           string    `gorm:"size:50;default:'newsletter'" json:"type"`   // newsletter, promo, maintenance, security_alert, update, other
	Target         string    `gorm:"size:50;default:'all'" json:"target"`        // all, merchants, tier1, tier2, tier0, unverified, verified
	Channel        string    `gorm:"size:20;default:'in_app'" json:"channel"`    // in_app, email, both
	SentByID       uint      `json:"sent_by_id"`
	SentByEmail    string    `gorm:"size:255" json:"sent_by_email"`
	RecipientCount int       `json:"recipient_count"`
	Status         string    `gorm:"size:20;default:'sent'" json:"status"` // sent, failed
	CreatedAt      time.Time `json:"created_at"`
}
