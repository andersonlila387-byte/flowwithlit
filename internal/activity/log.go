package activity

import (
	"encoding/json"
	"log"
	"strings"
	"time"

	"flowwithlit/internal/database"
	"flowwithlit/internal/models"
)

// Log writes a monitoring event. Never panics — safe to call from any handler.
func Log(area, level, event, message string, userID *uint, reference, ip string, meta map[string]interface{}) {
	area = strings.TrimSpace(strings.ToLower(area))
	level = strings.TrimSpace(strings.ToLower(level))
	if level == "" {
		level = "info"
	}
	if area == "" {
		area = "system"
	}
	metaStr := ""
	if meta != nil {
		if b, err := json.Marshal(meta); err == nil {
			metaStr = string(b)
		}
	}
	entry := models.ActivityLog{
		Area:      area,
		Level:     level,
		Event:     strings.TrimSpace(event),
		Message:   strings.TrimSpace(message),
		UserID:    userID,
		Reference: strings.TrimSpace(reference),
		Meta:      metaStr,
		IPAddress: strings.TrimSpace(ip),
		CreatedAt: time.Now(),
	}
	if err := database.DB.Create(&entry).Error; err != nil {
		log.Printf("[Activity] failed to write log area=%s event=%s: %v", area, event, err)
	}
}

// Info / Warning / Error / Success helpers
func Info(area, event, message string, userID *uint, reference, ip string) {
	Log(area, "info", event, message, userID, reference, ip, nil)
}
func Warning(area, event, message string, userID *uint, reference, ip string) {
	Log(area, "warning", event, message, userID, reference, ip, nil)
}
func Error(area, event, message string, userID *uint, reference, ip string) {
	Log(area, "error", event, message, userID, reference, ip, nil)
}
func Success(area, event, message string, userID *uint, reference, ip string) {
	Log(area, "success", event, message, userID, reference, ip, nil)
}

// UID helper
func UID(id uint) *uint { return &id }
