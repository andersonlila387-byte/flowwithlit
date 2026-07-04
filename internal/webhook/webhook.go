package webhook

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

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

// OnePipeWebhookPayload represents the expected JSON payload from OnePipe
type OnePipeWebhookPayload struct {
	Event         string  `json:"event"`
	TransactionID string  `json:"transaction_id"`
	AccountNumber string  `json:"account_number"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	Status        string  `json:"status"`
}

// OnePipeHandler receives transaction alerts from OnePipe
func OnePipeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from OnePipe")

	var payload OnePipeWebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logWebhook("onepipe", "unknown", "{}", "failed", "Invalid JSON payload", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	raw, _ := json.Marshal(payload)

	// TODO: Implement HMAC signature verification
	// TODO: Find user by ProviderAccountReference, credit wallet in ACID DB transaction

	if payload.Event == "transaction.successful" {
		log.Printf("[Webhook] OnePipe deposit successful: %s %.2f into account %s\n", payload.Currency, payload.Amount, payload.AccountNumber)
		logWebhook("onepipe", payload.Event, string(raw), "processed", "", ip, time.Since(start).Milliseconds())
	} else {
		logWebhook("onepipe", payload.Event, string(raw), "received", "", ip, time.Since(start).Milliseconds())
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

// FlutterwaveHandler receives transaction alerts from Flutterwave
func FlutterwaveHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from Flutterwave")

	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	raw, _ := json.Marshal(body)

	eventType := "unknown"
	if ev, ok := body["event"].(string); ok {
		eventType = ev
	}

	logWebhook("flutterwave", eventType, string(raw), "received", "", ip, time.Since(start).Milliseconds())

	w.WriteHeader(http.StatusOK)
}

// CircleHandler receives alerts from Circle for crypto deposits
func CircleHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	log.Println("[Webhook] Received incoming webhook from Circle")

	var body map[string]interface{}
	json.NewDecoder(r.Body).Decode(&body)
	raw, _ := json.Marshal(body)

	eventType := "unknown"
	if ev, ok := body["type"].(string); ok {
		eventType = ev
	}

	logWebhook("circle", eventType, string(raw), "received", "", ip, time.Since(start).Milliseconds())

	w.WriteHeader(http.StatusOK)
}

// SmileIDHandler receives async KYC job results from Smile ID.
func SmileIDHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ip := r.RemoteAddr

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logWebhook("smileid", "unknown", "{}", "failed", "Invalid JSON", ip, time.Since(start).Milliseconds())
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	raw, _ := json.Marshal(body)

	eventType := "kyc_result"
	if code, ok := body["ResultCode"].(string); ok {
		eventType = "kyc_" + code
	}

	cfg := settings.SmileID()
	if sig, _ := body["signature"].(string); sig != "" {
		ts, _ := body["timestamp"].(string)
		if ts != "" && cfg.Configured() {
			expected := smileid.GenerateSignature(cfg.APIKey, cfg.PartnerID, ts)
			if sig != expected {
				logWebhook("smileid", eventType, string(raw), "failed", "Invalid signature", ip, time.Since(start).Milliseconds())
				http.Error(w, "Invalid signature", http.StatusUnauthorized)
				return
			}
		}
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
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}
