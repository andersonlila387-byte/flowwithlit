package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowwithlit/internal/activity"
	"flowwithlit/internal/database"
	"flowwithlit/internal/integration/smileid"
	"flowwithlit/internal/models"
	"flowwithlit/internal/settings"
)

func logWebhook(provider, eventType, payload, status, errMsg, ip string, processingMs int64) {
	entry := models.WebhookLog{
		Provider:     provider,
		EventType:    eventType,
		Payload:      payload,
		StatusCode:   200,
		Status:       status,
		ErrorMessage: errMsg,
		ProcessingMs: processingMs,
		IPAddress:    ip,
	}
	database.DB.Create(&entry)
}

func readRawBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
}

// verifyHMACHex compares a hex-encoded HMAC of body using secret.
// Tries both SHA-256 and SHA-512 (providers vary).
func verifyHMACHex(secret string, body []byte, signature string) bool {
	sig := strings.TrimSpace(signature)
	sig = strings.TrimPrefix(sig, "sha256=")
	sig = strings.TrimPrefix(sig, "sha512=")
	if secret == "" || sig == "" {
		return false
	}

	mac256 := hmac.New(sha256.New, []byte(secret))
	mac256.Write(body)
	exp256 := hex.EncodeToString(mac256.Sum(nil))
	if hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(exp256))) {
		return true
	}

	mac512 := hmac.New(sha512.New, []byte(secret))
	mac512.Write(body)
	exp512 := hex.EncodeToString(mac512.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(sig)), []byte(strings.ToLower(exp512)))
}

// softVerify returns (ok, enforced).
// When secret is empty: ok=true, enforced=false (live keeps working; log once per call path).
// When secret is set: require a valid signature; reject otherwise.
func softVerify(provider, secret string, body []byte, candidates ...string) (ok bool, enforced bool) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		log.Printf("[Webhook] %s: webhook secret not configured — accepting payload without signature check (set secret in Admin → Settings to enforce)", provider)
		return true, false
	}
	enforced = true
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		// Direct shared-secret header match (e.g. Flutterwave verif-hash)
		if hmac.Equal([]byte(c), []byte(secret)) {
			return true, true
		}
		// HMAC of raw body
		if verifyHMACHex(secret, body, c) {
			return true, true
		}
	}
	return false, true
}

// OnePipeWebhookPayload represents the expected JSON payload from OnePipe
type OnePipeWebhookPayload struct {
	Event         string  `json:"event"`
	TransactionID string  `json:"transaction_id"`
	AccountNumber string  `json:"account_number"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
}

// OnePipeHandler receives transaction alerts from OnePipe.
// Signature enforcement is soft: only when onepipe_webhook_secret (or fallback onepipe_secret) is set.
func OnePipeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from OnePipe")

	raw, err := readRawBody(r)
	if err != nil {
		logWebhook("onepipe", "unknown", "{}", "failed", "Failed to read body", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Only the dedicated webhook secret enforces signatures. API key/secret alone
	// never blocks deposits (live stays open until you explicitly set webhook secret).
	secret := settings.Get("onepipe_webhook_secret")
	sigCandidates := []string{
		r.Header.Get("X-OnePipe-Signature"),
		r.Header.Get("X-Signature"),
		r.Header.Get("X-Hub-Signature-256"),
		r.Header.Get("Signature"),
	}
	if ok, enforced := softVerify("onepipe", secret, raw, sigCandidates...); !ok {
		logWebhook("onepipe", "unknown", string(raw), "failed", "Invalid or missing signature", ip, time.Since(start).Milliseconds())
		activity.Error("webhook", "onepipe_sig_failed", "Invalid or missing OnePipe signature", nil, "", ip)
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	} else if enforced {
		log.Println("[Webhook] OnePipe signature verified")
	}

	var payload OnePipeWebhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		logWebhook("onepipe", "unknown", string(raw), "failed", "Invalid JSON payload", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Credit path remains deferred: find user by ProviderAccountReference, credit wallet in ACID txn.
	if payload.Event == "transaction.successful" {
		log.Printf("[Webhook] OnePipe deposit successful: %s %.2f into account %s\n", payload.Currency, payload.Amount, payload.AccountNumber)
		logWebhook("onepipe", payload.Event, string(raw), "processed", "", ip, time.Since(start).Milliseconds())
		activity.Success("webhook", "onepipe_deposit", "OnePipe deposit alert "+payload.Currency, nil, payload.TransactionID, ip)
	} else {
		event := payload.Event
		if event == "" {
			event = "received"
		}
		logWebhook("onepipe", event, string(raw), "received", "", ip, time.Since(start).Milliseconds())
		activity.Info("webhook", "onepipe_"+event, "OnePipe event received", nil, payload.TransactionID, ip)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// FlutterwaveHandler receives transaction alerts from Flutterwave.
// Enforces verif-hash only when flutterwave_webhook_hash is configured in admin.
func FlutterwaveHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from Flutterwave")

	raw, err := readRawBody(r)
	if err != nil {
		logWebhook("flutterwave", "unknown", "{}", "failed", "Failed to read body", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	secret := settings.Get("flutterwave_webhook_hash")
	verifHash := r.Header.Get("verif-hash")
	if verifHash == "" {
		verifHash = r.Header.Get("Verif-Hash")
	}
	if ok, enforced := softVerify("flutterwave", secret, raw, verifHash); !ok {
		logWebhook("flutterwave", "unknown", string(raw), "failed", "Invalid or missing verif-hash", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	} else if enforced {
		log.Println("[Webhook] Flutterwave verif-hash verified")
	}

	var body map[string]interface{}
	_ = json.Unmarshal(raw, &body)

	eventType := "unknown"
	if ev, ok := body["event"].(string); ok {
		eventType = ev
	}

	logWebhook("flutterwave", eventType, string(raw), "received", "", ip, time.Since(start).Milliseconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// CircleHandler receives alerts from Circle for crypto deposits.
// Soft signature check when circle_webhook_secret is set.
func CircleHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from Circle")

	raw, err := readRawBody(r)
	if err != nil {
		logWebhook("circle", "unknown", "{}", "failed", "Failed to read body", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	secret := settings.Get("circle_webhook_secret")
	sigCandidates := []string{
		r.Header.Get("X-Circle-Signature"),
		r.Header.Get("Circle-Signature"),
		r.Header.Get("X-Signature"),
	}
	if ok, enforced := softVerify("circle", secret, raw, sigCandidates...); !ok {
		logWebhook("circle", "unknown", string(raw), "failed", "Invalid or missing signature", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	} else if enforced {
		log.Println("[Webhook] Circle signature verified")
	}

	var body map[string]interface{}
	_ = json.Unmarshal(raw, &body)

	eventType := "unknown"
	if ev, ok := body["type"].(string); ok {
		eventType = ev
	}

	logWebhook("circle", eventType, string(raw), "received", "", ip, time.Since(start).Milliseconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// PalmPayHandler is prepared for the future NGN rail switch.
// Safe to deploy now: accepts/logs only when called; enforces signature when secret is set.
// OnePipe remains the live NGN default until Admin → ngn_bank_provider = palmpay.
func PalmPayHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from PalmPay")

	raw, err := readRawBody(r)
	if err != nil {
		logWebhook("palmpay", "unknown", "{}", "failed", "Failed to read body", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Dedicated webhook secret only — API keys never auto-enforce signatures.
	secret := settings.Get("palmpay_webhook_secret")
	sigCandidates := []string{
		r.Header.Get("X-PalmPay-Signature"),
		r.Header.Get("X-Signature"),
		r.Header.Get("Signature"),
		r.Header.Get("Authorization"),
	}
	if ok, enforced := softVerify("palmpay", secret, raw, sigCandidates...); !ok {
		logWebhook("palmpay", "unknown", string(raw), "failed", "Invalid or missing signature", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	} else if enforced {
		log.Println("[Webhook] PalmPay signature verified")
	}

	var body map[string]interface{}
	_ = json.Unmarshal(raw, &body)

	eventType := "received"
	for _, key := range []string{"event", "eventType", "type", "notifyType"} {
		if ev, ok := body[key].(string); ok && ev != "" {
			eventType = ev
			break
		}
	}

	logWebhook("palmpay", eventType, string(raw), "received", "PalmPay rail reserved — credit path not active until ngn_bank_provider=palmpay", ip, time.Since(start).Milliseconds())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// SmileIDHandler receives async KYC job results from Smile ID.
func SmileIDHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	raw, err := readRawBody(r)
	if err != nil {
		logWebhook("smileid", "unknown", "{}", "failed", "Failed to read body", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		logWebhook("smileid", "unknown", string(raw), "failed", "Invalid JSON", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	eventType := "kyc_result"
	if code, ok := body["ResultCode"].(string); ok {
		eventType = "kyc_" + code
	}

	// Signature verification is mandatory, not best-effort: this webhook can upgrade
	// a user's KYC tier, so skipping the check whenever a field is missing would let
	// anyone self-approve KYC with a forged payload and no signature at all.
	cfg := settings.SmileID()
	if !cfg.Configured() {
		logWebhook("smileid", eventType, string(raw), "failed", "Smile ID not configured", ip, time.Since(start).Milliseconds())
		http.Error(w, "Not configured", http.StatusServiceUnavailable)
		return
	}
	sig, _ := body["signature"].(string)
	ts, _ := body["timestamp"].(string)
	if sig == "" || ts == "" {
		logWebhook("smileid", eventType, string(raw), "failed", "Missing signature", ip, time.Since(start).Milliseconds())
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}
	expected := smileid.GenerateSignature(cfg.APIKey, cfg.PartnerID, ts)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		logWebhook("smileid", eventType, string(raw), "failed", "Invalid signature", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Upgrade KYC tier when Smile ID validates (0810 = ID validated)
	if code, _ := body["ResultCode"].(string); code == "0810" {
		if pp, ok := body["PartnerParams"].(map[string]interface{}); ok {
			if uid, ok := pp["user_id"].(string); ok && uid != "" {
				if id, err := strconv.ParseUint(uid, 10, 64); err == nil {
					database.DB.Model(&models.User{}).Where("id = ?", uint(id)).Update("kyc_level", 1)
				}
			}
		}
	}

	logWebhook("smileid", eventType, string(raw), "processed", "", ip, time.Since(start).Milliseconds())
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
