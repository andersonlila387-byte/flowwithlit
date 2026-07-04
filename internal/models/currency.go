package models

import (
	"time"

	"gorm.io/gorm"
)

// Country — ISO reference (seed data).
type Country struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Code                string         `gorm:"size:2;uniqueIndex;not null" json:"code"`
	Name                string         `gorm:"size:100;not null" json:"name"`
	DefaultCurrencyCode string         `gorm:"size:3;not null" json:"default_currency_code"`
	IsActive            bool           `gorm:"default:true" json:"is_active"`
}

// Currency — fiat currencies enabled for payments / display.
type Currency struct {
	ID                    uint           `gorm:"primarykey" json:"id"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"`
	Code                  string         `gorm:"size:3;uniqueIndex;not null" json:"code"`
	Name                  string         `gorm:"size:80;not null" json:"name"`
	Symbol                string         `gorm:"size:8;not null" json:"symbol"`
	Decimals              int            `gorm:"default:2" json:"decimals"`
	MinorUnit             string         `gorm:"size:20;default:'cents'" json:"minor_unit"`
	FlutterwaveSupported  bool           `gorm:"default:false" json:"flutterwave_supported"`
	BankTransferSupported bool           `gorm:"default:false" json:"bank_transfer_supported"`
	CardSupported         bool           `gorm:"default:false" json:"card_supported"`
	IsEnabled             bool           `gorm:"default:true" json:"is_enabled"`
	IsBase                bool           `gorm:"default:false" json:"is_base"`
	SortOrder             int            `gorm:"default:0" json:"sort_order"`
}

// CryptoAsset — coins supported at checkout / swaps.
type CryptoAsset struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
	Code       string         `gorm:"size:10;uniqueIndex;not null" json:"code"`
	Name       string         `gorm:"size:50;not null" json:"name"`
	Network    string         `gorm:"size:30;not null" json:"network"`
	NetworkTag string         `gorm:"size:20;not null" json:"network_tag"`
	Decimals   int            `gorm:"default:2" json:"decimals"`
	IconKey    string         `gorm:"size:80" json:"icon_key"`
	Color      string         `gorm:"size:7;default:'#26A17B'" json:"color"`
	IsEnabled  bool           `gorm:"default:true" json:"is_enabled"`
	SortOrder  int            `gorm:"default:0" json:"sort_order"`
}

// ExchangeRate — latest manual/oracle rate per pair.
type ExchangeRate struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	FromCurrency  string         `gorm:"size:10;not null;uniqueIndex:idx_rate_pair" json:"from_currency"`
	ToCurrency    string         `gorm:"size:10;not null;uniqueIndex:idx_rate_pair" json:"to_currency"`
	Rate          float64        `gorm:"type:decimal(24,12);not null" json:"rate"`
	Source        string         `gorm:"size:20;default:'manual'" json:"source"`
	SpreadPercent float64        `gorm:"type:decimal(8,4);default:0" json:"spread_percent"`
	UpdatedBy     *uint          `json:"updated_by,omitempty"`
}

// RateChangeLog — audit trail for admin rate edits.
type RateChangeLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	FromCurrency string    `gorm:"size:10;not null;index" json:"from_currency"`
	ToCurrency   string    `gorm:"size:10;not null;index" json:"to_currency"`
	OldRate      float64   `gorm:"type:decimal(24,12)" json:"old_rate"`
	NewRate      float64   `gorm:"type:decimal(24,12);not null" json:"new_rate"`
	ChangedBy    uint      `gorm:"not null" json:"changed_by"`
	Reason       string    `gorm:"size:255" json:"reason"`
}

// FlowTagPaymentRequest — P2P payment request (mobile + web).
type FlowTagPaymentRequest struct {
	ID            uint           `gorm:"primarykey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
	Reference     string         `gorm:"uniqueIndex;not null" json:"reference"`
	RequesterID   uint           `gorm:"not null;index" json:"requester_id"`
	PayerEmail    string         `gorm:"size:191;not null;index" json:"payer_email"`
	PayerID       *uint          `gorm:"index" json:"payer_id,omitempty"`
	Currency      string         `gorm:"size:10;not null;default:'NGN'" json:"currency"`
	Amount        float64        `gorm:"type:decimal(20,4);not null" json:"amount"`
	Note          string         `gorm:"size:255" json:"note"`
	Status        string         `gorm:"size:20;default:'pending'" json:"status"` // pending, paid, declined, expired, cancelled
	PayToken      string         `gorm:"uniqueIndex;not null" json:"pay_token"`
	ExpiresAt     time.Time      `json:"expires_at"`
	PaidAt        *time.Time     `json:"paid_at,omitempty"`
	DeclinedAt    *time.Time     `json:"declined_at,omitempty"`
}