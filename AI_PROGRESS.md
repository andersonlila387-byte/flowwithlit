# Flowwithlit — AI / Dev Progress Record

**Last updated:** 2026-07-26  
**Owner context:** Phase 1 NGN bank rail (OnePipe) — **server-side only**, never brand/expose vendor to public site or mobile.

Use this file to resume work. Update the **Progress log** at the bottom when you change anything.

### 2026-07-26 — Scan + biometrics + refresh

- Fixed `POST /auth/refresh` (was stub) — validates refresh JWT, rotates session token.
- Frontend: `GAP_SCAN.md` full gap list; web biometrics (Settings enable + login fingerprint via WebAuthn + existing `/user/biometric/*` APIs).
- Earlier: Family / bills / admin terminal / crypto wallet / checkout method locks.

### 2026-07-26 — Family / bills / admin terminal

- `internal/admin/activity.go`: `limit` query (1–500) for terminal copy; search includes `meta`; areas include `family`.
- Frontend (mirror): Family page UX, `app/services.php` bills UI, `admin/terminal.php` Railway-style logs.

---

## Hard product rules (do not violate)

1. **No vendor branding on public surfaces** — no OnePipe.js, no “Powered by OnePipe/Flutterwave/PalmPay/Circle” on app, demo, checkout, marketing, mobile.
2. **All bank-rail / KYC vendor calls are server-side only** — API keys stay in Admin DB / env; browser never calls vendor APIs.
3. **Admin** may name vendors (settings, webhook logs) for ops.
4. Customer-facing errors must be sanitized (no vendor names).

---

## Repos

| Repo | Path |
|------|------|
| Backend (Go API) | `C:\Users\DELL\Documents\flowwithlit-backend` |
| Frontend (PHP) | `C:\xampp\htdocs\flowwithlit-frontend` |
| Frontend progress mirror | `C:\xampp\htdocs\flowwithlit-frontend\AI_PROGRESS.md` |

Production API: `https://api.flowwithlit.com`  
Webhook: `https://api.flowwithlit.com/webhooks/onepipe`  
Checkout: `https://checkout.flowwithlit.com`  
App: `https://app.flowwithlit.com`

---

## Phase 1 goal (current focus)

```
KYC BVN/NIN (server) → open virtual account (server) → show bank details to user
→ customer transfers → webhook → credit wallet
```

Optional later: disburse payouts, OnePipe.js (rejected for public product branding).

---

## Done ✅

### Backend

| Item | Location | Notes |
|------|----------|--------|
| OnePipe HTTP client | `internal/integration/onepipe/client.go` | `POST /v2/transact`, MD5 sig `request_ref;secret` |
| `open_account` | same | `OpenVirtualAccount`, `GenerateVirtualAccount` |
| BVN/NIN lookup | same | `LookupBVN`, `LookupNIN` (`lookup_bvn_mid` / `lookup_nin_mid`) |
| Disburse stub live call | same | `ProcessTransfer` → `disburse` (kobo) |
| Settings client wiring | `internal/settings/loader.go` | `onepipe_api_key`, `onepipe_secret`, `onepipe_auth_provider`, `onepipe_mock_mode` |
| Bank rails resolve | `internal/bankrails/rails.go` | Uses `OpenVirtualAccount`; sanitizes public errors |
| Deposit accounts | `internal/wallet/deposit_accounts.go` | Creates VA; **strips provider** from user JSON |
| KYC OnePipe provider | `internal/kyc/strategy.go` | `OnePipeProvider`; name returned as `"internal"` |
| KYC activate | `internal/kyc/handler.go` | Fills user name/phone/DOB; on approve → `EnsureDefaultDepositAccount`; no `provider_used` in response |
| Admin KYC approve | `internal/admin/kyc.go` | Already called `EnsureDefaultDepositAccount` |
| Webhook credit | `internal/webhook/webhook.go` | Match `deposit_accounts.account_number` → `FundWallet`; MD5/HMAC soft verify |
| Checkout bank details | `internal/checkout/handler.go` | No vendor names in response |
| **Mobile personal KYC** | `internal/kyc/mobile.go` | `GET/POST /kyc/mobile/*` — BVN/NIN verify → deposit VA |
| Reprovision VA | `wallet.ReprovisionDefaultDepositAccount` | Replaces mock/stale default account after keys work |
| Deposit API hygiene | `deposit_accounts.go`, `wallet/handler.go` | No provider field; crypto optional on deposit-details |
| Build | `go build ./cmd/api` | Succeeded 2026-07-26 (incl. mobile KYC) |

### Frontend

| Item | Location | Notes |
|------|----------|--------|
| Charge URL `.php` fix | `includes/url_helpers.php` | Production was 404 on `/charge` |
| Charge error handling | `checkout/index.php` | Better non-JSON errors |
| Add Fund UI | `app/add-fund.php` | No “Powered by {provider}” |
| Admin OnePipe fields | `admin/settings.php` | auth_provider, mock_mode, keys, webhook URL note |
| KYC provider option | `admin/settings.php` | “NGN bank rail BVN/NIN…” |
| Docs marketing | `documentation.php` | Removed public OnePipe mention |
| Merchant docs accuracy | `doc/*` | status envelope, popup not iframe, etc. |

### Docs / reference (local)

- `OnePipe v2 - Documentation.html` (+ `_files/`) in frontend root (saved Postman docs dump)
- Official: https://v2.docs.onepipe.io/
- `key-get.md` — where to put keys (update when auth_provider fields mature)

---

## Left to complete before “done + deploy all at once” 🔲

### A. Config you must set (not code)

| # | Task | Who | Status |
|---|------|-----|--------|
| A1 | Confirm host-bank **auth_provider** string (e.g. `PolarisVirtual`, `FidelityVirtual`) | You / OnePipe | ⬜ |
| A2 | Admin → save API key, secret, **auth_provider**, mock_mode=`inspect` then `live` | You | ⬜ |
| A3 | Admin → KYC provider = NGN bank rail / onepipe | You | ⬜ |
| A4 | Admin → NGN rail = onepipe | You | ⬜ |
| A5 | Partner dashboard: webhook `https://api.flowwithlit.com/webhooks/onepipe` | You | ⬜ |
| A6 | Confirm whether BVN needs encryption for `auth.secure` on **your** product | You / OnePipe | ⬜ |

### B. Deploy

| # | Task | Status |
|---|------|--------|
| B1 | Deploy **backend** Go API (Railway) with latest `onepipe` + webhook + kyc + bankrails | ⬜ |
| B2 | Deploy **frontend** files listed below | ⬜ |
| B3 | Smoke test production after deploy | ⬜ |

**Backend deploy:** entire `flowwithlit-backend` repo (or Railway git push).

**Frontend files to upload (minimum for this phase):**

```
includes/url_helpers.php
checkout/index.php
app/add-fund.php
admin/settings.php
documentation.php
doc/checkout.php
doc/quickstart.php
doc/index.php
doc/errors.php
doc/verify.php
doc/api-reference.php
doc/test-cards.php
AI_PROGRESS.md
```

(Optional: do not upload `OnePipe v2 - Documentation.html` to public web root if you want zero vendor footprint on CDN — keep local only.)

### C. Data cleanup

| # | Task | Status |
|---|------|--------|
| C1 | Remove/replace old mock `deposit_accounts` rows so users don’t see fake numbers | ⬜ |
| C2 | Re-run KYC approve or hit Add Fund to mint **new** VAs | ⬜ |

```sql
-- Review first, then delete mocks if pattern known
SELECT id, user_id, account_number, bank_name, provider, created_at FROM deposit_accounts ORDER BY id DESC LIMIT 50;
```

### D. Code / product still incomplete (next phases)

| # | Item | Priority | Notes |
|---|------|----------|--------|
| D1 | **Live end-to-end test** open_account with real keys | P0 | Until A1–A6 + B done, unknown if payload matches host |
| D2 | **BVN OTP / encrypt secure** handling | P1 | If provider returns `WaitingForOTP`, UX or encrypt tools needed |
| D3 | **Mobile personal KYC** endpoints (BVN only, no business form) | ✅ | Built: `/kyc/mobile/verify`, `/status`, `/ensure-deposit` |
| D4 | Wire **transfer payouts** UI → `ProcessTransfer` / disburse | P2 | Client method exists; transfer handler may still need full wiring |
| D5 | Checkout bank path depends on VA generation for **merchant** user | P1 | Uses merchant profile names; needs working open_account |
| D6 | Amount unit double-check on webhook (kobo vs naira) | P1 | Currently amount/100 for NGN — verify first live notify |
| D7 | Idempotency / double-credit edge cases under concurrent webhooks | P2 | `FundWallet` uses reference unique |
| D8 | Flutterwave VA / Circle still stubs | P2 | Out of phase 1 scope |
| D9 | OnePipe.js public embed | ❌ | **Do not implement** on product (branding rule) |
| D10 | Remove public vendor mentions left in admin-only is OK; scrub `key-get.md` if ever public | P3 | |

### E. Suggested smoke test checklist (post-deploy)

1. Admin settings save OnePipe keys + auth_provider + mock `inspect`.
2. Call (server logs): open account via KYC approve or Add Fund for a test user.
3. `GET /public/bank-details?key=...` returns account_number (not 500).
4. App Add Fund shows bank + account, **no** provider brand.
5. Small live transfer → webhook log in Admin → Webhooks → wallet balance up.
6. Demo card payment still works (Flutterwave/test path).
7. Confirm no `js.onepipe.io` or vendor name on demo/checkout HTML source.

---

## Architecture (server-side only)

```
Browser / Mobile
    │
    ▼
Flowwithlit PHP / App  ──JWT──►  Flowwithlit Go API
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                 ▼                 ▼
              KYC lookup        open_account      disburse (later)
                    │                 │
                    └────────► deposit_accounts
                                      ▲
Partner bank rail ──webhook───────────┘
                 (credits wallet)
```

**Never:** Browser → OnePipe.

---

## Key code map

| Concern | File |
|---------|------|
| Vendor HTTP | `internal/integration/onepipe/client.go` |
| Pick NGN rail | `internal/bankrails/rails.go` |
| Persist VA | `internal/wallet/deposit_accounts.go` |
| Credit on deposit | `internal/webhook/webhook.go` → `wallet.FundWallet` |
| KYC | `internal/kyc/strategy.go`, `handler.go` |
| Admin approve | `internal/admin/kyc.go` |
| Checkout VA | `internal/checkout/handler.go` → `BankDetailsHandler` |
| Settings | `internal/settings/loader.go` + Admin PHP settings |

Admin setting keys:

- `onepipe_api_key`
- `onepipe_secret`
- `onepipe_webhook_secret`
- `onepipe_auth_provider` (default fallback in code: `PolarisVirtual`)
- `onepipe_mock_mode` (`live` | `inspect`)
- `ngn_bank_provider` (`onepipe` | `palmpay`)
- `kyc_provider` (`onepipe` | `flutterwave` | `smileid` | `mock`)

---

## Known risks

1. **auth_provider wrong** → open_account fails with generic public error; check Railway logs for raw message.
2. **WaitingForOTP** on BVN/open → auto-approve won’t complete; user goes pending manual review.
3. **Old mock VAs** still in DB confuse “is this real?” — clean C1.
4. **Webhook amount units** — wrong scale = wrong wallet credit; verify first notify.
5. **Frontend charge 404** was fixed in `url_helpers.php` — must be deployed or demo pay fails.

---

## Progress log

| Date | What happened |
|------|----------------|
| 2026-07-26 | Diagnosed demo payment 404 on `/charge`; fixed `url_helpers.php` + checkout charge error handling. |
| 2026-07-26 | Audited docs; fixed popup vs iframe, `status` envelope, options. |
| 2026-07-26 | Confirmed Add Fund numbers were stale/mock; open_account was stub. |
| 2026-07-26 | Read OnePipe v2 docs + saved HTML collection; mapped open_account, BVN, webhooks. |
| 2026-07-26 | **Phase 1 code:** full onepipe client, KYC, VA create, webhook credit, hide public vendor branding. Build OK. |
| 2026-07-26 | Created this `AI_PROGRESS.md` handoff. |
| 2026-07-26 | **Mobile KYC:** `POST /kyc/mobile/verify`, `GET /kyc/mobile/status`, `POST /kyc/mobile/ensure-deposit` — BVN/NIN → VA; `ReprovisionDefaultDepositAccount` for mock cleanup; strip provider from deposit-details; crypto optional. Build OK. |
| 2026-07-26 | **Transfers:** BanksHandler no longer dies on missing FLW key (static NIBSS list + OnePipe/FLW when available). Name enquiry FLW → OnePipe. UI bank-first + show account name + bank. FLW ProcessTransfer live POST /v3/transfers. Frontend transfers.php UX rewrite. |
| 2026-07-26 | **Unified web/mobile account:** Login + GET /user/me include has_transaction_pin + balances; GET /user/mobile/home for app home (same PIN + wallets as business web). |
| 2026-07-26 | **Transfers honesty UX:** No fake crypto balance/success; crypto marked Soon; real NGN balance; success only on API ok; fancy in-modal error step; create_bank_transfer normalizes status. |
| 2026-07-26 | **Transaction PIN security:** Fixed `has_transaction_pin` path in auth_init (`data.has_transaction_pin`); SetupPIN bcrypt-only (no password field touch); confirm + current PIN on change; weak PIN block; rate limit; settings UI confirm fields. |
| 2026-07-26 | **Checkout test vs live:** Test keys → simulated bank VA (`Test Bank`, `000…`), auto bank confirm, no real rails/money. Live keys → real deposit account / open_account + real card charge. Crypto only in test (sim) or hidden live. |
| 2026-07-26 | **Mobile tracking + vaults:** `GET /user/mobile/home` includes `tracking` (progress ring steps: email/phone/pin/kyc/deposit/biometric), `recent_activity`, `quick_actions`. Vaults list always `[]` when empty (no null/demo). |

---

## Instructions for the next AI

1. Read this file + frontend `AI_PROGRESS.md`.
2. Do **not** add OnePipe.js or public vendor branding.
3. Prefer fixing live open_account / webhook with real keys over new features.
4. After any meaningful change: append a row to **Progress log** and flip ⬜→✅ for completed tasks.
5. If implementing mobile KYC, reuse `OnePipeProvider` / same deposit account creation path.
6. Deploy backend **before** expecting production VA/webhook to work; frontend-only deploy is insufficient.
