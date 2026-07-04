package wallet

import (
	"errors"
	"fmt"
	"strings"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/email"
	"gorm.io/gorm"
)

// EnsureWallet returns an existing wallet or creates one with zero balance.
func EnsureWallet(tx *gorm.DB, userID uint, currency string) (models.Wallet, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	var wallet models.Wallet
	err := tx.Where("user_id = ? AND currency = ?", userID, currency).First(&wallet).Error
	if err == nil {
		return wallet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return wallet, err
	}
	wallet = models.Wallet{UserID: userID, Currency: currency, Balance: 0}
	if err := tx.Create(&wallet).Error; err != nil {
		return wallet, err
	}
	return wallet, nil
}

// CreditWalletBalance adds funds without creating a ledger row (checkout already recorded).
func CreditWalletBalance(userID uint, amount float64, currency string, isTest bool, txnRef string) error {
	if amount <= 0 {
		return nil
	}
	return database.DB.Transaction(func(tx *gorm.DB) error {
		wallet, err := EnsureWallet(tx, userID, currency)
		if err != nil {
			return err
		}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&wallet, wallet.ID).Error; err != nil {
			return err
		}
		wallet.Balance += amount
		return tx.Save(&wallet).Error
	})
}

// FundWallet securely adds money to a user's wallet using database locks
func FundWallet(userID uint, amount float64, currency, provider, providerRef, desc string) error {
	if amount <= 0 {
		return errors.New("amount must be greater than zero")
	}

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var wallet models.Wallet
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND currency = ?", userID, currency).
			First(&wallet).Error; err != nil {
			return fmt.Errorf("wallet not found for currency %s", currency)
		}

		var existingTx models.Transaction
		if err := tx.Where("reference = ?", providerRef).First(&existingTx).Error; err == nil {
			return nil
		}

		wallet.Balance += amount
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		txn := models.Transaction{
			UserID:            userID,
			Reference:         providerRef,
			Amount:            amount,
			Fee:               0,
			BalanceAfter:      wallet.Balance,
			Currency:          currency,
			Type:              "deposit",
			Status:            "successful",
			Provider:          provider,
			ProviderReference: providerRef,
			Description:       desc,
		}

		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		return nil
	})
}

// DebitWallet securely removes money (e.g. for a bank withdrawal)
func DebitWallet(userID uint, amount float64, fee float64, currency, provider, internalRef, desc string) error {
	if amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	totalDeduct := amount + fee

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var wallet models.Wallet
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND currency = ?", userID, currency).
			First(&wallet).Error; err != nil {
			return errors.New("wallet not found")
		}

		if wallet.Balance < totalDeduct {
			return errors.New("insufficient balance")
		}

		// Debit balance
		wallet.Balance -= totalDeduct
		if err := tx.Save(&wallet).Error; err != nil {
			return err
		}

		txn := models.Transaction{
			UserID:       userID,
			Reference:    internalRef,
			Amount:       amount,
			Fee:          fee,
			BalanceAfter: wallet.Balance,
			Currency:     currency,
			Type:         "withdrawal",
			Status:       "successful",
			Provider:     provider,
			Description:  desc,
		}

		if err := tx.Create(&txn).Error; err != nil {
			return err
		}

		return nil
	})
}

// InternalTransfer securely moves money between two users
func InternalTransfer(senderID, recipientID uint, amount float64, currency, ref, desc string) error {
	if amount <= 0 {
		return errors.New("amount must be greater than zero")
	}
	if senderID == recipientID {
		return errors.New("cannot transfer to self")
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// Enforce ordering to avoid deadlocks: Always lock lower ID first
		var firstID, secondID uint
		if senderID < recipientID {
			firstID, secondID = senderID, recipientID
		} else {
			firstID, secondID = recipientID, senderID
		}

		var firstWallet, secondWallet models.Wallet
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", firstID, currency).First(&firstWallet).Error; err != nil {
			return err
		}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("user_id = ? AND currency = ?", secondID, currency).First(&secondWallet).Error; err != nil {
			return err
		}

		var senderWallet, recipientWallet *models.Wallet
		if senderID == firstID {
			senderWallet = &firstWallet
			recipientWallet = &secondWallet
		} else {
			senderWallet = &secondWallet
			recipientWallet = &firstWallet
		}

		if senderWallet.Balance < amount {
			return errors.New("insufficient balance")
		}

		// Execute transfer
		senderWallet.Balance -= amount
		recipientWallet.Balance += amount

		if err := tx.Save(senderWallet).Error; err != nil {
			return err
		}
		if err := tx.Save(recipientWallet).Error; err != nil {
			return err
		}

		// Log sender txn
		senderTxn := models.Transaction{
			UserID:       senderID,
			Reference:    ref + "_OUT",
			Amount:       amount,
			BalanceAfter: senderWallet.Balance,
			Currency:     currency,
			Type:         "transfer_out",
			Status:       "successful",
			Provider:     "internal",
			Description:  desc,
		}
		if err := tx.Create(&senderTxn).Error; err != nil {
			return err
		}

		// Log recipient txn
		recipientTxn := models.Transaction{
			UserID:       recipientID,
			Reference:    ref + "_IN",
			Amount:       amount,
			BalanceAfter: recipientWallet.Balance,
			Currency:     currency,
			Type:         "transfer_in",
			Status:       "successful",
			Provider:     "internal",
			Description:  desc,
		}
		if err := tx.Create(&recipientTxn).Error; err != nil {
			return err
		}

		return nil
	})

	if err == nil {
		var sender, recipient models.User
		database.DB.First(&sender, senderID)
		database.DB.First(&recipient, recipientID)
		if sender.ID != 0 {
			_ = email.SendTransferSent(sender.Email, sender.FirstName, recipient.Email, amount, currency)
		}
		if recipient.ID != 0 {
			_ = email.SendTransferReceived(recipient.Email, recipient.FirstName, sender.Email, amount, currency)
		}
	}

	return err
}
