package models

import "time"

// Referral tracks a referrer → referee relationship and payout status.
type Referral struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	ReferrerID        uint       `gorm:"not null;index" json:"referrer_id"`
	RefereeID         uint       `gorm:"not null;uniqueIndex" json:"referee_id"`
	ReferralCode      string     `gorm:"size:32;not null" json:"referral_code"`
	Status            string     `gorm:"size:20;not null;default:'pending';index" json:"status"` // pending, paid
	QualifyingAmount  float64    `gorm:"type:decimal(20,4);default:0" json:"qualifying_amount"`
	RewardAmount      float64    `gorm:"type:decimal(20,4);default:0" json:"reward_amount"`
	RewardCurrency    string     `gorm:"size:10;default:'NGN'" json:"reward_currency"`
	QualifyingTxnRef  string     `gorm:"size:100" json:"qualifying_txn_ref"`
	PaidAt            *time.Time `json:"paid_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}