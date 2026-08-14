package middleware

import (
	"net/http"
	"os"
	"strings"
)

// SecurityHeaders sets standard defensive HTTP response headers on every request.
// These harden the API against common browser-based attack vectors and enforce
// transport security in production.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent MIME-type sniffing (browsers must respect declared Content-Type).
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Deny framing of API responses entirely (no UI, so no clickjacking risk,
		// but also no reason to ever be framed).
		w.Header().Set("X-Frame-Options", "DENY")

		// Limit referrer information sent to third-party services.
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Disable access to sensitive browser features from API responses.
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Strict-Transport-Security: enforced in production only so that local
		// HTTP dev still works. 1 year max-age + includeSubDomains.
		if strings.EqualFold(strings.TrimSpace(os.Getenv("ENVIRONMENT")), "production") {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// ContentTypeJSON rejects any POST, PUT, or PATCH request whose Content-Type
// is not application/json. Webhook routes are excluded — they may receive
// application/x-www-form-urlencoded or application/octet-stream payloads.
func ContentTypeJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
			// Allow multipart for file uploads and no content-type for empty bodies.
			if r.ContentLength != 0 &&
				!strings.HasPrefix(ct, "application/json") &&
				!strings.HasPrefix(ct, "multipart/form-data") &&
				!strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
				http.Error(w, `{"status":"error","message":"Content-Type must be application/json"}`, http.StatusUnsupportedMediaType)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
