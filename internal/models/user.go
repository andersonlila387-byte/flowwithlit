package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// User represents the central user account for the Neobank/Gateway
type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Email            string         `gorm:"uniqueIndex;not null;size:255" json:"email"`
	FlowTagUsername  string         `gorm:"uniqueIndex;size:50" json:"flowtag_username"`
	Password  string         `gorm:"not null" json:"-"` // never return password in JSON
	FirstName string         `gorm:"size:100" json:"first_name"`
	LastName  string         `gorm:"size:100" json:"last_name"`
	Phone        string `gorm:"size:20" json:"phone"`
	ProfileImage string `gorm:"size:500" json:"profile_image"`
	KYCLevel  int            `gorm:"default:0" json:"kyc_level"` // 0=None, 1=Basic, 2=Verified (Also known as Tier)
	// AccountType: USER (adult) | MERCHANT | JUNIOR (child sub-account in kid's name)
	AccountType string       `gorm:"default:'USER';size:50" json:"account_type"`
	DateOfBirth *time.Time   `json:"date_of_birth,omitempty"`
	// ParentUserID links JUNIOR accounts to the verified adult who controls limits.
	// Parent KYC covers compliance; wallet display name is still the child's.
	ParentUserID *uint `gorm:"index" json:"parent_user_id,omitempty"`
	// Parent-controlled junior limits (NGN). 0 = blocked for that rail.
	DailySpendLimit   float64 `gorm:"type:decimal(20,4);default:5000" json:"daily_spend_limit"`
	MonthlySpendLimit float64 `gorm:"type:decimal(20,4);default:50000" json:"monthly_spend_limit"`
	JuniorFrozen      bool    `gorm:"default:false" json:"junior_frozen"` // parent freeze
	// AllowAirtimeData: juniors may buy airtime/data if true; bank cash-out always blocked.
	AllowAirtimeData bool `gorm:"default:true" json:"allow_airtime_data"`

	ResetOTP       *string        `gorm:"size:6" json:"-"`
	ResetOTPExpiry *time.Time     `json:"-"`
	IsEmailVerified bool          `gorm:"default:false" json:"is_email_verified"`
	IsPhoneVerified bool          `gorm:"default:false" json:"is_phone_verified"`
	// Mobile phone OTP (SMS) — separate from email verification used on web
	PhoneOTP       *string    `gorm:"size:6" json:"-"`
	PhoneOTPExpiry *time.Time `json:"-"`
	VerificationOTP *string       `gorm:"size:6" json:"-"`
	VerificationOTPExpiry *time.Time `json:"-"`
	// New-device login email challenge (cleared after success)
	DeviceOTP              *string    `gorm:"size:6" json:"-"`
	DeviceOTPExpiry        *time.Time `json:"-"`
	DevicePendingFingerprint string   `gorm:"size:64" json:"-"`
	TwoFactorEnabled bool         `gorm:"default:false" json:"two_factor_enabled"`
	TwoFactorSecret  string       `gorm:"size:255" json:"-"`
	SmsNotificationsEnabled bool  `gorm:"default:true" json:"sms_notifications_enabled"`
	DefaultFiatCurrency     string `gorm:"size:10;default:'NGN'" json:"default_fiat_currency"`
	DefaultCryptoCurrency   string `gorm:"size:10;default:'USDT'" json:"default_crypto_currency"`
	TransactionPin   string       `gorm:"size:255" json:"-"`
	ReferralCode     string         `gorm:"uniqueIndex;size:32" json:"referral_code"`
	ReferredByID     *uint          `gorm:"index" json:"referred_by_id,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// IsJunior reports child sub-account.
func (u *User) IsJunior() bool {
	return u.AccountType == "JUNIOR"
}

// IsAdultAccount reports a normal adult/merchant account that can parent juniors.
func (u *User) IsAdultAccount() bool {
	return u.AccountType == "USER" || u.AccountType == "MERCHANT" || u.AccountType == ""
}

// HashPassword takes a plaintext password and returns a hashed version
func (u *User) HashPassword(password string) error {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return err
	}
	u.Password = string(bytes)
	return nil
}

// CheckPassword matches a plaintext password against the hashed password
func (u *User) CheckPassword(providedPassword string) error {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(providedPassword))
	if err != nil {
		return err
	}
	return nil
}
