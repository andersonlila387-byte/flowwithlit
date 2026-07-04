package checkout

import (
	"fmt"
	"log"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/internal/referral"
	"flowwithlit/internal/settlement"
	"flowwithlit/internal/wallet"
	"flowwithlit/pkg/email"
)

// recordCheckoutPayment stores the payment and credits the merchant's default settlement wallet.
func recordCheckoutPayment(
	userID uint,
	ref string,
	amountMajor float64,
	currency string,
	isTest bool,
	customer string,
	description string,
	meta map[string]interface{},
) error {
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		return err
	}

	isCrypto := settlement.IsCryptoPayment(meta)
	paymentMethod := settlement.PaymentMethodLabel(meta)
	fiatDef, cryptoDef := settlement.UserDefaults(&user)
	settledAmt, settledCur := settlement.Settle(amountMajor, currency, isCrypto, fiatDef, cryptoDef)

	if description == "" {
		description = fmt.Sprintf(
			"Checkout from %s — %s %.2f converted to %s %.2f",
			customer, strings.ToUpper(strings.TrimSpace(currency)), amountMajor, settledCur, settledAmt,
		)
	}

	provider := ""
	if isTest {
		provider = "test"
	} else if meta != nil {
		if p, ok := meta["payment_provider"].(string); ok && strings.TrimSpace(p) != "" {
			provider = strings.TrimSpace(p)
		}
	}

	txn := models.Transaction{
		UserID:          userID,
		Reference:       ref,
		Amount:          amountMajor,
		Currency:        strings.ToUpper(strings.TrimSpace(currency)),
		SettledAmount:   settledAmt,
		SettledCurrency: settledCur,
		PaymentMethod:   paymentMethod,
		Type:            "checkout_payment",
		Status:          "successful",
		IsTest:          isTest,
		Provider:        provider,
		Customer:        customer,
		Description:     description,
	}
	if err := database.DB.Create(&txn).Error; err != nil {
		return err
	}

	// Sandbox payments must not inflate real wallet balances shown in admin.
	if !isTest && settledAmt > 0 && settledCur != "" {
		if err := wallet.CreditWalletBalance(userID, settledAmt, settledCur, isTest, ref); err != nil {
			return err
		}
		if !isTest {
			referral.ProcessLiveWalletCredit(userID, settledAmt, settledCur, ref, false)
		}
	}

	sendCheckoutPaymentEmails(&user, ref, amountMajor, currency, settledAmt, settledCur, customer, meta, isTest)
	return nil
}

func sendCheckoutPaymentEmails(
	merchant *models.User,
	ref string,
	checkoutAmount float64,
	checkoutCurrency string,
	settledAmt float64,
	settledCur string,
	customerEmail string,
	meta map[string]interface{},
	isTest bool,
) {
	customerEmail = strings.TrimSpace(customerEmail)
	if customerEmail == "" {
		return
	}

	customerName := customerDisplayName(customerEmail, meta)
	paymentMethod := checkoutPaymentMethodLabel(meta)
	paidAt := time.Now().Format("Jan 02, 2006 at 03:04 PM")
	checkoutCur := strings.ToUpper(strings.TrimSpace(checkoutCurrency))

	if err := email.SendPaymentReceiptCustomer(
		customerEmail, customerName, ref,
		checkoutAmount, checkoutCur, paymentMethod, paidAt,
		merchantDisplayName(merchant.ID), merchantSupportEmail(merchant.ID),
		isTest,
	); err != nil {
		log.Printf("checkout: customer receipt email failed (%s): %v", customerEmail, err)
	}

	merchantEmail := strings.TrimSpace(merchant.Email)
	if merchantEmail == "" {
		return
	}
	merchantFirst := strings.TrimSpace(merchant.FirstName)
	if merchantFirst == "" {
		merchantFirst = "there"
	}
	if err := email.SendPaymentReceivedMerchant(
		merchantEmail, merchantFirst, customerEmail, ref,
		checkoutAmount, checkoutCur, settledAmt, settledCur, isTest,
	); err != nil {
		log.Printf("checkout: merchant payment email failed (%s): %v", merchantEmail, err)
	}
}

func customerDisplayName(email string, meta map[string]interface{}) string {
	if meta != nil {
		if v, ok := meta["customer_name"].(string); ok {
			if name := strings.TrimSpace(v); name != "" {
				return name
			}
		}
	}
	email = strings.TrimSpace(email)
	if at := strings.Index(email, "@"); at > 0 {
		part := strings.TrimSpace(email[:at])
		if part != "" {
			return part
		}
	}
	return "Customer"
}

func checkoutPaymentMethodLabel(meta map[string]interface{}) string {
	if meta == nil {
		return "Online payment"
	}
	pm, _ := meta["payment_method"].(string)
	switch strings.ToLower(strings.TrimSpace(pm)) {
	case "crypto":
		return "Cryptocurrency"
	case "bank":
		return "Bank transfer"
	case "card":
		return "Card"
	default:
		return "Online payment"
	}
}