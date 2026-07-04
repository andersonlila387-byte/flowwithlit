package support

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"flowwithlit/internal/admin"
	"flowwithlit/internal/chatbot"
	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
	"flowwithlit/pkg/middleware"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func genSessionRef() string {
	b := make([]byte, 4)
	rand.Read(b)
	return "CHAT_" + strings.ToUpper(hex.EncodeToString(b))
}

// StartChatHandler — POST /user/support/chat/start
func StartChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.Context().Value(middleware.UserIDKey).(uint)

	var user models.User
	database.DB.Select("first_name, last_name, email, kyc_level").Where("id = ?", userID).First(&user)

	session := models.ChatSession{
		SessionRef: genSessionRef(),
		Status:     "bot",
		UserID:     &userID,
		GuestName:  user.FirstName + " " + user.LastName,
		GuestEmail: user.Email,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Failed to start chat"})
		return
	}

	welcomeName := user.FirstName
	if welcomeName == "" {
		welcomeName = "there"
	}

	welcome := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   fmt.Sprintf("Hi %s! I'm Aria, Flowwithlit's AI assistant. I can help with account issues, transfers, API integrations, plugins, and more. What can I help you with today?", welcomeName),
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

// SendMessageHandler — POST /user/support/chat/message
func SendMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.Context().Value(middleware.UserIDKey).(uint)

	var req struct {
		SessionRef string `json:"session_ref"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionRef == "" || req.Content == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "session_ref and content are required"})
		return
	}

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ? AND user_id = ?", req.SessionRef, userID).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	userMsg := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "user",
		Content:   req.Content,
	}
	database.DB.Create(&userMsg)

	resp := map[string]interface{}{
		"status": true,
		"body": map[string]interface{}{
			"message_id": userMsg.ID,
		},
	}

	// Auto-respond with AI when in bot mode
	if session.Status == "bot" {
		var user models.User
		database.DB.Select("first_name, last_name, email, kyc_level").Where("id = ?", userID).First(&user)
		userCtx := fmt.Sprintf("Name: %s %s | Email: %s | KYC Level: %d",
			user.FirstName, user.LastName, user.Email, user.KYCLevel)

		botResp, _ := chatbot.GenerateResponse(req.SessionRef, req.Content, userCtx)

		botMsg := models.ChatMessage{
			SessionID: session.ID,
			Sender:    "bot",
			Content:   botResp.Reply,
		}
		database.DB.Create(&botMsg)

		if botResp.Escalate {
			database.DB.Model(&session).Update("status", "queued")
			queueMsg := models.ChatMessage{
				SessionID: session.ID,
				Sender:    "bot",
				Content:   "I've added you to the queue for a live support agent. Someone will join shortly.",
			}
			database.DB.Create(&queueMsg)
		}

		body := resp["body"].(map[string]interface{})
		body["bot_reply"] = botResp.Reply
		body["escalate"] = botResp.Escalate
		body["suggest_ticket"] = botResp.SuggestTicket
		body["bot_message_id"] = botMsg.ID
	}

	json.NewEncoder(w).Encode(resp)
}

// GetUserMessagesHandler — GET /user/support/chat/messages/{ref}
func GetUserMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.Context().Value(middleware.UserIDKey).(uint)

	ref := chi.URLParam(r, "ref")
	afterStr := r.URL.Query().Get("after")
	afterID, _ := strconv.ParseUint(afterStr, 10, 64)

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ? AND user_id = ?", ref, userID).First(&session).Error; err != nil {
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

// EscalateHandler — POST /user/support/chat/escalate
func EscalateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	userID := r.Context().Value(middleware.UserIDKey).(uint)

	var req struct {
		SessionRef string `json:"session_ref"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var session models.ChatSession
	database.DB.Where("session_ref = ? AND user_id = ?", req.SessionRef, userID).First(&session)
	database.DB.Model(&session).Update("status", "queued")

	queueMsg := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   "You've been added to the live agent queue. A support agent will join shortly. Please keep this window open.",
	}
	database.DB.Create(&queueMsg)

	json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "Escalated to agent queue"})
}

// ── Agent handlers (admin auth) ───────────────────────────────────────────────

type sessionMeta struct {
	models.ChatSession
	MessageCount int    `json:"message_count"`
	LastMessage  string `json:"last_message"`
}

// GetSessionsHandler — GET /admin/support/sessions
func GetSessionsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	statusFilter := r.URL.Query().Get("status")
	var sessions []models.ChatSession
	q := database.DB.Order("updated_at desc")
	if statusFilter != "" {
		q = q.Where("status = ?", statusFilter)
	}
	q.Find(&sessions)

	result := []sessionMeta{}
	for _, s := range sessions {
		var count int64
		var lastMsg models.ChatMessage
		database.DB.Model(&models.ChatMessage{}).Where("session_id = ?", s.ID).Count(&count)
		database.DB.Where("session_id = ?", s.ID).Order("created_at desc").First(&lastMsg)
		preview := lastMsg.Content
		if len(preview) > 80 {
			preview = preview[:80] + "…"
		}
		result = append(result, sessionMeta{ChatSession: s, MessageCount: int(count), LastMessage: preview})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"sessions": result},
	})
}

// GetSessionMessagesHandler — GET /admin/support/messages/{ref}
func GetSessionMessagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ref := chi.URLParam(r, "ref")
	afterStr := r.URL.Query().Get("after")
	afterID, _ := strconv.ParseUint(afterStr, 10, 64)

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ?", ref).First(&session).Error; err != nil {
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
			"session":        session,
		},
	})
}

// ClaimSessionHandler — POST /admin/support/claim/{ref}
func ClaimSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	agentID := r.Context().Value(admin.AdminIDKey).(uint)

	ref := chi.URLParam(r, "ref")
	var session models.ChatSession
	if err := database.DB.Where("session_ref = ?", ref).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	database.DB.Model(&session).Updates(map[string]interface{}{"agent_id": agentID, "status": "active"})

	agentJoined := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   "A live support agent has joined the chat. How can we help you?",
	}
	database.DB.Create(&agentJoined)

	json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "Session claimed successfully"})
}

// CloseSessionHandler — POST /admin/support/close/{ref}
func CloseSessionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	ref := chi.URLParam(r, "ref")
	var session models.ChatSession
	database.DB.Where("session_ref = ?", ref).First(&session)
	database.DB.Model(&session).Update("status", "closed")

	closeMsg := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "bot",
		Content:   "This chat session has been closed. Thank you for contacting Flowwithlit support! If you need further help, start a new chat.",
	}
	database.DB.Create(&closeMsg)

	json.NewEncoder(w).Encode(map[string]interface{}{"status": true, "message": "Session closed"})
}

// AgentMessageHandler — POST /admin/support/agent-message
func AgentMessageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		SessionRef string `json:"session_ref"`
		Content    string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionRef == "" || req.Content == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "session_ref and content required"})
		return
	}

	var session models.ChatSession
	if err := database.DB.Where("session_ref = ?", req.SessionRef).First(&session).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"status": false, "message": "Session not found"})
		return
	}

	msg := models.ChatMessage{
		SessionID: session.ID,
		Sender:    "agent",
		Content:   req.Content,
	}
	database.DB.Create(&msg)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": true,
		"body":   map[string]interface{}{"message_id": msg.ID},
	})
}
