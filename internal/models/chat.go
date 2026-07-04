package models

import (
	"gorm.io/gorm"
	"time"
)

type ChatSession struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	SessionRef string         `gorm:"uniqueIndex;not null;size:50" json:"session_ref"`
	UserID     *uint          `json:"user_id"`
	GuestName  string         `gorm:"size:100" json:"guest_name"`
	GuestEmail string         `gorm:"size:255" json:"guest_email"`
	AgentID    *uint          `json:"agent_id"`
	Status     string         `gorm:"not null;default:'bot';size:20" json:"status"` // bot, queued, active, closed
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type ChatMessage struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"index;not null" json:"session_id"`
	Sender    string    `gorm:"not null;size:20" json:"sender"` // user, agent, bot
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
