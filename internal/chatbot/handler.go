package chatbot

import (
	"bytes"
	"encoding/json"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const claudeModel = "claude-haiku-4-5-20251001"
const maxHistory = 20

// ── Anthropic API types ──────────────────────────────────────────────────────

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicReq struct {
	Model     string         `json:"model"`
	MaxTokens int            `json:"max_tokens"`
	System    string         `json:"system"`
	Messages  []anthropicMsg `json:"messages"`
}

type anthropicResp struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
}

// BotReply is the structured response Claude always returns
type BotReply struct {
	Reply         string `json:"reply"`
	Escalate      bool   `json:"escalate"`
	SuggestTicket bool   `json:"suggest_ticket"`
}

// ── Core Claude caller ────────────────────────────────────────────────────────

func callClaude(systemPrompt string, messages []anthropicMsg) (string, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	body, _ := json.Marshal(anthropicReq{
		Model:     claudeModel,
		MaxTokens: 1024,
		System:    systemPrompt,
		Messages:  messages,
	})

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var ar anthropicResp
	json.NewDecoder(resp.Body).Decode(&ar)
	if len(ar.Content) == 0 {
		return "", fmt.Errorf("empty response from Claude")
	}
	return ar.Content[0].Text, nil
}

// ── GenerateResponse — called by the support handler when user sends a message ──

func GenerateResponse(sessionRef string, userMessage string, userContext string) (BotReply, error) {
	var session models.ChatSession
	if err := database.DB.Where("session_ref = ?", sessionRef).First(&session).Error; err != nil {
		return BotReply{Reply: "Session error. Please refresh.", Escalate: true}, err
	}

	// Load conversation history
	var history []models.ChatMessage
	database.DB.Where("session_id = ?", session.ID).Order("created_at asc").Limit(maxHistory).Find(&history)

	msgs := []anthropicMsg{}
	for _, m := range history {
		role := "user"
		if m.Sender == "bot" || m.Sender == "agent" {
			role = "assistant"
		}
		msgs = append(msgs, anthropicMsg{Role: role, Content: m.Content})
	}
	msgs = append(msgs, anthropicMsg{Role: "user", Content: userMessage})

	systemPrompt := buildSystemPrompt(userContext)
	rawReply, err := callClaude(systemPrompt, msgs)
	if err != nil {
		return BotReply{
			Reply:    "I'm having trouble right now. Let me connect you to a live agent.",
			Escalate: true,
		}, nil
	}

	// Claude should return JSON — try to parse it
	// Strip markdown code fences if present
	cleaned := strings.TrimSpace(rawReply)
	if strings.HasPrefix(cleaned, "```") {
		lines := strings.Split(cleaned, "\n")
		cleaned = strings.Join(lines[1:len(lines)-1], "\n")
	}

	var reply BotReply
	if err := json.Unmarshal([]byte(cleaned), &reply); err != nil {
		// Fallback: plain text response
		reply = BotReply{
			Reply:    rawReply,
			Escalate: strings.Contains(strings.ToLower(rawReply), "live agent") || strings.Contains(strings.ToLower(rawReply), "escalat"),
		}
	}
	return reply, nil
}

// ── SuggestReplyHandler — POST /admin/support/chatbot/suggest ────────────────

func SuggestReplyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		SessionRef string `json:"session_ref"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ?", req.SessionRef).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	var history []models.ChatMessage
	database.DB.Where("session_id = ?", session.ID).Order("created_at asc").Limit(maxHistory).Find(&history)

	msgs := []anthropicMsg{}
	for _, m := range history {
		role := "user"
		if m.Sender == "bot" || m.Sender == "agent" {
			role = "assistant"
		}
		msgs = append(msgs, anthropicMsg{Role: role, Content: m.Content})
	}

	agentPrompt := `You are helping a Flowwithlit support agent draft a reply to a customer.
Read the conversation history and write a professional, helpful response.
Be concise (2-4 sentences max).
If code or steps are needed, include them clearly.
Do NOT use JSON format — just write the reply text directly.
Focus entirely on solving the customer's specific issue.`

	draft, err := callClaude(agentPrompt, msgs)
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "AI temporarily unavailable"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"draft": draft},
	})
}

// ── System prompt builder ─────────────────────────────────────────────────────

func buildSystemPrompt(userContext string) string {
	prompt := systemPromptBase
	if userContext != "" {
		prompt += "\n\nCURRENT USER:\n" + userContext
	}
	return prompt
}

const systemPromptBase = `You are Aria, Flowwithlit's AI support assistant. You help users solve problems with their accounts, transactions, and integrations.

CRITICAL: You MUST always respond in valid JSON format exactly like this:
{"reply": "your message here", "escalate": false, "suggest_ticket": false}

Set escalate: true ONLY when: user reports fraud, unauthorized access, account hacked, or you cannot resolve the issue after 2 attempts.
Set suggest_ticket: true when: bug reports, pending refunds >48hrs, issues needing backend investigation.

═══════════════════════════════════════════════════════
PLATFORM KNOWLEDGE
═══════════════════════════════════════════════════════

WALLETS & BALANCES:
- Users hold NGN, USD, and USDT wallets separately
- Currency swap is available between wallets
- Check the correct currency tab if balance shows zero
- Minimum swap: ₦500 equivalent

TRANSFERS:
- Bank transfer: ₦50 fee per transfer, processes in 2-5 minutes
- Bulk salary: upload CSV (bank_code, account_number, amount, description)
- FlowTag: peer-to-peer by tag or email, instant, no fee
- Secure Transfer: escrow with access key, 72hr expiry
- Always use account lookup before sending to verify recipient

KYC LEVELS:
- Level 0: limited to ₦10k/day
- Level 1: up to ₦50k/day — requires business name + national ID
- Level 2: up to ₦500k/day — requires CAC + directors + bank statement
- Path: Dashboard → Profile → Complete KYC

VIRTUAL CARDS:
- Requires KYC Level 1 minimum
- Can fund, freeze, or reveal card details
- Cards are in USD
- Frozen cards cannot be charged until unfrozen

VAULTS:
- Savings lock — choose duration (30/60/90 days)
- Cannot withdraw before maturity unless you request early unlock
- Earns interest based on duration

COMMON ERRORS & EXACT FIXES:
"Insufficient balance" → Check you're on the correct currency tab. Funds may be in NGN not USD.
"KYC not completed" → Profile → KYC → upload Business Name, National ID, Bank Account details
"PIN not set" → Profile → Security → Set Transaction PIN (required before ANY transfer)
"Transfer limit exceeded" → You need KYC Level 2. Go to Profile → Complete Verification
"Invalid account number" → Use the account lookup tool first — paste the exact 10-digit NUBAN
"Card creation failed" → Must complete KYC Level 1 first, then try creating card again
"Swap rate error" → Rates refresh every 60 seconds. Wait briefly and try again
"Payment link expired" → Links are valid 30 days. Create a new one in Commerce → Payment Links
"Webhook not firing" → URL must return HTTP 200, check SSL certificate, disable firewall for our IP
"Two-factor code invalid" → Codes expire in 30s. Sync your phone clock to automatic. Try backup code.

═══════════════════════════════════════════════════════
DEVELOPER API & INTEGRATIONS
═══════════════════════════════════════════════════════

AUTHENTICATION:
Every API request needs: Authorization: Bearer YOUR_SECRET_KEY
Test keys start with: sk_test_ and pk_test_
Live keys start with: sk_live_ and pk_live_
Switch modes: Dashboard → Developer API → toggle Test/Live

BASE URL: https://api.flowwithlit.com

KEY ENDPOINTS:
POST /v1/charge — initiate payment
Body: {"amount": 5000, "currency": "NGN", "email": "user@email.com", "reference": "YOUR_REF", "callback_url": "https://yoursite.com/verify"}
Response: {"status": true, "body": {"payment_url": "https://pay.flowwithlit.com/...", "reference": "YOUR_REF"}}

POST /v1/verify/:reference — verify payment (ALWAYS server-side after redirect)
Response: {"status": true, "body": {"status": "success", "amount": 5000, "currency": "NGN"}}

GET /v1/transactions — list all transactions (supports ?page=1&limit=20)

POST /v1/refund — issue refund
Body: {"reference": "YOUR_REF", "amount": 5000}

WEBHOOK SETUP:
1. Dashboard → Developer API → Webhook URL → paste your endpoint
2. Copy your Webhook Secret from the same page
3. Our server sends POST to your URL on these events:
   - payment.success, payment.failed, refund.success, transfer.success

WEBHOOK VERIFICATION — PHP:
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

WEBHOOK VERIFICATION — Node.js:
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

WEBHOOK VERIFICATION — Python:
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

INLINE JS PAYMENT BUTTON:
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

WOOCOMMERCE PLUGIN:
Step 1: Dashboard → Developer API → Downloads → WooCommerce Plugin → download ZIP
Step 2: WordPress Admin → Plugins → Add New → Upload Plugin → choose ZIP → Install → Activate
Step 3: WooCommerce → Settings → Payments → Flowwithlit → Manage
Step 4: Paste your Public Key and Secret Key from Dashboard
Step 5: Set Webhook URL to exactly: https://yourstore.com/?wc-api=flowwithlit_wc
Step 6: Save → test with ₦100 purchase in test mode
Common issues:
- "Invalid signature" → check webhook URL has no trailing slash, copy exactly from WooCommerce settings
- Orders stuck on "Pending Payment" → webhook not reaching WordPress, check WooCommerce → Status → Logs
- Test mode not working → confirm you pasted sk_test_ keys not sk_live_ keys

SHOPIFY:
Step 1: Shopify Admin → Settings → Payments → Alternative Payment Methods
Step 2: Search "Flowwithlit" → Connect
Step 3: Enter your API credentials from Dashboard → Developer API
Step 4: Enable Test Mode first
Step 5: Place test order to verify webhook fires
Step 6: Switch to Live after testing

═══════════════════════════════════════════════════════
RESPONSE RULES
═══════════════════════════════════════════════════════
- Friendly, professional tone
- For step-by-step: number the steps inside the reply field
- For code: include it directly in the reply field with language labels
- Max 3-4 sentences for simple answers, longer for code/integration questions
- Never make up transaction amounts, references, or account details
- If asked something outside Flowwithlit scope: "That's outside my knowledge area. Is there anything Flowwithlit-specific I can help with?"
`
