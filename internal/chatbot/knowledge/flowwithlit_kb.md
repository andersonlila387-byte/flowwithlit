<!--
  FLOWWITHLIT KNOWLEDGE BASE — for Aria, the AI customer-support chatbot.

  PURPOSE: This file is the reference material Aria draws on to answer customer
  questions. It is retrieved at runtime by simple keyword matching, NOT embeddings —
  the retrieval code finds the paragraph whose text best matches the user's question,
  then grabs the surrounding lines of that section and injects just that snippet into
  the LLM's context window, citing "see line N". It does NOT load the whole file.

  BECAUSE OF THAT: every subsection below is written so a single paragraph, read in
  total isolation from the rest of the document, still makes sense on its own. Don't
  assume the reader has seen a paragraph before or after it. Headings (both "## " and
  "### " lines) double as a table of contents and as keyword anchors, and each one
  marks the start of a self-contained, independently retrievable chunk — keep them
  descriptive and topic-specific so keyword search can land on the right spot.

  Facts, figures, and code samples here are drawn directly from Flowwithlit's public
  marketing site and from the curated platform knowledge already used in the support
  chatbot's system prompt. Do not add numbers or feature claims beyond what is stated
  here.
-->

## Table of Contents

- Company Overview
- Products & Wallets
- Pricing & Fees
- KYC & Verification Levels
- Transfers & FlowTag
- Virtual Cards
- Vaults
- Security & Compliance
- Common Errors & Exact Fixes
- Developer API Reference
- Webhook Setup & Verification
- Plugins & Integrations
- Use Cases
- About/Company Info

## Company Overview

### What Flowwithlit Is

Flowwithlit is a financial technology platform that combines payments, mobile banking, and crypto
conversion into one product. The company's tagline positions it as "Banking. Payments. Crypto. One
Platform." The core pitch is that a merchant or individual can accept payments through e-commerce
integrations like WordPress and Shopify, convert crypto to fiat instantly, and manage all of it from
a single mobile banking app, rather than stitching together separate vendors for each function.

### The Core Ecosystem

The platform is organized around three core products that show up throughout the marketing site
and documentation: an Omnichannel Payment Gateway for accepting global payments, a Mobile Banking
App for managing fiat and crypto balances and issuing virtual cards, and a Crypto Converter Engine
for turning cryptocurrency payments into stable fiat. These three are presented as a unified
ecosystem rather than separate standalone tools — a merchant can use the gateway to accept a
payment, the converter to settle crypto into fiat, and the banking app to manage the resulting
balance, all under one account. Each of these three products is described in full detail in the
"Products & Wallets" section of this document.

### Published Scale Figures

Flowwithlit publishes a handful of headline figures on its site describing scale: 150 supported
countries, 40+ integrations and plugins, and $4.2B in processed volume. These are the company's own
published figures, not independently audited statistics, and should be presented to users as
Flowwithlit's stated numbers rather than as guaranteed or externally verified facts.

### Ecosystem Figures by Product

Within the product ecosystem breakdown shown on the site, the Payment Gateway reports roughly
$2.45B in processing volume, about 12,500 active merchants, and 50+ supported payment methods. The
Banking App reports roughly $1.2B in total managed assets, 15+ supported currencies, and roughly
50,000 active virtual cards. The Crypto Converter reports roughly $450M in 24-hour trading volume
across 50+ supported crypto assets, with average slippage cited at around 0.01%. As with the
headline figures above, these are Flowwithlit's own published dashboard-style statistics.

### Recent Launch: E-Commerce Plugins

Flowwithlit's e-commerce plugins for Shopify and WordPress/WooCommerce are described on the site as
a recent launch, tagged "Live Now." They let merchants accept fiat cards, bank transfers, and
crypto at checkout through a native plugin install rather than custom integration work. Full detail
on all six supported e-commerce platforms (not just Shopify and WooCommerce) is in the "Plugins &
Integrations" section of this document.

## Products & Wallets

### Omnichannel Payment Gateway

The Payment Gateway is Flowwithlit's tool for accepting payments globally. It supports 150+
currencies and local payment methods such as iDEAL and Alipay in addition to standard card
payments. An intelligent routing engine automatically selects the acquiring bank most likely to
authorize each given transaction, which is intended to improve authorization rates compared to
routing every transaction through a single fixed processor. The gateway includes AI-based fraud
prevention and full 3D Secure 2.0 compliance for card transactions. Flowwithlit states it is
trusted by merchants in 40+ countries.

### Gateway Integration Paths

Developers can integrate the Payment Gateway through the REST API directly, through a drop-in
checkout widget, or through one of the CMS plugins for Shopify, WooCommerce, Magento, BigCommerce,
Wix, or PrestaShop. Which path makes sense depends on the merchant: a custom-built storefront will
usually want the REST API, while a merchant already running one of those six e-commerce platforms
will usually want the matching plugin instead of building a custom integration. See "Developer API
Reference" for the API and "Plugins & Integrations" for the plugin-based paths.

### Mobile Banking App

The Mobile Banking App is available on iOS and Android and lets a user manage fiat and crypto
balances from one place, issue virtual cards, and send transfers. Two of the headline features
called out for the app are virtual cards with real-time limits and controls, and global transfers
described as settling via SEPA and SWIFT rails "in seconds." Inside the app, users hold separate
currency wallets rather than one blended balance — see "Wallets & Balances" directly below for how
that works, and "KYC & Verification Levels" for how verification level gates which features (like
virtual cards) are available to a given account.

### Crypto Converter Engine

The Crypto Converter Engine lets a merchant accept payment in cryptocurrency while receiving
stable fiat, removing exposure to crypto price volatility for the merchant. It supports conversion
across 50+ cryptocurrencies. Flowwithlit describes the underlying liquidity as deep enough to keep
slippage minimal (the site cites an average slippage figure of around 0.01%), and settlement
happens automatically on confirmation of the crypto payment rather than requiring a manual
conversion step from the merchant.

### Wallets & Balances

Inside a Flowwithlit account, users hold NGN, USD, and USDT wallets as three separate balances
rather than one blended balance. Because the wallets are separate, money sent to or held in one
currency does not automatically show up in another currency's balance. A very common source of
user confusion is a balance that appears to "show zero" when funds are actually present but
sitting in a different currency wallet than the one currently selected on screen — checking the
correct currency tab resolves this in the large majority of cases.

### Currency Swap Between Wallets

A currency swap feature lets a user convert funds between their NGN, USD, and USDT wallets
directly from within the dashboard, without needing an external exchange. The minimum swap amount
is ₦500 (or the equivalent value in another currency). Swap exchange rates refresh every 60
seconds; if a swap fails with a rate-related error, waiting briefly and retrying is the correct fix
rather than assuming something is broken — see "Common Errors & Exact Fixes" for the exact wording
of that error.

## Pricing & Fees

### Billing Cycle & Annual Discount

Flowwithlit's published API/platform plans can be billed monthly or annually, with the annual
option advertised at a 20% discount compared to paying month-to-month. There are three
subscription tiers — Startup, Growth, and Enterprise — described individually below, plus a
separate set of per-transaction fees that apply to using wallets and transfers inside the product
itself (covered later in this section), which are distinct from these subscription tiers.

### Startup Plan

The Startup plan is aimed at new businesses still finding product-market fit. It costs $49/month
billed monthly, or $39/month if billed annually. It includes up to 10,000 API calls per month,
standard support, and basic analytics. It supports a single user account, and does not include
custom webhooks or SSO authentication — those become available on higher tiers.

### Growth Plan

The Growth plan is Flowwithlit's "Most Popular" tier, aimed at scaling companies with more advanced
needs. It costs $199/month billed monthly, or $159/month if billed annually. It includes up to
100,000 API calls per month, priority support, advanced analytics and reporting, and team
collaboration for up to 10 users. Growth is the entry-level tier at which custom webhooks become
available, but it still does not include SSO authentication — that remains Enterprise-only.

### Enterprise Plan

The Enterprise plan is custom-priced for large organizations rather than having a fixed monthly
rate. Prospective customers submit company details — including an estimated monthly processing
volume band of $100k–$500k, $500k–$1M, or $1M+ — to request a quote from the sales team. Enterprise
includes unlimited API calls, unlimited user accounts, 24/7 dedicated support, custom integrations,
and an SLA guarantee. Enterprise is also the only tier that includes SSO authentication.

### Feature Comparison at a Glance

Comparing the three tiers directly: API rate limit is 10k/month on Startup, 100k/month on Growth,
and unlimited on Enterprise. User accounts are limited to 1 on Startup, up to 10 on Growth, and
unlimited on Enterprise. Custom webhooks are unavailable on Startup but included on both Growth and
Enterprise. SSO authentication is unavailable on both Startup and Growth, and is exclusive to
Enterprise.

### High-Volume and Custom Quotes

For merchants processing millions of transactions, Flowwithlit offers tailored pricing beyond the
standard three tiers: dedicated infrastructure, 24/7 premium support, and volume-based discounts
negotiated directly with an account manager. This is requested through the same Enterprise quote
flow described above, by selecting the appropriate monthly processing volume band.

### Wallet & Transfer Fees (Distinct from Subscription Plans)

Separate from the subscription/API plans above, using the wallet and transfer features inside the
product carries its own transaction-level fees. A standard bank transfer costs ₦50 per transfer and
typically processes in 2–5 minutes. Peer-to-peer transfers sent via FlowTag (by tag or email) are
instant and carry no fee at all. The minimum amount for a currency swap between wallets is ₦500 (or
the equivalent). Full detail on transfer types is in the "Transfers & FlowTag" section of this
document.

## KYC & Verification Levels

### Why KYC Levels Exist

Flowwithlit gates daily transaction limits, and certain features like virtual cards, behind KYC
(Know Your Customer) verification levels. There are three levels — 0, 1, and 2 — each unlocking a
higher daily transaction limit in exchange for providing more identity and business documentation.
Moving up a level is almost always the fix when a user hits a limit-related error message; see
"Common Errors & Exact Fixes" for the exact wording used in that case.

### KYC Level 0

Level 0 is the default, unverified state every account starts in. It is limited to ₦10,000 in
transaction volume per day. No additional documentation is required to remain at Level 0, but it
is also the most restrictive level and does not permit virtual card creation.

### KYC Level 1

Level 1 raises the daily transaction limit to ₦50,000 per day. To reach Level 1, a user must
submit their business name and a national ID. Level 1 is also the minimum KYC level required to
create a virtual card — an account still sitting at Level 0 cannot issue a card until it completes
Level 1 verification.

### KYC Level 2

Level 2 raises the daily transaction limit to ₦500,000 per day. To reach Level 2, a user must
submit CAC (Corporate Affairs Commission business registration) documents, director information,
and a bank statement, in addition to everything already required at Level 1. Level 2 is the level
a user needs to reach if they are hitting a "Transfer limit exceeded" error.

### How to Start or Continue KYC

The path inside the product to start or continue KYC verification at any level is: Dashboard →
Profile → Complete KYC. Users should be directed there regardless of which level they are trying to
reach next; the same screen handles submission for Level 1 and Level 2 documentation.

## Transfers & FlowTag

### Standard Bank Transfers

A standard bank transfer costs ₦50 per transfer and typically processes in 2–5 minutes. This is the
default way to send money to a bank account that isn't another Flowwithlit user's FlowTag. Before
sending a standard bank transfer to a new recipient, the account lookup / bank verification tool
should always be used first to confirm the receiving account name — see "Always Verify Before
Sending" below.

### Bulk Salary Payments

For paying many people at once — for example, running payroll — Flowwithlit supports bulk salary
payments via CSV upload. The CSV needs columns for bank_code, account_number, amount, and
description, letting a business pay an entire team in one batch rather than sending transfers one
at a time.

### FlowTag (Peer-to-Peer Transfers)

FlowTag is Flowwithlit's peer-to-peer transfer method: users send money to each other by tag or
email address rather than needing to enter full bank account details. FlowTag transfers are
instant and carry no fee, making FlowTag the cheapest and fastest way to move money between two
Flowwithlit users specifically (it does not apply to sending money to an external bank account
that isn't on Flowwithlit).

### Secure Transfer (Escrow)

Secure Transfer is an escrow-style transfer that uses an access key the recipient must present to
claim the funds. A Secure Transfer expires after 72 hours if the recipient never claims it. This is
a reasonable option to recommend when a user wants to send money to someone whose account details
they don't want to enter directly, or when they want the transfer to remain reversible/unclaimed
until the recipient actively takes an action to receive it.

### Always Verify Before Sending

Regardless of which transfer type is used, a user should always run the account lookup / bank
verification tool before sending money to a new recipient, to confirm the receiving account name
matches who they intend to pay. This lookup is powered by a live bank name-enquiry integration
(Flutterwave, for Nigerian bank accounts): given a bank code and account number, it returns the
verified account holder name. For example, looking up bank code "044" with account number
"0123456789" returns the account name "JOHN DOE" in the sample response. If a transfer fails with
"Invalid account number," the fix is to run this lookup first and paste the exact 10-digit NUBAN
rather than retyping it by hand.

## Virtual Cards

### Requirements to Create a Card

Virtual cards in Flowwithlit require KYC Level 1 as a minimum — an account that hasn't completed
at least Level 1 verification cannot create a card. Cards are issued in USD. If a user reports
"Card creation failed," the fix is to complete KYC Level 1 first (Dashboard → Profile → Complete
KYC) and then retry card creation — most creation failures trace back to insufficient KYC level
rather than a technical problem on Flowwithlit's side.

### Managing a Card: Fund, Freeze, Reveal

From the app, a user can fund a virtual card, freeze it, or reveal its full details (card number,
expiry, and CVV) when needed for a purchase. These three actions — fund, freeze, and reveal — are
the primary controls available on an issued card.

### Frozen Cards Cannot Be Charged

A frozen card cannot be charged for any transaction until it is unfrozen again. This is a
deliberate security control, not a bug — so if a user reports a declined charge on a card they
don't remember freezing, checking the freeze/unfreeze state of that specific card in the app is the
first troubleshooting step before looking for any other cause.

## Vaults

### How Vaults Work

Vaults are Flowwithlit's savings-lock feature: a user commits funds for a fixed duration — 30, 60,
or 90 days — and earns interest that scales with the duration chosen, meaning longer locks earn
more interest than shorter ones. This is the mechanism to point users toward when they ask about
earning interest or "locking" savings inside Flowwithlit.

### Withdrawing Before Maturity

Funds in a Vault cannot be withdrawn before the maturity date under normal circumstances. If a user
needs access to locked funds early, they can submit an early-unlock request, but this should be
treated as an exception path rather than the default behavior — users should be told plainly that
early withdrawal isn't guaranteed to be instant or penalty-free under the standard flow, since the
product is designed around funds staying locked for the chosen duration.

## Security & Compliance

### Overall Security Posture

Flowwithlit describes security as being built into every layer of the platform rather than treated
as an afterthought, with the stated goal of protecting merchant funds, customer data, and
transaction integrity to industry-leading standards.

### Compliance Certifications

Flowwithlit maintains PCI-DSS Level 1 certification, described as the highest level of payment
card data security. It also maintains SOC 2 Type II certification, an independent audit covering
security, availability, and confidentiality controls, and ISO 27001 certification, a certified
information security management system. AML/KYC practices are stated to align with FATF (Financial
Action Task Force) recommendations and applicable local regulations.

### Technical Security Features

On the technical side, Flowwithlit uses bank-grade AES-256 encryption for data both at rest and in
transit. It supports multi-factor authentication (MFA), including hardware security keys, and runs
real-time fraud detection powered by machine learning. The company states it undergoes regular
penetration testing and vulnerability assessments performed by independent firms. API-level
security specifically includes rate limiting, input validation, and OAuth 2.0. All transaction
activity is recorded in immutable audit logs, and transactions are continuously monitored.

### Data Protection & Privacy

Flowwithlit states it adheres to GDPR, CCPA, and other applicable data protection regulations.
Customer data is not sold, and the platform provides tools for users to request data access,
export, or deletion. All data is stored in SOC 2- and ISO-certified data centers, with
geo-redundancy options available for customers who need it.

### Third-Party Audits & Bank Partnerships

Flowwithlit's infrastructure and internal processes are said to be regularly audited by
third-party firms. The company also maintains partnerships with top-tier banks and payment
processors, and those partners themselves undergo their own rigorous compliance reviews
independent of Flowwithlit's own audits.

### Responsible Disclosure & Security Contact

Flowwithlit operates a responsible disclosure process for security researchers: vulnerabilities
can be reported by emailing security@flowwithlit.com. The company commits to acknowledging any
report within 48 hours and providing ongoing updates as remediation proceeds. That same address,
security@flowwithlit.com, is also the correct contact point for compliance documentation or audit
requests generally — if a user or partner asks how to reach the security team for any reason, that
is the email to give them.

## Common Errors & Exact Fixes

The entries below are the exact error messages users encounter in the Flowwithlit product, each
paired with the exact fix. When a user reports one of these messages verbatim, use the matching
fix rather than improvising a different explanation.

### "Insufficient balance"

If a user sees "Insufficient balance" despite believing they have funds, the fix is to check
they're on the correct currency tab. Funds may be sitting in the NGN wallet rather than the USD
wallet (or vice versa) — the wallets are separate, not blended, so a balance can look empty simply
because the wrong currency tab is selected.

### "KYC not completed"

If a user sees "KYC not completed" when trying to perform an action that requires verification,
the fix is to go to Profile → KYC and upload Business Name, National ID, and Bank Account details.
This error appears regardless of which KYC level is actually required for the action being
attempted.

### "PIN not set"

If a user sees "PIN not set," the fix is to go to Profile → Security → Set Transaction PIN. A
transaction PIN is required before any transfer can be sent, so this error will block every
transfer attempt until the PIN is configured.

### "Transfer limit exceeded"

If a user sees "Transfer limit exceeded," the fix is that the account needs to reach KYC Level 2.
Direct them to Profile → Complete Verification to submit the additional documentation (CAC,
directors, bank statement) required for Level 2.

### "Invalid account number"

If a user sees "Invalid account number" while trying to send a transfer, the fix is to use the
account lookup tool first, and paste the exact 10-digit NUBAN rather than retyping it by hand —
manual retyping is the most common source of this error.

### "Card creation failed"

If a user sees "Card creation failed," the fix is that KYC Level 1 must be completed first
(Dashboard → Profile → Complete KYC); retry creating the card afterward. Virtual cards cannot be
issued to an account below Level 1.

### "Swap rate error"

If a user sees a "Swap rate error" while trying to convert between wallets, the fix is to wait
briefly and try the swap again — exchange rates refresh every 60 seconds, and the error typically
means the rate the user was quoted has already expired.

### "Payment link expired"

If a user reports "Payment link expired," the fix is that payment links are valid for 30 days from
creation. A new one needs to be created from Commerce → Payment Links; expired links cannot be
reactivated.

### "Webhook not firing"

If a developer reports "Webhook not firing," the fix is to check three things in order: the
receiving URL must return HTTP 200, the SSL certificate on that URL must be valid, and any
firewall on the receiving side must not be blocking Flowwithlit's outbound IP. See "Webhook Setup
& Verification" for the full webhook reference.

### "Two-factor code invalid"

If a user reports "Two-factor code invalid," the fix is that two-factor codes expire after 30
seconds — have them make sure their phone clock is set to automatic/network time rather than
manual, since a clock drift is the most common cause, or have them try a backup code instead.

## Developer API Reference

### API Design Philosophy

Flowwithlit's API is described as being designed API-first: predictable, resource-oriented REST
endpoints, Bearer-token authentication, and standard JSON responses throughout. The company
advertises a global edge network aimed at delivering low-latency responses to API calls from
anywhere.

### Authentication & Key Modes

Every API request needs an `Authorization: Bearer YOUR_SECRET_KEY` header. Test keys are prefixed
`sk_test_` (secret) and `pk_test_` (publishable); live keys are prefixed `sk_live_` and
`pk_live_`. A user can switch between Test and Live mode from Dashboard → Developer API by toggling
Test/Live — this toggle, not the keys themselves, determines which mode requests run in. A common
integration mistake is having live keys pasted into a plugin or script while the dashboard toggle
is still set to Test, or the reverse, so if a developer reports payments not behaving as expected,
checking that the toggle and the key prefix actually match is a good first troubleshooting step.

### Base URL

The base URL for all Flowwithlit API requests is `https://api.flowwithlit.com`.

### POST /v1/charge — Initiate a Payment

This endpoint initiates a payment. The request body takes the shape:

```
POST /v1/charge
Body: {"amount": 5000, "currency": "NGN", "email": "user@email.com", "reference": "YOUR_REF", "callback_url": "https://yoursite.com/verify"}
Response: {"status": true, "body": {"payment_url": "https://pay.flowwithlit.com/...", "reference": "YOUR_REF"}}
```

The response includes a `payment_url` that the customer should be redirected to in order to
complete the payment, along with the `reference` the integration supplied, which is used later to
verify the payment's outcome server-side.

### POST /v1/verify/:reference — Verify a Payment

This endpoint checks the outcome of a payment by its reference. It should ALWAYS be called
server-side after the customer is redirected back from payment — never trust a client-side success
callback alone, since a client-side callback firing does not by itself prove the payment succeeded.

```
POST /v1/verify/:reference
Response: {"status": true, "body": {"status": "success", "amount": 5000, "currency": "NGN"}}
```

An order or subscription should only be fulfilled after this server-side verification call
confirms a `success` status — not before.

### GET /v1/transactions — List Transactions

This endpoint lists all transactions on the account and supports pagination via
`?page=1&limit=20` query parameters, letting an integration page through transaction history
rather than fetching everything in one call.

### POST /v1/refund — Issue a Refund

This endpoint issues a refund for a previous transaction:

```
POST /v1/refund
Body: {"reference": "YOUR_REF", "amount": 5000}
```

The `reference` identifies which original transaction is being refunded, and `amount` specifies
how much of it to refund.

### Bank Account Verification Endpoint

For a lower-level building block used ahead of payouts, Flowwithlit exposes a direct bank-account
verification endpoint backed by a live Flutterwave integration for Nigerian bank accounts:

```
POST /component/bank_lookup.php
Body: {"bank_code": "044", "account_number": "0123456789"}
Response: {"status": true, "data": {"account_name": "JOHN DOE"}}
```

Given a bank code and account number, this returns the verified account holder's name, which
should be shown to the sender for confirmation before a transfer is executed. This is the same
lookup referenced in the "Transfers & FlowTag" section as the recommended step before sending
money to a new recipient.

### Inline JS Payment Button

For dropping a payment button directly into a web page without a full server-side integration,
Flowwithlit provides an inline JS widget:

```html
<script src="https://js.flowwithlit.com/v1/inline.js"></script>
<button onclick="FlowPay.open({
  key: 'pk_test_YOUR_KEY',
  amount: 5000,
  currency: 'NGN',
  email: 'customer@email.com',
  ref: 'ORDER_' + Date.now(),
  onSuccess: function(reference) {
    // IMPORTANT: verify server-side, never trust this callback alone
    fetch('/verify-payment', {method:'POST', body: JSON.stringify({ref: reference})});
  },
  onClose: function() { console.log('Payment closed'); }
})">Pay ₦50</button>
```

Note the same rule that applies to `/v1/verify/:reference` server-side verification: the
`onSuccess` callback firing in the browser is not proof of payment by itself — the reference must
still be verified server-side before an order is fulfilled.

### Official SDKs & Client Libraries

Flowwithlit lists official client libraries for Node.js (`npm install flowwithlit`), Python (`pip
install flowwithlit`), PHP (`composer require flowwithlit`), and Go (`go get -u flowwithlit`), with
more languages described as coming progressively. All SDKs are described as strongly typed and
expose an identical API shape between Test and Live modes, so switching from sandbox to production
shouldn't require changing integration code — only the keys and the mode toggle change.

### Sandbox & Testing

Flowwithlit provides a Sandbox environment to test integrations safely before going live,
including support for simulating errors and edge cases so an integration's failure-handling code
can be tested deliberately rather than only in production. Combined with the identical Test/Live
API shape described above, this means a developer can build and fully exercise an integration in
Sandbox and then flip to Live mode with minimal code changes.

### Getting Your First API Keys

Getting API keys for the first time is a short process: sign up for a Flowwithlit account,
complete basic verification, and then visit Dashboard → Developer API to copy the test and live
keys. The company describes this whole process as taking under two minutes.

## Webhook Setup & Verification

### How Webhook Delivery Works

Flowwithlit delivers real-time event notifications via signed HTTP POST requests to a URL the
developer configures, rather than requiring the integration to poll the API for status changes.
Delivery is sub-second, failed deliveries are retried automatically with exponential backoff (up
to 5 attempts), and every payload is signed and timestamped using HMAC-SHA256 so the receiving
endpoint can verify it genuinely came from Flowwithlit.

### Configuring a Webhook Endpoint

To configure a webhook, go to Dashboard → Developer API → Webhook URL and paste the endpoint that
should receive events, then copy the Webhook Secret shown on that same page. That secret is what's
used to compute and verify the HMAC signature on incoming payloads — it should be stored securely
on the server side and never exposed in client-side code.

### Events Sent

Once configured, Flowwithlit's servers POST to the configured URL whenever one of these events
occurs: `payment.success`, `payment.failed`, `refund.success`, `transfer.success`. An integration
should handle each event type it cares about and return HTTP 200 promptly to acknowledge receipt.

### Troubleshooting: Webhook Not Firing

If a webhook "isn't firing," the cause is almost always one of three things, and they should be
checked in this order: the receiving URL not returning HTTP 200, an invalid or expired SSL
certificate on that URL, or a firewall blocking Flowwithlit's outbound IP. Check all three before
assuming it's a bug on Flowwithlit's side.

### Signature Verification — General Shape

Signature verification looks slightly different per language, but the shape is the same
everywhere: read the raw request body, compute an HMAC-SHA256 digest of it using the webhook
secret, and compare that digest against the signature header using a constant-time comparison —
never a plain `==`/`!=` string compare, which can leak timing information to an attacker. The
signature always arrives in the `X-Flow-Signature` HTTP header.

### Verifying Signatures — PHP

```php
$payload = file_get_contents('php://input');
$sig = $_SERVER['HTTP_X_FLOW_SIGNATURE'] ?? '';
$expected = hash_hmac('sha256', $payload, 'YOUR_WEBHOOK_SECRET');
if (!hash_equals($expected, $sig)) { http_response_code(401); exit; }
$event = json_decode($payload, true);
if ($event['event'] === 'payment.success') {
    $ref = $event['data']['reference'];
    // fulfill order using $ref
}
http_response_code(200);
```

This PHP example reads the raw POST body, computes the expected HMAC-SHA256 signature using the
webhook secret, and uses `hash_equals` for a timing-safe comparison against the signature sent in
the `X-Flow-Signature` header before acting on the event.

### Verifying Signatures — Node.js

```javascript
const crypto = require('crypto');
app.post('/webhook', express.raw({type: 'application/json'}), (req, res) => {
  const sig = req.headers['x-flow-signature'];
  const expected = crypto.createHmac('sha256', process.env.WEBHOOK_SECRET)
                         .update(req.body).digest('hex');
  if (sig !== expected) return res.status(401).send('Unauthorized');
  const event = JSON.parse(req.body);
  if (event.event === 'payment.success') { /* fulfill */ }
  res.sendStatus(200);
});
```

This Node.js/Express example uses `express.raw()` specifically so the middleware does not parse
the body into JSON before the raw bytes are available for signature computation — parsing the body
first is a common cause of signature mismatches, described further below.

### Verifying Signatures — Python

```python
import hmac, hashlib
from flask import request, abort
@app.route('/webhook', methods=['POST'])
def webhook():
    sig = request.headers.get('X-Flow-Signature', '')
    expected = hmac.new(SECRET.encode(), request.data, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(sig, expected): abort(401)
    event = request.json
    if event['event'] == 'payment.success': pass  # fulfill
    return '', 200
```

This Flask example uses `request.data` (the raw bytes) for the HMAC computation and
`hmac.compare_digest` for a timing-safe comparison, mirroring the same pattern as the PHP and
Node.js examples above.

### Common Signature-Verification Pitfall

If a developer reports signature verification failing even though everything looks correct, the
most common cause is that a framework middleware has already parsed or mutated the request body —
for example turning it into a JSON object — before the raw bytes were captured for the HMAC. The
signature must be computed against the exact raw bytes of the body, not a re-serialized version of
it, which is why the Node.js example above deliberately uses `express.raw()` instead of the
default JSON body parser.

## Plugins & Integrations

### Installation Overview (Applies to All Six Platforms)

Flowwithlit offers zero-code, native integrations for six e-commerce platforms, aimed at merchants
who want to accept payments without writing custom integration code. Installation across all six
follows the same three-step process: (1) create a Flowwithlit account and complete basic
verification, which takes under two minutes and produces API keys; (2) install the plugin —
either from the platform's own app marketplace or by uploading a plugin ZIP file directly to the
store's admin panel; (3) paste the API keys into the plugin's settings and connect, at which point
the store can start accepting payments. Flowwithlit states it offers free implementation support
for all merchants who need help getting integrated.

### Shopify Integration

Install the Flowwithlit app from Shopify Admin → Settings → Payments → Alternative Payment
Methods, searching "Flowwithlit" and selecting Connect. Enter the API credentials from Dashboard →
Developer API, enable Test Mode first, place a test order to confirm the webhook fires, then
switch to Live. The Shopify integration is described as providing instant settlement, native cart
integration, and multi-currency support.

### WooCommerce Plugin

Download the WooCommerce plugin ZIP from Dashboard → Developer API → Downloads (or directly from
the Flowwithlit site). In WordPress Admin, go to Plugins → Add New → Upload Plugin, choose the
ZIP, then Install and Activate. Next, go to WooCommerce → Settings → Payments → Flowwithlit →
Manage, and paste the Public Key and Secret Key from the Dashboard. Set the Webhook URL to exactly
`https://yourstore.com/?wc-api=flowwithlit_wc` (no trailing slash), save, and test with a small
purchase (for example ₦100) in test mode. The plugin is described as offering 1-click
installation, automated webhooks, and seamless order sync.

### WooCommerce Troubleshooting

Common WooCommerce issues and their fixes: an "Invalid signature" error usually means the webhook
URL has a trailing slash, or wasn't copied exactly from the WooCommerce settings page. Orders
stuck on "Pending Payment" usually mean the webhook isn't reaching WordPress at all — check
WooCommerce → Status → Logs to confirm. If test mode doesn't seem to work, confirm `sk_test_` keys
were pasted rather than `sk_live_` keys, since pasting the wrong mode's keys is the most common
cause.

### Magento Extension

A fully certified extension brings Flowwithlit checkout to Magento / Adobe Commerce stores. It
supports multi-store setups, real-time exchange rates, and order status sync back to the store
admin.

### BigCommerce App

The BigCommerce app is installed from the BigCommerce marketplace, where it is listed as
marketplace verified. It exposes a headless-ready API and automatic tax sync, and is described as
able to go live without touching a line of code.

### Wix Integration

The Wix integration is added from the Wix App Market and offers drag-and-drop setup, a
mobile-optimized checkout, and built-in fraud protection.

### PrestaShop Module

The PrestaShop module is described as an official, lightweight module that enables Flowwithlit as
a payment method with one-click configuration, refund management, and multi-language support.

### Main Connections (Non-Plugin, Production-Ready Integrations)

Beyond the six e-commerce plugins, Flowwithlit describes a smaller set of "Main Connections" as
the integrations that are production-ready today, ahead of full reference documentation being
published for everything else. These are: bank account verification and transfers (real
name-enquiry lookups backed by Flutterwave for Nigerian bank accounts, described as fully live and
working with real bank codes), API key and webhook management (described as full key management,
live), and funding connections — NGN funding via OnePipe and crypto funding via Circle, with real
virtual accounts and deposits rather than simulated ones.

## Use Cases

### E-commerce & Retail

Merchants accept local and international payments at checkout through a single integration,
supporting cards, bank transfers, and crypto with automatic settlement. This covers Shopify,
WooCommerce, and custom-built storefronts, with multi-currency and crypto checkout plus instant
payouts to local banks.

### SaaS & Subscription Businesses

Flowwithlit supports billing customers globally with recurring payments, smart dunning (automated
retry logic for failed recurring charges), and flexible plan structures — including upgrades,
downgrades, and trial periods. This covers recurring and metered/usage-based billing, global
payment method support, and automatic tax and compliance handling.

### Marketplaces & Platforms

For two-sided marketplaces, Flowwithlit supports splitting a single payment across multiple
recipients, paying out vendors instantly, and managing funds in escrow. This is specifically
called out as built for two-sided marketplaces across Africa and beyond, and covers vendor
onboarding with KYC, split payouts and escrow, and real-time reporting.

### Fintech & Crypto Companies

Other fintech, wallet, exchange, or remittance products can be built on top of Flowwithlit's own
APIs — using it as underlying infrastructure rather than a customer-facing brand. This covers
fiat on/off ramps, virtual accounts and cards, and converting between fiat and crypto at what
Flowwithlit describes as low fees and high speed, all through developer-first APIs.

## About/Company Info

### Mission

Flowwithlit describes its mission as "tearing down the borders of global finance" — building
financial infrastructure meant to bring seamless payment processing, borderless transfers, and
crypto integration to everyone, everywhere.

### The Problem We Set Out to Solve

The company's own account of the problem it set out to solve: for a long time, cross-border
payments have been slow, expensive, and fragmented across incompatible systems, leaving businesses
and individuals stuck. Traditional banking systems were built for an earlier era, and at the same
time the emerging crypto landscape was seen as too complex for everyday commerce to actually use.
The company's stated founding idea is: "money should move as freely and instantly as information
does on the internet."

### Our Philosophy: Hyper-Secure

Flowwithlit lists "Hyper-Secure" as one of three core values guiding its product decisions.
Security is treated as foundational rather than an afterthought, backed by bank-grade encryption
and advanced fraud detection so that every transaction is meant to be "bulletproof" — the
company's own word for it.

### Our Philosophy: Lightning Speed

The second core value is "Lightning Speed." The infrastructure is built for instant processing and
real-time liquidity, on the premise that waiting days for a settlement is no longer acceptable for
users or merchants.

### Our Philosophy: Truly Borderless

The third core value is "Truly Borderless." The company states it doesn't believe in financial
borders, and that the platform experience should be identical whether a user is in New York or
Nairobi — the same product, the same features, everywhere it operates.

### Founders

Flowwithlit's two named founders are Famakinwa Emmanuel, Founder & CEO, and Olagundoye Joseph,
Founder & Lead Developer. Famakinwa Emmanuel is quoted describing the company's vision as building
"the ultimate foundation for the future of digital commerce," aiming to make global finance
"seamless, beautiful, and hyper-secure for everyone." Olagundoye Joseph is quoted describing the
engineering priority as building "the most robust and elegant API infrastructure," with the
explicit goal of eliminating friction in cross-border payments so that developers building on top
of Flowwithlit can focus on their own product instead of payments plumbing.

### FAQ: Where is Flowwithlit available?

Flowwithlit states it currently supports merchants and individuals in over 150 countries, with
infrastructure designed to bridge traditional banking systems across North America, Europe,
Africa, and Asia.

### FAQ: How do you ensure the security of funds?

Flowwithlit states it uses military-grade AES-256 encryption, rigorous PCI-DSS compliance, and
partnerships with top-tier financial institutions, with customer funds held in safeguarded
accounts. Full detail on certifications and technical security controls is in the "Security &
Compliance" section of this document.

### FAQ: Can I integrate Flowwithlit into my existing app?

Flowwithlit states it provides comprehensive, developer-friendly APIs and SDKs for all major
platforms, positioned so that payment infrastructure can be added to an existing application in
just a few lines of code. See "Developer API Reference" for the actual endpoints and SDK details.

### FAQ: Do you support cryptocurrency?

Flowwithlit states it seamlessly bridges fiat and crypto — a business can accept crypto payments
while settling in fiat, or convert balances directly within the dashboard. See "Crypto Converter
Engine" earlier in this document for how that conversion works in practice.
