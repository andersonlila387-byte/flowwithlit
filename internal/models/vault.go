package models

import (
	"time"
)

// Vault represents a high-yield locked savings account
type Vault struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	UserID        uint       `gorm:"not null;index" json:"user_id"`
	
	Name          string     `gorm:"size:255;not null" json:"name"`
	Currency      string     `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	
	TargetAmount  float64    `gorm:"type:decimal(20,4);default:0" json:"target_amount"`
	CurrentAmount float64    `gorm:"type:decimal(20,4);default:0" json:"current_amount"`
	APY           float64    `gorm:"type:decimal(5,2);default:10.0" json:"apy"` // e.g. 10.50%
	
	LockUntil     *time.Time `json:"lock_until"` // When funds can be withdrawn without penalty
	Status        string     `gorm:"size:20;default:'active'" json:"status"` // active, matured, broken
	
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
