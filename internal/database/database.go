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

	if dbUser == "" { dbUser = "root" }
	if dbHost == "" { dbHost = "127.0.0.1" }
	if dbPort == "" { dbPort = "3306" }
	if dbName == "" { dbName = "flowwithlit_db" }

	// Format: user:pass@tcp(host:port)/?charset=utf8mb4&parseTime=True&loc=Local
	serverDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local", dbUser, dbPass, dbHost, dbPort)
	
	tempDB, err := gorm.Open(mysql.Open(serverDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to MySQL server: %v", err)
	}

	// Create the flowwithlit_db database if it doesn't already exist
	err = tempDB.Exec("CREATE DATABASE IF NOT EXISTS flowwithlit_db").Error
	if err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}

	// Now connect directly to the target database
	dbDSN := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := gorm.Open(mysql.Open(dbDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to flowwithlit_db: %v", err)
	}

	DB = db
	log.Println("🚀 Successfully connected to MySQL Database (flowwithlit_db)!")

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
	)
	if err != nil {
		log.Fatalf("Failed to auto-migrate database: %v", err)
	}
	log.Println("✅ Database tables migrated successfully!")
	SeedReferenceData()
	BackfillFlowTagUsernames()
}
