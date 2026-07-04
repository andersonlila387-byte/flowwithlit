package models

import "time"

type AdminInvite struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Email       string    `gorm:"not null;size:255" json:"email"`
	Role        string    `gorm:"not null;size:50" json:"role"`
	Token       string    `gorm:"uniqueIndex;not null;size:255" json:"token"`
	Status      string    `gorm:"not null;default:'pending';size:20" json:"status"` // pending, accepted, expired
	InvitedByID uint      `json:"invited_by_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
