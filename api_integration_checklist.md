# 🔌 Flowwithlit API Integration Checklist

Now that authentication (Sign Up, Login, Forgot Password) is connected, the next major step is to wire up the core functionalities of the dashboard to the Golang backend.

Below is a comprehensive list of all the API endpoints we need to build on the Golang backend and connect to the PHP frontend, categorized by module.

> [!TIP]
> **What to connect next?** 
> We should start with **Onboarding / KYC** and **Wallet Balances**. Once a user has a balance and an active account, everything else (transfers, cards, swaps) becomes possible.

---

## 1. Onboarding & KYC (`onboarding.php` / `business-activation.php`)
Before a user can transact, they need to activate their account.
- `[ ]` **GET `/api/v1/user/onboarding-status`**: Check if user has completed KYC (determines if they are redirected to onboarding).
- `[ ]` **POST `/api/v1/user/kyc/individual`**: Submit personal identity details (Address, ID verification).
- `[ ]` **POST `/api/v1/user/kyc/business`**: Submit company details (Registration number, Directors, Documents).

## 2. Core Dashboard (`index.php`)
The main dashboard requires a quick snapshot of the user's financial state.
- `[ ]` **GET `/api/v1/wallets/summary`**: Fetch total balance across all currencies (converted to a base currency like USD) and percentage changes.
- `[ ]` **GET `/api/v1/transactions/recent?limit=5`**: Fetch the 5 most recent activities for the dashboard feed.
- `[ ]` **GET `/api/v1/analytics/spending`**: Fetch data points for the dashboard spending/income charts.

## 3. Wallets & Crypto Swap (`balances-swap.php`)
Handling fiat and crypto wallets.
- `[ ]` **GET `/api/v1/wallets`**: List all active wallets (USD, EUR, NGN, BTC, USDT, etc.) with their specific balances.
- `[ ]` **GET `/api/v1/rates?from=BTC&to=USD`**: Fetch real-time exchange rates for the swap calculator.
- `[ ]` **POST `/api/v1/wallets/swap`**: Execute a conversion between two wallets (e.g., BTC to USD).

## 4. Transfers & Payments (`transfers.php` / `flowtags.php`)
Moving money internally and externally.
- `[ ]` **GET `/api/v1/flowtags/search?q={tag}`**: Lookup a user by their `@flowtag` for instant internal transfers.
- `[ ]` **POST `/api/v1/transfers/internal`**: Send money to another Flowwithlit user instantly via Flowtag.
- `[ ]` **POST `/api/v1/transfers/bank`**: Initiate a local bank payout.
- `[ ]` **POST `/api/v1/transfers/international`**: Initiate a SWIFT/Wire transfer.

## 5. Virtual Cards (`virtual-cards.php`)
Card issuing and management.
- `[ ]` **GET `/api/v1/cards`**: List all active/inactive virtual cards.
- `[ ]` **POST `/api/v1/cards/create`**: Issue a new virtual USD card.
- `[ ]` **POST `/api/v1/cards/{id}/fund`**: Move money from the main wallet to a specific card.
- `[ ]` **POST `/api/v1/cards/{id}/freeze`**: Temporarily block a card.

## 6. Commerce (Invoices & Payment Links)
For business users getting paid.
- `[ ]` **POST `/api/v1/payment-links`**: Create a new reusable payment link.
- `[ ]` **GET `/api/v1/payment-links`**: List generated links and their total collection amounts.
- `[ ]` **POST `/api/v1/invoices`**: Generate and email an invoice to a customer.

## 7. Advanced Tools (Vaults & Bulk Salary)
- `[ ]` **POST `/api/v1/vaults`**: Create a savings vault with a target goal.
- `[ ]` **POST `/api/v1/vaults/{id}/deposit`**: Lock funds into a vault.
- `[ ]` **POST `/api/v1/transfers/bulk`**: Upload a payload (parsed from CSV) to execute batch payouts (payroll).

## 8. Settings & Developer (`settings.php` / `developer-api.php`)
- `[ ]` **GET `/api/v1/user/profile`**: Fetch current profile settings.
- `[ ]` **PUT `/api/v1/user/profile`**: Update personal details or security settings (2FA).
- `[ ]` **POST `/api/v1/developer/keys`**: Generate a new API key pair for WooCommerce/Shopify integration.
- `[ ]` **POST `/api/v1/developer/webhooks`**: Set webhook endpoint URLs.
