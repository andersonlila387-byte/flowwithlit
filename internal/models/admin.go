package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AdminUser represents a bank staff or system administrator
type AdminUser struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Email     string         `gorm:"uniqueIndex;not null;size:255" json:"email"`
	Password  string         `gorm:"not null" json:"-"`
	Role      string         `gorm:"size:50;not null;default:'SUPPORT_AGENT'" json:"role"` // e.g., SUPER_ADMIN, SUPPORT_AGENT, COMPLIANCE_OFFICER
	IsActive  bool           `gorm:"not null;default:true" json:"is_active"`
	LastLogin *time.Time     `json:"last_login"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// HashPassword takes a plaintext password and returns a hashed version
func (a *AdminUser) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return err
	}
	a.Password = string(bytes)
	return nil
}

// CheckPassword matches a plaintext password against the hashed password
func (a *AdminUser) CheckPassword(providedPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(a.Password), []byte(providedPassword))
}
