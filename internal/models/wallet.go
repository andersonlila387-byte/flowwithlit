package models

import (
	"time"
)

type Wallet struct {
	ID                       uint      `gorm:"primaryKey" json:"id"`
	UserID                   uint      `gorm:"not null;index" json:"user_id"`
	Currency                 string    `gorm:"not null;size:10;index" json:"currency"` // e.g. NGN, USD, USDC
	Balance                  float64   `gorm:"type:decimal(20,4);default:0" json:"balance"`
	LedgerBalance            float64   `gorm:"type:decimal(20,4);default:0" json:"ledger_balance"` // For pending funds
	ProviderAccountReference string    `gorm:"size:255" json:"provider_account_reference"` // Virtual account number tied to this wallet
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}
