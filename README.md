# flowwithlit

FlowWithLit Go API backend — payments, wallets, KYC, checkout, webhooks.

## Requirements

- Go 1.23+
- MySQL 8+

## Local run

```bash
cp .env.example .env
# Edit .env (DB, JWT_SECRET, MAIL_DISPATCH_URL)
go run ./cmd/api
```

Health check: `http://localhost:8081/ping`

## Deploy (Render)

- Connect this repo as a **Docker** web service (`Dockerfile` included).
- Set env vars from `.env.example` (`ENVIRONMENT=production`, `DB_HOST`, `MAIL_DISPATCH_URL`, etc.).
- PHP mail runs on Hostinger: `https://flowwithlit.com/mail/dispatch.php`

## Secrets

Never commit `.env`. Use `.env.example` as a template.