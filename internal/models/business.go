package models

import (
	"time"

	"gorm.io/gorm"
)

// BusinessProfile stores the user's KYC and business information
type BusinessProfile struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	UserID           uint           `gorm:"uniqueIndex;not null" json:"user_id"`
	
	// Core Info
	BusinessName     string         `gorm:"size:255" json:"business_name"`
	Industry         string         `gorm:"size:100" json:"industry"`
	SupportEmail     string         `gorm:"size:255" json:"support_email"`
	Phone            string         `gorm:"size:50" json:"phone"`
	Address          string         `gorm:"type:text" json:"address"`
	
	// Global Config
	CountryCode      string         `gorm:"size:2;not null" json:"country_code"` // ISO 2-letter e.g. "NG", "US"
	BaseCurrency     string         `gorm:"size:3;not null" json:"base_currency"` // e.g. "NGN", "USD"
	
	// Dynamic KYC
	PrimaryIDType    string         `gorm:"size:50" json:"primary_id_type"` // "BVN", "SSN", "PASSPORT"
	PrimaryIDValue   string         `gorm:"size:255" json:"primary_id_value"` // The actual number/hash
	SecondaryIDType  string         `gorm:"size:50" json:"secondary_id_type"` // e.g. "NIN"
	SecondaryIDValue string         `gorm:"size:255" json:"secondary_id_value"`
	
	// Settlement Bank
	BankCode         string         `gorm:"size:50" json:"bank_code"`
	AccountNumber    string         `gorm:"size:100" json:"account_number"`
	RoutingNumber    string         `gorm:"size:100" json:"routing_number"` // mostly for US/Intl
	
	// KYC Review
	KYCStatus        string         `gorm:"size:50;default:'pending'" json:"kyc_status"` // pending, approved, rejected, needs_info
	AdminNotes       string         `gorm:"type:text" json:"admin_notes"`
	ReviewedByID     uint           `json:"reviewed_by_id"`
	ReviewedByEmail  string         `gorm:"size:255" json:"reviewed_by_email"`

	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
