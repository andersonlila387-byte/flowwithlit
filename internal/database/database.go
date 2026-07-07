package database

import (
	"fmt"
	"log"
	"os"

	"flowwithlit/internal/models"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DB is the global database connection instance
var DB *gorm.DB

// Connect initializes the MySQL database connection
func Connect() {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")

	if dbUser == "" {
		dbUser = "root"
	}
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	if dbPort == "" {
		dbPort = "3306"
	}
	if dbName == "" {
		dbName = "flowwithlit_db"
	}

	// Optional TLS for remote MySQL providers that require/prefer an encrypted
	// connection. Off by default to stay compatible with local dev and hosts
	// (like Hostinger) that don't need it. Set DB_TLS=true / skip-verify / preferred.
	tlsParam := ""
	if dbTLS := os.Getenv("DB_TLS"); dbTLS != "" {
		tlsParam = "&tls=" + dbTLS
	}

	// Format: user:pass@tcp(host:port)/?charset=utf8mb4&parseTime=True&loc=Local
	serverDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local%s", dbUser, dbPass, dbHost, dbPort, tlsParam)

	tempDB, err := gorm.Open(mysql.Open(serverDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to MySQL server: %v", err)
	}

	// Create the target database if it doesn't already exist. Many managed/shared
	// MySQL hosts (e.g. Hostinger) scope the app's DB user to a pre-provisioned
	// database and don't grant CREATE DATABASE — so this is best-effort and must
	// not crash the app when it's denied; we just proceed to connect directly.
	if err := tempDB.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", dbName)).Error; err != nil {
		log.Printf("⚠️  Could not create database %q (likely lacks CREATE privilege on a remote/managed host) — assuming it already exists: %v", dbName, err)
	}

	// Now connect directly to the target database
	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local%s", dbUser, dbPass, dbHost, dbPort, dbName, tlsParam)
	db, err := gorm.Open(mysql.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", dbName, err)
	}

	DB = db
	log.Printf("🚀 Successfully connected to MySQL Database (%s)!", dbName)

	// Auto-Migrate Models
	err = DB.AutoMigrate(
		&models.User{},
		&models.BusinessProfile{},
		&models.SystemSetting{},
		&models.AdminUser{},
		&models.Wallet{},
		&models.Transaction{},
		&models.FlowTag{},
		&models.Session{},
		&models.Customer{},
		&models.PaymentLink{},
		&models.Invoice{},
		&models.CheckoutSession{},
		&models.Notification{},
		&models.SystemProvider{},
		&models.Dispute{},
		&models.WebhookLog{},
		&models.SupportTicket{},
		&models.AuditLog{},
		&models.BroadcastMessage{},
		&models.VirtualCard{},
		&models.Vault{},
		&models.ApiCredentials{},
		&models.SecureTransfer{},
		&models.AdminInvite{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.OutboundWebhookLog{},
		&models.Country{},
		&models.Currency{},
		&models.CryptoAsset{},
		&models.ExchangeRate{},
		&models.RateChangeLog{},
		&models.FlowTagPaymentRequest{},
		&models.Referral{},
		&models.PayrollEmployee{},
		&models.PayrollSettings{},
		&models.PayrollRun{},
		&models.PayrollRunItem{},
		&models.DepositAccount{},
		&models.CryptoDepositAddress{},
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	log.Println("✅ Database tables migrated successfully!")
	SeedReferenceData()
	BackfillFlowTagUsernames()
}
