package referral

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settings"
	walletPkg "flowwithlit/internal/wallet"
)

type Config struct {
	Enabled             bool    `json:"enabled"`
	RewardAmount        float64 `json:"reward_amount"`
	RewardCurrency      string  `json:"reward_currency"`
	MinQualifyingAmount float64 `json:"min_qualifying_amount"`
}

func LoadConfig() Config {
	enabled := strings.ToLower(settings.Get("referral_enabled")) != "false"
	amount, _ := strconv.ParseFloat(settings.Get("referral_reward_amount"), 64)
	if amount <= 0 {
		amount = 500
	}
	minQ, _ := strconv.ParseFloat(settings.Get("referral_min_qualifying_amount"), 64)
	if minQ <= 0 {
		minQ = 1000
	}
	cur := strings.ToUpper(strings.TrimSpace(settings.Get("referral_reward_currency")))
	if cur == "" {
		cur = "NGN"
	}
	return Config{
		Enabled:             enabled,
		RewardAmount:        amount,
		RewardCurrency:      cur,
		MinQualifyingAmount: minQ,
	}
}

func GenerateCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "FWL" + strings.ToUpper(hex.EncodeToString(b)), nil
}

func EnsureUserCode(userID uint) (string, error) {
	var user models.User
	if err := database.DB.Select("id", "referral_code").First(&user, userID).Error; err != nil {
		return "", err
	}
	if strings.TrimSpace(user.ReferralCode) != "" {
		return user.ReferralCode, nil
	}
	for i := 0; i < 8; i++ {
		code, err := GenerateCode()
		if err != nil {
			return "", err
		}
		var count int64
		database.DB.Model(&models.User{}).Where("referral_code = ?", code).Count(&count)
		if count > 0 {
			continue
		}
		if err := database.DB.Model(&user).Update("referral_code", code).Error; err != nil {
			return "", err
		}
		return code, nil
	}
	return "", errors.New("could not generate referral code")
}

func AttachReferrer(refereeID uint, code string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil
	}

	var referrer models.User
	if err := database.DB.Where("referral_code = ?", code).First(&referrer).Error; err != nil {
		return nil // invalid code — ignore silently on signup
	}
	if referrer.ID == refereeID {
		return nil
	}

	var existing models.Referral
	if err := database.DB.Where("referee_id = ?", refereeID).First(&existing).Error; err == nil {
		return nil
	}

	ref := models.Referral{
		ReferrerID:   referrer.ID,
		RefereeID:    refereeID,
		ReferralCode: code,
		Status:       "pending",
	}
	if err := database.DB.Create(&ref).Error; err != nil {
		return err
	}
	rid := referrer.ID
	return database.DB.Model(&models.User{}).Where("id = ?", refereeID).Update("referred_by_id", rid).Error
}

// ProcessLiveWalletCredit checks if a live credit to referee wallet triggers a referral reward.
func ProcessLiveWalletCredit(refereeID uint, amount float64, currency string, txnRef string, isTest bool) {
	if isTest || amount <= 0 {
		return
	}
	cfg := LoadConfig()
	if !cfg.Enabled {
		return
	}

	var ref models.Referral
	if err := database.DB.Where("referee_id = ? AND status = ?", refereeID, "pending").First(&ref).Error; err != nil {
		return
	}

	var referee models.User
	if err := database.DB.Select("kyc_level").First(&referee, refereeID).Error; err != nil {
		return
	}
	if referee.KYCLevel < 1 {
		return
	}

	if amount < cfg.MinQualifyingAmount {
		return
	}

	reward := cfg.RewardAmount
	rewardCur := cfg.RewardCurrency
	refID := fmt.Sprintf("REF-EARN-%d-%d", ref.ReferrerID, ref.RefereeID)

	if err := walletPkg.CreditWalletBalance(ref.ReferrerID, reward, rewardCur, false, refID); err != nil {
		log.Printf("referral: failed to credit referrer %d: %v", ref.ReferrerID, err)
		return
	}

	now := time.Now()
	database.DB.Model(&ref).Updates(map[string]interface{}{
		"status":             "paid",
		"qualifying_amount":  amount,
		"reward_amount":      reward,
		"reward_currency":    rewardCur,
		"qualifying_txn_ref": txnRef,
		"paid_at":            &now,
	})

	_ = database.DB.Create(&models.Transaction{
		UserID:      ref.ReferrerID,
		Reference:   refID,
		Amount:      reward,
		Currency:    rewardCur,
		Type:        "referral_reward",
		Status:      "successful",
		Provider:    "internal",
		Description: fmt.Sprintf("Referral reward — user #%d first live deposit", refereeID),
	}).Error
}