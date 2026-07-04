# Flowwithlit Backend Architecture & Development Plan

## 1. Executive Summary
This document outlines the architectural blueprint and development phases for the Flowwithlit backend. Built with **Golang (Go)**, the system is designed to be highly concurrent, secure, and scalable. It acts as the central hub connecting the web dashboard, mobile apps (iOS & Android), e-commerce plugins (WordPress/Shopify), and external developer integrations via APIs.

Because Flowwithlit operates as a **hybrid Payment Gateway, Neobank, and Crypto Exchange**, this backend must support all current and future features of a massive fintech ecosystem—even for UI screens that have not been designed yet.

---

## 2. Exhaustive Feature Scope & Domains

### A. Core Banking Engine (Ledger & Wallets)
- **Double-Entry Accounting System**: Every transaction must have matching debits and credits to ensure absolute financial accuracy.
- **Instant Payout Availability**: Funds deposited or received via payment gateways will be immediately available in the user's Total Balance for easy and instant payouts (No T+1 settlement delay), provided the user has passed strict KYC requirements.
- **Strict KYC & Verification Engine**: Before any payouts or major features are enabled, users MUST complete identity verification.
  - Integration with Identity Verification Providers (e.g., **Smile Identity, Dojah, or QoreID**).
  - Mandatory **BVN (Bank Verification Number)** and **NIN (National Identity Number)** checks for Nigerian users.
- **Multi-Currency Wallets**: Independent balances for Fiat (NGN, USD) and Crypto (USDT, BTC).
- **E-Transfer Engine**: Logic for generating and validating "Claim Access Keys" for secure P2P transfers.
*   **Tiered KYC & AML Engine**: Dynamic transaction limits and feature gating based on user KYC levels (Tier 1, 2, 3).
*   **Virtual Account Engine (NUBAN/IBAN)**: Generating dynamic and static accounts for users to receive funds.
    *   *Phase 1*: Integrated via Palmpay/Flutterwave.
    *   *Phase 2*: Flowwithlit proprietary core banking implementation once licensed.
*   **Bulk Disbursements & Salary Engine**: Processing massive lists of payouts via Go routines.
*   **Vaults & Savings**: Time-locked savings accounts with automated interest calculation.
*   **Escrow Infrastructure**: Holding funds securely for P2P transactions or marketplace trades.

### B. Crypto Exchange & Infrastructure
*   **Hot & Cold Wallet Management**: Secure infrastructure for managing private keys (e.g., HashiCorp Vault or AWS KMS).
*   **Multi-Chain Integration**: Direct RPC node connections for BTC, ETH, TRON (TRC-20), Solana, and BSC.
*   **Swap & Exchange Engine**: Real-time conversion between Fiat<->Crypto and Crypto<->Crypto using integrated liquidity providers or price oracles (e.g., Binance API).
*   **Crypto Payment Gateway**: Enabling merchants to accept crypto for invoices and instantly settle in Fiat.
*   **On-Chain AML Screening**: Integration with blockchain analytics to block tainted funds.

### C. Payment Gateway & Merchant Services
*   **Checkout & Payment Links**: Endpoints to generate hosted checkout URLs.
*   **Developer API & SDKs**: Comprehensive REST API for PHP, Node.js, Java, and Python developers.
*   **Webhooks Dispatcher**: Highly reliable, Redis-backed retry engine to fire `payment.success` events to WordPress/Shopify plugins and custom backends.
*   **Subscriptions & Recurring Billing**: Cron-based engine to auto-charge cards or deduct from wallet balances.
*   **Split Payments & Marketplaces**: Routing a single transaction to multiple sub-merchant accounts instantly.
*   **Invoicing**: Auto-generating PDF invoices and sending payment reminders.
*   **Disputes & Chargebacks**: API for merchants to respond to flagged transactions.

### D. Card Services (Issuing)
*   **Virtual Cards (Visa/Mastercard)**: APIs to issue, fund, freeze, and terminate USD/NGN virtual cards.
*   **Physical Card Management**: Infrastructure to handle physical card requests, delivery tracking, and activation.
*   **3D Secure & Transaction Limits**: Cardholder controls.

### E. Security, Fraud, & Operations
*   **Authentication & Session Management**: JWT, OAuth2, Biometric token handling, and 2FA (Authenticator App/SMS).
*   **Fraud Rules Engine**: Real-time velocity checks and anomaly detection (e.g., blocking impossible travel logins).
*   **Role-Based Access Control (RBAC)**: For corporate accounts (Maker/Checker approval workflows).
*   **Backoffice Admin API**: Extensive endpoints for Flowwithlit staff to manage users, override disputes, and monitor liquidity.

---

## 3. Technology Stack & Architecture
*   **Language**: Go (Golang) v1.21+
*   **Routing Framework**: `chi` or `gin` (lightweight and fast).
*   **Database**: **PostgreSQL** (ACID compliance is mandatory for financial ledgers).
*   **Caching & Queues**: **Redis** (for rate limiting, caching exchange rates, and background job queues).
*   **Documentation**: Swagger / OpenAPI (Crucial for SDK developers).
*   **Deployment**: Docker, CI/CD pipeline targeting AWS ECS or Kubernetes.
*   **Architecture Pattern**: Modular Monolith transitioning to Microservices (Auth, Ledger, Crypto, Gateway, Notification).

---

## 4. Recommended Project Layout (Standard Go Structure)
```text
flowwithlit-backend/
├── cmd/
│   ├── api/            # Main REST API server entrypoint
│   └── worker/         # Background workers (webhooks, crypto node listeners, interest calculators)
├── internal/
│   ├── auth/           # JWT, 2FA, API Key logic
│   ├── ledger/         # Core banking ledger & double-entry math
│   ├── gateway/
│   │   ├── fiat/       # Palmpay, Flutterwave integrations
│   │   └── crypto/     # Tron, BTC network RPC integrations
│   ├── cards/          # Virtual/Physical card issuing logic
│   ├── merchant/       # Invoicing, payment links, split payments
│   ├── models/         # Database structs and DB queries
│   └── webhooks/       # Dispatcher for WooCommerce/Shopify/Custom integrations
├── pkg/                # Reusable utilities (logger, error handling, math/decimal)
├── docs/               # Swagger documentation
├── go.mod
└── Makefile
```

---

## 5. Phased Development Plan

### Phase 1: Core Foundation & Neobank Ledger
1. **Initialize Project & Database**: Setup PostgreSQL schemas with strict decimal precision for balances.
2. **Auth & Security**: User registration, KYC verification, JWT, and 2FA.
3. **Ledger Engine**: Build the double-entry accounting system to handle internal transfers and balance checks.
4. **Fiat Integration**: Connect Flutterwave/Palmpay to generate static virtual accounts for deposits and handle outgoing bank transfers.

### Phase 2: Crypto Infrastructure & Swaps
1. **Node Integration**: Connect Tron (TRC-20) and BTC nodes. Setup wallet generation per user.
2. **Blockchain Listeners**: Build background workers to detect incoming blockchain transactions and credit the internal ledger.
3. **Swap Engine**: Integrate price feeds to allow users to swap NGN <-> USDT <-> BTC instantly.

### Phase 3: Payment Gateway & Developer Ecosystem
1. **Merchant API Keys**: Allow businesses to generate live/test API keys.
2. **Checkout APIs**: Build endpoints for Payment Links and E-commerce checkout sessions.
3. **Webhook Engine**: Develop the robust retry mechanism for notifying WordPress/Shopify plugins of successful payments.
4. **SDKs**: Generate the public API documentation (Swagger) to build the PHP/Node.js SDKs.

### Phase 4: Advanced Fintech Features
1. **Virtual Cards**: Integrate with card issuers to allow users to create and fund USD/NGN cards from their balances.
2. **Vaults & Bulk Processing**: Implement time-locked savings interest calculators and batch processing for salary disbursements.
3. **Corporate Workflows**: Add Maker/Checker rules for business accounts.
4. **Bank Independence**: Shift the fiat account generation engine from third-party aggregators to proprietary Flowwithlit infrastructure.
