package models

import "time"

// DepositAccount is a persisted fiat collection/receiving account for a user.
// The default (IsDefault) one is auto-created the moment KYC is approved, in the
// user's own name. Additional currencies can be added on demand, capped at 4 total.
type DepositAccount struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"not null;index;uniqueIndex:idx_user_currency" json:"user_id"`
	Currency      string    `gorm:"size:10;not null;uniqueIndex:idx_user_currency" json:"currency"`
	CountryCode   string    `gorm:"size:2" json:"country_code"`
	AccountNumber string    `gorm:"size:34;not null" json:"account_number"`
	BankName      string    `gorm:"size:255" json:"bank_name"`
	AccountName   string    `gorm:"size:255" json:"account_name"`
	Provider      string    `gorm:"size:20" json:"provider"` // onepipe | flutterwave
	IsDefault     bool      `gorm:"default:false" json:"is_default"`
	CreatedAt     time.Time `json:"created_at"`
}

// CryptoDepositAddress is a persisted receiving address for a crypto asset.
// USDT is the settlement ledger currency for the wallet system — funds landing on
// any other asset's address (e.g. BTC) convert to USDT on receipt; the user can
// still choose to withdraw as that original asset later (see WithdrawCryptoHandler).
type CryptoDepositAddress struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index;uniqueIndex:idx_user_asset" json:"user_id"`
	Asset     string    `gorm:"size:10;not null;uniqueIndex:idx_user_asset" json:"asset"` // USDT | BTC | ...
	Network   string    `gorm:"size:20" json:"network"`
	Address   string    `gorm:"size:128;not null" json:"address"`
	CreatedAt time.Time `json:"created_at"`
}
