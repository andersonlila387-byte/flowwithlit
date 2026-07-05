package support

import (
	"encoding/json"
	"flowwithlit/internal/chatbot"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Guest chat — same Aria AI engine as the logged-in support chat, but for anonymous
// visitors on the public marketing site (no JWT). Sessions are scoped with
// user_id IS NULL throughout so a guest session_ref can never be used to read or
// write a real user's authenticated support session, and vice versa.

const (
	maxGuestMessageLen  = 4000
	maxGuestMessagesCap = 40 // per session — protects the Anthropic bill from anonymous abuse
)

// StartGuestChatHandler — POST /public/support/chat/start
func StartGuestChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req) // optional body — a blank/missing body is fine

	guestName := strings.TrimSpace(req.Name)
	if guestName == "" {
		guestName = "Visitor"
	}

	session := models.ChatSession{
		SessionRef: genSessionRef(),
		Status:     "bot",
		GuestName:  guestName,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to start chat"})
		return
	}

	welcome := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   "Hi! I'm Aria, Flowwithlit's AI assistant. Ask me about pricing, plugins, integrations, or how Flowwithlit works — and I can connect you to a human if you need more help.",
	}
	database.DB.Create(&welcome)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body": map[string]interface{}{
			"session_ref": session.SessionRef,
			"status":      session.Status,
		},
	})
}

// SendGuestMessageHandler — POST /public/support/chat/message
func SendGuestMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		SessionRef string `json:"session_ref"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionRef == "" || strings.TrimSpace(req.Content) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "session_ref and content are required"})
		return
	}
	if len(req.Content) > maxGuestMessageLen {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Message is too long"})
		return
	}

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ? AND user_id IS NULL", req.SessionRef).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	var userMsgCount int64
	database.DB.Model(&models.ChatMessage{}).Where("session_id = ? AND sender = ?", session.ID, "user").Count(&userMsgCount)

	userMsg := models.ChatMessage{SessionID: session.ID, Sender: "user", Content: req.Content}
	database.DB.Create(&userMsg)

	resp := map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"message_id": userMsg.ID},
	}
	body := resp["body"].(map[string]interface{})

	if userMsgCount >= maxGuestMessagesCap {
		// Cost guard: stop calling Claude for very long anonymous sessions, push to a human channel instead.
		capMsg := models.ChatMessage{
			SessionID: session.ID,
			Sender:    "bot",
			Content:   "We've covered a lot here! For anything further, please reach out via our Contact page and a team member will follow up directly.",
		}
		database.DB.Create(&capMsg)
		body["bot_reply"] = capMsg.Content
		body["escalate"] = false
		body["bot_message_id"] = capMsg.ID
		json.NewEncoder(w).Encode(resp)
		return
	}

	if session.Status == "bot" {
		guestCtx := "Guest visitor on the public marketing website — not signed in, no account yet. " +
			"They're likely asking about pricing, features, plugins/integrations, or how Flowwithlit works before signing up. " +
			"Never claim to access account, balance, or transaction data — if they ask something account-specific, tell them to sign in or create an account first."

		botResp, _ := chatbot.GenerateResponse(req.SessionRef, req.Content, guestCtx)

		botMsg := models.ChatMessage{SessionID: session.ID, Sender: "bot", Content: botResp.Reply}
		database.DB.Create(&botMsg)

		if botResp.Escalate {
			database.DB.Model(&session).Update("status", "queued")
			database.DB.Create(&models.ChatMessage{
				SessionID: session.ID,
				Sender:    "bot",
				Content:   "I've added you to the queue for a live team member. Someone will join shortly — feel free to leave your email if you'd rather we follow up there.",
			})
		}

		body["bot_reply"] = botResp.Reply
		body["escalate"] = botResp.Escalate
		body["suggest_ticket"] = botResp.SuggestTicket
		body["bot_message_id"] = botMsg.ID
	}

	json.NewEncoder(w).Encode(resp)
}

// GetGuestMessagesHandler — GET /public/support/chat/messages/{ref}
func GetGuestMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ref := chi.URLParam(r, "ref")
	afterID, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ? AND user_id IS NULL", ref).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	var messages []models.ChatMessage
	q := database.DB.Where("session_id = ?", session.ID).Order("created_at asc")
	if afterID > 0 {
		q = q.Where("id > ?", afterID)
	}
	q.Find(&messages)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body": map[string]interface{}{
			"messages":       messages,
			"session_status": session.Status,
		},
	})
}

// EscalateGuestHandler — POST /public/support/chat/escalate
func EscalateGuestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		SessionRef string `json:"session_ref"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ? AND user_id IS NULL", req.SessionRef).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}
	database.DB.Model(&session).Update("status", "queued")

	database.DB.Create(&models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   "You've been added to the live team queue. A team member will join shortly. Please keep this window open.",
	})

	json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "Escalated to team queue"})
}
